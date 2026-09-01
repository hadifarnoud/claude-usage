// Package pricing maps Claude model names to per-million-token prices (USD)
// and computes the cost of a token usage breakdown.
//
// Prices are Anthropic first-party API list rates. Cache write is the 5-minute
// TTL rate (1.25x input) and cache read is 0.1x input, which holds for every
// current model.
package pricing

import (
	"regexp"
	"strings"
)

// Price holds USD per 1M tokens for each token category.
type Price struct {
	Input      float64
	Output     float64
	CacheWrite float64 // 5m cache write
	CacheRead  float64
}

// std builds a Price from the input and output rates using the standard
// cache multipliers: write = 1.25x input, read = 0.1x input.
func std(input, output float64) Price {
	return Price{
		Input:      input,
		Output:     output,
		CacheWrite: input * 1.25,
		CacheRead:  input * 0.1,
	}
}

// Named price tiers, so the table below reads as a price list.
var (
	priceFable  = std(10, 50) // Fable 5, Mythos 5
	priceOpus   = std(5, 25)  // Opus 5, 4.8, 4.7, 4.6
	priceOpusV1 = std(15, 75) // Opus 4.5 and older, Opus 3
	priceSonnet = std(2, 10)  // Sonnet 5
	priceSonnV1 = std(3, 15)  // Sonnet 4.6 and older
	priceHaiku  = std(1, 5)   // Haiku 4.5
	priceHaikV1 = std(0.8, 4) // Haiku 3.5
	priceFree   = Price{}     // synthetic / local messages: no API charge
)

// tiers is ordered: the first pattern that matches wins, so more specific
// patterns must come first.
var tiers = []struct {
	pattern *regexp.Regexp
	price   Price
}{
	// Synthetic transcript entries are not billed.
	{regexp.MustCompile(`(?i)^<synthetic>$`), priceFree},

	// Fable / Mythos tier.
	{regexp.MustCompile(`(?i)^(claude-)?(fable|mythos)`), priceFable},

	// Opus 4.6+ dropped to the $5/$25 tier. Keep this above the generic
	// opus-4 rule, which still prices 4.5 and older at $15/$75.
	{regexp.MustCompile(`(?i)^claude-opus-(5|4-6|4-7|4-8|4-9)`), priceOpus},
	{regexp.MustCompile(`(?i)^claude-opus-[34]`), priceOpusV1},
	// Bare alias "opus" means the current default Opus.
	{regexp.MustCompile(`(?i)^opus$`), priceOpus},

	// Sonnet 5 dropped to $2/$10; Sonnet 4.6 and older stay at $3/$15.
	{regexp.MustCompile(`(?i)^claude-sonnet-5`), priceSonnet},
	{regexp.MustCompile(`(?i)^claude-sonnet-[34]`), priceSonnV1},
	{regexp.MustCompile(`(?i)^sonnet$`), priceSonnet},

	// Haiku.
	{regexp.MustCompile(`(?i)^claude-haiku-[45]`), priceHaiku},
	{regexp.MustCompile(`(?i)^claude-haiku-3`), priceHaikV1},
	{regexp.MustCompile(`(?i)^haiku$`), priceHaiku},
}

// Lookup returns the price for a model, falling back to a conservative default.
func Lookup(model string) Price {
	m := strings.TrimSpace(model)
	for _, t := range tiers {
		if t.pattern.MatchString(m) {
			return t.price
		}
	}
	// Unknown model: assume the current Opus tier so new releases are not
	// silently under-reported.
	return priceOpus
}

// Known reports whether the model matched a real price tier. Callers use it to
// mark estimated costs in reports.
func Known(model string) bool {
	m := strings.TrimSpace(model)
	for _, t := range tiers {
		if t.pattern.MatchString(m) {
			return true
		}
	}
	return false
}

// Breakdown is the dollar cost split by token category.
type Breakdown struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
	Total      float64
}

// Cost computes the dollar cost of a set of token counts at the model's price.
func Cost(input, output, cacheWrite, cacheRead int, model string) Breakdown {
	p := Lookup(model)
	m := float64(1_000_000)
	b := Breakdown{
		Input:      float64(input) * p.Input / m,
		Output:     float64(output) * p.Output / m,
		CacheWrite: float64(cacheWrite) * p.CacheWrite / m,
		CacheRead:  float64(cacheRead) * p.CacheRead / m,
	}
	b.Total = b.Input + b.Output + b.CacheWrite + b.CacheRead
	return b
}
