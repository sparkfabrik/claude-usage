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
// Prices are per 1M tokens. CacheWrite = 5-minute cache write (1.25x input);
// 1-hour cache writes cost 2x input and are not modelled, so a session that
// uses the longer cache is slightly under-estimated. CacheRead = cache hit or
// refresh (0.1x input, 0.05x on Opus 5.5 and Sonnet 5.5, 0.025x on Fable 5.1).
// Haiku 5.5 is priced at its rate for prompts up to 100K tokens; the higher
// rate above that is not modelled.
// Source: https://platform.claude.com/docs/en/about-claude/pricing (2026-10-28)
var DefaultPricing = map[string]ModelPrice{
	// Fable 5.1 and Mythos 5.1 ($10/$50)
	"claude-fable-5-1":  {10.0, 50.0, 12.5, 0.25},
	"claude-mythos-5-1": {10.0, 50.0, 12.5, 0.25},
	// Fable 5 and Mythos 5 ($10/$50)
	"claude-fable-5":  {10.0, 50.0, 12.5, 1.0},
	"claude-mythos-5": {10.0, 50.0, 12.5, 1.0},
	// Opus 5.5 ($4/$20)
	"claude-opus-5-5": {4.0, 20.0, 5.0, 0.20},
	// Opus 5 ($5/$25)
	"claude-opus-5": {5.0, 25.0, 6.25, 0.50},
	// Sonnet 5.5 ($2/$10)
	"claude-sonnet-5-5": {2.0, 10.0, 2.5, 0.10},
	// Sonnet 5 ($2/$10)
	"claude-sonnet-5": {2.0, 10.0, 2.5, 0.20},
	// Haiku 5.5 ($0.10/$0.50 for prompts up to 100K tokens)
	"claude-haiku-5-5": {0.10, 0.50, 0.125, 0.01},
	// Opus 4.5+ generation ($5/$25)
	"claude-opus-4-8":          {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-7":          {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-6":          {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-5":          {5.0, 25.0, 6.25, 0.50},
	"claude-opus-4-5-20251101": {5.0, 25.0, 6.25, 0.50},
	// Opus 4.0–4.1 generation ($15/$75)
	"claude-opus-4-1":          {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4-1-20250805": {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4":            {15.0, 75.0, 18.75, 1.50},
	"claude-opus-4-20250514":   {15.0, 75.0, 18.75, 1.50},
	// Sonnet ($3/$15)
	"claude-sonnet-4-6":          {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-5":          {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-5-20250929": {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4":            {3.0, 15.0, 3.75, 0.30},
	"claude-sonnet-4-20250514":   {3.0, 15.0, 3.75, 0.30},
	// Haiku 4.5 ($1/$5)
	"claude-haiku-4-5":          {1.0, 5.0, 1.25, 0.10},
	"claude-haiku-4-5-20251001": {1.0, 5.0, 1.25, 0.10},
	// Legacy Haiku 3.5 ($0.80/$4)
	"claude-3-5-haiku-20241022": {0.80, 4.0, 1.0, 0.08},
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
	{"claude-fable-5-1", "claude-fable-5-1"},
	{"claude-fable-5", "claude-fable-5"},
	{"claude-mythos-5-1", "claude-mythos-5-1"},
	{"claude-mythos-5", "claude-mythos-5"},
	{"claude-opus-5-5", "claude-opus-5-5"},
	{"claude-opus-5", "claude-opus-5"},
	{"claude-sonnet-5-5", "claude-sonnet-5-5"},
	{"claude-sonnet-5", "claude-sonnet-5"},
	{"claude-haiku-5-5", "claude-haiku-5-5"},
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

// Known reports whether a model has a price of its own: an override, an
// exact row or a prefix match. A model that is not known is priced with the
// Sonnet fallback by Get, which is a guess rather than a list price.
func Known(model string, overrides map[string]ModelPrice) bool {
	if _, ok := overrides[model]; ok {
		return true
	}
	if _, ok := DefaultPricing[model]; ok {
		return true
	}
	for _, pm := range prefixMap {
		if strings.HasPrefix(model, pm.prefix) {
			return true
		}
	}
	return false
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
