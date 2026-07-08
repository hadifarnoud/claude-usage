package pricing

import (
	"testing"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		model    string
		wantIn   float64
		wantOut  float64
	}{
		{"claude-opus-4-7", 15, 75},
		{"claude-opus-4-8", 15, 75},
		{"claude-sonnet-4-6", 3, 15},
		{"claude-sonnet-5", 3, 15},
		{"claude-haiku-4-5-20251001", 1, 5},
		{"claude-haiku-3-5", 0.8, 4},
		{"claude-fable-5", 5, 25},
		{"unknown-model", 3, 15}, // conservative default
	}
	for _, c := range cases {
		p := Lookup(c.model)
		if p.Input != c.wantIn || p.Output != c.wantOut {
			t.Errorf("Lookup(%q) = %+v, want input=%.1f output=%.1f", c.model, p, c.wantIn, c.wantOut)
		}
	}
}

func TestCost(t *testing.T) {
	// 1M input tokens of opus should cost $15
	b := Cost(1_000_000, 0, 0, 0, "claude-opus-4-8")
	if b.Input != 15.0 {
		t.Errorf("input cost = %.4f, want 15.0", b.Input)
	}
	if b.Total != 15.0 {
		t.Errorf("total = %.4f, want 15.0", b.Total)
	}

	// cache write is 1.25x input
	b = Cost(0, 0, 1_000_000, 0, "claude-sonnet-5")
	if b.CacheWrite != 3.75 {
		t.Errorf("cache write cost = %.4f, want 3.75", b.CacheWrite)
	}

	// cache read is 0.1x input for sonnet
	b = Cost(0, 0, 0, 1_000_000, "claude-sonnet-5")
	if b.CacheRead != 0.3 {
		t.Errorf("cache read cost = %.4f, want 0.3", b.CacheRead)
	}
}
