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

// QuotaCache is the JSON structure stored in the cache file.
type QuotaCache struct {
	Utilization5h float64 `json:"utilization_5h"`
	Reset5h       string  `json:"reset_5h,omitempty"` // ISO 8601
	Status5h      string  `json:"status_5h,omitempty"`
	Utilization7d float64 `json:"utilization_7d"`
	Reset7d       string  `json:"reset_7d,omitempty"` // ISO 8601
	PolledAt      string  `json:"polled_at"`          // ISO 8601
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
