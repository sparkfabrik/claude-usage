package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/Monska85/claude-usage/internal/analyzer"
	"github.com/Monska85/claude-usage/internal/auth"
	"github.com/Monska85/claude-usage/internal/cache"
	"github.com/Monska85/claude-usage/internal/config"
)

func TestDimText(t *testing.T) {
	out := DimText("hello")
	if out == "" {
		t.Error("DimText should return non-empty string")
	}
	// The rendered string contains ANSI codes wrapping "hello"
	if !strings.Contains(out, "hello") {
		t.Errorf("DimText should contain original text, got %q", out)
	}
}

func TestFormatTimeRemaining(t *testing.T) {
	tests := []struct {
		name    string
		minutes *float64
		want    string
	}{
		{"nil", nil, "?"},
		{"less than 1", ptrF(0.5), "<1m"},
		{"minutes only", ptrF(45), "45m"},
		{"hours and minutes", ptrF(90), "1h30m"},
		{"days and hours", ptrF(1500), "1d01h"},
		{"exact hour", ptrF(60), "1h00m"},
		{"zero", ptrF(0), "<1m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimeRemaining(tt.minutes)
			if got != tt.want {
				t.Errorf("FormatTimeRemaining(%v) = %q, want %q", tt.minutes, got, tt.want)
			}
		})
	}
}

func TestRenderAccount_NilCreds(t *testing.T) {
	out := RenderAccount(nil)
	if !strings.Contains(out, "No Claude Code login") {
		t.Errorf("expected login message, got %q", out)
	}
}

func TestRenderAccount_ValidCreds(t *testing.T) {
	creds := &auth.Credentials{
		SubscriptionType: "pro",
		RateLimitTier:    "t5",
		ExpiresAt:        time.Now().Add(1 * time.Hour).UnixMilli(),
	}
	out := RenderAccount(creds)
	if !strings.Contains(out, "pro") {
		t.Errorf("expected 'pro' in output, got %q", out)
	}
	if !strings.Contains(out, "t5") {
		t.Errorf("expected 't5' in output, got %q", out)
	}
}

func TestRenderAccount_ExpiredCreds(t *testing.T) {
	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli(),
	}
	out := RenderAccount(creds)
	if !strings.Contains(out, "expired") {
		t.Errorf("expected 'expired' in output, got %q", out)
	}
}

func TestRenderAccount_EmptyFields(t *testing.T) {
	creds := &auth.Credentials{
		ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli(),
	}
	out := RenderAccount(creds)
	if !strings.Contains(out, "unknown") {
		t.Errorf("expected 'unknown' for empty fields, got %q", out)
	}
}

func TestRenderQuota_Nil(t *testing.T) {
	cfg := config.Default()
	out := RenderQuota(nil, cfg)
	if !strings.Contains(out, "No quota data") {
		t.Errorf("expected 'No quota data', got %q", out)
	}
}

func TestRenderQuota_WithData(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.42,
		Utilization7d: 0.67,
		PolledAt:      time.Now().Format(time.RFC3339Nano),
		Reset5h:       time.Now().Add(1 * time.Hour).Format(time.RFC3339Nano),
		Reset7d:       time.Now().Add(24 * time.Hour).Format(time.RFC3339Nano),
	}
	out := RenderQuota(q, cfg)
	if !strings.Contains(out, "Rate Limits") {
		t.Errorf("expected 'Rate Limits' header, got %q", out)
	}
	if !strings.Contains(out, "42.0%") {
		t.Errorf("expected '42.0%%', got %q", out)
	}
}

func TestRenderQuota_HighUtilization(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.95, // > OrangeBelow (90) → red
		Utilization7d: 0.85, // > GreenBelow (80) → orange
		PolledAt:      time.Now().Format(time.RFC3339Nano),
		Reset5h:       time.Now().Add(30 * time.Minute).Format(time.RFC3339Nano),
		Reset7d:       time.Now().Add(48 * time.Hour).Format(time.RFC3339Nano),
	}
	out := RenderQuota(q, cfg)
	if !strings.Contains(out, "95.0%") {
		t.Errorf("expected '95.0%%', got %q", out)
	}
	if !strings.Contains(out, "85.0%") {
		t.Errorf("expected '85.0%%', got %q", out)
	}
}

func TestRenderQuota_Stale(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.10,
		PolledAt:      time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano),
	}
	out := RenderQuota(q, cfg)
	if !strings.Contains(out, "(?)") {
		t.Errorf("expected stale marker '(?)', got %q", out)
	}
}

func TestRenderQuota_WithStatus(t *testing.T) {
	cfg := config.Default()
	q := &cache.QuotaCache{
		Utilization5h: 0.10,
		Status5h:      "limited",
		PolledAt:      time.Now().Format(time.RFC3339Nano),
	}
	out := RenderQuota(q, cfg)
	if !strings.Contains(out, "limited") {
		t.Errorf("expected 'limited' status, got %q", out)
	}
}

func TestRenderUsageTable_Empty(t *testing.T) {
	out := RenderUsageTable(nil, false)
	if !strings.Contains(out, "No usage data") {
		t.Errorf("expected 'No usage data', got %q", out)
	}
}

func TestRenderUsageTable_WithData(t *testing.T) {
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	summaries := []*analyzer.Summary{
		{
			PeriodLabel:         "today",
			TotalInputTokens:    10000,
			TotalOutputTokens:   5000,
			TotalCacheWriteTokens: 2000,
			TotalCacheReadTokens:  1000,
			TotalCostUSD:        0.15,
			MessageCount:        5,
			StartTime:           &start,
			EndTime:             &now,
		},
	}

	out := RenderUsageTable(summaries, true)
	if !strings.Contains(out, "Usage by Period") {
		t.Errorf("expected 'Usage by Period' header")
	}
	if !strings.Contains(out, "today") {
		t.Errorf("expected 'today' period")
	}
	if !strings.Contains(out, "$0.15") {
		t.Errorf("expected cost '$0.15'")
	}
}

func TestRenderUsageTable_NoCost(t *testing.T) {
	now := time.Now()
	summaries := []*analyzer.Summary{
		{
			PeriodLabel:      "today",
			TotalInputTokens: 1000,
			MessageCount:     1,
			StartTime:        &now,
			EndTime:          &now,
		},
	}

	out := RenderUsageTable(summaries, false)
	if strings.Contains(out, "Cost") {
		t.Errorf("should not contain Cost header when showCost=false")
	}
}

func TestRenderModelTable_Empty(t *testing.T) {
	out := RenderModelTable(nil, false)
	if out != "" {
		t.Errorf("expected empty string for nil summaries, got %q", out)
	}
}

func TestRenderModelTable_WithData(t *testing.T) {
	now := time.Now()
	summaries := []*analyzer.Summary{
		{
			PeriodLabel:           "claude-sonnet-4",
			TotalInputTokens:     5000,
			TotalOutputTokens:    2000,
			TotalCacheWriteTokens: 3000,
			TotalCacheReadTokens:  1000,
			MessageCount:         3,
			TotalCostUSD:         0.10,
			StartTime:            &now,
			EndTime:              &now,
		},
	}

	out := RenderModelTable(summaries, true)
	if !strings.Contains(out, "Usage by Model") {
		t.Errorf("expected 'Usage by Model' header")
	}
	if !strings.Contains(out, "claude-sonnet-4") {
		t.Errorf("expected model name in output")
	}
	if !strings.Contains(out, "Cache W/R") {
		t.Errorf("expected 'Cache W/R' column header")
	}
	if !strings.Contains(out, "$0.10") {
		t.Errorf("expected cost '$0.10'")
	}
}

func TestRenderModelTable_NoCost(t *testing.T) {
	now := time.Now()
	summaries := []*analyzer.Summary{
		{
			PeriodLabel:      "claude-opus-4-7",
			TotalInputTokens: 1000,
			MessageCount:     1,
			StartTime:        &now,
			EndTime:          &now,
		},
	}

	out := RenderModelTable(summaries, false)
	if strings.Contains(out, "Cost") {
		t.Errorf("should not contain Cost header when showCost=false")
	}
	if !strings.Contains(out, "Cache W/R") {
		t.Errorf("expected 'Cache W/R' column even without cost")
	}
}

func TestRenderModelTable_ColumnsMatchPeriodTable(t *testing.T) {
	// Both tables should have the same column structure (except first col name).
	now := time.Now()
	summary := &analyzer.Summary{
		PeriodLabel:           "test",
		TotalInputTokens:     10000,
		TotalOutputTokens:    5000,
		TotalCacheWriteTokens: 2000,
		TotalCacheReadTokens:  1000,
		MessageCount:         5,
		TotalCostUSD:         1.23,
		StartTime:            &now,
		EndTime:              &now,
	}

	periodOut := RenderUsageTable([]*analyzer.Summary{summary}, true)
	modelOut := RenderModelTable([]*analyzer.Summary{summary}, true)

	// Both should have these columns
	for _, col := range []string{"Msgs", "Input", "Output", "Cache W/R", "Total", "Cost"} {
		if !strings.Contains(periodOut, col) {
			t.Errorf("period table missing column %q", col)
		}
		if !strings.Contains(modelOut, col) {
			t.Errorf("model table missing column %q", col)
		}
	}
}

func TestRenderBurnRate_Empty(t *testing.T) {
	out := RenderBurnRate(nil)
	if out != "" {
		t.Errorf("expected empty string, got %q", out)
	}
}

func TestRenderBurnRate_NegativeRate(t *testing.T) {
	summaries := []*analyzer.Summary{
		{PeriodLabel: "today"},
	}
	out := RenderBurnRate(summaries)
	if out != "" {
		t.Errorf("expected empty for negative rate, got %q", out)
	}
}

func TestRenderBurnRate_ValidRate(t *testing.T) {
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now()
	summaries := []*analyzer.Summary{
		{
			PeriodLabel:      "today",
			TotalInputTokens: 15000,
			StartTime:        &start,
			EndTime:          &end,
		},
	}
	out := RenderBurnRate(summaries)
	if out == "" {
		t.Error("expected non-empty burn rate output")
	}
	if !strings.Contains(out, "Burn rate") {
		t.Errorf("expected 'Burn rate' in output, got %q", out)
	}
}

// --- helpers ---

func ptrF(f float64) *float64 { return &f }
