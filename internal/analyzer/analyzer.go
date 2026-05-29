// Package analyzer aggregates usage entries into summaries.
package analyzer

import (
	"time"

	"github.com/Monska85/claude-usage/internal/pricing"
	"github.com/Monska85/claude-usage/internal/reader"
)

// Summary holds aggregated usage for a time period.
type Summary struct {
	PeriodLabel         string
	TotalInputTokens    int
	TotalOutputTokens   int
	TotalCacheWriteTokens int
	TotalCacheReadTokens  int
	TotalCostUSD        float64
	MessageCount        int
	SessionCount        int
	ModelsUsed          map[string]int
	StartTime           *time.Time
	EndTime             *time.Time
}

// TotalTokens returns the sum of all token types.
func (s *Summary) TotalTokens() int {
	return s.TotalInputTokens + s.TotalOutputTokens + s.TotalCacheWriteTokens + s.TotalCacheReadTokens
}

// BurnRatePerMinute returns tokens/minute, or -1 if not calculable.
func (s *Summary) BurnRatePerMinute() float64 {
	if s.StartTime == nil || s.EndTime == nil {
		return -1
	}
	dur := s.EndTime.Sub(*s.StartTime).Minutes()
	if dur <= 0 {
		return -1
	}
	return float64(s.TotalTokens()) / dur
}

// Aggregate creates a summary from a list of entries.
func Aggregate(entries []reader.UsageEntry, label string, overrides map[string]pricing.ModelPrice) *Summary {
	s := &Summary{
		PeriodLabel: label,
		ModelsUsed:  make(map[string]int),
	}
	if len(entries) == 0 {
		return s
	}

	sessions := make(map[string]struct{})
	var minTime, maxTime time.Time
	for _, e := range entries {
		s.TotalInputTokens += e.InputTokens
		s.TotalOutputTokens += e.OutputTokens
		s.TotalCacheWriteTokens += e.CacheCreationTokens
		s.TotalCacheReadTokens += e.CacheReadTokens
		s.ModelsUsed[e.Model]++
		sessions[e.SessionID] = struct{}{}
		s.TotalCostUSD += pricing.Calculate(
			e.InputTokens, e.OutputTokens,
			e.CacheCreationTokens, e.CacheReadTokens,
			e.Model, overrides,
		)
		if minTime.IsZero() || e.Timestamp.Before(minTime) {
			minTime = e.Timestamp
		}
		if maxTime.IsZero() || e.Timestamp.After(maxTime) {
			maxTime = e.Timestamp
		}
	}

	s.MessageCount = len(entries)
	s.SessionCount = len(sessions)
	s.StartTime = &minTime
	s.EndTime = &maxTime

	return s
}

// TimePeriod represents a named time range.
type TimePeriod struct {
	Label string
	Start time.Time
	End   time.Time
}

// StandardPeriods returns the default time periods.
func StandardPeriods() []TimePeriod {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	return []TimePeriod{
		{"today", todayStart, now},
		{"7d", now.Add(-7 * 24 * time.Hour), now},
		{"30d", now.Add(-30 * 24 * time.Hour), now},
		{"all", time.Time{}, now},
	}
}

// SummarizeByPeriods creates summaries for multiple time periods.
func SummarizeByPeriods(entries []reader.UsageEntry, periodLabels []string, overrides map[string]pricing.ModelPrice) []*Summary {
	periods := StandardPeriods()

	// Filter to requested periods
	wanted := make(map[string]bool)
	for _, l := range periodLabels {
		wanted[l] = true
	}

	var results []*Summary
	for _, p := range periods {
		if !wanted[p.Label] {
			continue
		}
		filtered := filterEntries(entries, p.Start, p.End)
		results = append(results, Aggregate(filtered, p.Label, overrides))
	}
	return results
}

// SummarizeByModel breaks down usage by model.
func SummarizeByModel(entries []reader.UsageEntry, overrides map[string]pricing.ModelPrice) []*Summary {
	byModel := make(map[string][]reader.UsageEntry)
	for _, e := range entries {
		byModel[e.Model] = append(byModel[e.Model], e)
	}

	var results []*Summary
	for model, modelEntries := range byModel {
		results = append(results, Aggregate(modelEntries, model, overrides))
	}
	return results
}

// FindPeriod looks up a period by label. Returns the period and true if found.
func FindPeriod(label string) (TimePeriod, bool) {
	for _, p := range StandardPeriods() {
		if p.Label == label {
			return p, true
		}
	}
	return TimePeriod{}, false
}

// FilterEntries filters entries to a time range.
func FilterEntries(entries []reader.UsageEntry, start, end time.Time) []reader.UsageEntry {
	return filterEntries(entries, start, end)
}

func filterEntries(entries []reader.UsageEntry, start, end time.Time) []reader.UsageEntry {
	out := make([]reader.UsageEntry, 0, len(entries))
	for _, e := range entries {
		if !start.IsZero() && e.Timestamp.Before(start) {
			continue
		}
		if e.Timestamp.After(end) {
			continue
		}
		out = append(out, e)
	}
	return out
}
