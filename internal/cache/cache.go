// Package cache provides atomic read/write for the shared quota cache file.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// defaultDisplayStaleSeconds is the threshold for IsStale() used by display/extension.
// Data older than this is shown with visual staleness indicators.
const defaultDisplayStaleSeconds = 120

// Source identifies where quota data came from.
const (
	// SourceOAuth is the dedicated usage endpoint. It costs no quota and
	// reports model-scoped windows.
	SourceOAuth = "oauth"
	// SourceHeaders is the 1-token poll that reads rate-limit response
	// headers. It only ever reports the session and weekly windows.
	SourceHeaders = "headers"
)

// Window keys for the two windows every source reports.
const (
	WindowSession = "session"
	WindowWeekly  = "weekly"
)

// Window is one rate-limit window. The session and weekly windows are always
// present; model-scoped windows (for example "Opus Weekly") appear only when
// the OAuth usage endpoint reports them.
type Window struct {
	// Key is stable and machine-readable: "session", "weekly", or a
	// model-scoped key such as "opus:weekly".
	Key string `json:"key"`
	// Title is display-ready, for example "Session (5-hour)" or "Opus Weekly".
	// It is resolved by the producer, which knows the window kind, rather than
	// parsed by consumers: a model named "Opus 5 (1M context)" would otherwise
	// read as a one-minute window.
	Title string `json:"title"`
	// Model is the display name of the scoped model, empty for flat windows.
	Model string `json:"model,omitempty"`
	// Utilization is a fraction in [0,1].
	Utilization float64 `json:"utilization"`
	// ResetsAt is RFC 3339, empty when the source does not report one.
	ResetsAt string `json:"resets_at,omitempty"`
}

// ResetTime parses ResetsAt. Returns nil when absent or malformed.
func (w Window) ResetTime() *time.Time {
	return parseOptionalTime(w.ResetsAt)
}

// MinutesToReset returns minutes until this window resets, nil when unknown.
func (w Window) MinutesToReset() *float64 {
	return minutesToReset(w.ResetTime())
}

// IsOpen reports whether the window still applies at now. A window with no
// reset time is always considered open: absent information must not silently
// drop data. This is what keeps a cached percentage usable across a failed
// refresh while preventing a stale 78% from being shown against a window that
// has already rolled over.
func (w Window) IsOpen(now time.Time) bool {
	t := w.ResetTime()
	if t == nil {
		return true
	}
	return t.After(now)
}

// QuotaCache is the JSON structure stored in the cache file.
type QuotaCache struct {
	Utilization5h float64 `json:"utilization_5h"`
	Reset5h       string  `json:"reset_5h,omitempty"` // ISO 8601
	Status5h      string  `json:"status_5h,omitempty"`
	Utilization7d float64 `json:"utilization_7d"`
	Reset7d       string  `json:"reset_7d,omitempty"` // ISO 8601
	PolledAt      string  `json:"polled_at"`          // ISO 8601

	// Windows carries every window the source reported, including
	// model-scoped ones. Older cache files predate this field and unmarshal
	// with it nil; SyncWindows rebuilds it from the flat fields.
	Windows []Window `json:"windows,omitempty"`
	// Source is SourceOAuth or SourceHeaders. Empty in caches written before
	// the OAuth endpoint was introduced.
	Source string `json:"source,omitempty"`
}

// OpenWindows returns the windows that have not yet reset, newest data first
// in the order the source reported them.
func (c *QuotaCache) OpenWindows(now time.Time) []Window {
	if c == nil {
		return nil
	}
	open := make([]Window, 0, len(c.Windows))
	for _, w := range c.Windows {
		if w.IsOpen(now) {
			open = append(open, w)
		}
	}
	return open
}

// SyncWindows fills Windows from the flat session/weekly fields when the
// producer did not populate it, so consumers can rely on Windows alone. Cache
// files written by older versions land here.
func (c *QuotaCache) SyncWindows() {
	if c == nil || len(c.Windows) > 0 {
		return
	}
	c.Windows = []Window{
		{Key: WindowSession, Title: "Session (5-hour)", Utilization: c.Utilization5h, ResetsAt: c.Reset5h},
		{Key: WindowWeekly, Title: "Weekly (7-day)", Utilization: c.Utilization7d, ResetsAt: c.Reset7d},
	}
}

// DefaultPath returns the default cache file location following XDG Base Directory spec.
// Uses $XDG_CACHE_HOME/claude-code-usage/quota.json, falling back to ~/.cache/claude-code-usage/quota.json.
func DefaultPath() (string, error) {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheDir, "claude-code-usage", "quota.json"), nil
}

// PolledAtTime parses the polled_at timestamp.
func (c *QuotaCache) PolledAtTime() time.Time {
	t, err := time.Parse(time.RFC3339Nano, c.PolledAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// AgeSeconds returns seconds since data was polled.
func (c *QuotaCache) AgeSeconds() float64 {
	return time.Since(c.PolledAtTime()).Seconds()
}

// IsFresh returns true if cache is newer than freshness threshold.
func (c *QuotaCache) IsFresh(freshnessSeconds int) bool {
	return c.AgeSeconds() < float64(freshnessSeconds)
}

// IsStale returns true if data is older than the display staleness threshold.
func (c *QuotaCache) IsStale() bool {
	return c.AgeSeconds() > defaultDisplayStaleSeconds
}

// ResetTime5h parses the 5h reset timestamp.
func (c *QuotaCache) ResetTime5h() *time.Time {
	return parseOptionalTime(c.Reset5h)
}

// ResetTime7d parses the 7d reset timestamp.
func (c *QuotaCache) ResetTime7d() *time.Time {
	return parseOptionalTime(c.Reset7d)
}

// MinutesToReset5h returns minutes until 5h reset.
func (c *QuotaCache) MinutesToReset5h() *float64 {
	return minutesToReset(c.ResetTime5h())
}

// MinutesToReset7d returns minutes until 7d reset.
func (c *QuotaCache) MinutesToReset7d() *float64 {
	return minutesToReset(c.ResetTime7d())
}

func minutesToReset(t *time.Time) *float64 {
	if t == nil {
		return nil
	}
	mins := time.Until(*t).Minutes()
	if mins < 0 {
		mins = 0
	}
	return &mins
}

func parseOptionalTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return &t
}

// Read loads cached quota info from the given path. Returns nil if no cache or parse error.
func Read(cachePath string) *QuotaCache {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	var c QuotaCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	return &c
}

// Write atomically writes quota info to the given cache path.
func Write(cachePath string, c *QuotaCache) error {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename
	tmp, err := os.CreateTemp(dir, ".claude-usage-cache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, cachePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename %s -> %s: %w", tmpName, cachePath, err)
	}
	return nil
}
