package stats

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sparkfabrik/claude-usage/internal/reader"
)

func entry(ts time.Time, model string, tokens int) reader.UsageEntry {
	return reader.UsageEntry{
		Timestamp:   ts,
		Model:       model,
		InputTokens: tokens,
		SessionID:   "s1",
		MessageID:   ts.Format(time.RFC3339Nano) + model,
	}
}

func TestComputeRecentDaysAlwaysSpansSevenDays(t *testing.T) {
	now := time.Date(2026, 8, 25, 15, 0, 0, 0, time.Local)
	s := Compute([]reader.UsageEntry{entry(now, "claude-opus-4-8", 100)}, now, nil)

	if len(s.RecentDays) != 7 {
		t.Fatalf("got %d days, want 7", len(s.RecentDays))
	}
	if !s.RecentDays[6].Today {
		t.Error("the last day should be marked as today")
	}
	if s.RecentDays[0].Today {
		t.Error("the first day should not be today")
	}
	if s.RecentDays[6].Tokens != 100 {
		t.Errorf("today's tokens = %d, want 100", s.RecentDays[6].Tokens)
	}
	// Days with no activity still appear, so a reader renders a fixed chart.
	if s.RecentDays[0].Tokens != 0 {
		t.Errorf("idle day tokens = %d, want 0", s.RecentDays[0].Tokens)
	}
}

// An entry made in the local evening belongs to today, even though its UTC
// timestamp has already rolled into tomorrow. Grouping by UTC would file the
// evening's work under the following day.
func TestComputeAttributesDaysLocally(t *testing.T) {
	loc := time.FixedZone("UTC-5", -5*60*60)
	now := time.Date(2026, 8, 25, 22, 0, 0, 0, loc)
	// 21:00 at UTC-5 is 02:00 UTC on 26 August: tomorrow in UTC, today here.
	evening := time.Date(2026, 8, 25, 21, 0, 0, 0, loc)
	if evening.UTC().Day() != 26 {
		t.Fatalf("test premise broken: UTC day is %d", evening.UTC().Day())
	}

	s := Compute([]reader.UsageEntry{entry(evening, "claude-opus-4-8", 50)}, now, nil)

	today := s.RecentDays[len(s.RecentDays)-1]
	if !today.Today {
		t.Fatal("the last day should be today")
	}
	if today.Tokens != 50 {
		t.Errorf("today tokens = %d, want 50: the entry was filed under another day", today.Tokens)
	}
}

func TestComputeModelsAreOrderedAndNamed(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	entries := []reader.UsageEntry{
		entry(now, "claude-haiku-4-5-20251001", 10),
		entry(now.Add(time.Second), "claude-opus-4-8", 900),
		entry(now.Add(2*time.Second), "claude-fable-5", 500),
	}

	s := Compute(entries, now, nil)
	if len(s.Models) != 3 {
		t.Fatalf("got %d models, want 3", len(s.Models))
	}
	if s.Models[0].Name != "Opus 4.8" {
		t.Errorf("heaviest model = %q, want %q", s.Models[0].Name, "Opus 4.8")
	}
	if s.Models[1].Name != "Fable 5" {
		t.Errorf("second model = %q, want %q", s.Models[1].Name, "Fable 5")
	}
	if s.Models[2].Name != "Haiku 4.5" {
		t.Errorf("third model = %q, want %q", s.Models[2].Name, "Haiku 4.5")
	}
}

func TestComputeToday(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1)

	s := Compute([]reader.UsageEntry{
		entry(now, "claude-opus-4-8", 100),
		entry(yesterday, "claude-opus-4-8", 999),
	}, now, nil)

	if s.Today.Tokens != 100 {
		t.Errorf("today tokens = %d, want 100", s.Today.Tokens)
	}
	if s.Today.Messages != 1 {
		t.Errorf("today messages = %d, want 1", s.Today.Messages)
	}
}

func TestFriendlyModelName(t *testing.T) {
	tests := map[string]string{
		"claude-opus-4-8":           "Opus 4.8",
		"claude-opus-4-8-20260115":  "Opus 4.8",
		"claude-fable-5":            "Fable 5",
		"claude-haiku-4-5-20251001": "Haiku 4.5",
		"claude-sonnet-4-5":         "Sonnet 4.5",
		"gpt-5":                     "GPT 5",
		"":                          "Unknown",
		"claude-":                   "Unknown",
	}

	for id, want := range tests {
		if got := FriendlyModelName(id); got != want {
			t.Errorf("FriendlyModelName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestIsFresh(t *testing.T) {
	fresh := &Stats{ComputedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if !fresh.IsFresh(60) {
		t.Error("a just-computed aggregation should be fresh")
	}

	old := &Stats{ComputedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)}
	if old.IsFresh(60) {
		t.Error("a two-hour-old aggregation should not be fresh")
	}

	var missing *Stats
	if missing.IsFresh(60) {
		t.Error("a missing aggregation is never fresh")
	}

	broken := &Stats{ComputedAt: "not a time"}
	if broken.IsFresh(60) {
		t.Error("an unparseable timestamp must not read as fresh")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	want := Compute([]reader.UsageEntry{entry(now, "claude-opus-4-8", 42)}, now, nil)

	if err := Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := Read(path)
	if got == nil {
		t.Fatal("Read returned nil")
	}
	if got.Today.Tokens != want.Today.Tokens {
		t.Errorf("today tokens = %d, want %d", got.Today.Tokens, want.Today.Tokens)
	}
	if len(got.RecentDays) != len(want.RecentDays) {
		t.Errorf("days = %d, want %d", len(got.RecentDays), len(want.RecentDays))
	}
}

func TestReadMissingFile(t *testing.T) {
	if s := Read(filepath.Join(t.TempDir(), "absent.json")); s != nil {
		t.Error("Read should return nil for a missing file")
	}
}

func TestDefaultPathSitsBesideTheQuotaCache(t *testing.T) {
	got := DefaultPath("/home/u/.cache/claude-code-usage/quota.json")
	want := "/home/u/.cache/claude-code-usage/stats.json"
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

// A fresh cache must be reused rather than triggering a rescan: the scan walks
// the whole transcript history, and readers poll every minute.
func TestLoadReusesFreshCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	cached := &Stats{
		ComputedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Today:      Today{Tokens: 12345},
	}
	if err := Write(path, cached); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// An unreadable projects path would fail a real scan.
	got, err := Load(path, filepath.Join(t.TempDir(), "absent"), 900, false, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Today.Tokens != 12345 {
		t.Errorf("tokens = %d, want the cached 12345", got.Today.Tokens)
	}
}

func TestLoadForceBypassesCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	if err := Write(path, &Stats{
		ComputedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Today:      Today{Tokens: 12345},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The projects dir is absent, so the rescan finds nothing and the stale
	// figure is replaced rather than reused.
	got, err := Load(path, filepath.Join(dir, "absent"), 900, true, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Today.Tokens != 0 {
		t.Errorf("tokens = %d, want 0 after a forced rescan", got.Today.Tokens)
	}
}
