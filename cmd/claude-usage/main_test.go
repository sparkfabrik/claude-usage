package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/claude-usage/internal/auth"
	"github.com/Monska85/claude-usage/internal/cache"
	"github.com/Monska85/claude-usage/internal/config"
	"github.com/Monska85/claude-usage/internal/pricing"
)

// --- initVersion / applyBuildInfo ---

func TestInitVersion_SkipsWhenAlreadySet(t *testing.T) {
	// Save and restore package-level vars.
	origVersion, origCommit := version, commit
	t.Cleanup(func() { version, commit = origVersion, origCommit })

	version = "1.2.3"
	commit = "abc"
	initVersion()

	if version != "1.2.3" {
		t.Errorf("version changed: got %q, want %q", version, "1.2.3")
	}
	if commit != "abc" {
		t.Errorf("commit changed: got %q, want %q", commit, "abc")
	}
}

func TestInitVersion_FallsBackToDev(t *testing.T) {
	origVersion := version
	t.Cleanup(func() { version = origVersion })

	// Empty version triggers initVersion logic. In test binary context,
	// debug.ReadBuildInfo returns the test binary info — version will be
	// either populated from VCS or fall back to "dev".
	version = ""
	initVersion()

	if version == "" {
		t.Error("version should not remain empty after initVersion")
	}
}

func TestApplyBuildInfo_FullVCS(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() { version, commit, date = origVersion, origCommit, origDate })

	version, commit, date = "", "", ""
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.5.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc1234"},
			{Key: "vcs.time", Value: "2025-06-01T00:00:00Z"},
		},
	}
	applyBuildInfo(info)

	if version != "v1.5.0" {
		t.Errorf("version = %q, want v1.5.0", version)
	}
	if commit != "abc1234" {
		t.Errorf("commit = %q, want abc1234", commit)
	}
	if date != "2025-06-01T00:00:00Z" {
		t.Errorf("date = %q, want 2025-06-01T00:00:00Z", date)
	}
}

func TestApplyBuildInfo_DirtyCommit(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() { version, commit, date = origVersion, origCommit, origDate })

	version, commit, date = "", "", ""
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.0.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "def5678"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	applyBuildInfo(info)

	if commit != "def5678-dirty" {
		t.Errorf("commit = %q, want def5678-dirty", commit)
	}
}

func TestApplyBuildInfo_ModifiedButNoCommit(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() { version, commit, date = origVersion, origCommit, origDate })

	version, commit, date = "", "", ""
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.0.0"},
		Settings: []debug.BuildSetting{
			// vcs.modified before vcs.revision — commit is still empty
			{Key: "vcs.modified", Value: "true"},
		},
	}
	applyBuildInfo(info)

	// commit should remain empty, not "-dirty"
	if commit != "" {
		t.Errorf("commit = %q, want empty (no revision set)", commit)
	}
}

func TestApplyBuildInfo_DevelVersion(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() { version, commit, date = origVersion, origCommit, origDate })

	version, commit, date = "", "", ""
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
	}
	applyBuildInfo(info)

	if version != "dev" {
		t.Errorf("version = %q, want dev for (devel)", version)
	}
}

func TestApplyBuildInfo_EmptyVersion(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() { version, commit, date = origVersion, origCommit, origDate })

	version, commit, date = "", "", ""
	info := &debug.BuildInfo{
		Main: debug.Module{Version: ""},
	}
	applyBuildInfo(info)

	if version != "dev" {
		t.Errorf("version = %q, want dev for empty", version)
	}
}

func TestApplyBuildInfo_NoSettings(t *testing.T) {
	origVersion, origCommit, origDate := version, commit, date
	t.Cleanup(func() { version, commit, date = origVersion, origCommit, origDate })

	version, commit, date = "", "", ""
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v2.0.0"},
	}
	applyBuildInfo(info)

	if version != "v2.0.0" {
		t.Errorf("version = %q, want v2.0.0", version)
	}
	if commit != "" {
		t.Errorf("commit = %q, want empty", commit)
	}
	if date != "" {
		t.Errorf("date = %q, want empty", date)
	}
}

// --- authStatus ---

func TestAuthStatus(t *testing.T) {
	tests := []struct {
		name  string
		creds *auth.Credentials
		want  string
	}{
		{
			name:  "nil credentials",
			creds: nil,
			want:  "missing",
		},
		{
			name: "expired credentials",
			creds: &auth.Credentials{
				ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
			},
			want: "expired",
		},
		{
			name: "valid credentials",
			creds: &auth.Credentials{
				ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
			},
			want: "valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authStatus(tt.creds)
			if got != tt.want {
				t.Errorf("authStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- colorForPct ---

func TestColorForPct(t *testing.T) {
	cfg := config.Default()
	// Defaults: GreenBelow=80, OrangeBelow=90

	tests := []struct {
		pct  int
		want string
	}{
		{0, "#32c850"},   // green
		{50, "#32c850"},  // green
		{79, "#32c850"},  // green (just below threshold)
		{80, "#e6961e"},  // orange (at GreenBelow)
		{89, "#e6961e"},  // orange
		{90, "#dc3232"},  // red (at OrangeBelow)
		{100, "#dc3232"}, // red
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := colorForPct(tt.pct, cfg)
			if got != tt.want {
				t.Errorf("colorForPct(%d) = %q, want %q", tt.pct, got, tt.want)
			}
		})
	}
}

func TestColorForPct_CustomThresholds(t *testing.T) {
	cfg := &config.Config{
		Colors: config.Colors{GreenBelow: 50, OrangeBelow: 75},
	}

	tests := []struct {
		pct  int
		want string
	}{
		{49, "#32c850"},
		{50, "#e6961e"},
		{74, "#e6961e"},
		{75, "#dc3232"},
	}

	for _, tt := range tests {
		got := colorForPct(tt.pct, cfg)
		if got != tt.want {
			t.Errorf("colorForPct(%d) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}

// --- resolveCachePath ---

func TestResolveCachePath_ConfigOverride(t *testing.T) {
	cfg := &config.Config{
		Cache: config.Cache{Path: "/custom/cache.json"},
	}
	got, err := resolveCachePath(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/custom/cache.json" {
		t.Errorf("got %q, want %q", got, "/custom/cache.json")
	}
}

func TestResolveCachePath_Default(t *testing.T) {
	cfg := &config.Config{}
	got, err := resolveCachePath(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("default cache path should not be empty")
	}
	if !strings.HasSuffix(got, "claude-code-usage/quota.json") {
		t.Errorf("default path should end with claude-code-usage/quota.json, got %q", got)
	}
}

// --- buildStatusResponse ---

func TestBuildStatusResponse_NilQuota(t *testing.T) {
	cfg := config.Default()
	resp := buildStatusResponse(nil, cfg, "some error")

	if resp.CPct != 0 {
		t.Errorf("CPct = %d, want 0", resp.CPct)
	}
	if resp.WPct != 0 {
		t.Errorf("WPct = %d, want 0", resp.WPct)
	}
	if resp.CReset != "?" {
		t.Errorf("CReset = %q, want %q", resp.CReset, "?")
	}
	if resp.WReset != "?" {
		t.Errorf("WReset = %q, want %q", resp.WReset, "?")
	}
	if resp.Error != "some error" {
		t.Errorf("Error = %q, want %q", resp.Error, "some error")
	}
}

func TestBuildStatusResponse_WithQuota(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.42,
		Utilization7d: 0.67,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}

	resp := buildStatusResponse(q, cfg, "")

	if resp.CPct != 42 {
		t.Errorf("CPct = %d, want 42", resp.CPct)
	}
	if resp.WPct != 67 {
		t.Errorf("WPct = %d, want 67", resp.WPct)
	}
	if resp.Error != "" {
		t.Errorf("Error = %q, want empty", resp.Error)
	}
	// 42% is below GreenBelow (80) → green
	if resp.CColor != "#32c850" {
		t.Errorf("CColor = %q, want #32c850", resp.CColor)
	}
	// 67% is still below GreenBelow (80) → green
	if resp.WColor != "#32c850" {
		t.Errorf("WColor = %q, want #32c850", resp.WColor)
	}
}

func TestBuildStatusResponse_HighUtilization(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.95,
		Utilization7d: 0.85,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}

	resp := buildStatusResponse(q, cfg, "")

	if resp.CPct != 95 {
		t.Errorf("CPct = %d, want 95", resp.CPct)
	}
	// 95% >= OrangeBelow (90) → red
	if resp.CColor != "#dc3232" {
		t.Errorf("CColor = %q, want #dc3232", resp.CColor)
	}
	// 85% >= GreenBelow (80) but < OrangeBelow (90) → orange
	if resp.WColor != "#e6961e" {
		t.Errorf("WColor = %q, want #e6961e", resp.WColor)
	}
}

func TestBuildStatusResponse_Stale(t *testing.T) {
	cfg := config.Default()
	staleTime := time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano)
	q := &cache.QuotaCache{
		Utilization5h: 0.10,
		PolledAt:      staleTime,
	}

	resp := buildStatusResponse(q, cfg, "")

	if !resp.Stale {
		t.Error("expected Stale=true for data polled 5 minutes ago")
	}
}

func TestBuildStatusResponse_Fresh(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.10,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}

	resp := buildStatusResponse(q, cfg, "")

	if resp.Stale {
		t.Error("expected Stale=false for freshly polled data")
	}
}

// --- buildPricingOverrides ---

func TestBuildPricingOverrides_Nil(t *testing.T) {
	cfg := &config.Config{}
	got := buildPricingOverrides(cfg)
	if got != nil {
		t.Errorf("expected nil for empty pricing, got %v", got)
	}
}

func TestBuildPricingOverrides_Populated(t *testing.T) {
	cfg := &config.Config{
		Pricing: map[string]config.ModelPricing{
			"claude-sonnet-4": {
				Input:      3.0,
				Output:     15.0,
				CacheWrite: 3.75,
				CacheRead:  0.30,
			},
		},
	}

	got := buildPricingOverrides(cfg)
	if got == nil {
		t.Fatal("expected non-nil overrides")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 override, got %d", len(got))
	}

	mp, ok := got["claude-sonnet-4"]
	if !ok {
		t.Fatal("expected claude-sonnet-4 in overrides")
	}
	want := pricing.ModelPrice{
		Input:      3.0,
		Output:     15.0,
		CacheWrite: 3.75,
		CacheRead:  0.30,
	}
	if mp != want {
		t.Errorf("got %+v, want %+v", mp, want)
	}
}

// --- StatusResponse JSON ---

func TestStatusResponse_JSONOmitsEmptyError(t *testing.T) {
	resp := StatusResponse{
		CPct:  42,
		CColor: "#32c850",
		Auth:  "valid",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	if strings.Contains(string(data), `"error"`) {
		t.Error("empty error should be omitted from JSON (omitempty)")
	}
}

func TestStatusResponse_JSONIncludesError(t *testing.T) {
	resp := StatusResponse{
		Error: "something broke",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	if !strings.Contains(string(data), `"error":"something broke"`) {
		t.Errorf("expected error in JSON, got %s", data)
	}
}

// --- outputJSON ---

func TestOutputJSON_WritesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	resp := StatusResponse{CPct: 42, CColor: "#32c850", Auth: "valid"}

	if err := outputJSON(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.CPct != 42 {
		t.Errorf("CPct = %d, want 42", got.CPct)
	}
	if got.Auth != "valid" {
		t.Errorf("Auth = %q, want %q", got.Auth, "valid")
	}
}

func TestOutputJSON_TrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	err := outputJSON(&buf, map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("output should end with newline")
	}
}

func TestOutputJSON_MarshalError(t *testing.T) {
	var buf bytes.Buffer
	err := outputJSON(&buf, make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshallable type")
	}
	if !strings.Contains(err.Error(), "JSON marshal") {
		t.Errorf("error = %q, want 'JSON marshal' prefix", err)
	}
	if buf.Len() != 0 {
		t.Errorf("buffer should be empty on error, got %q", buf.String())
	}
}

// --- runPollOnly ---

func TestRunPollOnly_NilCreds(t *testing.T) {
	cfg := config.Default()
	err := runPollOnly(nil, cfg, "/nonexistent", "")
	if err == nil {
		t.Fatal("expected error for nil creds")
	}
	if !strings.Contains(err.Error(), "no credentials found") {
		t.Errorf("error = %q, want 'no credentials found'", err)
	}
}

func TestRunPollOnly_ExpiredCreds(t *testing.T) {
	cfg := config.Default()
	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
	}
	err := runPollOnly(creds, cfg, "/nonexistent", "")
	if err == nil {
		t.Fatal("expected error for expired creds")
	}
	if !strings.Contains(err.Error(), "credentials expired") {
		t.Errorf("error = %q, want 'credentials expired'", err)
	}
}

// fakeAPIServer returns an httptest.Server that mimics Anthropic rate-limit headers.
func fakeAPIServer(t *testing.T, util5h, util7d string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", util5h)
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", util7d)
		w.WriteHeader(200)
		w.Write([]byte(`{"content":[{"text":"."}]}`))
	}))
}

func TestRunPollOnly_Success(t *testing.T) {
	server := fakeAPIServer(t, "0.25", "0.40")
	defer server.Close()

	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	err := runPollOnly(creds, cfg, cachePath, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache was written
	q := cache.Read(cachePath)
	if q == nil {
		t.Fatal("expected cache file to be written")
	}
	if q.Utilization5h != 0.25 {
		t.Errorf("Utilization5h = %f, want 0.25", q.Utilization5h)
	}
	if q.Utilization7d != 0.40 {
		t.Errorf("Utilization7d = %f, want 0.40", q.Utilization7d)
	}
}

func TestRunPollOnly_PollError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	err := runPollOnly(creds, cfg, cachePath, server.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "poll:") {
		t.Errorf("error = %q, want 'poll:' prefix", err)
	}
}

// --- runStatus ---

func TestRunStatus_NoPollNilCredsNoCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/nonexistent.json"

	runStatus(&buf, nil, cfg, cachePath, true, false, "")

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.Auth != "unknown" {
		t.Errorf("Auth = %q, want %q", resp.Auth, "unknown")
	}
	if resp.Error != "no cached data available" {
		t.Errorf("Error = %q, want %q", resp.Error, "no cached data available")
	}
}

func TestRunStatus_NoPollNilCredsWithCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	q := &cache.QuotaCache{
		Utilization5h: 0.33,
		Utilization7d: 0.22,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	runStatus(&buf, nil, cfg, cachePath, true, false, "")

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.CPct != 33 {
		t.Errorf("CPct = %d, want 33", resp.CPct)
	}
	if resp.WPct != 22 {
		t.Errorf("WPct = %d, want 22", resp.WPct)
	}
	if resp.Auth != "unknown" {
		t.Errorf("Auth = %q, want %q", resp.Auth, "unknown")
	}
	if resp.Error != "" {
		t.Errorf("Error = %q, want empty", resp.Error)
	}
}

func TestRunStatus_NoPollWithValidCreds(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	q := &cache.QuotaCache{
		Utilization5h: 0.50,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	runStatus(&buf, creds, cfg, cachePath, true, false, "")

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.CPct != 50 {
		t.Errorf("CPct = %d, want 50", resp.CPct)
	}
	// With valid creds and noPoll, auth should still be "valid" (not "unknown")
	if resp.Auth != "valid" {
		t.Errorf("Auth = %q, want %q", resp.Auth, "valid")
	}
}

func TestRunStatus_PollDisabledNoCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/nonexistent.json"

	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	runStatus(&buf, creds, cfg, cachePath, false, false, "")

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.Error != "polling disabled" {
		t.Errorf("Error = %q, want %q", resp.Error, "polling disabled")
	}
}

func TestRunStatus_NilCredsNoCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/nonexistent.json"

	runStatus(&buf, nil, cfg, cachePath, false, false, "")

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.Error != "no credentials found" {
		t.Errorf("Error = %q, want %q", resp.Error, "no credentials found")
	}
	if resp.Auth != "missing" {
		t.Errorf("Auth = %q, want %q", resp.Auth, "missing")
	}
}

func TestRunStatus_ExpiredCredsNoCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/nonexistent.json"

	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
	}

	runStatus(&buf, creds, cfg, cachePath, false, false, "")

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.Error != "credentials expired" {
		t.Errorf("Error = %q, want %q", resp.Error, "credentials expired")
	}
	if resp.Auth != "expired" {
		t.Errorf("Auth = %q, want %q", resp.Auth, "expired")
	}
}

func TestRunStatus_PollSuccess(t *testing.T) {
	server := fakeAPIServer(t, "0.42", "0.67")
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	f := false
	cfg.API.OnlyWhenActive = &f // disable so test works without Claude running
	cachePath := t.TempDir() + "/nonexistent.json"

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	// No cache exists → should poll → get data from mock server
	runStatus(&buf, creds, cfg, cachePath, false, false, server.URL)

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.CPct != 42 {
		t.Errorf("CPct = %d, want 42", resp.CPct)
	}
	if resp.WPct != 67 {
		t.Errorf("WPct = %d, want 67", resp.WPct)
	}
	if resp.Error != "" {
		t.Errorf("Error = %q, want empty", resp.Error)
	}
	if resp.Auth != "valid" {
		t.Errorf("Auth = %q, want valid", resp.Auth)
	}
}

func TestRunStatus_PollError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	f := false
	cfg.API.OnlyWhenActive = &f
	cachePath := t.TempDir() + "/nonexistent.json"

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	runStatus(&buf, creds, cfg, cachePath, false, false, server.URL)

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if resp.Error == "" {
		t.Error("expected non-empty error for poll failure")
	}
	if !strings.Contains(resp.Error, "500") {
		t.Errorf("Error = %q, want '500' in message", resp.Error)
	}
}

func TestRunStatus_ForcePoll(t *testing.T) {
	server := fakeAPIServer(t, "0.10", "0.20")
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	// Write fresh cache — force-poll should ignore it
	q := &cache.QuotaCache{
		Utilization5h: 0.99,
		Utilization7d: 0.99,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	runStatus(&buf, creds, cfg, cachePath, false, true, server.URL)

	var resp StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	// Should have fresh data from mock, not stale 99%
	if resp.CPct != 10 {
		t.Errorf("CPct = %d, want 10 (force-poll should override cache)", resp.CPct)
	}
	if resp.WPct != 20 {
		t.Errorf("WPct = %d, want 20", resp.WPct)
	}
}

// --- pollOrReadCache ---

func TestPollOrReadCache_NoPoll(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	q := &cache.QuotaCache{
		Utilization5h: 0.75,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	got := pollOrReadCache(&buf, nil, cfg, cachePath, true, false, "")
	if got == nil {
		t.Fatal("expected non-nil cache result")
	}
	if got.Utilization5h != 0.75 {
		t.Errorf("Utilization5h = %f, want 0.75", got.Utilization5h)
	}
}

func TestPollOrReadCache_NoPollNoCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/nonexistent.json"

	got := pollOrReadCache(&buf, nil, cfg, cachePath, true, false, "")
	if got != nil {
		t.Errorf("expected nil for missing cache, got %+v", got)
	}
}

func TestPollOrReadCache_FreshCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	q := &cache.QuotaCache{
		Utilization5h: 0.60,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, false, "")
	if got == nil {
		t.Fatal("expected non-nil cache result")
	}
	if got.Utilization5h != 0.60 {
		t.Errorf("Utilization5h = %f, want 0.60", got.Utilization5h)
	}
	// Should print "Using cached rate limits" message
	if !strings.Contains(buf.String(), "Using cached") {
		t.Errorf("expected 'Using cached' message, got %q", buf.String())
	}
}

func TestPollOrReadCache_DisabledAPI(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/quota.json"

	q := &cache.QuotaCache{
		Utilization5h: 0.40,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, false, "")
	if got == nil {
		t.Fatal("expected non-nil cache result")
	}
	if got.Utilization5h != 0.40 {
		t.Errorf("Utilization5h = %f, want 0.40", got.Utilization5h)
	}
}

func TestPollOrReadCache_StaleCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false // can't poll, but cache is stale
	cachePath := t.TempDir() + "/quota.json"

	staleTime := time.Now().Add(-5 * time.Minute)
	q := &cache.QuotaCache{
		Utilization5h: 0.40,
		PolledAt:      staleTime.Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	// API disabled → canPoll=false → returns cached even if stale
	got := pollOrReadCache(&buf, nil, cfg, cachePath, false, false, "")
	if got == nil {
		t.Fatal("expected non-nil cache result")
	}
	if got.Utilization5h != 0.40 {
		t.Errorf("Utilization5h = %f, want 0.40", got.Utilization5h)
	}
}

func TestPollOrReadCache_ExpiredCredsNoCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/nonexistent.json"

	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
	}

	// Expired creds → canPoll=false → returns nil (no cache)
	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, false, "")
	if got != nil {
		t.Errorf("expected nil for expired creds + no cache, got %+v", got)
	}
}

func TestPollOrReadCache_NilCredsWithCache(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	q := &cache.QuotaCache{
		Utilization5h: 0.55,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	// Nil creds → canPoll=false → returns cached data
	got := pollOrReadCache(&buf, nil, cfg, cachePath, false, false, "")
	if got == nil {
		t.Fatal("expected non-nil cache result")
	}
	if got.Utilization5h != 0.55 {
		t.Errorf("Utilization5h = %f, want 0.55", got.Utilization5h)
	}
}

func TestPollOrReadCache_StaleCacheTriggersPoll(t *testing.T) {
	server := fakeAPIServer(t, "0.30", "0.50")
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	// Write stale cache
	staleTime := time.Now().Add(-5 * time.Minute)
	q := &cache.QuotaCache{
		Utilization5h: 0.99,
		PolledAt:      staleTime.Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, false, server.URL)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	// Should have fresh data from mock server, not stale 99%
	if got.Utilization5h != 0.30 {
		t.Errorf("Utilization5h = %f, want 0.30 (poll should replace stale)", got.Utilization5h)
	}
	if !strings.Contains(buf.String(), "Polling API") {
		t.Errorf("expected 'Polling API' message, got %q", buf.String())
	}
}

func TestPollOrReadCache_ForcePoll(t *testing.T) {
	server := fakeAPIServer(t, "0.15", "0.25")
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	// Write fresh cache — force-poll should ignore freshness
	q := &cache.QuotaCache{
		Utilization5h: 0.99,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, true, server.URL)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Utilization5h != 0.15 {
		t.Errorf("Utilization5h = %f, want 0.15 (force-poll should override)", got.Utilization5h)
	}
}

func TestPollOrReadCache_PollFails_ReturnsCached(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	// Write stale cache — poll will fail, should fall back to cached
	staleTime := time.Now().Add(-5 * time.Minute)
	q := &cache.QuotaCache{
		Utilization5h: 0.77,
		PolledAt:      staleTime.Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, false, server.URL)
	if got == nil {
		t.Fatal("expected non-nil result (fallback to cached)")
	}
	if got.Utilization5h != 0.77 {
		t.Errorf("Utilization5h = %f, want 0.77 (should fall back to cached)", got.Utilization5h)
	}
}

func TestPollOrReadCache_NoCachePollSuccess(t *testing.T) {
	server := fakeAPIServer(t, "0.20", "0.35")
	defer server.Close()

	var buf bytes.Buffer
	cfg := config.Default()
	cachePath := t.TempDir() + "/quota.json"

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	// No cache exists → should poll
	got := pollOrReadCache(&buf, creds, cfg, cachePath, false, false, server.URL)
	if got == nil {
		t.Fatal("expected non-nil result from poll")
	}
	if got.Utilization5h != 0.20 {
		t.Errorf("Utilization5h = %f, want 0.20", got.Utilization5h)
	}
}

// --- runDashboard ---

// createFakeProjectsDir creates a temp directory with a fake JSONL session file
// mimicking Claude Code's ~/.claude/projects/<project>/<session>.jsonl structure.
func createFakeProjectsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	projectDir := dir + "/my-project"
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Now().UTC()
	jsonl := ""
	// Two assistant messages with usage data
	for i, ts := range []time.Time{now.Add(-1 * time.Hour), now.Add(-30 * time.Minute)} {
		line := fmt.Sprintf(
			`{"type":"assistant","timestamp":"%s","uuid":"msg-%d","requestId":"req-%d","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":200,"cache_read_input_tokens":100}}}`,
			ts.Format(time.RFC3339Nano), i, i,
		)
		jsonl += line + "\n"
	}
	// A non-assistant message (should be ignored)
	jsonl += `{"type":"human","timestamp":"` + now.Format(time.RFC3339Nano) + `","message":{}}` + "\n"

	if err := os.WriteFile(projectDir+"/session-abc.jsonl", []byte(jsonl), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	return dir
}

func TestRunDashboard_NoEntries(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/quota.json"
	emptyDir := t.TempDir() // no .jsonl files

	err := runDashboard(&buf, nil, cfg, cachePath, emptyDir, true, false, false, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No usage data found") {
		t.Errorf("expected 'No usage data found' message, got:\n%s", output)
	}
}

func TestRunDashboard_WithEntries(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/quota.json"
	projectsDir := createFakeProjectsDir(t)

	err := runDashboard(&buf, nil, cfg, cachePath, projectsDir, true, false, false, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should contain usage table output (not "No usage data")
	if strings.Contains(output, "No usage data found") {
		t.Error("should have found usage data")
	}
	// Output should be non-trivial
	if len(output) < 50 {
		t.Errorf("output too short, expected dashboard content:\n%s", output)
	}
}

func TestRunDashboard_WithEntriesNoCost(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cfg.Display.ShowCost = true
	cachePath := t.TempDir() + "/quota.json"
	projectsDir := createFakeProjectsDir(t)

	err := runDashboard(&buf, nil, cfg, cachePath, projectsDir, true, false, true, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// noCost=true should not error
	if buf.Len() < 50 {
		t.Errorf("output too short:\n%s", buf.String())
	}
}

func TestRunDashboard_SpecificPeriod(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/quota.json"
	projectsDir := createFakeProjectsDir(t)

	err := runDashboard(&buf, nil, cfg, cachePath, projectsDir, true, false, false, "today", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() < 50 {
		t.Errorf("output too short:\n%s", buf.String())
	}
}

func TestRunDashboard_InvalidPeriod(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/quota.json"
	projectsDir := createFakeProjectsDir(t)

	err := runDashboard(&buf, nil, cfg, cachePath, projectsDir, true, false, false, "bogus", "")
	if err == nil {
		t.Fatal("expected error for invalid period")
	}
	if !strings.Contains(err.Error(), "unknown period") {
		t.Errorf("error = %q, want 'unknown period'", err)
	}
}

func TestRunDashboard_NonexistentProjectsDir(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.Default()
	cfg.API.Enabled = false
	cachePath := t.TempDir() + "/quota.json"

	err := runDashboard(&buf, nil, cfg, cachePath, "/tmp/nonexistent-projects-dir", true, false, false, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No usage data found") {
		t.Errorf("expected 'No usage data found' for nonexistent dir, got:\n%s", output)
	}
}

// --- CLI integration tests ---

// TestCLI_Version builds the binary and tests --version / -V output.
func TestCLI_Version(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binary := buildTestBinary(t)

	tests := []struct {
		name string
		flag string
	}{
		{"long flag", "--version"},
		{"short flag", "-V"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := exec.Command(binary, tt.flag).CombinedOutput()
			if err != nil {
				t.Fatalf("command failed: %v\noutput: %s", err, out)
			}

			output := string(out)
			if !strings.HasPrefix(output, "claude-usage ") {
				t.Errorf("expected output to start with 'claude-usage ', got %q", output)
			}
		})
	}
}

func TestCLI_VersionWithLdflags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binary := buildTestBinaryWithLdflags(t, "1.2.3", "abc1234", "2025-01-01T00:00:00Z", "test")

	out, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, out)
	}

	output := string(out)
	for _, want := range []string{
		"claude-usage 1.2.3",
		"commit:  abc1234",
		"built:   2025-01-01T00:00:00Z",
		"builder: test",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, output)
		}
	}
}

func TestCLI_StatusNoPollNoCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binary := buildTestBinary(t)

	// Use a non-existent cache path via config to ensure no cached data.
	nonExistentCache := filepath.Join(t.TempDir(), "test-nonexistent-cache.json")
	cfgPath := writeTempConfig(t, fmt.Sprintf(`
cache:
  path: %s
`, nonExistentCache))

	out, err := exec.Command(binary, "--status", "--no-poll", "--config", cfgPath).CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, out)
	}

	var resp StatusResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}

	if resp.Error != "no cached data available" {
		t.Errorf("Error = %q, want %q", resp.Error, "no cached data available")
	}
	if resp.Auth != "unknown" {
		t.Errorf("Auth = %q, want %q (--no-poll skips auth)", resp.Auth, "unknown")
	}
}

func TestCLI_StatusNoPollWithCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binary := buildTestBinary(t)

	// Write a cache file, then point config at it.
	cachePath := t.TempDir() + "/quota.json"
	q := &cache.QuotaCache{
		Utilization5h: 0.55,
		Utilization7d: 0.30,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	if err := cache.Write(cachePath, q); err != nil {
		t.Fatalf("cache write: %v", err)
	}

	cfgPath := writeTempConfig(t, "cache:\n  path: "+cachePath+"\n")

	out, err := exec.Command(binary, "--status", "--no-poll", "--config", cfgPath).CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\noutput: %s", err, out)
	}

	var resp StatusResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}

	if resp.CPct != 55 {
		t.Errorf("CPct = %d, want 55", resp.CPct)
	}
	if resp.WPct != 30 {
		t.Errorf("WPct = %d, want 30", resp.WPct)
	}
	if resp.Error != "" {
		t.Errorf("Error = %q, want empty", resp.Error)
	}
}

// --- test helpers ---

func buildTestBinary(t *testing.T) string {
	t.Helper()
	binary := t.TempDir() + "/claude-usage-test"
	cmd := exec.Command("go", "build", "-o", binary, "./")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func buildTestBinaryWithLdflags(t *testing.T, ver, cmt, dt, builder string) string {
	t.Helper()
	binary := t.TempDir() + "/claude-usage-test"
	ldflags := strings.Join([]string{
		"-X main.version=" + ver,
		"-X main.commit=" + cmt,
		"-X main.date=" + dt,
		"-X main.builtBy=" + builder,
	}, " ")
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binary, "./")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	f.Close()
	return f.Name()
}
