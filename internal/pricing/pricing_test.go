package pricing

import (
	"math"
	"testing"
)

func TestGet_ExactMatch(t *testing.T) {
	p := Get("claude-sonnet-4", nil)
	if p.Input != 3.0 {
		t.Errorf("Input = %f, want 3.0", p.Input)
	}
	if p.Output != 15.0 {
		t.Errorf("Output = %f, want 15.0", p.Output)
	}
}

func TestGet_PrefixFallback(t *testing.T) {
	// Unknown variant should match prefix
	p := Get("claude-sonnet-4-20260101", nil)
	if p.Input != 3.0 {
		t.Errorf("Input = %f, want 3.0 (sonnet prefix fallback)", p.Input)
	}
}

func TestGet_OpusPrefix(t *testing.T) {
	// claude-opus-4-20260101 matches "claude-opus-4" prefix → deprecated opus pricing
	p := Get("claude-opus-4-20260101", nil)
	if p.Input != 15.0 {
		t.Errorf("Input = %f, want 15.0 (deprecated opus)", p.Input)
	}
}

func TestGet_Opus47(t *testing.T) {
	p := Get("claude-opus-4-7", nil)
	if p.Input != 5.0 {
		t.Errorf("Input = %f, want 5.0 (opus 4.7)", p.Input)
	}
	if p.Output != 25.0 {
		t.Errorf("Output = %f, want 25.0 (opus 4.7)", p.Output)
	}
}

func TestGet_Opus47DateVariant(t *testing.T) {
	p := Get("claude-opus-4-7-20260301", nil)
	if p.Input != 5.0 {
		t.Errorf("Input = %f, want 5.0 (opus 4.7 prefix)", p.Input)
	}
}

func TestGet_Opus48(t *testing.T) {
	p := Get("claude-opus-4-8", nil)
	if p.Input != 5.0 {
		t.Errorf("Input = %f, want 5.0 (opus 4.8)", p.Input)
	}
}

func TestGet_Opus41(t *testing.T) {
	p := Get("claude-opus-4-1", nil)
	if p.Input != 15.0 {
		t.Errorf("Input = %f, want 15.0 (opus 4.1)", p.Input)
	}
}

func TestGet_HaikuPrefix(t *testing.T) {
	p := Get("claude-haiku-4-5-20260101", nil)
	if p.Input != 1.0 {
		t.Errorf("Input = %f, want 1.0 (haiku 4.5)", p.Input)
	}
	if p.Output != 5.0 {
		t.Errorf("Output = %f, want 5.0 (haiku 4.5)", p.Output)
	}
}

func TestGet_UnknownModel(t *testing.T) {
	// Completely unknown model falls back to sonnet
	p := Get("completely-unknown-model", nil)
	if p.Input != 3.0 {
		t.Errorf("Input = %f, want 3.0 (default sonnet fallback)", p.Input)
	}
}

func TestGet_OverridesTakePriority(t *testing.T) {
	overrides := map[string]ModelPrice{
		"claude-sonnet-4": {Input: 99.0, Output: 99.0, CacheWrite: 99.0, CacheRead: 99.0},
	}
	p := Get("claude-sonnet-4", overrides)
	if p.Input != 99.0 {
		t.Errorf("Input = %f, want 99.0 (override)", p.Input)
	}
}

func TestGet_OverrideOnlyForSpecificModel(t *testing.T) {
	overrides := map[string]ModelPrice{
		"claude-opus-4": {Input: 99.0},
	}
	// Sonnet should still use default
	p := Get("claude-sonnet-4", overrides)
	if p.Input != 3.0 {
		t.Errorf("Input = %f, want 3.0 (not overridden)", p.Input)
	}
}

func TestGet_LegacySonnet(t *testing.T) {
	p := Get("claude-3-5-sonnet-20241022", nil)
	if p.Input != 3.0 {
		t.Errorf("Input = %f, want 3.0", p.Input)
	}
}

func TestGet_LegacySonnetPrefix(t *testing.T) {
	p := Get("claude-3-5-sonnet-99999999", nil)
	if p.Input != 3.0 {
		t.Errorf("Input = %f, want 3.0 (legacy prefix fallback)", p.Input)
	}
}

func TestCalculate(t *testing.T) {
	cost := Calculate(1_000_000, 500_000, 200_000, 100_000, "claude-sonnet-4", nil)
	// 1M * 3.0/1M + 500K * 15.0/1M + 200K * 3.75/1M + 100K * 0.30/1M
	// = 3.0 + 7.5 + 0.75 + 0.03 = 11.28
	want := 11.28
	if math.Abs(cost-want) > 0.01 {
		t.Errorf("Calculate = %f, want %f", cost, want)
	}
}

func TestCalculate_ZeroTokens(t *testing.T) {
	cost := Calculate(0, 0, 0, 0, "claude-sonnet-4", nil)
	if cost != 0 {
		t.Errorf("Calculate(0,0,0,0) = %f, want 0", cost)
	}
}

func TestCalculate_WithOverrides(t *testing.T) {
	overrides := map[string]ModelPrice{
		"test-model": {Input: 10.0, Output: 10.0, CacheWrite: 10.0, CacheRead: 10.0},
	}
	cost := Calculate(1_000_000, 0, 0, 0, "test-model", overrides)
	if math.Abs(cost-10.0) > 0.01 {
		t.Errorf("Calculate = %f, want 10.0", cost)
	}
}

func TestDefaultPricing_AllModelsPresent(t *testing.T) {
	models := []string{
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-opus-4-5",
		"claude-opus-4-5-20251101",
		"claude-opus-4-1",
		"claude-opus-4-1-20250805",
		"claude-opus-4",
		"claude-opus-4-20250514",
		"claude-sonnet-4-6",
		"claude-sonnet-4-5",
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4",
		"claude-sonnet-4-20250514",
		"claude-haiku-4-5",
		"claude-haiku-4-5-20251001",
		"claude-3-5-haiku-20241022",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-20240620",
	}
	for _, m := range models {
		if _, ok := DefaultPricing[m]; !ok {
			t.Errorf("missing pricing for %q", m)
		}
	}
}

func TestGet_Claude5Generation(t *testing.T) {
	tests := []struct {
		model                 string
		input, output, cw, cr float64
	}{
		{"claude-fable-5-1", 10.0, 50.0, 12.5, 0.25},
		{"claude-fable-5-1-20260901", 10.0, 50.0, 12.5, 0.25},
		{"claude-mythos-5-1", 10.0, 50.0, 12.5, 0.25},
		{"claude-fable-5", 10.0, 50.0, 12.5, 1.0},
		{"claude-opus-5-5", 4.0, 20.0, 5.0, 0.20},
		{"claude-opus-5-5-20260701", 4.0, 20.0, 5.0, 0.20},
		{"claude-opus-5", 5.0, 25.0, 6.25, 0.50},
		{"claude-sonnet-5-5", 2.0, 10.0, 2.5, 0.10},
		{"claude-sonnet-5", 2.0, 10.0, 2.5, 0.20},
		{"claude-haiku-5-5", 0.10, 0.50, 0.125, 0.01},
		{"claude-haiku-5-5-20260301", 0.10, 0.50, 0.125, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p := Get(tt.model, nil)
			if p.Input != tt.input || p.Output != tt.output || p.CacheWrite != tt.cw || p.CacheRead != tt.cr {
				t.Errorf("Get(%q) = %+v, want %v/%v/%v/%v", tt.model, p, tt.input, tt.output, tt.cw, tt.cr)
			}
		})
	}
}

// The longer prefix must win: claude-opus-5-5 is not priced as claude-opus-5.
func TestGet_LongerPrefixWins(t *testing.T) {
	if p := Get("claude-opus-5-5-20260701", nil); p.Input != 4.0 {
		t.Errorf("opus 5.5 variant priced at %f, want 4.0", p.Input)
	}
	if p := Get("claude-fable-5-1-x", nil); p.CacheRead != 0.25 {
		t.Errorf("fable 5.1 variant cache read %f, want 0.25", p.CacheRead)
	}
}

func TestKnown(t *testing.T) {
	if !Known("claude-opus-5-5", nil) {
		t.Error("exact row should be known")
	}
	if !Known("claude-opus-5-5-20260701", nil) {
		t.Error("prefix match should be known")
	}
	if Known("completely-unknown-model", nil) {
		t.Error("unknown model should not be known")
	}
	overrides := map[string]ModelPrice{"completely-unknown-model": {Input: 1}}
	if !Known("completely-unknown-model", overrides) {
		t.Error("overridden model should be known")
	}
}

func TestCalculate_Fable51(t *testing.T) {
	cost := Calculate(1_000_000, 100_000, 200_000, 4_000_000, "claude-fable-5-1", nil)
	// 10 + 5 + 2.5 + 1.0 = 18.5
	if math.Abs(cost-18.5) > 0.0001 {
		t.Errorf("Calculate = %f, want 18.5", cost)
	}
}
