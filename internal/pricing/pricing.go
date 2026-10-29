// Package pricing provides per-model token pricing and cost calculation.
package pricing

import "strings"

// ModelPrice holds pricing for a single model (per 1M tokens, USD).
// CacheWrite is the 5-minute cache write rate and CacheWrite1h the 1-hour
// one; Claude Code records which of the two every request used.
// LongPrompt, when set, is a second set of rates for a request whose prompt
// is longer than its threshold; every other model has one flat rate across
// its context window.
type ModelPrice struct {
	Input        float64
	Output       float64
	CacheWrite   float64
	CacheWrite1h float64
	CacheRead    float64
	LongPrompt   *PromptTier
}

// PromptTier is the set of rates a request pays when its prompt, counted as
// every input token including cache reads and cache writes, is over
// Threshold tokens. The request is priced on its own: all of its tokens,
// output included, pay the tier rates. Price.LongPrompt is ignored.
type PromptTier struct {
	Threshold int
	Price     ModelPrice
}

// price builds a flat ModelPrice: input, output, 5-minute cache write,
// 1-hour cache write, cache read.
func price(input, output, cacheWrite, cacheWrite1h, cacheRead float64) ModelPrice {
	return ModelPrice{Input: input, Output: output, CacheWrite: cacheWrite, CacheWrite1h: cacheWrite1h, CacheRead: cacheRead}
}

// tiered adds a long-prompt tier to a flat price.
func tiered(p ModelPrice, threshold int, over ModelPrice) ModelPrice {
	p.LongPrompt = &PromptTier{Threshold: threshold, Price: over}
	return p
}

// DefaultPricing is the built-in pricing table.
// Prices are per 1M tokens, in the order input, output, 5-minute cache write
// (1.25x input), 1-hour cache write (2x input), cache read (0.1x input, 0.05x
// on Opus 5.5 and Sonnet 5.5, 0.025x on Fable 5.1 and Mythos 5.1).
// Haiku 5.5 is the only model priced by prompt length: a request whose prompt
// is over 100K tokens pays 5x on every token.
// Source: https://platform.claude.com/docs/en/about-claude/pricing (2026-10-28)
var DefaultPricing = map[string]ModelPrice{
	// Fable 5.1 and Mythos 5.1 ($10/$50)
	"claude-fable-5-1":  price(10.0, 50.0, 12.5, 20.0, 0.25),
	"claude-mythos-5-1": price(10.0, 50.0, 12.5, 20.0, 0.25),
	// Fable 5 and Mythos 5 ($10/$50)
	"claude-fable-5":  price(10.0, 50.0, 12.5, 20.0, 1.0),
	"claude-mythos-5": price(10.0, 50.0, 12.5, 20.0, 1.0),
	// Opus 5.5 ($4/$20)
	"claude-opus-5-5": price(4.0, 20.0, 5.0, 8.0, 0.20),
	// Opus 5 ($5/$25)
	"claude-opus-5": price(5.0, 25.0, 6.25, 10.0, 0.50),
	// Sonnet 5.5 ($2/$10)
	"claude-sonnet-5-5": price(2.0, 10.0, 2.5, 4.0, 0.10),
	// Sonnet 5 ($2/$10)
	"claude-sonnet-5": price(2.0, 10.0, 2.5, 4.0, 0.20),
	// Haiku 5.5 ($0.10/$0.50 for prompts up to 100K tokens, $0.50/$2.50 over)
	"claude-haiku-5-5": tiered(price(0.10, 0.50, 0.125, 0.20, 0.01), 100_000, price(0.50, 2.50, 0.625, 1.0, 0.05)),
	// Opus 4.5+ generation ($5/$25)
	"claude-opus-4-8":          price(5.0, 25.0, 6.25, 10.0, 0.50),
	"claude-opus-4-7":          price(5.0, 25.0, 6.25, 10.0, 0.50),
	"claude-opus-4-6":          price(5.0, 25.0, 6.25, 10.0, 0.50),
	"claude-opus-4-5":          price(5.0, 25.0, 6.25, 10.0, 0.50),
	"claude-opus-4-5-20251101": price(5.0, 25.0, 6.25, 10.0, 0.50),
	// Opus 4.0–4.1 generation ($15/$75)
	"claude-opus-4-1":          price(15.0, 75.0, 18.75, 30.0, 1.50),
	"claude-opus-4-1-20250805": price(15.0, 75.0, 18.75, 30.0, 1.50),
	"claude-opus-4":            price(15.0, 75.0, 18.75, 30.0, 1.50),
	"claude-opus-4-20250514":   price(15.0, 75.0, 18.75, 30.0, 1.50),
	// Sonnet ($3/$15)
	"claude-sonnet-4-6":          price(3.0, 15.0, 3.75, 6.0, 0.30),
	"claude-sonnet-4-5":          price(3.0, 15.0, 3.75, 6.0, 0.30),
	"claude-sonnet-4-5-20250929": price(3.0, 15.0, 3.75, 6.0, 0.30),
	"claude-sonnet-4":            price(3.0, 15.0, 3.75, 6.0, 0.30),
	"claude-sonnet-4-20250514":   price(3.0, 15.0, 3.75, 6.0, 0.30),
	// Haiku 4.5 ($1/$5)
	"claude-haiku-4-5":          price(1.0, 5.0, 1.25, 2.0, 0.10),
	"claude-haiku-4-5-20251001": price(1.0, 5.0, 1.25, 2.0, 0.10),
	// Legacy Haiku 3.5 ($0.80/$4)
	"claude-3-5-haiku-20241022": price(0.80, 4.0, 1.0, 1.6, 0.08),
	// Legacy Sonnet 3.5 ($3/$15)
	"claude-3-5-sonnet-20241022": price(3.0, 15.0, 3.75, 6.0, 0.30),
	"claude-3-5-sonnet-20240620": price(3.0, 15.0, 3.75, 6.0, 0.30),
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
// An override without a 1-hour cache write rate gets 1.6x its 5-minute rate,
// the ratio every row of the pricing page follows (2x over 1.25x input); the
// same applies to the override's long-prompt tier. An override without a
// tier has none, whatever the built-in row says.
func Get(model string, overrides map[string]ModelPrice) ModelPrice {
	if overrides != nil {
		if p, ok := overrides[model]; ok {
			if p.CacheWrite1h == 0 {
				p.CacheWrite1h = p.CacheWrite * 1.6
			}
			if p.LongPrompt != nil && p.LongPrompt.Price.CacheWrite1h == 0 {
				tier := *p.LongPrompt
				tier.Price.CacheWrite1h = tier.Price.CacheWrite * 1.6
				p.LongPrompt = &tier
			}
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

// Calculate returns cost in USD for a single usage entry, which is one
// request. cacheWriteTokens is every token written to the cache;
// cacheWrite1hTokens is the part of it written with the 1-hour TTL, priced at
// its own rate, and the rest at the 5-minute rate. When the model has a
// long-prompt tier and the request's prompt (input plus cache writes plus
// cache reads) is over its threshold, every token is priced at the tier rates.
func Calculate(inputTokens, outputTokens, cacheWriteTokens, cacheWrite1hTokens, cacheReadTokens int, model string, overrides map[string]ModelPrice) float64 {
	p := Get(model, overrides)
	if t := p.LongPrompt; t != nil && inputTokens+cacheWriteTokens+cacheReadTokens > t.Threshold {
		p = t.Price
	}
	if cacheWrite1hTokens > cacheWriteTokens {
		cacheWrite1hTokens = cacheWriteTokens
	}
	cacheWrite5mTokens := cacheWriteTokens - cacheWrite1hTokens
	return float64(inputTokens)/1e6*p.Input +
		float64(outputTokens)/1e6*p.Output +
		float64(cacheWrite5mTokens)/1e6*p.CacheWrite +
		float64(cacheWrite1hTokens)/1e6*p.CacheWrite1h +
		float64(cacheReadTokens)/1e6*p.CacheRead
}
