// Package pricing provides per-model token pricing and cost calculation.
package pricing

import "strings"

// ModelPrice holds pricing for a single model (per 1M tokens, USD).
type ModelPrice struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
}

// DefaultPricing is the built-in pricing table.
// Prices are per 1M tokens. CacheWrite = 5-minute cache write (1.25x input).
// CacheRead = cache hit/refresh (0.1x input).
// Source: https://docs.anthropic.com/en/docs/about-claude/pricing
var DefaultPricing = map[string]ModelPrice{
	// Opus 4.5+ generation ($5/$25)
	"claude-opus-4-8":            {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-7":            {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-6":            {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-5":            {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-5-20251101":   {5.0, 25.0, 6.25, 0.50},
	// Opus 4.0–4.1 generation ($15/$75)
	"claude-opus-4-1":            {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4-1-20250805":   {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4":              {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4-20250514":     {15.0, 75.0, 18.75, 1.50},
	// Sonnet ($3/$15)
	"claude-sonnet-4-6":          {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-5":          {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-5-20250929": {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4":            {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-20250514":   {3.0, 15.0, 3.75, 0.30},
	// Haiku 4.5 ($1/$5)
	"claude-haiku-4-5":           {1.0, 5.0, 1.25, 0.10},
	"claude-haiku-4-5-20251001":  {1.0, 5.0, 1.25, 0.10},
	// Legacy Haiku 3.5 ($0.80/$4)
	"claude-3-5-haiku-20241022":  {0.80, 4.0, 1.0, 0.08},
	// Legacy Sonnet 3.5 ($3/$15)
	"claude-3-5-sonnet-20241022": {3.0, 15.0, 3.75, 0.30},
	"claude-3-5-sonnet-20240620": {3.0, 15.0, 3.75, 0.30},
}

// prefixMap for fallback matching of unknown model variants.
// Longer prefixes first so date-suffixed variants match correctly.
var prefixMap = []struct {
	prefix string
	key    string
}{
	{"claude-opus-4-8", "claude-opus-4-8"},
	{"claude-opus-4-7", "claude-opus-4-7"},
	{"claude-opus-4-6", "claude-opus-4-6"},
	{"claude-opus-4-5", "claude-opus-4-5"},
	{"claude-opus-4-1", "claude-opus-4-1"},
	{"claude-opus-4", "claude-opus-4"},
	{"claude-sonnet-4-6", "claude-sonnet-4-6"},
	{"claude-sonnet-4-5", "claude-sonnet-4-5"},
	{"claude-sonnet-4", "claude-sonnet-4"},
	{"claude-haiku-4-5", "claude-haiku-4-5"},
	{"claude-3-5-sonnet", "claude-3-5-sonnet-20241022"},
	{"claude-3-5-haiku", "claude-3-5-haiku-20241022"},
}

// Get returns pricing for a model. Falls back to prefix matching, then Sonnet.
func Get(model string, overrides map[string]ModelPrice) ModelPrice {
	if overrides != nil {
		if p, ok := overrides[model]; ok {
			return p
		}
	}
	if p, ok := DefaultPricing[model]; ok {
		return p
	}
	for _, pm := range prefixMap {
		if strings.HasPrefix(model, pm.prefix) {
			return DefaultPricing[pm.key]
		}
	}
	return DefaultPricing["claude-sonnet-4"]
}

// Calculate returns cost in USD for a single usage entry.
func Calculate(inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int, model string, overrides map[string]ModelPrice) float64 {
	p := Get(model, overrides)
	return float64(inputTokens)/1e6*p.Input +
		float64(outputTokens)/1e6*p.Output +
		float64(cacheWriteTokens)/1e6*p.CacheWrite +
		float64(cacheReadTokens)/1e6*p.CacheRead
}
