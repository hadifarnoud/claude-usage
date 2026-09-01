package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hadifarnoud/claude-usage/internal/pricing"
)

func sampleReports() ([]SessionReport, *Aggregate) {
	r := SessionReport{
		SessionID:   "abc",
		Title:       "  a   very\nnoisy   title  ",
		FirstPrompt: strings.Repeat("x", 5000),
		Project:     "demo",
		FirstSeen:   time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		LastSeen:    time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC),
		Duration:    time.Hour,
		Models: []ModelRow{{
			Model: "claude-opus-5", Input: 100, Output: 200,
			Cost: pricing.Cost(100, 200, 0, 0, "claude-opus-5"),
		}},
		TotalInput:  100,
		TotalOutput: 200,
		TotalCost:   pricing.Cost(100, 200, 0, 0, "claude-opus-5"),
		Subagents: []SubagentRow{{
			AgentType: "Explore", Description: "look around", Turns: 3,
			Models: []ModelRow{{Model: "claude-sonnet-5"}},
		}},
		SubagentCount: 1,
	}
	agg := NewAggregate()
	agg.Add(r)
	return []SessionReport{r}, agg
}

func TestRenderHTMLIsSelfContained(t *testing.T) {
	reports, agg := sampleReports()
	out, err := RenderHTML(reports, agg)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if !strings.HasPrefix(out, "<!doctype html>") {
		t.Error("output must start with a doctype")
	}
	// The SVG namespace URL is not a fetch, so look for the constructs that
	// would actually pull a resource over the network.
	for _, bad := range []string{"<link", "<script src", "src=", "@import", "fetch(", "XMLHttpRequest"} {
		if strings.Contains(out, bad) {
			t.Errorf("export must not reference external resources, found %q", bad)
		}
	}
}

func TestRenderHTMLPayload(t *testing.T) {
	reports, agg := sampleReports()
	out, err := RenderHTML(reports, agg)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}

	start := strings.Index(out, `type="application/json">`)
	if start < 0 {
		t.Fatal("no embedded JSON payload")
	}
	start += len(`type="application/json">`)
	end := strings.Index(out[start:], "</script>")
	if end < 0 {
		t.Fatal("payload script tag is not closed")
	}
	raw := out[start : start+end]

	var p htmlPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if len(p.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(p.Sessions))
	}
	s := p.Sessions[0]
	if s.Title != "a very noisy title" {
		t.Errorf("title = %q, want whitespace collapsed", s.Title)
	}
	if len([]rune(s.Prompt)) > 700 {
		t.Errorf("prompt is %d runes, want it capped at 700", len([]rune(s.Prompt)))
	}
	if len(s.Subagents) != 1 || s.Subagents[0].AgentType != "Explore" {
		t.Errorf("subagents = %+v, want one Explore row", s.Subagents)
	}
	if p.Totals.Sessions != 1 {
		t.Errorf("totals.sessions = %d, want 1", p.Totals.Sessions)
	}
}

func TestRenderHTMLEscapesScriptTags(t *testing.T) {
	reports, agg := sampleReports()
	reports[0].Title = "</script><img onerror=alert(1)>"
	agg = NewAggregate()
	agg.Add(reports[0])

	out, err := RenderHTML(reports, agg)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	// Exactly two script tags close in the document: the payload and the
	// renderer. A title that breaks out would add a third.
	if n := strings.Count(out, "</script>"); n != 2 {
		t.Errorf("found %d closing script tags, want 2 — payload may escape its tag", n)
	}
}
