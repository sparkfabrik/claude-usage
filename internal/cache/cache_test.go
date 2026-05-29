package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("path should not be empty")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path should be absolute, got %q", path)
	}
	if filepath.Base(path) != "quota.json" {
		t.Errorf("expected quota.json, got %q", filepath.Base(path))
	}
}

func TestDefaultPath_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/test-xdg-cache")
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/tmp/test-xdg-cache/claude-code-usage/quota.json"
	if path != want {
		t.Errorf("got %q, want %q", path, want)
	}
}

func TestPolledAtTime_Valid(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	c := &QuotaCache{PolledAt: now.Format(time.RFC3339Nano)}
	got := c.PolledAtTime()
	if got.Sub(now).Abs() > time.Millisecond {
		t.Errorf("PolledAtTime = %v, want ~%v", got, now)
	}
}

func TestPolledAtTime_Invalid(t *testing.T) {
	c := &QuotaCache{PolledAt: "not-a-time"}
	got := c.PolledAtTime()
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid PolledAt, got %v", got)
	}
}

func TestAgeSeconds(t *testing.T) {
	c := &QuotaCache{PolledAt: time.Now().Add(-10 * time.Second).Format(time.RFC3339Nano)}
	age := c.AgeSeconds()
	if age < 9 || age > 12 {
		t.Errorf("AgeSeconds = %f, expected ~10", age)
	}
}

func TestIsFresh(t *testing.T) {
	tests := []struct {
		name     string
		age      time.Duration
		thresh   int
		wantFresh bool
	}{
		{"fresh", -5 * time.Second, 60, true},
		{"stale", -120 * time.Second, 60, false},
		{"boundary", -60 * time.Second, 60, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &QuotaCache{PolledAt: time.Now().Add(tt.age).Format(time.RFC3339Nano)}
			if got := c.IsFresh(tt.thresh); got != tt.wantFresh {
				t.Errorf("IsFresh(%d) = %v, want %v", tt.thresh, got, tt.wantFresh)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	fresh := &QuotaCache{PolledAt: time.Now().Format(time.RFC3339Nano)}
	if fresh.IsStale() {
		t.Error("freshly polled data should not be stale")
	}

	stale := &QuotaCache{PolledAt: time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano)}
	if !stale.IsStale() {
		t.Error("data polled 5min ago should be stale")
	}
}

func TestResetTime5h(t *testing.T) {
	// nil when empty
	c := &QuotaCache{}
	if c.ResetTime5h() != nil {
		t.Error("expected nil for empty Reset5h")
	}

	// valid time
	future := time.Now().Add(1 * time.Hour).UTC()
	c.Reset5h = future.Format(time.RFC3339Nano)
	got := c.ResetTime5h()
	if got == nil {
		t.Fatal("expected non-nil ResetTime5h")
	}
	if got.Sub(future).Abs() > time.Millisecond {
		t.Errorf("ResetTime5h = %v, want ~%v", got, future)
	}
}

func TestResetTime7d(t *testing.T) {
	c := &QuotaCache{}
	if c.ResetTime7d() != nil {
		t.Error("expected nil for empty Reset7d")
	}

	future := time.Now().Add(24 * time.Hour).UTC()
	c.Reset7d = future.Format(time.RFC3339Nano)
	got := c.ResetTime7d()
	if got == nil {
		t.Fatal("expected non-nil ResetTime7d")
	}
}

func TestMinutesToReset5h(t *testing.T) {
	c := &QuotaCache{}
	if c.MinutesToReset5h() != nil {
		t.Error("expected nil when no reset time")
	}

	future := time.Now().Add(90 * time.Minute).UTC()
	c.Reset5h = future.Format(time.RFC3339Nano)
	mins := c.MinutesToReset5h()
	if mins == nil {
		t.Fatal("expected non-nil minutes")
	}
	if *mins < 85 || *mins > 95 {
		t.Errorf("MinutesToReset5h = %f, want ~90", *mins)
	}
}

func TestMinutesToReset_PastTime(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).UTC()
	c := &QuotaCache{Reset5h: past.Format(time.RFC3339Nano)}
	mins := c.MinutesToReset5h()
	if mins == nil {
		t.Fatal("expected non-nil")
	}
	if *mins != 0 {
		t.Errorf("past reset should clamp to 0, got %f", *mins)
	}
}

func TestMinutesToReset7d(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC()
	c := &QuotaCache{Reset7d: future.Format(time.RFC3339Nano)}
	mins := c.MinutesToReset7d()
	if mins == nil {
		t.Fatal("expected non-nil")
	}
	if *mins < 2800 || *mins > 2900 {
		t.Errorf("MinutesToReset7d = %f, want ~2880", *mins)
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "quota.json")

	q := &QuotaCache{
		Utilization5h: 0.42,
		Utilization7d: 0.67,
		PolledAt:      time.Now().UTC().Format(time.RFC3339Nano),
		Reset5h:       time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339Nano),
		Status5h:      "active",
	}

	if err := Write(path, q); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify directory permissions
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perms = %o, want 0700", perm)
	}

	got := Read(path)
	if got == nil {
		t.Fatal("Read returned nil")
	}
	if got.Utilization5h != 0.42 {
		t.Errorf("Utilization5h = %f, want 0.42", got.Utilization5h)
	}
	if got.Utilization7d != 0.67 {
		t.Errorf("Utilization7d = %f, want 0.67", got.Utilization7d)
	}
	if got.Status5h != "active" {
		t.Errorf("Status5h = %q, want %q", got.Status5h, "active")
	}
}

func TestRead_NonexistentFile(t *testing.T) {
	got := Read("/nonexistent/path/quota.json")
	if got != nil {
		t.Errorf("expected nil for nonexistent file, got %+v", got)
	}
}

func TestRead_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)
	got := Read(path)
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", got)
	}
}

func TestWrite_ValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quota.json")
	q := &QuotaCache{Utilization5h: 0.5, PolledAt: time.Now().Format(time.RFC3339Nano)}
	if err := Write(path, q); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var parsed QuotaCache
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if parsed.Utilization5h != 0.5 {
		t.Errorf("got %f, want 0.5", parsed.Utilization5h)
	}
}
