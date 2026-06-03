package reader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUsageEntry_TotalTokens(t *testing.T) {
	e := UsageEntry{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 200,
		CacheReadTokens:     100,
	}
	if got := e.TotalTokens(); got != 1800 {
		t.Errorf("TotalTokens = %d, want 1800", got)
	}
}

func TestUsageEntry_DedupKey(t *testing.T) {
	e := UsageEntry{MessageID: "msg-1", RequestID: "req-1"}
	if got := e.DedupKey(); got != "msg-1:req-1" {
		t.Errorf("DedupKey = %q, want msg-1:req-1", got)
	}
}

func TestProjectsPath_Default(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	path, err := ProjectsPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path should be absolute, got %q", path)
	}
	if filepath.Base(path) != "projects" {
		t.Errorf("expected 'projects', got %q", filepath.Base(path))
	}
}

func TestProjectsPath_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/test-claude")
	path, err := ProjectsPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/tmp/test-claude/projects"
	if path != want {
		t.Errorf("got %q, want %q", path, want)
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{10000, "10.0K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.count), func(t *testing.T) {
			got := FormatTokens(tt.count)
			if got != tt.want {
				t.Errorf("FormatTokens(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestLoadEntries_NonexistentDir(t *testing.T) {
	entries, err := LoadEntries("/nonexistent/dir", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestLoadEntries_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestLoadEntries_ValidJSONL(t *testing.T) {
	dir := createTestProjectsDir(t)
	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Should be sorted by timestamp
	if !entries[0].Timestamp.Before(entries[1].Timestamp) {
		t.Error("entries should be sorted by timestamp")
	}

	// Check fields
	e := entries[0]
	if e.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want claude-sonnet-4-20250514", e.Model)
	}
	if e.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", e.InputTokens)
	}
	if e.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", e.OutputTokens)
	}
	if e.SessionID != "session-abc" {
		t.Errorf("SessionID = %q, want session-abc", e.SessionID)
	}
	if e.Project != "my-project" {
		t.Errorf("Project = %q, want my-project", e.Project)
	}
}

func TestLoadEntries_SkipsNonAssistant(t *testing.T) {
	dir := createTestProjectsDir(t)
	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The test data has 2 assistant messages and 1 human message
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (human skipped), got %d", len(entries))
	}
}

func TestLoadEntries_Deduplication(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	now := time.Now().UTC()
	// Two messages with same UUID:RequestID
	jsonl := fmt.Sprintf(
		`{"type":"assistant","timestamp":"%s","uuid":"msg-dup","requestId":"req-dup","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"assistant","timestamp":"%s","uuid":"msg-dup","requestId":"req-dup","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
`,
		now.Format(time.RFC3339Nano), now.Add(1*time.Second).Format(time.RFC3339Nano),
	)
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (deduplicated), got %d", len(entries))
	}
}

func TestLoadEntries_SinceUntilFilter(t *testing.T) {
	dir := createTestProjectsDir(t)
	now := time.Now()
	since := now.Add(-90 * time.Minute)
	until := now.Add(-45 * time.Minute)

	entries, err := LoadEntries(dir, &since, &until)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the entry at now-1h should match (now-90m to now-45m window)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry in time window, got %d", len(entries))
	}
}

func TestLoadEntries_SkipsZeroTokens(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	now := time.Now().UTC()
	jsonl := fmt.Sprintf(
		`{"type":"assistant","timestamp":"%s","uuid":"msg-0","requestId":"req-0","message":{"model":"claude-sonnet-4","usage":{"input_tokens":0,"output_tokens":0}}}
`,
		now.Format(time.RFC3339Nano),
	)
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (zero tokens), got %d", len(entries))
	}
}

func TestLoadEntries_EpochMillisTimestamp(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	// Use epoch millis timestamp
	ms := time.Now().UTC().UnixMilli()
	jsonl := fmt.Sprintf(
		`{"type":"assistant","timestamp":%d,"uuid":"msg-e","requestId":"req-e","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
`, ms)
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestLoadEntries_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	jsonl := "this is not json\n{also bad\n"
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for invalid JSON, got %d", len(entries))
	}
}

func TestLoadEntries_DefaultPath(t *testing.T) {
	// When projectsPath is empty, it should use ProjectsPath()
	// Just verify no panic — result depends on ~/.claude/projects existing
	_, _ = LoadEntries("", nil, nil)
}

func TestLoadEntries_EmptyLines(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	now := time.Now().UTC()
	// Mix of empty lines and valid data
	jsonl := fmt.Sprintf("\n\n"+
		`{"type":"assistant","timestamp":"%s","uuid":"msg-1","requestId":"req-1","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}`+
		"\n\n",
		now.Format(time.RFC3339Nano),
	)
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (empty lines skipped), got %d", len(entries))
	}
}

func TestLoadEntries_ZeroTimestamp(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	// Valid assistant message but with unparseable timestamp
	jsonl := `{"type":"assistant","timestamp":"not-a-date","uuid":"msg-1","requestId":"req-1","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}` + "\n"
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (zero timestamp skipped), got %d", len(entries))
	}
}

func TestLoadEntries_EmptyDedupKey(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	now := time.Now().UTC()
	// Two messages with empty UUID and RequestID — key is ":", should not deduplicate
	jsonl := fmt.Sprintf(
		`{"type":"assistant","timestamp":"%s","uuid":"","requestId":"","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"assistant","timestamp":"%s","uuid":"","requestId":"","message":{"model":"claude-sonnet-4","usage":{"input_tokens":200,"output_tokens":100}}}
`,
		now.Format(time.RFC3339Nano), now.Add(1*time.Second).Format(time.RFC3339Nano),
	)
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (empty dedup key, no dedup), got %d", len(entries))
	}
}

func TestLoadEntries_ScannerError(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	os.MkdirAll(projectDir, 0o755)

	now := time.Now().UTC()
	// Create a line longer than the 10MB max buffer to trigger scanner error
	// Then follow with a valid entry that should be in the parsed entries before the error
	validLine := fmt.Sprintf(
		`{"type":"assistant","timestamp":"%s","uuid":"msg-ok","requestId":"req-ok","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}`,
		now.Format(time.RFC3339Nano),
	)
	// 11MB line — exceeds scanner's 10MB max
	longLine := strings.Repeat("x", 11*1024*1024)
	jsonl := validLine + "\n" + longLine + "\n"
	os.WriteFile(filepath.Join(projectDir, "session.jsonl"), []byte(jsonl), 0o644)

	entries, err := LoadEntries(dir, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have parsed the valid entry before hitting the scanner error
	if len(entries) != 1 {
		t.Errorf("expected 1 entry before scanner error, got %d", len(entries))
	}
}

// --- parseTimestamp ---

func TestParseTimestamp_Empty(t *testing.T) {
	got := parseTimestamp(nil)
	if !got.IsZero() {
		t.Errorf("expected zero time for nil input, got %v", got)
	}

	got = parseTimestamp([]byte{})
	if !got.IsZero() {
		t.Errorf("expected zero time for empty input, got %v", got)
	}
}

func TestParseTimestamp_RFC3339Nano(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 123456789, time.UTC)
	raw := []byte(`"` + ts.Format(time.RFC3339Nano) + `"`)
	got := parseTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Sub(ts).Abs() > time.Millisecond {
		t.Errorf("got %v, want ~%v", got, ts)
	}
}

func TestParseTimestamp_RFC3339(t *testing.T) {
	raw := []byte(`"2025-06-15T10:30:00Z"`)
	got := parseTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for RFC3339")
	}
	want := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTimestamp_CustomLayout(t *testing.T) {
	// "2006-01-02T15:04:05.000-07:00"
	raw := []byte(`"2025-06-15T10:30:00.000+02:00"`)
	got := parseTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for custom layout")
	}
	// Should be converted to UTC: 10:30 +02:00 = 08:30 UTC
	if got.Hour() != 8 || got.Minute() != 30 {
		t.Errorf("got %v, expected 08:30 UTC", got)
	}
}

func TestParseTimestamp_UnparsableString(t *testing.T) {
	raw := []byte(`"not-a-timestamp"`)
	got := parseTimestamp(raw)
	if !got.IsZero() {
		t.Errorf("expected zero time for unparsable string, got %v", got)
	}
}

func TestParseTimestamp_EpochMillis(t *testing.T) {
	// 2025-01-01T00:00:00Z = 1735689600000 ms
	raw := []byte(`1735689600000`)
	got := parseTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for epoch millis")
	}
	want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTimestamp_NumericString(t *testing.T) {
	// Epoch millis as a quoted number (some JSONL variants)
	raw := []byte(`"1735689600000"`)
	// This is a string that doesn't match any ISO layout, then falls through
	// to the numeric string path
	got := parseTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for numeric string")
	}
	want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTimestamp_GarbageBytes(t *testing.T) {
	raw := []byte(`[1,2,3]`)
	got := parseTimestamp(raw)
	if !got.IsZero() {
		t.Errorf("expected zero time for array input, got %v", got)
	}
}

// --- helpers ---

func createTestProjectsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "my-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Now().UTC()
	jsonl := ""
	for i, ts := range []time.Time{now.Add(-1 * time.Hour), now.Add(-30 * time.Minute)} {
		line := fmt.Sprintf(
			`{"type":"assistant","timestamp":"%s","uuid":"msg-%d","requestId":"req-%d","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":200,"cache_read_input_tokens":100}}}`,
			ts.Format(time.RFC3339Nano), i, i,
		)
		jsonl += line + "\n"
	}
	// Non-assistant message
	jsonl += `{"type":"human","timestamp":"` + now.Format(time.RFC3339Nano) + `","message":{}}` + "\n"

	if err := os.WriteFile(filepath.Join(projectDir, "session-abc.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}
