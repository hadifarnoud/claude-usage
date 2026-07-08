// Package pricing maps Claude model names to per-million-token prices (USD)
// and computes the cost of a token usage breakdown.
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

// tier is a bucket of prices keyed by model family.
var tiers = []struct {
	pattern *regexp.Regexp
	price   Price
}{
	// Claude Opus 4.x (including 4.1, 4.5, etc.)
	{regexp.MustCompile(`(?i)^claude-opus-4`), Price{Input: 15, Output: 75, CacheWrite: 18.75, CacheRead: 1.5}},
	// Claude Opus 4.7+ / future opus pricing stays same tier.
	// Claude Sonnet 4.x / 5
	{regexp.MustCompile(`(?i)^claude-sonnet-[45]`), Price{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.3}},
	// Claude Sonnet 3.7 / 3.5
	{regexp.MustCompile(`(?i)^claude-sonnet-3`), Price{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.3}},
	// Claude Haiku 4.5
	{regexp.MustCompile(`(?i)^claude-haiku-4`), Price{Input: 1, Output: 5, CacheWrite: 1.25, CacheRead: 0.1}},
	// Claude Haiku 3.5
	{regexp.MustCompile(`(?i)^claude-haiku-3`), Price{Input: 0.8, Output: 4, CacheWrite: 1, CacheRead: 0.08}},
	// Fables
	{regexp.MustCompile(`(?i)^claude-fable`), Price{Input: 5, Output: 25, CacheWrite: 6.25, CacheRead: 0.5}},
}

// Lookup returns the price for a model, falling back to a conservative default.
func Lookup(model string) Price {
	m := strings.TrimSpace(model)
	for _, t := range tiers {
		if t.pattern.MatchString(m) {
			return t.price
		}
	}
	// Conservative default ~ Sonnet tier.
	return Price{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.3}
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
