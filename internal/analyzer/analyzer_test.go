package analyzer

import (
	"testing"
	"time"

	"github.com/Monska85/claude-usage/internal/reader"
)

func makeEntry(ts time.Time, model string, input, output, cacheW, cacheR int, session string) reader.UsageEntry {
	return reader.UsageEntry{
		Timestamp:           ts,
		Model:               model,
		InputTokens:         input,
		OutputTokens:        output,
		CacheCreationTokens: cacheW,
		CacheReadTokens:     cacheR,
		SessionID:           session,
	}
}

func TestSummary_TotalTokens(t *testing.T) {
	s := &Summary{
		TotalInputTokens:      1000,
		TotalOutputTokens:     500,
		TotalCacheWriteTokens: 200,
		TotalCacheReadTokens:  100,
	}
	if got := s.TotalTokens(); got != 1800 {
		t.Errorf("TotalTokens = %d, want 1800", got)
	}
}

func TestBurnRatePerMinute_NilTimes(t *testing.T) {
	s := &Summary{}
	if got := s.BurnRatePerMinute(); got != -1 {
		t.Errorf("BurnRatePerMinute = %f, want -1", got)
	}
}

func TestBurnRatePerMinute_ZeroDuration(t *testing.T) {
	now := time.Now()
	s := &Summary{
		TotalInputTokens: 1000,
		StartTime:        &now,
		EndTime:          &now,
	}
	if got := s.BurnRatePerMinute(); got != -1 {
		t.Errorf("BurnRatePerMinute = %f, want -1 for zero duration", got)
	}
}

func TestBurnRatePerMinute_Valid(t *testing.T) {
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now()
	s := &Summary{
		TotalInputTokens:  1000,
		TotalOutputTokens: 500,
		StartTime:         &start,
		EndTime:           &end,
	}
	rate := s.BurnRatePerMinute()
	if rate < 140 || rate > 160 {
		t.Errorf("BurnRatePerMinute = %f, want ~150", rate)
	}
}

func TestAggregate_EmptyEntries(t *testing.T) {
	s := Aggregate(nil, "test", nil)
	if s.PeriodLabel != "test" {
		t.Errorf("PeriodLabel = %q, want test", s.PeriodLabel)
	}
	if s.TotalTokens() != 0 {
		t.Errorf("TotalTokens = %d, want 0", s.TotalTokens())
	}
	if s.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0", s.MessageCount)
	}
}

func TestAggregate_WithEntries(t *testing.T) {
	now := time.Now()
	entries := []reader.UsageEntry{
		makeEntry(now.Add(-2*time.Hour), "claude-sonnet-4", 1000, 500, 200, 100, "s1"),
		makeEntry(now.Add(-1*time.Hour), "claude-sonnet-4", 2000, 800, 300, 150, "s1"),
		makeEntry(now, "claude-haiku-4-5", 500, 200, 0, 0, "s2"),
	}

	s := Aggregate(entries, "today", nil)

	if s.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3", s.MessageCount)
	}
	if s.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", s.SessionCount)
	}
	if s.TotalInputTokens != 3500 {
		t.Errorf("TotalInputTokens = %d, want 3500", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 1500 {
		t.Errorf("TotalOutputTokens = %d, want 1500", s.TotalOutputTokens)
	}
	if s.TotalCacheWriteTokens != 500 {
		t.Errorf("TotalCacheWriteTokens = %d, want 500", s.TotalCacheWriteTokens)
	}
	if s.TotalCacheReadTokens != 250 {
		t.Errorf("TotalCacheReadTokens = %d, want 250", s.TotalCacheReadTokens)
	}
	if len(s.ModelsUsed) != 2 {
		t.Errorf("ModelsUsed = %d models, want 2", len(s.ModelsUsed))
	}
	if s.StartTime == nil || s.EndTime == nil {
		t.Fatal("StartTime/EndTime should not be nil")
	}
	if s.TotalCostUSD <= 0 {
		t.Errorf("TotalCostUSD = %f, should be > 0", s.TotalCostUSD)
	}
}

func TestStandardPeriods(t *testing.T) {
	periods := StandardPeriods()
	if len(periods) != 4 {
		t.Fatalf("expected 4 periods, got %d", len(periods))
	}

	labels := map[string]bool{}
	for _, p := range periods {
		labels[p.Label] = true
	}
	for _, want := range []string{"today", "7d", "30d", "all"} {
		if !labels[want] {
			t.Errorf("missing period %q", want)
		}
	}
}

func TestFindPeriod_Valid(t *testing.T) {
	for _, label := range []string{"today", "7d", "30d", "all"} {
		p, ok := FindPeriod(label)
		if !ok {
			t.Errorf("FindPeriod(%q) returned false", label)
		}
		if p.Label != label {
			t.Errorf("Label = %q, want %q", p.Label, label)
		}
	}
}

func TestFindPeriod_Invalid(t *testing.T) {
	_, ok := FindPeriod("bogus")
	if ok {
		t.Error("FindPeriod(bogus) should return false")
	}
}

func TestFilterEntries(t *testing.T) {
	now := time.Now()
	entries := []reader.UsageEntry{
		makeEntry(now.Add(-48*time.Hour), "m", 100, 50, 0, 0, "s"),
		makeEntry(now.Add(-12*time.Hour), "m", 200, 100, 0, 0, "s"),
		makeEntry(now.Add(-1*time.Hour), "m", 300, 150, 0, 0, "s"),
	}

	start := now.Add(-24 * time.Hour)
	filtered := FilterEntries(entries, start, now)
	if len(filtered) != 2 {
		t.Errorf("expected 2 entries, got %d", len(filtered))
	}
}

func TestFilterEntries_AfterEnd(t *testing.T) {
	now := time.Now()
	entries := []reader.UsageEntry{
		makeEntry(now.Add(-2*time.Hour), "m", 100, 50, 0, 0, "s"),
		makeEntry(now.Add(1*time.Hour), "m", 200, 100, 0, 0, "s"), // future, after end
	}

	filtered := FilterEntries(entries, time.Time{}, now)
	if len(filtered) != 1 {
		t.Errorf("expected 1 entry (future filtered out), got %d", len(filtered))
	}
}

func TestFilterEntries_AllPeriod(t *testing.T) {
	now := time.Now()
	entries := []reader.UsageEntry{
		makeEntry(now.Add(-720*time.Hour), "m", 100, 50, 0, 0, "s"),
		makeEntry(now, "m", 200, 100, 0, 0, "s"),
	}

	filtered := FilterEntries(entries, time.Time{}, now)
	if len(filtered) != 2 {
		t.Errorf("expected 2 entries with zero start, got %d", len(filtered))
	}
}

func TestSummarizeByPeriods(t *testing.T) {
	now := time.Now().UTC()
	entries := []reader.UsageEntry{
		makeEntry(now.Add(-1*time.Hour), "claude-sonnet-4", 1000, 500, 0, 0, "s1"),
	}

	summaries := SummarizeByPeriods(entries, []string{"today", "7d"}, nil)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].PeriodLabel != "today" {
		t.Errorf("first summary = %q, want today", summaries[0].PeriodLabel)
	}
	if summaries[1].PeriodLabel != "7d" {
		t.Errorf("second summary = %q, want 7d", summaries[1].PeriodLabel)
	}
}

func TestSummarizeByModel(t *testing.T) {
	now := time.Now()
	entries := []reader.UsageEntry{
		makeEntry(now, "claude-sonnet-4", 1000, 500, 0, 0, "s1"),
		makeEntry(now, "claude-sonnet-4", 2000, 800, 0, 0, "s1"),
		makeEntry(now, "claude-haiku-4-5", 500, 200, 0, 0, "s1"),
	}

	summaries := SummarizeByModel(entries, nil)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 model summaries, got %d", len(summaries))
	}

	// Find sonnet summary
	var sonnet *Summary
	for _, s := range summaries {
		if s.PeriodLabel == "claude-sonnet-4" {
			sonnet = s
		}
	}
	if sonnet == nil {
		t.Fatal("expected claude-sonnet-4 summary")
	}
	if sonnet.MessageCount != 2 {
		t.Errorf("sonnet MessageCount = %d, want 2", sonnet.MessageCount)
	}
}
