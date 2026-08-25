package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"runtime/debug"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/sparkfabrik/claude-usage/internal/analyzer"
	"github.com/sparkfabrik/claude-usage/internal/auth"
	"github.com/sparkfabrik/claude-usage/internal/cache"
	"github.com/sparkfabrik/claude-usage/internal/config"
	"github.com/sparkfabrik/claude-usage/internal/dashboard"
	"github.com/sparkfabrik/claude-usage/internal/poller"
	"github.com/sparkfabrik/claude-usage/internal/pricing"
	"github.com/sparkfabrik/claude-usage/internal/process"
	"github.com/sparkfabrik/claude-usage/internal/reader"
	"github.com/sparkfabrik/claude-usage/internal/stats"
	"github.com/sparkfabrik/claude-usage/internal/usage"
)

// Build-time variables injected via ldflags.
var (
	version = ""
	commit  = ""
	date    = ""
	builtBy = ""
)

// initVersion populates version info from Go's embedded build metadata
// when ldflags are not set (e.g. plain go install).
func initVersion() {
	if version != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		version = "dev"
		return
	}
	applyBuildInfo(info)
}

// applyBuildInfo extracts version, commit, and date from debug.BuildInfo.
func applyBuildInfo(info *debug.BuildInfo) {
	version = info.Main.Version
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			date = s.Value
		case "vcs.modified":
			if s.Value == "true" && commit != "" {
				commit += "-dirty"
			}
		}
	}
	if version == "" || version == "(devel)" {
		version = "dev"
	}
}

// defaultPollTimeout is the HTTP timeout for API polling requests.
const defaultPollTimeout = 15 * time.Second

// StatusLimit is one rate-limit window in the status payload. The session and
// weekly windows are always present; model-scoped windows appear only when the
// OAuth usage endpoint reports them.
type StatusLimit struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Model string `json:"model,omitempty"`
	Pct   int    `json:"pct"`
	Reset string `json:"reset"`
	Color string `json:"color"`
}

// StatusDay is one day of token usage.
type StatusDay struct {
	Date    string `json:"date"`
	Weekday string `json:"weekday"`
	Tokens  int    `json:"tokens"`
	Today   bool   `json:"today"`
}

// StatusModel is one model's token usage.
type StatusModel struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// StatusToday summarizes the current local day.
type StatusToday struct {
	Tokens   int     `json:"tokens"`
	Messages int     `json:"messages"`
	Sessions int     `json:"sessions"`
	CostUSD  float64 `json:"cost_usd"`
}

// StatusResponse is the JSON structure output by --status mode.
//
// The first ten fields are the original contract and are always populated, so
// readers written against it keep working untouched. Everything below them is
// additive.
type StatusResponse struct {
	CPct          int    `json:"c_pct"`
	CReset        string `json:"c_reset"`
	CColor        string `json:"c_color"`
	WPct          int    `json:"w_pct"`
	WReset        string `json:"w_reset"`
	WColor        string `json:"w_color"`
	Stale         bool   `json:"stale"`
	ClaudeRunning bool   `json:"claude_running"`
	Auth          string `json:"auth"`
	Error         string `json:"error,omitempty"`

	// Source is "oauth" or "headers", telling a reader whether model-scoped
	// windows can be expected.
	Source string `json:"source,omitempty"`
	// Limits carries every open window, including model-scoped ones.
	Limits []StatusLimit `json:"limits,omitempty"`
	// RecentDays is seven days of token totals, oldest first.
	RecentDays []StatusDay `json:"recent_days,omitempty"`
	// Models is per-model token usage, heaviest first.
	Models []StatusModel `json:"models,omitempty"`
	// Today summarizes the current local day.
	Today *StatusToday `json:"today,omitempty"`
}

// printConfigFooter appends the Configuration: block to --help output: the
// default search chain in literal $XDG_CONFIG_HOME form, the active config
// (explicit --config path, resolved winner, or built-in defaults), and the
// reference file path when config.default.yaml exists.
func printConfigFooter(w io.Writer, explicitConfig string) {
	fmt.Fprintln(w, "\nConfiguration:")
	fmt.Fprintln(w, "  Search chain (first existing file wins):")
	for _, p := range config.SearchChainDisplay() {
		fmt.Fprintf(w, "    %s\n", p)
	}

	switch {
	case explicitConfig != "":
		fmt.Fprintf(w, "  Active config: %s\n", explicitConfig)
	default:
		if active, found := config.ResolvePath(); found {
			fmt.Fprintf(w, "  Active config: %s\n", active)
		} else {
			fmt.Fprintln(w, "  Active config: none — built-in default configuration applied")
		}
	}

	if ref, found := config.ReferencePath(); found {
		fmt.Fprintf(w, "  Reference file: %s\n", ref)
	}
}

func main() {
	showVersion := flag.BoolP("version", "V", false, "Print version information and exit")
	noPoll := flag.Bool("no-poll", false, "Skip API polling, use cached data only")
	forcePoll := flag.Bool("force-poll", false, "Force API poll even if cache is fresh")
	noCost := flag.Bool("no-cost", false, "Hide cost estimates")
	period := flag.String("period", "", "Show only a specific time period (today, 7d, 30d, all)")
	configPath := flag.StringP("config", "c", "", "Path to config file (overrides the default search chain)")
	pollOnly := flag.Bool("poll-only", false, "Poll API and update cache, no output (for scripting)")
	status := flag.Bool("status", false, "Output JSON status for GNOME extension or other consumers")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		printConfigFooter(os.Stderr, *configPath)
	}
	flag.Parse()

	if *showVersion {
		initVersion()
		fmt.Printf("claude-usage %s\n", version)
		if commit != "" {
			fmt.Printf("  commit:  %s\n", commit)
		}
		if date != "" {
			fmt.Printf("  built:   %s\n", date)
		}
		if builtBy != "" {
			fmt.Printf("  builder: %s\n", builtBy)
		}
		return
	}

	cfg := config.Load(*configPath)

	cachePath, err := resolveCachePath(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving cache path: %v\n", err)
		os.Exit(1)
	}

	// --status --no-poll needs no credentials at all (pure cache read).
	// Defer auth.Load() so the fast path avoids disk I/O for credentials.
	if *status && *noPoll {
		runStatus(os.Stdout, nil, cfg, cachePath, "", true, false, "")
		return
	}

	creds, err := auth.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading credentials: %v\n", err)
		os.Exit(1)
	}

	if *pollOnly {
		if err := runPollOnly(creds, cfg, cachePath, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *status {
		runStatus(os.Stdout, creds, cfg, cachePath, "", false, *forcePoll, "")
		return
	}

	if err := runDashboard(os.Stdout, creds, cfg, cachePath, "", *noPoll, *forcePoll, *noCost, *period, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// resolveCachePath returns the cache file path from config override or XDG default.
func resolveCachePath(cfg *config.Config) (string, error) {
	if cfg.Cache.Path != "" {
		return cfg.Cache.Path, nil
	}
	return cache.DefaultPath()
}

// runPollOnly polls the API and writes cache, with no output.
func runPollOnly(creds *auth.Credentials, cfg *config.Config, cachePath, apiURL string) error {
	if creds == nil {
		return fmt.Errorf("no credentials found")
	}
	if creds.IsExpired() {
		return fmt.Errorf("credentials expired")
	}
	q, err := poller.Poll(creds.AccessToken, cfg.API.Model, defaultPollTimeout, apiURL)
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}
	if err := cache.Write(cachePath, q); err != nil {
		return fmt.Errorf("cache write: %w", err)
	}
	return nil
}

// runStatus outputs JSON status for the GNOME extension or other consumers.
func runStatus(w io.Writer, creds *auth.Credentials, cfg *config.Config, cachePath, projectsPath string, noPoll, forcePoll bool, apiURL string) {
	var pollErr string

	cached := cache.Read(cachePath)
	claudeRunning := process.IsClaudeRunning()
	authState := authStatus(creds)

	if noPoll {
		// Creds may be nil when --no-poll skips auth.Load() — report "unknown" not "missing".
		if creds == nil {
			authState = "unknown"
		}
		if cached == nil {
			resp := buildStatusResponse(nil, cfg, "no cached data available")
			resp.ClaudeRunning = claudeRunning
			resp.Auth = authState
			attachStats(&resp, cfg, cachePath, projectsPath, false)
			_ = outputJSON(w, resp)
			return
		}
		resp := buildStatusResponse(cached, cfg, "")
		resp.ClaudeRunning = claudeRunning
		resp.Auth = authState
		attachStats(&resp, cfg, cachePath, projectsPath, false)
		_ = outputJSON(w, resp)
		return
	}

	canPoll := cfg.API.Enabled && creds != nil && !creds.IsExpired()
	// Skip polling if Claude isn't running and only_when_active is enabled (unless force-poll)
	if canPoll && cfg.API.IsOnlyWhenActive() && !claudeRunning && !forcePoll {
		canPoll = false
	}
	needPoll := forcePoll || cached == nil || !cached.IsFresh(cfg.API.StaleAfter)

	if canPoll && needPoll {
		q, err := usage.Collect(creds.AccessToken, cfg.API.UsageEndpoint, cfg.API.Model, apiURL, defaultPollTimeout)
		if err != nil {
			pollErr = err.Error()
		} else {
			if err := cache.Write(cachePath, q); err != nil {
				pollErr = fmt.Sprintf("cache write: %v", err)
			}
			cached = q
		}
	} else if !canPoll && cached == nil {
		if creds == nil {
			pollErr = "no credentials found"
		} else if creds.IsExpired() {
			pollErr = "credentials expired"
		} else {
			pollErr = "polling disabled"
		}
	}

	resp := buildStatusResponse(cached, cfg, pollErr)
	resp.ClaudeRunning = claudeRunning
	resp.Auth = authState
	// A forced poll also forces the transcript rescan: that is the explicit
	// refresh, as opposed to the cheap periodic one readers make.
	attachStats(&resp, cfg, cachePath, projectsPath, forcePoll)
	_ = outputJSON(w, resp)
}

// authStatus returns the auth state string for the status JSON.
func authStatus(creds *auth.Credentials) string {
	if creds == nil {
		return "missing"
	}
	if creds.IsExpired() {
		return "expired"
	}
	return "valid"
}

// outputJSON marshals v to JSON and writes to w.
func outputJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("JSON marshal: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// buildStatusResponse constructs a StatusResponse from cached quota data.
func buildStatusResponse(q *cache.QuotaCache, cfg *config.Config, errMsg string) StatusResponse {
	resp := StatusResponse{
		CReset: "?",
		WReset: "?",
		CColor: colorForPct(0, cfg),
		WColor: colorForPct(0, cfg),
		Error:  errMsg,
	}

	if q == nil {
		return resp
	}

	cPct := int(math.Round(q.Utilization5h * 100))
	wPct := int(math.Round(q.Utilization7d * 100))

	resp.CPct = cPct
	resp.CReset = dashboard.FormatTimeRemaining(q.MinutesToReset5h())
	resp.CColor = colorForPct(cPct, cfg)
	resp.WPct = wPct
	resp.WReset = dashboard.FormatTimeRemaining(q.MinutesToReset7d())
	resp.WColor = colorForPct(wPct, cfg)
	resp.Stale = q.IsStale()
	resp.Source = q.Source

	// Older cache files predate the windows field; rebuild it so the limits
	// list is populated whatever wrote the cache.
	q.SyncWindows()
	for _, w := range q.OpenWindows(time.Now()) {
		pct := int(math.Round(w.Utilization * 100))
		resp.Limits = append(resp.Limits, StatusLimit{
			Key:   w.Key,
			Title: w.Title,
			Model: w.Model,
			Pct:   pct,
			Reset: dashboard.FormatTimeRemaining(w.MinutesToReset()),
			Color: colorForPct(pct, cfg),
		})
	}

	return resp
}

// attachStats fills the local-transcript half of the status payload. A failure
// is not fatal: the quota half is what readers show by default, so the fields
// are simply left out.
func attachStats(resp *StatusResponse, cfg *config.Config, cachePath, projectsPath string, force bool) {
	s, err := stats.Load(stats.DefaultPath(cachePath), projectsPath, stats.DefaultTTLSeconds, force, buildPricingOverrides(cfg))
	if s == nil {
		if err != nil && resp.Error == "" {
			resp.Error = err.Error()
		}
		return
	}

	for _, d := range s.RecentDays {
		resp.RecentDays = append(resp.RecentDays, StatusDay{
			Date: d.Date, Weekday: d.Weekday, Tokens: d.Tokens, Today: d.Today,
		})
	}
	for _, m := range s.Models {
		resp.Models = append(resp.Models, StatusModel{Name: m.Name, Tokens: m.Tokens})
	}
	resp.Today = &StatusToday{
		Tokens:   s.Today.Tokens,
		Messages: s.Today.Messages,
		Sessions: s.Today.Sessions,
		CostUSD:  s.Today.CostUSD,
	}
}

// colorForPct returns a hex color string based on utilization percentage and config thresholds.
func colorForPct(pct int, cfg *config.Config) string {
	if pct >= cfg.Colors.OrangeBelow {
		return "#dc3232"
	}
	if pct >= cfg.Colors.GreenBelow {
		return "#e6961e"
	}
	return "#32c850"
}

// runDashboard renders the full CLI dashboard.
func runDashboard(w io.Writer, creds *auth.Credentials, cfg *config.Config, cachePath, projectsPath string, noPoll, forcePoll, noCost bool, period, apiURL string) error {
	showCost := cfg.Display.ShowCost && !noCost

	fmt.Fprintln(w, dashboard.RenderAccount(creds))
	fmt.Fprintln(w)

	quota := pollOrReadCache(w, creds, cfg, cachePath, noPoll, forcePoll, apiURL)
	fmt.Fprintln(w, dashboard.RenderQuota(quota, cfg))
	fmt.Fprintln(w)

	entries, err := reader.LoadEntries(projectsPath, nil, nil)
	if err != nil {
		return fmt.Errorf("loading usage data: %w", err)
	}
	if len(entries) == 0 {
		fmt.Fprintln(w, dashboard.DimText("No usage data found in ~/.claude/projects/"))
		return nil
	}

	periods := cfg.Display.Periods
	if period != "" {
		if _, ok := analyzer.FindPeriod(period); !ok {
			return fmt.Errorf("unknown period %q (valid: today, 7d, 30d, all)", period)
		}
		periods = []string{period}
	}

	overrides := buildPricingOverrides(cfg)

	summaries := analyzer.SummarizeByPeriods(entries, periods, overrides)
	fmt.Fprintln(w, dashboard.RenderUsageTable(summaries, showCost))
	fmt.Fprintln(w)

	// Model breakdown for the widest requested period (reuse already-loaded entries).
	if len(summaries) > 0 {
		last := summaries[len(summaries)-1]
		if p, ok := analyzer.FindPeriod(last.PeriodLabel); ok {
			filtered := analyzer.FilterEntries(entries, p.Start, p.End)
			modelSummaries := analyzer.SummarizeByModel(filtered, overrides)
			if len(modelSummaries) > 0 {
				fmt.Fprintln(w, dashboard.RenderModelTable(modelSummaries, showCost))
			}
		}
	}

	burnRate := dashboard.RenderBurnRate(summaries)
	if burnRate != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, burnRate)
	}
	return nil
}

// pollOrReadCache handles quota polling logic.
func pollOrReadCache(w io.Writer, creds *auth.Credentials, cfg *config.Config, cachePath string, noPoll, forcePoll bool, apiURL string) *cache.QuotaCache {
	canPoll := cfg.API.Enabled && creds != nil && !creds.IsExpired() && !noPoll

	if !canPoll {
		return cache.Read(cachePath)
	}

	cached := cache.Read(cachePath)
	needPoll := forcePoll || cached == nil || !cached.IsFresh(cfg.API.StaleAfter)

	if !needPoll {
		age := int(cached.AgeSeconds())
		fmt.Fprintln(w, dashboard.DimText(fmt.Sprintf("Using cached rate limits (%ds old, stale after %ds)", age, cfg.API.StaleAfter)))
		return cached
	}

	fmt.Fprintln(w, dashboard.DimText("Polling API for rate limits..."))
	q, err := poller.Poll(creds.AccessToken, cfg.API.Model, defaultPollTimeout, apiURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Poll failed: %v\n", err)
		return cached
	}
	if err := cache.Write(cachePath, q); err != nil {
		fmt.Fprintf(os.Stderr, "Cache write error: %v\n", err)
	}
	return q
}

// buildPricingOverrides converts config pricing to pricing.ModelPrice map.
func buildPricingOverrides(cfg *config.Config) map[string]pricing.ModelPrice {
	if len(cfg.Pricing) == 0 {
		return nil
	}
	overrides := make(map[string]pricing.ModelPrice, len(cfg.Pricing))
	for model, p := range cfg.Pricing {
		overrides[model] = pricing.ModelPrice{
			Input:      p.Input,
			Output:     p.Output,
			CacheWrite: p.CacheWrite,
			CacheRead:  p.CacheRead,
		}
	}
	return overrides
}
