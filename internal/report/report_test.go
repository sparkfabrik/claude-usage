package report

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/sparkfabrik/claude-usage/internal/analyzer"
	"github.com/sparkfabrik/claude-usage/internal/pricing"
	"github.com/sparkfabrik/claude-usage/internal/reader"
)

func entry(ts time.Time, model, session string, input, output, cacheW, cacheR int) reader.UsageEntry {
	return reader.UsageEntry{
		Timestamp:           ts,
		Model:               model,
		SessionID:           session,
		InputTokens:         input,
		OutputTokens:        output,
		CacheCreationTokens: cacheW,
		CacheReadTokens:     cacheR,
	}
}

func period(start, end time.Time) analyzer.TimePeriod {
	return analyzer.TimePeriod{Label: "30d", Start: start, End: end}
}

func TestBuild_EmptyWindow(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	r := Build(nil, period(now.Add(-30*24*time.Hour), now), now, "test", nil)

	if r.Schema != Schema {
		t.Errorf("Schema = %q, want %q", r.Schema, Schema)
	}
	if r.Totals.Tokens.Total != 0 || r.Totals.CostUSD != 0 || r.Totals.Sessions != 0 || r.Totals.ActiveDays != 0 {
		t.Errorf("empty window should be all zeros, got %+v", r.Totals)
	}
	if r.Sessions.Count != 0 || r.Sessions.CostMedianUSD != 0 || r.Sessions.CostP90USD != 0 {
		t.Errorf("empty window sessions should be zeros, got %+v", r.Sessions)
	}
	if r.Models == nil || len(r.Models) != 0 {
		t.Errorf("Models should be an empty list, got %#v", r.Models)
	}
	if r.Pricing != "bundled" {
		t.Errorf("Pricing = %q, want bundled", r.Pricing)
	}
	if r.CostBasis != CostBasis {
		t.Errorf("CostBasis = %q, want %q", r.CostBasis, CostBasis)
	}
}

func TestBuild_WindowAndMetadata(t *testing.T) {
	loc := time.FixedZone("Europe/Rome", 2*3600)
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, loc)
	start := now.Add(-7 * 24 * time.Hour)
	p := analyzer.TimePeriod{Label: "7d", Start: start, End: now}

	r := Build(nil, p, now, "v1.2.3", nil)

	if r.Window.Label != "7d" {
		t.Errorf("Window.Label = %q, want 7d", r.Window.Label)
	}
	if r.Window.From != start.UTC().Format(time.RFC3339) {
		t.Errorf("Window.From = %q, want %q", r.Window.From, start.UTC().Format(time.RFC3339))
	}
	if r.Window.To != now.UTC().Format(time.RFC3339) {
		t.Errorf("Window.To = %q, want %q", r.Window.To, now.UTC().Format(time.RFC3339))
	}
	if r.Window.Timezone != "Europe/Rome" {
		t.Errorf("Window.Timezone = %q, want Europe/Rome", r.Window.Timezone)
	}
	if r.Window.UTCOffset != "+02:00" {
		t.Errorf("Window.UTCOffset = %q, want +02:00", r.Window.UTCOffset)
	}
	if r.Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", r.Version)
	}
	if r.GeneratedAt != now.UTC().Format(time.RFC3339) {
		t.Errorf("GeneratedAt = %q, want %q", r.GeneratedAt, now.UTC().Format(time.RFC3339))
	}
}

func TestBuild_TotalsModelsAndFiltering(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	start := now.Add(-30 * 24 * time.Hour)
	entries := []reader.UsageEntry{
		entry(now.Add(-time.Hour), "claude-opus-5-5", "s1", 1000, 100, 2000, 3000),
		entry(now.Add(-2*time.Hour), "claude-opus-5-5", "s1", 500, 50, 0, 0),
		entry(now.Add(-48*time.Hour), "claude-haiku-5-5", "s2", 100, 10, 0, 0),
		// Outside the window: must be ignored.
		entry(start.Add(-time.Hour), "claude-opus-5-5", "s0", 9999, 9999, 9999, 9999),
	}

	r := Build(entries, period(start, now), now, "test", nil)

	want := Tokens{Input: 1600, Output: 160, CacheWrite: 2000, CacheRead: 3000, Total: 6760}
	if r.Totals.Tokens != want {
		t.Errorf("Totals.Tokens = %+v, want %+v", r.Totals.Tokens, want)
	}
	if r.Totals.Requests != 3 {
		t.Errorf("Totals.Requests = %d, want 3", r.Totals.Requests)
	}
	if r.Totals.Sessions != 2 {
		t.Errorf("Totals.Sessions = %d, want 2", r.Totals.Sessions)
	}
	if r.Totals.ActiveDays != 2 {
		t.Errorf("Totals.ActiveDays = %d, want 2", r.Totals.ActiveDays)
	}

	// Opus 5.5: 1500 in, 150 out, 2000 write, 3000 read at 4 / 20 / 5 / 0.20
	// per million = 0.006 + 0.003 + 0.01 + 0.0006 = 0.0196.
	// Haiku 5.5: 100 in, 10 out at 0.10 / 0.50 = 0.00001 + 0.000005 = 0.000015.
	wantCost := 0.0196 + 0.000015
	if math.Abs(r.Totals.CostUSD-wantCost) > 1e-9 {
		t.Errorf("Totals.CostUSD = %f, want %f", r.Totals.CostUSD, wantCost)
	}

	if len(r.Models) != 2 {
		t.Fatalf("got %d models, want 2", len(r.Models))
	}
	if r.Models[0].ID != "claude-opus-5-5" || r.Models[0].Name != "Opus 5.5" {
		t.Errorf("first model = %q / %q, want claude-opus-5-5 / Opus 5.5 (heaviest first)", r.Models[0].ID, r.Models[0].Name)
	}
	if r.Models[0].Requests != 2 || r.Models[0].Tokens.Total != 6650 {
		t.Errorf("opus row = %+v", r.Models[0])
	}
	if !r.Models[0].Priced || !r.Models[1].Priced {
		t.Errorf("known models should be priced: %+v", r.Models)
	}
	if r.UnpricedModels != nil {
		t.Errorf("UnpricedModels = %v, want none", r.UnpricedModels)
	}
	if r.Sessions.Count != 2 {
		t.Errorf("Sessions.Count = %d, want 2", r.Sessions.Count)
	}
}

func TestBuild_UnpricedModelIsFlagged(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	entries := []reader.UsageEntry{
		entry(now.Add(-time.Hour), "claude-future-9", "s1", 100, 10, 0, 0),
	}
	r := Build(entries, period(now.Add(-24*time.Hour), now), now, "test", nil)

	if len(r.Models) != 1 || r.Models[0].Priced {
		t.Fatalf("unknown model should be reported unpriced: %+v", r.Models)
	}
	if len(r.UnpricedModels) != 1 || r.UnpricedModels[0] != "claude-future-9" {
		t.Errorf("UnpricedModels = %v, want [claude-future-9]", r.UnpricedModels)
	}
	// The cost is still the fallback estimate, like the dashboard shows.
	if r.Models[0].CostUSD <= 0 {
		t.Errorf("fallback cost should be positive, got %f", r.Models[0].CostUSD)
	}
}

func TestBuild_PricingOverridesAreReported(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	overrides := map[string]pricing.ModelPrice{
		"claude-future-9": {Input: 1, Output: 1, CacheWrite: 1, CacheRead: 1},
	}
	entries := []reader.UsageEntry{
		entry(now.Add(-time.Hour), "claude-future-9", "s1", 1_000_000, 0, 0, 0),
	}
	r := Build(entries, period(now.Add(-24*time.Hour), now), now, "test", overrides)

	if r.Pricing != "config" {
		t.Errorf("Pricing = %q, want config", r.Pricing)
	}
	if !r.Models[0].Priced {
		t.Errorf("an overridden model is priced")
	}
	if math.Abs(r.Models[0].CostUSD-1.0) > 1e-9 {
		t.Errorf("CostUSD = %f, want 1.0 from the override", r.Models[0].CostUSD)
	}
}

func TestBuild_ActiveDaysFollowLocalCalendar(t *testing.T) {
	// 23:30 and 00:30 UTC on consecutive days are the same local day in a
	// zone two hours ahead only if both fall after local midnight; here
	// 23:30Z is 01:30 local on day 2 and 00:30Z is 02:30 local on day 2.
	loc := time.FixedZone("UTC+2", 2*3600)
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, loc)
	entries := []reader.UsageEntry{
		entry(time.Date(2026, 3, 1, 23, 30, 0, 0, time.UTC), "claude-opus-5-5", "s1", 10, 1, 0, 0),
		entry(time.Date(2026, 3, 2, 0, 30, 0, 0, time.UTC), "claude-opus-5-5", "s1", 10, 1, 0, 0),
	}
	r := Build(entries, period(now.Add(-7*24*time.Hour), now), now, "test", nil)
	if r.Totals.ActiveDays != 1 {
		t.Errorf("ActiveDays = %d, want 1 on the local calendar", r.Totals.ActiveDays)
	}

	utcNow := now.UTC()
	r = Build(entries, period(utcNow.Add(-7*24*time.Hour), utcNow), utcNow, "test", nil)
	if r.Totals.ActiveDays != 2 {
		t.Errorf("ActiveDays = %d, want 2 on the UTC calendar", r.Totals.ActiveDays)
	}
}

func TestZoneName(t *testing.T) {
	named := time.Date(2026, 3, 10, 12, 0, 0, 0, time.FixedZone("Europe/Rome", 3600))
	if got := zoneName(named); got != "Europe/Rome" {
		t.Errorf("zoneName(named) = %q, want Europe/Rome", got)
	}
	if got := zoneName(time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)); got != "UTC" {
		t.Errorf("zoneName(UTC) = %q, want UTC", got)
	}
	// An unnamed local zone reports its abbreviation, never the literal "Local".
	local := time.Date(2026, 3, 10, 12, 0, 0, 0, time.Local)
	if got := zoneName(local); got == "Local" || got == "" {
		t.Errorf("zoneName(local) = %q, want an abbreviation", got)
	}
}

func TestSessionCosts_MedianAndP90(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	// Ten sessions on an override priced at 1 USD per million input tokens,
	// costing 1, 2, ..., 10 USD.
	overrides := map[string]pricing.ModelPrice{"m": {Input: 1}}
	var entries []reader.UsageEntry
	for i := 1; i <= 10; i++ {
		entries = append(entries, entry(now.Add(-time.Duration(i)*time.Minute), "m", "s"+string(rune('0'+i)), i*1_000_000, 0, 0, 0))
	}

	s := sessionCosts(entries, overrides)
	if s.Count != 10 {
		t.Fatalf("Count = %d, want 10", s.Count)
	}
	if math.Abs(s.CostMedianUSD-5.5) > 1e-9 {
		t.Errorf("median = %f, want 5.5 (mean of the two middle values)", s.CostMedianUSD)
	}
	// Nearest rank: ceil(0.9 * 10) = 9, the ninth value ascending.
	if math.Abs(s.CostP90USD-9.0) > 1e-9 {
		t.Errorf("p90 = %f, want 9.0", s.CostP90USD)
	}
}

func TestSessionCosts_SumsAcrossEntriesOfOneSession(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	overrides := map[string]pricing.ModelPrice{"m": {Input: 1}}
	entries := []reader.UsageEntry{
		entry(now, "m", "same", 1_000_000, 0, 0, 0),
		entry(now, "m", "same", 2_000_000, 0, 0, 0),
	}
	s := sessionCosts(entries, overrides)
	if s.Count != 1 || math.Abs(s.CostMedianUSD-3.0) > 1e-9 || math.Abs(s.CostP90USD-3.0) > 1e-9 {
		t.Errorf("one session of 3 USD expected, got %+v", s)
	}
}

func TestMedianAndPercentile_Edges(t *testing.T) {
	if got := median([]float64{7}); got != 7 {
		t.Errorf("median of one = %f, want 7", got)
	}
	if got := median([]float64{1, 3}); got != 2 {
		t.Errorf("median of two = %f, want 2", got)
	}
	if got := median([]float64{1, 2, 10}); got != 2 {
		t.Errorf("median of three = %f, want 2", got)
	}
	if got := percentile([]float64{5}, 0.9); got != 5 {
		t.Errorf("p90 of one = %f, want 5", got)
	}
	if got := percentile([]float64{1, 2}, 0.9); got != 2 {
		t.Errorf("p90 of two = %f, want 2 (rank ceil(1.8)=2)", got)
	}
	if got := percentile([]float64{1, 2, 3, 4, 5}, 0.9); got != 5 {
		t.Errorf("p90 of five = %f, want 5 (rank ceil(4.5)=5)", got)
	}
}

func TestReport_JSONShape(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	r := Build(nil, period(now.Add(-24*time.Hour), now), now, "test", nil)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema", "version", "generated_at", "window", "cost_basis", "pricing", "totals", "models", "sessions"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}
	if _, ok := got["unpriced_models"]; ok {
		t.Error("unpriced_models should be omitted when empty")
	}
	for _, forbidden := range []string{"project", "projects", "session_id", "path"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("identifying key %q must not be present", forbidden)
		}
	}
}
