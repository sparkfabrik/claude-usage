// Package poller sends a minimal API request to extract rate-limit headers.
package poller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/Monska85/claude-usage/internal/cache"
)

// DefaultAPIURL is the production Anthropic Messages API endpoint.
const DefaultAPIURL = "https://api.anthropic.com/v1/messages"

// maxErrorBodyBytes limits how much of an API error response body we read.
const maxErrorBodyBytes = 4096

type pollRequest struct {
	Model    string        `json:"model"`
	MaxToks  int           `json:"max_tokens"`
	Messages []pollMessage `json:"messages"`
	Temp     float64       `json:"temperature"`
}

type pollMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Poll sends a 1-token request and extracts rate-limit headers.
// If apiURL is empty, DefaultAPIURL is used.
func Poll(accessToken, model string, timeout time.Duration, apiURL string) (*cache.QuotaCache, error) {
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	body := pollRequest{
		Model:    model,
		MaxToks:  1,
		Messages: []pollMessage{{Role: "user", Content: "."}},
		Temp:     0,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body

	if resp.StatusCode != 200 {
		// Drain (bounded) and discard body — don't include in error to avoid leaking sensitive info.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}
	io.Copy(io.Discard, resp.Body)

	return parseHeaders(resp.Header), nil
}

func parseHeaders(h http.Header) *cache.QuotaCache {
	now := time.Now().UTC()

	util5h := floatHeader(h, "anthropic-ratelimit-unified-5h-utilization")
	reset5hTs := floatHeader(h, "anthropic-ratelimit-unified-5h-reset")
	status5h := h.Get("anthropic-ratelimit-unified-5h-status")
	util7d := floatHeader(h, "anthropic-ratelimit-unified-7d-utilization")
	reset7dTs := floatHeader(h, "anthropic-ratelimit-unified-7d-reset")

	var reset5h, reset7d string
	if reset5hTs > 0 {
		reset5h = epochToTime(reset5hTs).Format(time.RFC3339Nano)
	}
	if reset7dTs > 0 {
		reset7d = epochToTime(reset7dTs).Format(time.RFC3339Nano)
	}

	return &cache.QuotaCache{
		Utilization5h: clamp01(util5h),
		Reset5h:       reset5h,
		Status5h:      status5h,
		Utilization7d: clamp01(util7d),
		Reset7d:       reset7d,
		PolledAt:      now.Format(time.RFC3339Nano),
	}
}

// clamp01 restricts a float64 value to the [0.0, 1.0] range.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func floatHeader(h http.Header, key string) float64 {
	v := h.Get(key)
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

// epochToTime converts a Unix epoch seconds (float64) to time.Time.
func epochToTime(epoch float64) time.Time {
	sec, frac := math.Modf(epoch)
	return time.Unix(int64(sec), int64(frac*1e9)).UTC()
}
