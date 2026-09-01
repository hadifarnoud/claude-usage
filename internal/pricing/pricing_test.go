package pricing

import (
	"math"
	"testing"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		model   string
		wantIn  float64
		wantOut float64
	}{
		// Current tier.
		{"claude-fable-5", 10, 50},
		{"claude-mythos-5", 10, 50},
		{"claude-opus-5", 5, 25},
		{"claude-opus-4-8", 5, 25},
		{"claude-opus-4-7", 5, 25},
		{"claude-opus-4-6", 5, 25},
		{"claude-sonnet-5", 2, 10},
		{"claude-sonnet-4-6", 3, 15},
		{"claude-haiku-4-5-20251001", 1, 5},

		// Older models keep their old prices.
		{"claude-opus-4-5", 15, 75},
		{"claude-opus-4-1", 15, 75},
		{"claude-opus-4", 15, 75},
		{"claude-sonnet-3-7", 3, 15},
		{"claude-haiku-3-5", 0.8, 4},

		// Bare aliases map to the current default of the family.
		{"opus", 5, 25},
		{"sonnet", 2, 10},
		{"fable", 10, 50},
		{"haiku", 1, 5},

		// Synthetic entries are free.
		{"<synthetic>", 0, 0},

		// Unknown model falls back to the Opus tier.
		{"claude-something-9", 5, 25},
	}
	for _, c := range cases {
		p := Lookup(c.model)
		if p.Input != c.wantIn || p.Output != c.wantOut {
			t.Errorf("Lookup(%q) = in %.2f out %.2f, want in %.2f out %.2f",
				c.model, p.Input, p.Output, c.wantIn, c.wantOut)
		}
	}
}

func TestCacheMultipliers(t *testing.T) {
	for _, model := range []string{"claude-opus-5", "claude-sonnet-5", "claude-fable-5", "claude-haiku-4-5"} {
		p := Lookup(model)
		if math.Abs(p.CacheWrite-p.Input*1.25) > 1e-9 {
			t.Errorf("%s: cache write %.4f, want %.4f", model, p.CacheWrite, p.Input*1.25)
		}
		if math.Abs(p.CacheRead-p.Input*0.1) > 1e-9 {
			t.Errorf("%s: cache read %.4f, want %.4f", model, p.CacheRead, p.Input*0.1)
		}
	}
}

func TestKnown(t *testing.T) {
	if !Known("claude-opus-5") {
		t.Error("claude-opus-5 must be a known model")
	}
	if Known("claude-something-9") {
		t.Error("claude-something-9 must not be a known model")
	}
}

func TestCost(t *testing.T) {
	// 1M input tokens of Opus 5 costs $5.
	b := Cost(1_000_000, 0, 0, 0, "claude-opus-5")
	if b.Input != 5.0 || b.Total != 5.0 {
		t.Errorf("opus input cost = %.4f (total %.4f), want 5.0", b.Input, b.Total)
	}

	// 1M output tokens of Fable 5 costs $50.
	b = Cost(0, 1_000_000, 0, 0, "claude-fable-5")
	if b.Output != 50.0 {
		t.Errorf("fable output cost = %.4f, want 50.0", b.Output)
	}

	// Sonnet 5 cache write is 1.25x its $2 input rate.
	b = Cost(0, 0, 1_000_000, 0, "claude-sonnet-5")
	if math.Abs(b.CacheWrite-2.5) > 1e-9 {
		t.Errorf("sonnet cache write cost = %.4f, want 2.5", b.CacheWrite)
	}

	// Sonnet 5 cache read is 0.1x its $2 input rate.
	b = Cost(0, 0, 0, 1_000_000, "claude-sonnet-5")
	if math.Abs(b.CacheRead-0.2) > 1e-9 {
		t.Errorf("sonnet cache read cost = %.4f, want 0.2", b.CacheRead)
	}

	// Synthetic entries cost nothing.
	b = Cost(1_000_000, 1_000_000, 1_000_000, 1_000_000, "<synthetic>")
	if b.Total != 0 {
		t.Errorf("synthetic total = %.4f, want 0", b.Total)
	}
}
