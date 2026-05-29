package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/Monska85/claude-usage/internal/analyzer"
	"github.com/Monska85/claude-usage/internal/auth"
	"github.com/Monska85/claude-usage/internal/cache"
	"github.com/Monska85/claude-usage/internal/config"
	"github.com/Monska85/claude-usage/internal/dashboard"
	"github.com/Monska85/claude-usage/internal/poller"
	"github.com/Monska85/claude-usage/internal/pricing"
	"github.com/Monska85/claude-usage/internal/process"
	"github.com/Monska85/claude-usage/internal/reader"
)

// defaultPollTimeout is the HTTP timeout for API polling requests.
const defaultPollTimeout = 15 * time.Second

// StatusResponse is the JSON structure output by --status mode.
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
}

func main() {
	noPoll := flag.Bool("no-poll", false, "Skip API polling, use cached data only")
	forcePoll := flag.Bool("force-poll", false, "Force API poll even if cache is fresh")
	noCost := flag.Bool("no-cost", false, "Hide cost estimates")
	period := flag.String("period", "", "Show only a specific time period (today, 7d, 30d, all)")
	configPath := flag.String("config", "", "Path to config file")
	pollOnly := flag.Bool("poll-only", false, "Poll API and update cache, no output (for scripting)")
	status := flag.Bool("status", false, "Output JSON status for GNOME extension or other consumers")
	flag.Parse()

	cfg := config.Load(*configPath)

	cachePath, err := resolveCachePath(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving cache path: %v\n", err)
		os.Exit(1)
	}

	// --status --no-poll needs no credentials at all (pure cache read).
	// Defer auth.Load() so the fast path avoids disk I/O for credentials.
	if *status && *noPoll {
		runStatus(nil, cfg, cachePath, true, false)
		return
	}

	creds, err := auth.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading credentials: %v\n", err)
		os.Exit(1)
	}

	if *pollOnly {
		runPollOnly(creds, cfg, cachePath)
		return
	}

	if *status {
		runStatus(creds, cfg, cachePath, false, *forcePoll)
		return
	}

	runDashboard(creds, cfg, cachePath, *noPoll, *forcePoll, *noCost, *period)
}

// resolveCachePath returns the cache file path from config override or XDG default.
func resolveCachePath(cfg *config.Config) (string, error) {
	if cfg.Cache.Path != "" {
		return cfg.Cache.Path, nil
	}
	return cache.DefaultPath()
}

// runPollOnly polls the API and writes cache, with no output.
func runPollOnly(creds *auth.Credentials, cfg *config.Config, cachePath string) {
	if creds == nil {
		fmt.Fprintf(os.Stderr, "Error: no credentials found\n")
		os.Exit(1)
	}
	if creds.IsExpired() {
		fmt.Fprintf(os.Stderr, "Error: credentials expired\n")
		os.Exit(1)
	}
	q, err := poller.Poll(creds.AccessToken, cfg.API.Model, defaultPollTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Poll error: %v\n", err)
		os.Exit(1)
	}
	if err := cache.Write(cachePath, q); err != nil {
		fmt.Fprintf(os.Stderr, "Cache write error: %v\n", err)
		os.Exit(1)
	}
}

// runStatus outputs JSON status for the GNOME extension or other consumers.
func runStatus(creds *auth.Credentials, cfg *config.Config, cachePath string, noPoll, forcePoll bool) {
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
			outputJSON(resp)
			return
		}
		resp := buildStatusResponse(cached, cfg, "")
		resp.ClaudeRunning = claudeRunning
		resp.Auth = authState
		outputJSON(resp)
		return
	}

	canPoll := cfg.API.Enabled && creds != nil && !creds.IsExpired()
	// Skip polling if Claude isn't running and only_when_active is enabled (unless force-poll)
	if canPoll && cfg.API.IsOnlyWhenActive() && !claudeRunning && !forcePoll {
		canPoll = false
	}
	needPoll := forcePoll || cached == nil || !cached.IsFresh(cfg.API.StaleAfter)

	if canPoll && needPoll {
		q, err := poller.Poll(creds.AccessToken, cfg.API.Model, defaultPollTimeout)
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
	outputJSON(resp)
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

// outputJSON marshals v to JSON and writes to stdout.
func outputJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	os.Stdout.Write(data)
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

	return resp
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
func runDashboard(creds *auth.Credentials, cfg *config.Config, cachePath string, noPoll, forcePoll, noCost bool, period string) {
	showCost := cfg.Display.ShowCost && !noCost

	fmt.Println(dashboard.RenderAccount(creds))
	fmt.Println()

	quota := pollOrReadCache(creds, cfg, cachePath, noPoll, forcePoll)
	fmt.Println(dashboard.RenderQuota(quota, cfg))
	fmt.Println()

	entries, err := reader.LoadEntries("", nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading usage data: %v\n", err)
		return
	}
	if len(entries) == 0 {
		fmt.Println(dashboard.DimText("No usage data found in ~/.claude/projects/"))
		return
	}

	periods := cfg.Display.Periods
	if period != "" {
		if _, ok := analyzer.FindPeriod(period); !ok {
			fmt.Fprintf(os.Stderr, "Unknown period %q (valid: today, 7d, 30d, all)\n", period)
			os.Exit(1)
		}
		periods = []string{period}
	}

	overrides := buildPricingOverrides(cfg)

	summaries := analyzer.SummarizeByPeriods(entries, periods, overrides)
	fmt.Println(dashboard.RenderUsageTable(summaries, showCost))
	fmt.Println()

	// Model breakdown for the widest requested period (reuse already-loaded entries).
	if len(summaries) > 0 {
		last := summaries[len(summaries)-1]
		if p, ok := analyzer.FindPeriod(last.PeriodLabel); ok {
			filtered := analyzer.FilterEntries(entries, p.Start, p.End)
			modelSummaries := analyzer.SummarizeByModel(filtered, overrides)
			if len(modelSummaries) > 0 {
				fmt.Println(dashboard.RenderModelTable(modelSummaries, showCost))
			}
		}
	}

	burnRate := dashboard.RenderBurnRate(summaries)
	if burnRate != "" {
		fmt.Println()
		fmt.Println(burnRate)
	}
}

// pollOrReadCache handles quota polling logic.
func pollOrReadCache(creds *auth.Credentials, cfg *config.Config, cachePath string, noPoll, forcePoll bool) *cache.QuotaCache {
	canPoll := cfg.API.Enabled && creds != nil && !creds.IsExpired() && !noPoll

	if !canPoll {
		return cache.Read(cachePath)
	}

	cached := cache.Read(cachePath)
	needPoll := forcePoll || cached == nil || !cached.IsFresh(cfg.API.StaleAfter)

	if !needPoll {
		age := int(cached.AgeSeconds())
		fmt.Println(dashboard.DimText(fmt.Sprintf("Using cached rate limits (%ds old, stale after %ds)", age, cfg.API.StaleAfter)))
		return cached
	}

	fmt.Println(dashboard.DimText("Polling API for rate limits..."))
	q, err := poller.Poll(creds.AccessToken, cfg.API.Model, defaultPollTimeout)
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
