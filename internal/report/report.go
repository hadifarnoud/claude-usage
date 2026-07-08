// Package report aggregates parsed sessions into cost/token summaries
// suitable for display.
package report

import (
	"sort"
	"time"

	"github.com/hadifarnoud/claude-usage/internal/pricing"
	"github.com/hadifarnoud/claude-usage/internal/session"
)

// ModelRow is one line in a per-model breakdown for a session or group.
type ModelRow struct {
	Model      string
	Input      int
	CacheWrite int
	CacheRead  int
	Output     int
	Cost       pricing.Breakdown
	Turns      int
}

// SessionReport is the cost view of a single session.
type SessionReport struct {
	SessionID      string
	Title          string
	Project        string
	GitBranch      string
	Cwd            string
	Summary        string
	FirstSeen      time.Time
	LastSeen       time.Time
	Duration       time.Duration
	Models         []ModelRow
	TotalCost      pricing.Breakdown
	TotalInput     int
	TotalCacheW    int
	TotalCacheR    int
	TotalOutput    int
	AssistantTurns int
	UserTurns      int
	IsSidechain    bool
}

// FromSession builds a SessionReport from a parsed session.
func FromSession(s *session.Session) SessionReport {
	r := SessionReport{
		SessionID:      s.SessionID,
		Title:          s.Title,
		Project:        s.Project,
		GitBranch:      s.GitBranch,
		Cwd:            s.Cwd,
		Summary:        s.Summary,
		FirstSeen:      s.FirstSeen,
		LastSeen:       s.LastSeen,
		Duration:       s.Duration,
		AssistantTurns: s.AssistantTurns,
		UserTurns:      s.UserTurns,
		IsSidechain:    s.IsSidechain,
	}
	for _, m := range s.SortedModels() {
		cost := pricing.Cost(m.Input, m.Output, m.CacheWrite, m.CacheRead, m.Model)
		r.Models = append(r.Models, ModelRow{
			Model:      m.Model,
			Input:      m.Input,
			CacheWrite: m.CacheWrite,
			CacheRead:  m.CacheRead,
			Output:     m.Output,
			Cost:       cost,
			Turns:      m.Turns,
		})
		r.TotalCost.Total += cost.Total
		r.TotalCost.Input += cost.Input
		r.TotalCost.Output += cost.Output
		r.TotalCost.CacheWrite += cost.CacheWrite
		r.TotalCost.CacheRead += cost.CacheRead
		r.TotalInput += m.Input
		r.TotalCacheW += m.CacheWrite
		r.TotalCacheR += m.CacheRead
		r.TotalOutput += m.Output
	}
	return r
}

// Aggregate is the combined cost view across many sessions.
type Aggregate struct {
	TotalCost   pricing.Breakdown
	TotalInput  int
	TotalCacheW int
	TotalCacheR int
	TotalOutput int
	Sessions    int
	Models      map[string]*ModelRow
	ByDay       map[string]*DayRow
	ByProject   map[string]*ProjectRow
}

// DayRow aggregates cost by calendar day.
type DayRow struct {
	Day    string
	Cost   float64
	Tokens int
}

// ProjectRow aggregates cost by project directory.
type ProjectRow struct {
	Project  string
	Cost     float64
	Tokens   int
	Sessions int
}

// NewAggregate returns a ready-to-fill aggregate.
func NewAggregate() *Aggregate {
	return &Aggregate{
		Models:    make(map[string]*ModelRow),
		ByDay:     make(map[string]*DayRow),
		ByProject: make(map[string]*ProjectRow),
	}
}

// Add folds a SessionReport into the aggregate totals.
func (a *Aggregate) Add(r SessionReport) {
	a.Sessions++
	a.TotalCost.Total += r.TotalCost.Total
	a.TotalCost.Input += r.TotalCost.Input
	a.TotalCost.Output += r.TotalCost.Output
	a.TotalCost.CacheWrite += r.TotalCost.CacheWrite
	a.TotalCost.CacheRead += r.TotalCost.CacheRead

	for _, m := range r.Models {
		row := a.Models[m.Model]
		if row == nil {
			row = &ModelRow{Model: m.Model}
			a.Models[m.Model] = row
		}
		row.Input += m.Input
		row.CacheWrite += m.CacheWrite
		row.CacheRead += m.CacheRead
		row.Output += m.Output
		row.Turns += m.Turns
		row.Cost.Input += m.Cost.Input
		row.Cost.Output += m.Cost.Output
		row.Cost.CacheWrite += m.Cost.CacheWrite
		row.Cost.CacheRead += m.Cost.CacheRead
		row.Cost.Total += m.Cost.Total

		a.TotalInput += m.Input
		a.TotalCacheW += m.CacheWrite
		a.TotalCacheR += m.CacheRead
		a.TotalOutput += m.Output
	}

	if !r.FirstSeen.IsZero() {
		day := r.FirstSeen.Format("2006-01-02")
		dr := a.ByDay[day]
		if dr == nil {
			dr = &DayRow{Day: day}
			a.ByDay[day] = dr
		}
		dr.Cost += r.TotalCost.Total
		dr.Tokens += r.TotalInput + r.TotalCacheW + r.TotalCacheR + r.TotalOutput
	}

	pr := a.ByProject[r.Project]
	if pr == nil {
		pr = &ProjectRow{Project: r.Project}
		a.ByProject[r.Project] = pr
	}
	pr.Cost += r.TotalCost.Total
	pr.Tokens += r.TotalInput + r.TotalCacheW + r.TotalCacheR + r.TotalOutput
	pr.Sessions++
}

// SortedModels returns per-model rows sorted by cost desc.
func (a *Aggregate) SortedModels() []ModelRow {
	out := make([]ModelRow, 0, len(a.Models))
	for _, m := range a.Models {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost.Total > out[j].Cost.Total })
	return out
}

// SortedDays returns per-day rows sorted by day asc.
func (a *Aggregate) SortedDays() []DayRow {
	out := make([]DayRow, 0, len(a.ByDay))
	for _, d := range a.ByDay {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}

// SortedProjects returns per-project rows sorted by cost desc.
func (a *Aggregate) SortedProjects() []ProjectRow {
	out := make([]ProjectRow, 0, len(a.ByProject))
	for _, p := range a.ByProject {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost > out[j].Cost })
	return out
}
