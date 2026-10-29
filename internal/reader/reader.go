// Package reader parses JSONL session files from Claude Code's local storage.
package reader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// UsageEntry is a single usage record from a JSONL assistant message.
type UsageEntry struct {
	Timestamp           time.Time
	Model               string
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	// CacheWrite1hTokens is the part of CacheCreationTokens written with the
	// 1-hour TTL, from usage.cache_creation.ephemeral_1h_input_tokens; 0 when
	// the transcript carries no breakdown.
	CacheWrite1hTokens int
	CacheReadTokens    int
	// MessageID is the assistant message id from message.id. It is stable
	// across resumed and forked sessions, where the same message reappears
	// under a new uuid.
	MessageID string
	// EntryUUID is the transcript line uuid, used only when no message id
	// is present.
	EntryUUID string
	RequestID string
	// SessionID is the session the entry belongs to: the transcript's own
	// sessionId field, which a subagent transcript shares with the session
	// that spawned it, or the file name when the field is absent.
	SessionID string
	Project   string
}

// TotalTokens returns the sum of all token types.
func (e *UsageEntry) TotalTokens() int {
	return e.InputTokens + e.OutputTokens + e.CacheCreationTokens + e.CacheReadTokens
}

// DedupKey returns a unique key for deduplication. The assistant message id
// is preferred: it survives a session being resumed or forked, where the same
// message is written again under a fresh uuid and would otherwise be counted
// twice. The uuid and request id are the fallback for transcripts that carry
// no message id.
func (e *UsageEntry) DedupKey() string {
	if e.MessageID != "" {
		return e.MessageID
	}
	return e.EntryUUID + ":" + e.RequestID
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
	SessionID string          `json:"sessionId"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreation            struct {
				Ephemeral1h int `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
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
		// Claude Code writes "<synthetic>" for messages it produces itself,
		// not the API; they are not usage and have no price.
		if rec.Message.Model == "" || rec.Message.Model == "<synthetic>" {
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

		// A subagent transcript lives under <session>/subagents/ and names
		// its parent in sessionId, so its requests count toward that session
		// instead of becoming a session of their own.
		session := rec.SessionID
		if session == "" {
			session = sessionID
		}

		// The TTL breakdown never exceeds the total it splits; clamp so a
		// malformed line cannot price more 1-hour tokens than were written.
		write1h := usage.CacheCreation.Ephemeral1h
		if write1h > usage.CacheCreationInputTokens {
			write1h = usage.CacheCreationInputTokens
		}
		if write1h < 0 {
			write1h = 0
		}

		entry := UsageEntry{
			Timestamp:           ts,
			Model:               rec.Message.Model,
			InputTokens:         usage.InputTokens,
			OutputTokens:        usage.OutputTokens,
			CacheCreationTokens: usage.CacheCreationInputTokens,
			CacheWrite1hTokens:  write1h,
			CacheReadTokens:     usage.CacheReadInputTokens,
			MessageID:           rec.Message.ID,
			EntryUUID:           rec.UUID,
			RequestID:           rec.RequestID,
			SessionID:           session,
			Project:             project,
		}

		key := entry.DedupKey()
		if key != ":" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}

		*entries = append(*entries, entry)
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

	// Try string (ISO 8601, then numeric string for epoch millis)
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
		// String didn't match any ISO layout — try as numeric epoch millis
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return epochMillisToTime(f)
		}
		return time.Time{}
	}

	// Try number (epoch milliseconds)
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return epochMillisToTime(n)
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
