// Package reader parses JSONL session files from Claude Code's local storage.
package reader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// UsageEntry is a single usage record from a JSONL assistant message.
type UsageEntry struct {
	Timestamp              time.Time
	Model                  string
	InputTokens            int
	OutputTokens           int
	CacheCreationTokens    int
	CacheReadTokens        int
	MessageID              string
	RequestID              string
	SessionID              string
	Project                string
}

// TotalTokens returns the sum of all token types.
func (e *UsageEntry) TotalTokens() int {
	return e.InputTokens + e.OutputTokens + e.CacheCreationTokens + e.CacheReadTokens
}

// DedupKey returns a unique key for deduplication.
func (e *UsageEntry) DedupKey() string {
	return e.MessageID + ":" + e.RequestID
}

// ProjectsPath returns the default projects directory.
func ProjectsPath() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "projects"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// LoadEntries loads and deduplicates usage entries from all JSONL files.
func LoadEntries(projectsPath string, since, until *time.Time) ([]UsageEntry, error) {
	if projectsPath == "" {
		p, err := ProjectsPath()
		if err != nil {
			return nil, err
		}
		projectsPath = p
	}

	if _, err := os.Stat(projectsPath); err != nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var entries []UsageEntry

	if err := filepath.WalkDir(projectsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")
		project := filepath.Base(filepath.Dir(path))
		parseFile(path, sessionID, project, since, until, seen, &entries)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walking projects directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}

// jsonlRecord is the raw JSONL structure.
type jsonlRecord struct {
	Type      string          `json:"type"`
	Timestamp json.RawMessage `json:"timestamp"`
	UUID      string          `json:"uuid"`
	RequestID string          `json:"requestId"`
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func parseFile(path, sessionID, project string, since, until *time.Time, seen map[string]bool, entries *[]UsageEntry) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 128*1024), 10*1024*1024) // 128KB initial, 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec jsonlRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.Type != "assistant" {
			continue
		}
		usage := rec.Message.Usage
		if usage.InputTokens == 0 && usage.OutputTokens == 0 {
			continue
		}

		ts := parseTimestamp(rec.Timestamp)
		if ts.IsZero() {
			continue
		}

		if since != nil && ts.Before(*since) {
			continue
		}
		if until != nil && ts.After(*until) {
			continue
		}

		key := rec.UUID + ":" + rec.RequestID
		if key != ":" && seen[key] {
			continue
		}
		if key != ":" {
			seen[key] = true
		}

		*entries = append(*entries, UsageEntry{
			Timestamp:           ts,
			Model:               rec.Message.Model,
			InputTokens:         usage.InputTokens,
			OutputTokens:        usage.OutputTokens,
			CacheCreationTokens: usage.CacheCreationInputTokens,
			CacheReadTokens:     usage.CacheReadInputTokens,
			MessageID:           rec.UUID,
			RequestID:           rec.RequestID,
			SessionID:           sessionID,
			Project:             project,
		})
	}
	// Check for scanner errors (e.g. line too long, I/O error).
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error reading %s: %v\n", path, err)
	}
}

func parseTimestamp(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}

	// Try string (ISO 8601)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.000-07:00",
		} {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC()
			}
		}
		return time.Time{}
	}

	// Try number (epoch milliseconds)
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return epochMillisToTime(n)
	}

	// Try numeric string
	str := strings.Trim(string(raw), "\"")
	if f, err := strconv.ParseFloat(str, 64); err == nil {
		return epochMillisToTime(f)
	}

	return time.Time{}
}

// epochMillisToTime converts epoch milliseconds to time.Time.
func epochMillisToTime(ms float64) time.Time {
	sec := int64(ms / 1000)
	nsec := (int64(ms) % 1000) * 1e6
	return time.Unix(sec, nsec).UTC()
}

// FormatTokens formats token count with K/M suffix.
func FormatTokens(count int) string {
	if count >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(count)/1e6)
	}
	if count >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(count)/1e3)
	}
	return strconv.Itoa(count)
}
