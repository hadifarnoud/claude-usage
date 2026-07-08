package report

import (
	"testing"
	"time"

	"github.com/hadifarnoud/claude-usage/internal/pricing"
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
