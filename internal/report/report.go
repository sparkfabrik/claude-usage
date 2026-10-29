// Package report aggregates local transcript usage for one period into a
// JSON document: tokens by type and cost, in total and per model, session
// count with median and p90 cost, and active days.
//
// The report is offline by construction: it reads the transcripts and the
// price table, nothing else, and it carries no project path, session id or
// other identifying field, so a file can be shared as it is.
package report

import (
	"math"
	"sort"
	"time"

	"github.com/sparkfabrik/claude-usage/internal/analyzer"
	"github.com/sparkfabrik/claude-usage/internal/pricing"
	"github.com/sparkfabrik/claude-usage/internal/reader"
	"github.com/sparkfabrik/claude-usage/internal/stats"
)

// Schema identifies the document shape. It changes only when a field is
// renamed or removed; additions keep the value.
const Schema = "claude-usage/report/1"

// CostBasis states how every cost_usd in the document was computed.
const CostBasis = "list prices, cache writes at their recorded TTL rate"

// Tokens is a token count split by type. CacheWrite1h is the part of
// CacheWrite written with the 1-hour TTL, which costs more; the rest was
// written with the 5-minute TTL. Total does not count it twice.
type Tokens struct {
	Input        int `json:"input"`
	Output       int `json:"output"`
	CacheWrite   int `json:"cache_write"`
	CacheWrite1h int `json:"cache_write_1h"`
	CacheRead    int `json:"cache_read"`
	Total        int `json:"total"`
}

// Window is the period the report covers. From and To are RFC 3339 in UTC.
// Timezone and UTCOffset describe the local zone whose calendar decides
// active days: the zone name when the process knows it, otherwise the
// abbreviation in force at generation time, and the offset as +hh:mm.
type Window struct {
	Label     string `json:"label"`
	From      string `json:"from"`
	To        string `json:"to"`
	Timezone  string `json:"timezone"`
	UTCOffset string `json:"utc_offset"`
}

// Totals is the whole period in one row.
type Totals struct {
	Tokens     Tokens  `json:"tokens"`
	CostUSD    float64 `json:"cost_usd"`
	Requests   int     `json:"requests"`
	Sessions   int     `json:"sessions"`
	ActiveDays int     `json:"active_days"`
}

// Model is one model's share of the period, heaviest first in the report.
type Model struct {
	// ID is the model identifier as recorded in the transcript; Name is its
	// display form.
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Tokens   Tokens  `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	Requests int     `json:"requests"`
	// Priced is false when the price table has no row or prefix for the
	// model and the cost was computed with the fallback price.
	Priced bool `json:"priced"`
}

// Sessions summarizes the cost per session. Median and p90 are 0 when the
// period has no sessions.
type Sessions struct {
	Count         int     `json:"count"`
	CostMedianUSD float64 `json:"cost_median_usd"`
	CostP90USD    float64 `json:"cost_p90_usd"`
}

// Report is the JSON document printed by --report.
type Report struct {
	Schema      string   `json:"schema"`
	Version     string   `json:"version"`
	GeneratedAt string   `json:"generated_at"`
	Window      Window   `json:"window"`
	CostBasis   string   `json:"cost_basis"`
	Pricing     string   `json:"pricing"`
	Totals      Totals   `json:"totals"`
	Models      []Model  `json:"models"`
	Sessions    Sessions `json:"sessions"`
	// UnpricedModels lists the model ids priced with the fallback, so the
	// reader of the file knows which costs are a guess.
	UnpricedModels []string `json:"unpriced_models,omitempty"`
}

// Build aggregates the entries that fall inside period into a Report.
// Entries are filtered here, so callers may pass everything the reader
// returned. Days are counted on now's calendar, like the dashboard.
func Build(entries []reader.UsageEntry, period analyzer.TimePeriod, now time.Time, version string, overrides map[string]pricing.ModelPrice) *Report {
	inWindow := analyzer.FilterEntries(entries, period.Start, period.End)

	r := &Report{
		Schema:      Schema,
		Version:     version,
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Window: Window{
			Label:     period.Label,
			From:      period.Start.UTC().Format(time.RFC3339),
			To:        period.End.UTC().Format(time.RFC3339),
			Timezone:  zoneName(now),
			UTCOffset: now.Format("-07:00"),
		},
		CostBasis: CostBasis,
		Pricing:   pricingSource(overrides),
		Models:    []Model{},
	}

	total := analyzer.Aggregate(inWindow, period.Label, overrides)
	r.Totals = Totals{
		Tokens:     tokensOf(total),
		CostUSD:    total.TotalCostUSD,
		Requests:   total.MessageCount,
		Sessions:   total.SessionCount,
		ActiveDays: activeDays(inWindow, now.Location()),
	}

	for _, s := range analyzer.SummarizeByModel(inWindow, overrides) {
		m := Model{
			ID:       s.PeriodLabel,
			Name:     stats.FriendlyModelName(s.PeriodLabel),
			Tokens:   tokensOf(s),
			CostUSD:  s.TotalCostUSD,
			Requests: s.MessageCount,
			Priced:   pricing.Known(s.PeriodLabel, overrides),
		}
		if !m.Priced {
			r.UnpricedModels = append(r.UnpricedModels, m.ID)
		}
		r.Models = append(r.Models, m)
	}

	r.Sessions = sessionCosts(inWindow, overrides)
	return r
}

// zoneName returns the location name, or the abbreviation in force at t
// when the location is the unnamed local zone ("Local" is what Go reports
// when TZ is not set).
func zoneName(t time.Time) string {
	if name := t.Location().String(); name != "Local" {
		return name
	}
	abbr, _ := t.Zone()
	return abbr
}

// pricingSource says where the prices came from: the built-in table, or the
// table with overrides from the configuration file.
func pricingSource(overrides map[string]pricing.ModelPrice) string {
	if len(overrides) == 0 {
		return "bundled"
	}
	return "config"
}

func tokensOf(s *analyzer.Summary) Tokens {
	return Tokens{
		Input:        s.TotalInputTokens,
		Output:       s.TotalOutputTokens,
		CacheWrite:   s.TotalCacheWriteTokens,
		CacheWrite1h: s.TotalCacheWrite1hTokens,
		CacheRead:    s.TotalCacheReadTokens,
		Total:        s.TotalTokens(),
	}
}

// activeDays counts the distinct calendar days, in loc, with at least one
// request.
func activeDays(entries []reader.UsageEntry, loc *time.Location) int {
	days := make(map[string]struct{})
	for _, e := range entries {
		days[e.Timestamp.In(loc).Format("2006-01-02")] = struct{}{}
	}
	return len(days)
}

// sessionCosts sums the cost of every session and reports the count, the
// median and the 90th percentile. The percentile uses the nearest-rank
// method on the ascending list: the value at position ceil(0.9 * n).
func sessionCosts(entries []reader.UsageEntry, overrides map[string]pricing.ModelPrice) Sessions {
	bySession := make(map[string]float64)
	for _, e := range entries {
		bySession[e.SessionID] += pricing.Calculate(
			e.InputTokens, e.OutputTokens,
			e.CacheCreationTokens, e.CacheWrite1hTokens, e.CacheReadTokens,
			e.Model, overrides,
		)
	}
	if len(bySession) == 0 {
		return Sessions{}
	}

	costs := make([]float64, 0, len(bySession))
	for _, c := range bySession {
		costs = append(costs, c)
	}
	sort.Float64s(costs)

	return Sessions{
		Count:         len(costs),
		CostMedianUSD: median(costs),
		CostP90USD:    percentile(costs, 0.9),
	}
}

// median of an ascending, non-empty slice.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// percentile returns the nearest-rank percentile of an ascending, non-empty
// slice: the element at rank ceil(p * n), 1-based.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	rank := int(math.Ceil(float64(n) * p))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}
