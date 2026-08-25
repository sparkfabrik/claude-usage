// Package usage reads rate-limit windows from Anthropic's OAuth usage
// endpoint, falling back to the 1-token header poll when that endpoint is
// unavailable.
//
// The endpoint is authenticated with the same OAuth token the credentials file
// already holds, consumes no quota, and reports model-scoped windows that the
// rate-limit response headers cannot express.
package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sparkfabrik/claude-usage/internal/cache"
	"github.com/sparkfabrik/claude-usage/internal/poller"
)

// DefaultEndpoint is the OAuth usage endpoint.
const DefaultEndpoint = "https://api.anthropic.com/api/oauth/usage"

// maxBodyBytes bounds how much of a response we read, for both the success and
// the error path.
const maxBodyBytes = 1 << 20

// payload mirrors the parts of the endpoint response we consume. Unknown
// fields are ignored, so the endpoint can grow without breaking us.
type payload struct {
	FiveHour          *bucket       `json:"five_hour"`
	SevenDay          *bucket       `json:"seven_day"`
	SevenDayOAuthApps *bucket       `json:"seven_day_oauth_apps"`
	Limits            []scopedLimit `json:"limits"`
}

type bucket struct {
	Utilization json.RawMessage `json:"utilization"`
	ResetsAt    json.RawMessage `json:"resets_at"`
}

type scopedLimit struct {
	Kind     string          `json:"kind"`
	Percent  json.RawMessage `json:"percent"`
	ResetsAt json.RawMessage `json:"resets_at"`
	Scope    struct {
		Model struct {
			DisplayName string `json:"display_name"`
			ID          string `json:"id"`
		} `json:"model"`
	} `json:"scope"`
}

// Fetch calls the usage endpoint and normalizes the payload into windows.
func Fetch(accessToken, endpoint string, timeout time.Duration) ([]cache.Window, error) {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if accessToken == "" {
		return nil, fmt.Errorf("no access token")
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// The body may echo account details, so it is never surfaced.
		return nil, fmt.Errorf("usage endpoint returned %d", resp.StatusCode)
	}

	return ParseResponse(body)
}

// ParseResponse normalizes a raw endpoint response into windows. It is
// exported so the normalization can be tested without a network.
func ParseResponse(body []byte) ([]cache.Window, error) {
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("decoding usage payload: %w", err)
	}

	percentScale := detectPercentScale(p)

	var windows []cache.Window

	if b := firstBucket(p.SevenDayOAuthApps, p.SevenDay); b != nil {
		windows = append(windows, cache.Window{
			Key:         cache.WindowWeekly,
			Title:       "Weekly (7-day)",
			Utilization: normalizeUtilization(b.Utilization, percentScale),
			ResetsAt:    normalizeResetAt(b.ResetsAt),
		})
	}
	if p.FiveHour != nil {
		windows = append([]cache.Window{{
			Key:         cache.WindowSession,
			Title:       "Session (5-hour)",
			Utilization: normalizeUtilization(p.FiveHour.Utilization, percentScale),
			ResetsAt:    normalizeResetAt(p.FiveHour.ResetsAt),
		}}, windows...)
	}

	windows = append(windows, scopedWindows(p.Limits, percentScale)...)

	if len(windows) == 0 {
		return nil, fmt.Errorf("usage payload carried no windows")
	}
	return windows, nil
}

// scopedWindows turns the model-scoped entries into windows, skipping
// duplicates. One model can hold more than one window, so the identity is the
// model and kind together.
func scopedWindows(limits []scopedLimit, percentScale bool) []cache.Window {
	seen := make(map[string]bool, len(limits))
	var out []cache.Window

	for _, l := range limits {
		name := strings.TrimSpace(l.Scope.Model.DisplayName)
		if name == "" {
			name = strings.TrimSpace(l.Scope.Model.ID)
		}
		if name == "" {
			// Not model-scoped: the flat buckets already cover it.
			continue
		}
		window := windowName(l.Kind)
		if window == "" {
			continue
		}
		key := strings.ToLower(name) + ":" + strings.ToLower(window)
		if seen[key] {
			continue
		}
		seen[key] = true

		out = append(out, cache.Window{
			Key:         key,
			Title:       name + " " + window,
			Model:       name,
			Utilization: normalizeUtilization(l.Percent, percentScale),
			ResetsAt:    normalizeResetAt(l.ResetsAt),
		})
	}
	return out
}

// firstBucket returns the first non-nil bucket. The weekly figure lives under
// seven_day_oauth_apps for OAuth clients and under seven_day otherwise.
func firstBucket(buckets ...*bucket) *bucket {
	for _, b := range buckets {
		if b != nil {
			return b
		}
	}
	return nil
}

// windowName maps a raw kind to a human window name. Returns "" when the kind
// is not recognized, so unknown windows are dropped rather than mislabelled.
func windowName(kind string) string {
	k := strings.ToLower(kind)
	switch {
	case strings.Contains(k, "month"):
		return "Monthly"
	case strings.Contains(k, "week"), strings.Contains(k, "day"):
		return "Weekly"
	case strings.Contains(k, "hour"), strings.Contains(k, "session"):
		return "Session"
	default:
		return ""
	}
}

// detectPercentScale decides once, for the whole payload, whether utilization
// is expressed as a percentage or as a fraction. The endpoint is internally
// consistent but varies between accounts, and deciding per value would render
// a genuine 1.0% as 100%.
func detectPercentScale(p payload) bool {
	raw := []json.RawMessage{}
	for _, b := range []*bucket{p.FiveHour, p.SevenDay, p.SevenDayOAuthApps} {
		if b != nil {
			raw = append(raw, b.Utilization)
		}
	}
	for _, l := range p.Limits {
		raw = append(raw, l.Percent)
	}

	for _, r := range raw {
		if v, ok := parseNumber(r); ok && v > 1 {
			return true
		}
	}
	return false
}

// normalizeUtilization converts a raw utilization to a fraction in [0,1].
// A negative or unparseable value yields 0.
func normalizeUtilization(raw json.RawMessage, percentScale bool) float64 {
	v, ok := parseNumber(raw)
	if !ok || v < 0 || math.IsNaN(v) {
		return 0
	}
	if percentScale || v > 1 {
		v /= 100
	}
	if v > 1 {
		return 1
	}
	return v
}

// parseNumber accepts a JSON number or a numeric string, with an optional
// trailing percent sign.
func parseNumber(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	// json.Unmarshal decodes a literal null into a zero float without error,
	// which would turn an absent value into a real 0.
	if string(raw) == "null" {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// normalizeResetAt converts a reset marker to RFC 3339. It accepts an ISO
// string or an epoch, in seconds or milliseconds. Returns "" when absent or
// unparseable, which callers read as "window always open".
func normalizeResetAt(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	if v, ok := parseNumber(raw); ok && v > 0 {
		return epochToTime(v).Format(time.RFC3339Nano)
	}

	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000-07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339Nano)
		}
	}
	return ""
}

// epochToTime converts an epoch to UTC, treating values below 1e12 as seconds
// and the rest as milliseconds.
func epochToTime(epoch float64) time.Time {
	if epoch < 1e12 {
		sec, frac := math.Modf(epoch)
		return time.Unix(int64(sec), int64(frac*1e9)).UTC()
	}
	return time.UnixMilli(int64(epoch)).UTC()
}

// Collect returns quota data, preferring the OAuth usage endpoint and falling
// back to the header poll. The fallback keeps behavior intact if the endpoint
// changes: it is undocumented and ships behind a beta header.
//
// pollModel and apiURL configure the fallback only.
func Collect(accessToken, endpoint, pollModel, apiURL string, timeout time.Duration) (*cache.QuotaCache, error) {
	windows, err := Fetch(accessToken, endpoint, timeout)
	if err == nil {
		return quotaFromWindows(windows), nil
	}

	q, pollErr := poller.Poll(accessToken, pollModel, timeout, apiURL)
	if pollErr != nil {
		// Report the endpoint failure: it is the path we want to keep working.
		return nil, fmt.Errorf("usage endpoint: %v; header poll: %w", err, pollErr)
	}
	q.Source = cache.SourceHeaders
	q.SyncWindows()
	return q, nil
}

// quotaFromWindows builds a cache record, mirroring the session and weekly
// windows into the flat fields that existing readers still consume.
func quotaFromWindows(windows []cache.Window) *cache.QuotaCache {
	q := &cache.QuotaCache{
		Windows:  windows,
		Source:   cache.SourceOAuth,
		PolledAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	for _, w := range windows {
		switch w.Key {
		case cache.WindowSession:
			q.Utilization5h = w.Utilization
			q.Reset5h = w.ResetsAt
		case cache.WindowWeekly:
			q.Utilization7d = w.Utilization
			q.Reset7d = w.ResetsAt
		}
	}
	return q
}
