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
var DefaultPricing = map[string]ModelPrice{
	// Opus
	"claude-opus-4":   {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4-7": {15.0, 75.0, 18.75, 1.50},
	// Sonnet
	"claude-sonnet-4":              {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-5":            {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-5-20250514":   {3.0, 15.0, 3.75, 0.30},
	// Haiku
	"claude-haiku-4-5":             {0.80, 4.0, 1.0, 0.08},
	"claude-haiku-4-5-20251001":    {0.80, 4.0, 1.0, 0.08},
	// Legacy Sonnet 3.5
	"claude-3-5-sonnet-20241022":   {3.0, 15.0, 3.75, 0.30},
	"claude-3-5-sonnet-20240620":   {3.0, 15.0, 3.75, 0.30},
}

// prefixMap for fallback matching of unknown model variants.
var prefixMap = []struct {
	prefix string
	key    string
}{
	{"claude-opus-4", "claude-opus-4"},
	{"claude-sonnet-4-5", "claude-sonnet-4-5"},
	{"claude-sonnet-4", "claude-sonnet-4"},
	{"claude-haiku-4-5", "claude-haiku-4-5"},
	{"claude-3-5-sonnet", "claude-3-5-sonnet-20241022"},
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
