// Package stats aggregates local transcript usage into the compact shape the
// readers consume, and caches it on disk.
//
// The aggregation walks every JSONL transcript, which on a working machine can
// be hundreds of megabytes. Readers poll every minute, so the result is cached
// and only recomputed when it goes stale or when a refresh is forced.
package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sparkfabrik/claude-usage/internal/analyzer"
	"github.com/sparkfabrik/claude-usage/internal/pricing"
	"github.com/sparkfabrik/claude-usage/internal/reader"
)

// DefaultTTLSeconds is how long a computed aggregation stays usable. It is
// deliberately much longer than the readers' poll interval: token totals move
// slowly, and rescanning every minute would read the whole transcript history
// each time.
const DefaultTTLSeconds = 900

// recentDayCount is the size of the per-day history, matching a week.
const recentDayCount = 7

// Day is one calendar day of token usage.
type Day struct {
	// Date is the local calendar date, YYYY-MM-DD.
	Date string `json:"date"`
	// Weekday is the short English weekday name, for readers that render a
	// label without parsing the date.
	Weekday string `json:"weekday"`
	Tokens  int    `json:"tokens"`
	// Today marks the current local day.
	Today bool `json:"today"`
}

// Model is one model's all-time token usage.
type Model struct {
	// ID is the raw model identifier as recorded in the transcript.
	ID string `json:"id"`
	// Name is the display form, for example "Opus 4.8".
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// Today summarizes the current local day.
type Today struct {
	Tokens   int     `json:"tokens"`
	Messages int     `json:"messages"`
	Sessions int     `json:"sessions"`
	CostUSD  float64 `json:"cost_usd"`
}

// Stats is the cached aggregation.
type Stats struct {
	ComputedAt string  `json:"computed_at"` // RFC 3339
	RecentDays []Day   `json:"recent_days"`
	Models     []Model `json:"models"`
	Today      Today   `json:"today"`
}

// AgeSeconds returns seconds since the aggregation was computed.
func (s *Stats) AgeSeconds() float64 {
	t, err := time.Parse(time.RFC3339Nano, s.ComputedAt)
	if err != nil {
		return float64(1 << 30)
	}
	return time.Since(t).Seconds()
}

// IsFresh reports whether the aggregation is younger than ttlSeconds.
func (s *Stats) IsFresh(ttlSeconds int) bool {
	if s == nil {
		return false
	}
	return s.AgeSeconds() < float64(ttlSeconds)
}

// DefaultPath returns the stats cache location, alongside the quota cache.
func DefaultPath(quotaCachePath string) string {
	return filepath.Join(filepath.Dir(quotaCachePath), "stats.json")
}

// Compute aggregates entries into Stats, relative to now.
func Compute(entries []reader.UsageEntry, now time.Time, overrides map[string]pricing.ModelPrice) *Stats {
	s := &Stats{
		ComputedAt: now.UTC().Format(time.RFC3339Nano),
		RecentDays: recentDays(entries, now),
		Models:     models(entries),
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today := analyzer.Aggregate(analyzer.FilterEntries(entries, todayStart, now), "today", overrides)
	s.Today = Today{
		Tokens:   today.TotalTokens(),
		Messages: today.MessageCount,
		Sessions: today.SessionCount,
		CostUSD:  today.TotalCostUSD,
	}
	return s
}

// recentDays returns exactly seven days ending today, oldest first, so readers
// can render a fixed-width chart without filling gaps themselves.
func recentDays(entries []reader.UsageEntry, now time.Time) []Day {
	// Every date is resolved in the same zone as now. A transcript timestamp
	// is UTC, and grouping by it would file evening work under tomorrow;
	// mixing zones between the entries and the day keys would misalign them.
	loc := now.Location()

	totals := make(map[string]int, recentDayCount)
	for _, e := range entries {
		totals[e.Timestamp.In(loc).Format("2006-01-02")] += e.TotalTokens()
	}

	todayKey := now.Format("2006-01-02")
	days := make([]Day, 0, recentDayCount)
	for offset := recentDayCount - 1; offset >= 0; offset-- {
		d := now.AddDate(0, 0, -offset)
		key := d.Format("2006-01-02")
		days = append(days, Day{
			Date:    key,
			Weekday: d.Format("Mon"),
			Tokens:  totals[key],
			Today:   key == todayKey,
		})
	}
	return days
}

// models returns per-model totals, heaviest first. Ties break on name so the
// order never depends on map iteration.
func models(entries []reader.UsageEntry) []Model {
	totals := make(map[string]int)
	for _, e := range entries {
		totals[e.Model] += e.TotalTokens()
	}

	out := make([]Model, 0, len(totals))
	for id, tokens := range totals {
		if id == "" {
			continue
		}
		out = append(out, Model{ID: id, Name: FriendlyModelName(id), Tokens: tokens})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// FriendlyModelName turns a model id into a display name: "claude-opus-4-8"
// becomes "Opus 4.8". Ids arrive hyphenated with the version split across
// segments, so the numeric run is rejoined and the surrounding words are
// title-cased. A trailing date stamp is dropped.
func FriendlyModelName(id string) string {
	if id == "" {
		return "Unknown"
	}

	name := strings.TrimPrefix(id, "claude-")
	parts := strings.Split(name, "-")
	if n := len(parts); n > 1 && len(parts[n-1]) == 8 && isDigits(parts[n-1]) {
		parts = parts[:n-1]
	}

	var words, version []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part[0] >= '0' && part[0] <= '9' {
			version = append(version, part)
			continue
		}
		if len(version) > 0 {
			words = append(words, strings.Join(version, "."))
			version = nil
		}
		words = append(words, titleWord(part))
	}
	if len(version) > 0 {
		words = append(words, strings.Join(version, "."))
	}

	if len(words) == 0 {
		return "Unknown"
	}
	return strings.Join(words, " ")
}

func titleWord(word string) string {
	switch strings.ToLower(word) {
	case "gpt":
		return "GPT"
	case "ai":
		return "AI"
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Read loads cached stats. Returns nil when absent or unreadable.
func Read(path string) *Stats {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s Stats
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

// Write atomically stores stats.
func Write(path string, s *Stats) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".claude-usage-stats-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()        //nolint:errcheck // already failing
		os.Remove(tmpName) //nolint:errcheck // best effort cleanup
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best effort cleanup
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best effort cleanup
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// Load returns cached stats when fresh, otherwise recomputes from the
// transcripts and stores the result. A recompute failure is reported with any
// stale cache still returned, so a reader keeps showing the last known values.
func Load(cachePath, projectsPath string, ttlSeconds int, force bool, overrides map[string]pricing.ModelPrice) (*Stats, error) {
	cached := Read(cachePath)
	if !force && cached.IsFresh(ttlSeconds) {
		return cached, nil
	}

	entries, err := reader.LoadEntries(projectsPath, nil, nil)
	if err != nil {
		return cached, err
	}

	s := Compute(entries, time.Now(), overrides)
	if err := Write(cachePath, s); err != nil {
		return s, fmt.Errorf("stats cache write: %w", err)
	}
	return s, nil
}
