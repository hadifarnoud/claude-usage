package report

import (
	"strings"
	"testing"
	"time"

	"github.com/hadifarnoud/claude-usage/internal/pricing"
	"github.com/hadifarnoud/claude-usage/internal/session"
)

func TestFromSessionAndAggregate(t *testing.T) {
	// build two fake session reports
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	r1 := SessionReport{
		SessionID:  "s1",
		Project:    "proj-a",
		FirstSeen:  now,
		TotalCost:  pricing.Breakdown{Total: 10.5, Input: 1, Output: 2, CacheWrite: 3, CacheRead: 4.5},
		TotalInput: 100,
		Models: []ModelRow{
			{Model: "claude-opus-4-8", Input: 100, Cost: pricing.Breakdown{Total: 10.5, Input: 1}},
		},
	}
	r2 := SessionReport{
		SessionID:  "s2",
		Project:    "proj-a",
		FirstSeen:  now.AddDate(0, 0, 1),
		TotalCost:  pricing.Breakdown{Total: 5.0},
		TotalInput: 50,
		Models: []ModelRow{
			{Model: "claude-sonnet-5", Input: 50, Cost: pricing.Breakdown{Total: 5.0}},
		},
	}

	agg := NewAggregate()
	agg.Add(r1)
	agg.Add(r2)

	if agg.Sessions != 2 {
		t.Errorf("Sessions = %d, want 2", agg.Sessions)
	}
	if agg.TotalCost.Total != 15.5 {
		t.Errorf("TotalCost = %.2f, want 15.50", agg.TotalCost.Total)
	}

	models := agg.SortedModels()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].Model != "claude-opus-4-8" {
		t.Errorf("top model = %q, want opus", models[0].Model)
	}

	projects := agg.SortedProjects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Sessions != 2 {
		t.Errorf("project sessions = %d, want 2", projects[0].Sessions)
	}

	days := agg.SortedDays()
	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}
	if days[0].Day != "2026-01-15" {
		t.Errorf("first day = %q", days[0].Day)
	}
}

func TestTruncStr(t *testing.T) {
	if got := truncStr("hello", 10); got != "hello" {
		t.Errorf("truncStr short = %q", got)
	}
	if got := truncStr("hello world", 8); got != "hello w\u2026" {
		t.Errorf("truncStr long = %q", got)
	}
}

func TestFromSessionWithSubagents(t *testing.T) {
	// A parent session with one sonnet turn + one subagent using haiku.
	transcript := `{"type":"user","sessionId":"s1","timestamp":"2026-01-01T10:00:00Z","message":{"role":"user","content":"hi"}}
{"type":"assistant","sessionId":"s1","timestamp":"2026-01-01T10:00:05Z","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":1000,"output_tokens":500}}}
`
	s, err := session.Parse("test.jsonl", "/root", strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	// inject a subagent manually
	sa := s.Subagents["dead"]
	if sa == nil {
		sa = &session.Subagent{AgentID: "dead", Models: map[string]*session.ModelUsage{}, AgentType: "Explore", Description: "search"}
		s.Subagents["dead"] = sa
		s.SubagentOrder = append(s.SubagentOrder, "dead")
	}
	mu := &session.ModelUsage{Model: "claude-haiku-4-5", Input: 2000, Output: 400, Turns: 2}
	sa.Models["claude-haiku-4-5"] = mu
	sa.Total = session.Usage{InputTokens: 2000, OutputTokens: 400}
	sa.AssistantTurns = 2

	r := FromSession(s)

	if r.SubagentCount != 1 {
		t.Fatalf("SubagentCount = %d, want 1", r.SubagentCount)
	}
	if r.TotalCost.Total <= 0 || r.SubagentCost.Total <= 0 {
		t.Fatalf("costs should be positive: total=%.4f subagent=%.4f", r.TotalCost.Total, r.SubagentCost.Total)
	}
	// TotalCost is INCLUSIVE of subagent cost: it must be at least the
	// subagent cost plus a little for the parent sonnet turn.
	if r.TotalCost.Total <= r.SubagentCost.Total {
		t.Errorf("inclusive TotalCost %.4f must exceed subagent cost %.4f", r.TotalCost.Total, r.SubagentCost.Total)
	}
	// breakdown components must sum to the headline total (consistency)
	sumBreak := r.TotalCost.Input + r.TotalCost.Output + r.TotalCost.CacheWrite + r.TotalCost.CacheRead
	if !approxEq(sumBreak, r.TotalCost.Total) {
		t.Errorf("breakdown sum %.4f != TotalCost.Total %.4f", sumBreak, r.TotalCost.Total)
	}
	// token totals must include subagent tokens (2000 input from subagent)
	if r.TotalInput < 2000 {
		t.Errorf("TotalInput %d should include subagent's 2000 input tokens", r.TotalInput)
	}
	if len(r.Subagents) != 1 || r.Subagents[0].AgentType != "Explore" {
		t.Errorf("subagent row = %+v", r.Subagents)
	}

	// aggregate must equal the session's inclusive total (no double counting)
	agg := NewAggregate()
	agg.Add(r)
	if !approxEq(agg.TotalCost.Total, r.TotalCost.Total) {
		t.Errorf("aggregate total %.4f should equal session inclusive total %.4f", agg.TotalCost.Total, r.TotalCost.Total)
	}
}

func approxEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
