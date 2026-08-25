package usage

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sparkfabrik/claude-usage/internal/cache"
)

func findWindow(windows []cache.Window, key string) (cache.Window, bool) {
	for _, w := range windows {
		if w.Key == key {
			return w, true
		}
	}
	return cache.Window{}, false
}

func TestParseResponseFractionScale(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 0.42, "resets_at": "2026-08-25T09:30:00Z"},
		"seven_day": {"utilization": 0.67, "resets_at": "2026-08-29T01:00:00Z"}
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	session, ok := findWindow(windows, cache.WindowSession)
	if !ok {
		t.Fatal("session window missing")
	}
	if session.Utilization != 0.42 {
		t.Errorf("session utilization = %v, want 0.42", session.Utilization)
	}
	weekly, ok := findWindow(windows, cache.WindowWeekly)
	if !ok {
		t.Fatal("weekly window missing")
	}
	if weekly.Utilization != 0.67 {
		t.Errorf("weekly utilization = %v, want 0.67", weekly.Utilization)
	}
}

func TestParseResponsePercentScale(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 42.0, "resets_at": "2026-08-25T09:30:00Z"},
		"seven_day": {"utilization": 67.0, "resets_at": "2026-08-29T01:00:00Z"}
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	session, _ := findWindow(windows, cache.WindowSession)
	if session.Utilization != 0.42 {
		t.Errorf("session utilization = %v, want 0.42", session.Utilization)
	}
}

// A payload is internally consistent, so a lone 1.0 next to a 73.0 is one
// percent, not one hundred. Deciding the scale per value would report a nearly
// idle session as exhausted.
func TestParseResponseScaleIsDecidedPerPayload(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 1.0, "resets_at": "2026-08-25T09:30:00Z"},
		"seven_day": {"utilization": 73.0, "resets_at": "2026-08-29T01:00:00Z"}
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	session, _ := findWindow(windows, cache.WindowSession)
	if session.Utilization != 0.01 {
		t.Errorf("session utilization = %v, want 0.01", session.Utilization)
	}
}

// The scoped entries participate in the scale decision, so a payload whose flat
// buckets are both below 1 is still read as percent-scaled when a scoped entry
// exceeds it.
func TestParseResponseScopedEntriesDriveScale(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 0.0},
		"seven_day": {"utilization": 1.0},
		"limits": [
			{"kind": "seven_day", "percent": 73.0, "scope": {"model": {"display_name": "Fable"}}}
		]
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	weekly, _ := findWindow(windows, cache.WindowWeekly)
	if weekly.Utilization != 0.01 {
		t.Errorf("weekly utilization = %v, want 0.01", weekly.Utilization)
	}
	scoped, ok := findWindow(windows, "fable:weekly")
	if !ok {
		t.Fatal("scoped window missing")
	}
	if scoped.Utilization != 0.73 {
		t.Errorf("scoped utilization = %v, want 0.73", scoped.Utilization)
	}
}

func TestParseResponseModelScopedWindows(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 0.0, "resets_at": "2026-08-25T09:30:00Z"},
		"seven_day": {"utilization": 0.48, "resets_at": "2026-08-29T01:00:00Z"},
		"limits": [
			{"kind": "seven_day", "percent": 0.73, "resets_at": "2026-08-29T01:00:00Z",
			 "scope": {"model": {"display_name": "Fable", "id": "claude-fable-5"}}},
			{"kind": "five_hour", "percent": 0.10, "resets_at": "2026-08-25T09:30:00Z",
			 "scope": {"model": {"display_name": "Opus", "id": "claude-opus-4-8"}}}
		]
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(windows) != 4 {
		t.Fatalf("got %d windows, want 4", len(windows))
	}

	fable, ok := findWindow(windows, "fable:weekly")
	if !ok {
		t.Fatal("fable weekly window missing")
	}
	if fable.Title != "Fable Weekly" {
		t.Errorf("title = %q, want %q", fable.Title, "Fable Weekly")
	}
	if fable.Model != "Fable" {
		t.Errorf("model = %q, want %q", fable.Model, "Fable")
	}

	opus, ok := findWindow(windows, "opus:session")
	if !ok {
		t.Fatal("opus session window missing")
	}
	if opus.Title != "Opus Session" {
		t.Errorf("title = %q, want %q", opus.Title, "Opus Session")
	}
}

// One model can hold several windows, so the identity is model and kind
// together, and a repeated pair is reported once.
func TestParseResponseDeduplicatesScopedWindows(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 0.0},
		"limits": [
			{"kind": "seven_day", "percent": 0.73, "scope": {"model": {"display_name": "Fable"}}},
			{"kind": "seven_day", "percent": 0.99, "scope": {"model": {"display_name": "Fable"}}}
		]
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	count := 0
	for _, w := range windows {
		if w.Key == "fable:weekly" {
			count++
			if w.Utilization != 0.73 {
				t.Errorf("kept utilization %v, want the first (0.73)", w.Utilization)
			}
		}
	}
	if count != 1 {
		t.Errorf("fable:weekly appeared %d times, want 1", count)
	}
}

func TestParseResponsePrefersOAuthAppsWeekly(t *testing.T) {
	body := []byte(`{
		"seven_day": {"utilization": 0.10},
		"seven_day_oauth_apps": {"utilization": 0.48}
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	weekly, _ := findWindow(windows, cache.WindowWeekly)
	if weekly.Utilization != 0.48 {
		t.Errorf("weekly utilization = %v, want 0.48 from seven_day_oauth_apps", weekly.Utilization)
	}
}

func TestParseResponseRejectsEmptyPayload(t *testing.T) {
	if _, err := ParseResponse([]byte(`{}`)); err == nil {
		t.Fatal("expected an error for a payload with no windows")
	}
}

func TestParseResponseRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseResponse([]byte(`not json`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

// A model whose display name reads like a window ("Opus 5 (1M context)") must
// not have its title parsed by consumers; the producer resolves it here.
func TestParseResponseTitleSurvivesAwkwardModelNames(t *testing.T) {
	body := []byte(`{
		"five_hour": {"utilization": 0.0},
		"limits": [
			{"kind": "seven_day", "percent": 0.5,
			 "scope": {"model": {"display_name": "Opus 5 (1M context)"}}}
		]
	}`)

	windows, err := ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	w, ok := findWindow(windows, "opus 5 (1m context):weekly")
	if !ok {
		t.Fatal("scoped window missing")
	}
	if w.Title != "Opus 5 (1M context) Weekly" {
		t.Errorf("title = %q", w.Title)
	}
}

func TestNormalizeResetAtAcceptsEpochs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"epoch seconds", `1787654400`, "2026-08-25T10:40:00Z"},
		{"epoch millis", `1787654400000`, "2026-08-25T10:40:00Z"},
		{"rfc3339", `"2026-08-25T09:20:00Z"`, "2026-08-25T09:20:00Z"},
		{"empty string", `""`, ""},
		{"garbage", `"tomorrow"`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeResetAt([]byte(tc.raw))
			if got == "" && tc.want == "" {
				return
			}
			parsed, err := time.Parse(time.RFC3339Nano, got)
			if err != nil {
				t.Fatalf("unparseable result %q: %v", got, err)
			}
			if parsed.UTC().Format(time.RFC3339) != tc.want {
				t.Errorf("got %s, want %s", parsed.UTC().Format(time.RFC3339), tc.want)
			}
		})
	}
}

func TestParseNumberAcceptsStringsAndPercentSigns(t *testing.T) {
	tests := []struct {
		raw  string
		want float64
		ok   bool
	}{
		{`42.5`, 42.5, true},
		{`"42.5"`, 42.5, true},
		{`"42.5%"`, 42.5, true},
		{`" 42.5 % "`, 42.5, true},
		{`"abc"`, 0, false},
		{`null`, 0, false},
	}

	for _, tc := range tests {
		got, ok := parseNumber([]byte(tc.raw))
		if ok != tc.ok {
			t.Errorf("parseNumber(%s) ok = %v, want %v", tc.raw, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("parseNumber(%s) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestWindowNameMapsKinds(t *testing.T) {
	tests := map[string]string{
		"five_hour":            "Session",
		"seven_day":            "Weekly",
		"seven_day_oauth_apps": "Weekly",
		"thirty_day":           "Weekly",
		"monthly":              "Monthly",
		"session":              "Session",
		"":                     "",
		"fortnight":            "",
	}

	for kind, want := range tests {
		if got := windowName(kind); got != want {
			t.Errorf("windowName(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestFetchSendsOAuthHeaders(t *testing.T) {
	var gotAuth, gotBeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBeta = r.Header.Get("anthropic-beta")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":0.5},"seven_day":{"utilization":0.25}}`))
	}))
	defer srv.Close()

	windows, err := Fetch("tok-123", srv.URL, time.Second)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotBeta != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q", gotBeta)
	}
	if len(windows) != 2 {
		t.Errorf("got %d windows, want 2", len(windows))
	}
}

func TestFetchReportsStatusWithoutLeakingBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"secret account detail"}`))
	}))
	defer srv.Close()

	_, err := Fetch("tok", srv.URL, time.Second)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != "usage endpoint returned 401" {
		t.Errorf("error = %q, want the status only", got)
	}
}

func TestFetchRequiresToken(t *testing.T) {
	if _, err := Fetch("", "", time.Second); err == nil {
		t.Fatal("expected an error without an access token")
	}
}

func TestCollectFallsBackToHeaderPoll(t *testing.T) {
	// The usage endpoint is down.
	usageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer usageSrv.Close()

	// The messages endpoint answers with rate-limit headers.
	pollSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", "0.3")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", "0.6")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer pollSrv.Close()

	q, err := Collect("tok", usageSrv.URL, "", pollSrv.URL, time.Second)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if q.Source != cache.SourceHeaders {
		t.Errorf("source = %q, want %q", q.Source, cache.SourceHeaders)
	}
	if q.Utilization5h != 0.3 || q.Utilization7d != 0.6 {
		t.Errorf("utilizations = %v/%v, want 0.3/0.6", q.Utilization5h, q.Utilization7d)
	}
	// The fallback must still expose windows, so consumers need only one shape.
	if len(q.Windows) != 2 {
		t.Errorf("got %d windows, want 2", len(q.Windows))
	}
}

func TestCollectPrefersUsageEndpoint(t *testing.T) {
	usageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":0.11},"seven_day":{"utilization":0.22},
			"limits":[{"kind":"seven_day","percent":0.73,"scope":{"model":{"display_name":"Fable"}}}]}`))
	}))
	defer usageSrv.Close()

	pollSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("header poll must not run when the usage endpoint answers")
	}))
	defer pollSrv.Close()

	q, err := Collect("tok", usageSrv.URL, "", pollSrv.URL, time.Second)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if q.Source != cache.SourceOAuth {
		t.Errorf("source = %q, want %q", q.Source, cache.SourceOAuth)
	}
	if q.Utilization5h != 0.11 || q.Utilization7d != 0.22 {
		t.Errorf("flat fields not mirrored: %v/%v", q.Utilization5h, q.Utilization7d)
	}
	if len(q.Windows) != 3 {
		t.Errorf("got %d windows, want 3 including the scoped one", len(q.Windows))
	}
}

func TestCollectReportsBothFailures(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()

	_, err := Collect("tok", down.URL, "", down.URL, time.Second)
	if err == nil {
		t.Fatal("expected an error when both paths fail")
	}
}
