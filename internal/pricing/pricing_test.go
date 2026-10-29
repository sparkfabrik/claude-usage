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
	cost := Calculate(1_000_000, 500_000, 200_000, 0, 100_000, "claude-sonnet-4", nil)
	// 1M * 3.0/1M + 500K * 15.0/1M + 200K * 3.75/1M + 100K * 0.30/1M
	// = 3.0 + 7.5 + 0.75 + 0.03 = 11.28
	want := 11.28
	if math.Abs(cost-want) > 0.01 {
		t.Errorf("Calculate = %f, want %f", cost, want)
	}
}

func TestCalculate_ZeroTokens(t *testing.T) {
	cost := Calculate(0, 0, 0, 0, 0, "claude-sonnet-4", nil)
	if cost != 0 {
		t.Errorf("Calculate(0,0,0,0,0) = %f, want 0", cost)
	}
}

func TestCalculate_WithOverrides(t *testing.T) {
	overrides := map[string]ModelPrice{
		"test-model": {Input: 10.0, Output: 10.0, CacheWrite: 10.0, CacheRead: 10.0},
	}
	cost := Calculate(1_000_000, 0, 0, 0, 0, "test-model", overrides)
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
		model                       string
		input, output, cw, cw1h, cr float64
	}{
		{"claude-fable-5-1", 10.0, 50.0, 12.5, 20.0, 0.25},
		{"claude-fable-5-1-20260901", 10.0, 50.0, 12.5, 20.0, 0.25},
		{"claude-mythos-5-1", 10.0, 50.0, 12.5, 20.0, 0.25},
		{"claude-fable-5", 10.0, 50.0, 12.5, 20.0, 1.0},
		{"claude-opus-5-5", 4.0, 20.0, 5.0, 8.0, 0.20},
		{"claude-opus-5-5-20260701", 4.0, 20.0, 5.0, 8.0, 0.20},
		{"claude-opus-5", 5.0, 25.0, 6.25, 10.0, 0.50},
		{"claude-sonnet-5-5", 2.0, 10.0, 2.5, 4.0, 0.10},
		{"claude-sonnet-5", 2.0, 10.0, 2.5, 4.0, 0.20},
		{"claude-haiku-5-5", 0.10, 0.50, 0.125, 0.20, 0.01},
		{"claude-haiku-5-5-20260301", 0.10, 0.50, 0.125, 0.20, 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p := Get(tt.model, nil)
			if p.Input != tt.input || p.Output != tt.output || p.CacheWrite != tt.cw || p.CacheWrite1h != tt.cw1h || p.CacheRead != tt.cr {
				t.Errorf("Get(%q) = %+v, want %v/%v/%v/%v/%v", tt.model, p, tt.input, tt.output, tt.cw, tt.cw1h, tt.cr)
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
	cost := Calculate(1_000_000, 100_000, 200_000, 0, 4_000_000, "claude-fable-5-1", nil)
	// 10 + 5 + 2.5 + 1.0 = 18.5
	if math.Abs(cost-18.5) > 0.0001 {
		t.Errorf("Calculate = %f, want 18.5", cost)
	}
}

// Every row of the pricing page prices a 1-hour cache write at 2x input and a
// 5-minute one at 1.25x; the table must keep that shape.
func TestDefaultPricing_CacheWriteRates(t *testing.T) {
	for model, p := range DefaultPricing {
		if math.Abs(p.CacheWrite1h-2*p.Input) > 1e-9 {
			t.Errorf("%s: CacheWrite1h = %v, want 2x input = %v", model, p.CacheWrite1h, 2*p.Input)
		}
		if math.Abs(p.CacheWrite-1.25*p.Input) > 1e-9 {
			t.Errorf("%s: CacheWrite = %v, want 1.25x input = %v", model, p.CacheWrite, 1.25*p.Input)
		}
	}
}

// An override from the config file may leave the 1-hour rate out; it is then
// 1.6x the 5-minute rate, the ratio of 2x to 1.25x input.
func TestGet_OverrideDerivesCacheWrite1h(t *testing.T) {
	overrides := map[string]ModelPrice{
		"m": {Input: 10, Output: 50, CacheWrite: 12.5, CacheRead: 1},
	}
	if p := Get("m", overrides); math.Abs(p.CacheWrite1h-20.0) > 1e-9 {
		t.Errorf("CacheWrite1h = %v, want 20.0 derived from 12.5", p.CacheWrite1h)
	}
	overrides["m"] = ModelPrice{Input: 10, Output: 50, CacheWrite: 12.5, CacheWrite1h: 33, CacheRead: 1}
	if p := Get("m", overrides); p.CacheWrite1h != 33 {
		t.Errorf("CacheWrite1h = %v, want the explicit 33", p.CacheWrite1h)
	}
}

func TestCalculate_CacheWrite1hAtItsOwnRate(t *testing.T) {
	// Fable 5.1: 1M cache writes, 400K of them 1-hour.
	// 600K * 12.5 + 400K * 20 per million = 7.5 + 8 = 15.5
	cost := Calculate(0, 0, 1_000_000, 400_000, 0, "claude-fable-5-1", nil)
	if math.Abs(cost-15.5) > 1e-9 {
		t.Errorf("Calculate = %f, want 15.5", cost)
	}
	// All 1-hour: 20.
	if cost := Calculate(0, 0, 1_000_000, 1_000_000, 0, "claude-fable-5-1", nil); math.Abs(cost-20.0) > 1e-9 {
		t.Errorf("all 1h: Calculate = %f, want 20.0", cost)
	}
	// None 1-hour: unchanged from before, 12.5.
	if cost := Calculate(0, 0, 1_000_000, 0, 0, "claude-fable-5-1", nil); math.Abs(cost-12.5) > 1e-9 {
		t.Errorf("no 1h: Calculate = %f, want 12.5", cost)
	}
}

// More 1-hour tokens than cache writes cannot happen in a well-formed line;
// the excess is clamped so the cost never exceeds the all-1-hour price.
func TestCalculate_CacheWrite1hClampedToTotal(t *testing.T) {
	cost := Calculate(0, 0, 1_000_000, 5_000_000, 0, "claude-fable-5-1", nil)
	if math.Abs(cost-20.0) > 1e-9 {
		t.Errorf("Calculate = %f, want 20.0 (clamped)", cost)
	}
}

// Haiku 5.5 is the only model with a long-prompt tier; every other row is
// one flat rate across its context window.
func TestDefaultPricing_OnlyHaiku55HasTier(t *testing.T) {
	for model, p := range DefaultPricing {
		if model == "claude-haiku-5-5" {
			if p.LongPrompt == nil || p.LongPrompt.Threshold != 100_000 {
				t.Errorf("%s: want a 100K tier, got %+v", model, p.LongPrompt)
			}
			continue
		}
		if p.LongPrompt != nil {
			t.Errorf("%s: unexpected long-prompt tier", model)
		}
	}
	over := Get("claude-haiku-5-5", nil).LongPrompt.Price
	want := ModelPrice{Input: 0.50, Output: 2.50, CacheWrite: 0.625, CacheWrite1h: 1.0, CacheRead: 0.05}
	if over != want {
		t.Errorf("tier price = %+v, want %+v", over, want)
	}
	if Get("claude-haiku-5-5-20260301", nil).LongPrompt == nil {
		t.Error("a date variant resolves to the row with the tier")
	}
}

// The prompt is every input token of the request, cache reads and writes
// included; at the threshold the base rates apply, one token over the tier
// rates apply to everything, output included.
func TestCalculate_Haiku55PromptTier(t *testing.T) {
	const m = "claude-haiku-5-5"
	// 100,000 exactly: base rates. 50K input + 30K write + 20K read + 1K out
	// = 50K*0.10 + 30K*0.125 + 20K*0.01 + 1K*0.50 per million = 0.009450
	if got, want := Calculate(50_000, 1_000, 30_000, 0, 20_000, m, nil), 0.00945; math.Abs(got-want) > 1e-12 {
		t.Errorf("at threshold: %f, want %f", got, want)
	}
	// One cache-read token more: everything at 5x = 0.047255 (20,001 reads).
	if got, want := Calculate(50_000, 1_000, 30_000, 0, 20_001, m, nil), 0.047250+0.00000005; math.Abs(got-want) > 1e-12 {
		t.Errorf("over threshold: %f, want %f", got, want)
	}
	// Input alone over the threshold, 1-hour writes at the tier's 1-hour rate:
	// 100,001 in * 0.50 + 1M 1h writes * 1.0 = 0.0500005 + 1.0
	if got, want := Calculate(100_001, 0, 1_000_000, 1_000_000, 0, m, nil), 1.0500005; math.Abs(got-want) > 1e-12 {
		t.Errorf("input over threshold with 1h writes: %f, want %f", got, want)
	}
	// Flat models ignore prompt length: Fable at 900K input is still $10/M.
	if got, want := Calculate(900_000, 0, 0, 0, 0, "claude-fable-5-1", nil), 9.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("flat model: %f, want %f", got, want)
	}
}

// An override without a tier has one flat rate even for Haiku 5.5; an
// override with a tier uses it, deriving its 1-hour rate like the base one.
func TestGet_OverrideTier(t *testing.T) {
	flat := map[string]ModelPrice{"claude-haiku-5-5": {Input: 1, Output: 1, CacheWrite: 1, CacheRead: 1}}
	if Get("claude-haiku-5-5", flat).LongPrompt != nil {
		t.Error("override without a tier should have none")
	}
	if got := Calculate(200_000, 0, 0, 0, 0, "claude-haiku-5-5", flat); math.Abs(got-0.2) > 1e-12 {
		t.Errorf("flat override over 100K: %f, want 0.2", got)
	}
	withTier := map[string]ModelPrice{"m": {
		Input: 1, Output: 1, CacheWrite: 1, CacheRead: 1,
		LongPrompt: &PromptTier{Threshold: 10, Price: ModelPrice{Input: 5, Output: 5, CacheWrite: 5, CacheRead: 5}},
	}}
	p := Get("m", withTier)
	if p.LongPrompt == nil || math.Abs(p.LongPrompt.Price.CacheWrite1h-8.0) > 1e-9 {
		t.Errorf("tier CacheWrite1h should derive to 8.0, got %+v", p.LongPrompt)
	}
	if withTier["m"].LongPrompt.Price.CacheWrite1h != 0 {
		t.Error("Get must not mutate the caller's override")
	}
	if got := Calculate(11, 0, 0, 0, 0, "m", withTier); math.Abs(got-11*5/1e6) > 1e-15 {
		t.Errorf("tier override: %g, want %g", got, 11*5/1e6)
	}
}
