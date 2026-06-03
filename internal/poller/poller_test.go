package poller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClamp01(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}
	for _, tt := range tests {
		if got := clamp01(tt.in); got != tt.want {
			t.Errorf("clamp01(%f) = %f, want %f", tt.in, got, tt.want)
		}
	}
}

func TestFloatHeader(t *testing.T) {
	h := http.Header{}
	h.Set("x-rate", "0.42")
	h.Set("x-bad", "not-a-number")

	if got := floatHeader(h, "x-rate"); got != 0.42 {
		t.Errorf("got %f, want 0.42", got)
	}
	if got := floatHeader(h, "x-bad"); got != 0 {
		t.Errorf("got %f, want 0 for invalid", got)
	}
	if got := floatHeader(h, "x-missing"); got != 0 {
		t.Errorf("got %f, want 0 for missing", got)
	}
}

func TestEpochToTime(t *testing.T) {
	// 2025-01-01T00:00:00Z = 1735689600
	epoch := 1735689600.0
	got := epochToTime(epoch)
	want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("epochToTime(%f) = %v, want %v", epoch, got, want)
	}
}

func TestEpochToTime_WithFraction(t *testing.T) {
	epoch := 1735689600.5
	got := epochToTime(epoch)
	if got.Nanosecond() == 0 {
		t.Error("expected sub-second precision")
	}
}

func TestParseHeaders(t *testing.T) {
	now := time.Now().UTC()
	reset5h := now.Add(1 * time.Hour).Unix()
	reset7d := now.Add(48 * time.Hour).Unix()

	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.42")
	h.Set("anthropic-ratelimit-unified-5h-reset", fmt.Sprintf("%d", reset5h))
	h.Set("anthropic-ratelimit-unified-5h-status", "active")
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.67")
	h.Set("anthropic-ratelimit-unified-7d-reset", fmt.Sprintf("%d", reset7d))

	q := parseHeaders(h)

	if q.Utilization5h != 0.42 {
		t.Errorf("Utilization5h = %f, want 0.42", q.Utilization5h)
	}
	if q.Utilization7d != 0.67 {
		t.Errorf("Utilization7d = %f, want 0.67", q.Utilization7d)
	}
	if q.Status5h != "active" {
		t.Errorf("Status5h = %q, want active", q.Status5h)
	}
	if q.Reset5h == "" {
		t.Error("Reset5h should not be empty")
	}
	if q.Reset7d == "" {
		t.Error("Reset7d should not be empty")
	}
	if q.PolledAt == "" {
		t.Error("PolledAt should not be empty")
	}
}

func TestParseHeaders_MissingHeaders(t *testing.T) {
	h := http.Header{}
	q := parseHeaders(h)

	if q.Utilization5h != 0 {
		t.Errorf("Utilization5h = %f, want 0", q.Utilization5h)
	}
	if q.Reset5h != "" {
		t.Errorf("Reset5h = %q, want empty", q.Reset5h)
	}
}

func TestParseHeaders_ClampOver1(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-utilization", "1.5")
	q := parseHeaders(h)
	if q.Utilization5h != 1.0 {
		t.Errorf("expected clamped to 1.0, got %f", q.Utilization5h)
	}
}

// --- Poll integration tests ---

func TestPoll_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("bad auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("missing anthropic-version header")
		}
		if r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			t.Errorf("missing anthropic-beta header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing Content-Type header")
		}

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("invalid request JSON: %v", err)
		}
		if req["model"] != "claude-haiku-4-5-20251001" {
			t.Errorf("model = %v, want claude-haiku-4-5-20251001", req["model"])
		}
		if req["max_tokens"] != float64(1) {
			t.Errorf("max_tokens = %v, want 1", req["max_tokens"])
		}

		// Respond with rate-limit headers
		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", "0.33")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", "0.55")
		w.Header().Set("anthropic-ratelimit-unified-5h-status", "active")
		w.WriteHeader(200)
		w.Write([]byte(`{"content":[{"text":"."}]}`))
	}))
	defer server.Close()

	q, err := Poll("test-token", "", 5*time.Second, server.URL)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if q.Utilization5h != 0.33 {
		t.Errorf("Utilization5h = %f, want 0.33", q.Utilization5h)
	}
	if q.Utilization7d != 0.55 {
		t.Errorf("Utilization7d = %f, want 0.55", q.Utilization7d)
	}
	if q.Status5h != "active" {
		t.Errorf("Status5h = %q, want active", q.Status5h)
	}
	if q.PolledAt == "" {
		t.Error("PolledAt should not be empty")
	}
}

func TestPoll_CustomModel(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		gotModel = req["model"].(string)

		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", "0.10")
		w.WriteHeader(200)
		w.Write([]byte(`{"content":[{"text":"."}]}`))
	}))
	defer server.Close()

	_, err := Poll("tok", "claude-sonnet-4", 5*time.Second, server.URL)
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if gotModel != "claude-sonnet-4" {
		t.Errorf("model = %q, want claude-sonnet-4", gotModel)
	}
}

func TestPoll_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"rate limited"}}`))
	}))
	defer server.Close()

	_, err := Poll("test-token", "", 5*time.Second, server.URL)
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, want '429' in message", err)
	}
}

func TestPoll_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	_, err := Poll("test-token", "", 5*time.Second, server.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want '500' in message", err)
	}
}

func TestPoll_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	_, err := Poll("test-token", "", 100*time.Millisecond, server.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestPoll_DefaultAPIURL(t *testing.T) {
	// Verify empty apiURL defaults to DefaultAPIURL (can't actually call it)
	if DefaultAPIURL != "https://api.anthropic.com/v1/messages" {
		t.Errorf("DefaultAPIURL = %q", DefaultAPIURL)
	}
}
