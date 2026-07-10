// Package tui provides an interactive terminal UI built on Bubble Tea for
// browsing Claude Code session costs, with configurable auto-refresh.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hadifarnoud/claude-usage/internal/report"
)

// View identifies the currently-active tab.
type View int

const (
	ViewSessions View = iota
	ViewModels
	ViewProjects
	ViewByDay
)

func (v View) String() string {
	switch v {
	case ViewModels:
		return "Models"
	case ViewProjects:
		return "Projects"
	case ViewByDay:
		return "Daily"
	default:
		return "Sessions"
	}
}

var views = []View{ViewSessions, ViewModels, ViewProjects, ViewByDay}

// sortMode controls how the sessions table is ordered.
type sortMode int

const (
	sortByCost sortMode = iota
	sortByTime
)

func (s sortMode) String() string {
	if s == sortByTime {
		return "time"
	}
	return "cost"
}

// timeFilter limits sessions to a recent activity window.
type timeFilter int

const (
	filterAll timeFilter = iota
	filter24h
	filter7d
)

func (f timeFilter) String() string {
	switch f {
	case filter24h:
		return "24h"
	case filter7d:
		return "7d"
	default:
		return "all"
	}
}

// Window returns the filter duration; 0 means no limit.
func (f timeFilter) Window() time.Duration {
	switch f {
	case filter24h:
		return 24 * time.Hour
	case filter7d:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

// refreshResult carries freshly parsed reports back to the UI.
type refreshResult struct {
	reports []report.SessionReport
	err     error
	at      time.Time
}

// Model is the Bubble Tea model.
type Model struct {
	reports   []report.SessionReport
	aggregate *report.Aggregate
	loader    func() []report.SessionReport
	interval  time.Duration

	width, height int
	active        View
	tables        map[View]table.Model
	detail       viewport.Model
	detailText   string
	showDetail   bool
	ready        bool

	refreshing bool
	spinner    spinner.Model
	lastUpdate time.Time
	nextUpdate time.Time

	cursorRow int // preserved across refreshes for sessions table

	sortBy    sortMode   // sessions table ordering: by cost (default) or time
	timeRange timeFilter // sessions table activity window

	// styles
	titleStyle    lipgloss.Style
	subtitleStyle lipgloss.Style
	tabStyle      lipgloss.Style
	activeTab     lipgloss.Style
	dimStyle      lipgloss.Style
	accentStyle   lipgloss.Style
	okStyle       lipgloss.Style
}

// New constructs the TUI model from an initial set of reports and a loader
// used for refreshes.
func New(reports []report.SessionReport, loader func() []report.SessionReport, interval time.Duration) Model {
	agg := report.NewAggregate()
	sortSessions(reports, sortByCost)
	for i := range reports {
		agg.Add(reports[i])
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := Model{
		reports:    reports,
		aggregate:  agg,
		loader:     loader,
		interval:   interval,
		tables:     make(map[View]table.Model),
		spinner:    sp,
		lastUpdate: time.Now(),
	}
	if interval > 0 {
		m.nextUpdate = m.lastUpdate.Add(interval)
	}
	m.initStyles()
	m.detail = viewport.New(80, 20)
	m.buildTables()
	return m
}

func (m *Model) initStyles() {
	m.titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7DD3FC")).Padding(0, 1)
	m.subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).PaddingLeft(1)
	m.tabStyle = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("#64748B"))
	m.activeTab = lipgloss.NewStyle().Bold(true).Padding(0, 2).
		Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#7DD3FC"))
	m.dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	m.accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true)
	m.okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC"))
}

func (m *Model) buildTables() {
	m.tables[ViewSessions] = m.buildSessionsTable()
	m.tables[ViewModels] = m.buildModelsTable()
	m.tables[ViewProjects] = m.buildProjectsTable()
	m.tables[ViewByDay] = m.buildDaysTable()
}

func newBaseTable(cols []table.Column) table.Model {
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#334155")).
		Bold(true).Foreground(lipgloss.Color("#CBD5E1"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#7DD3FC")).Bold(true)
	// Leave Cell.Foreground unset so the Selected style's dark foreground can
	// override it; setting a cell foreground here would win over Selected and
	// make the highlighted row unreadable (light text on light background).
	s.Cell = s.Cell.Padding(0, 1)
	t.SetStyles(s)
	return t
}

func (m *Model) buildSessionsTable() table.Model {
	cols := []table.Column{
		{Title: "Session Title", Width: 50},
		{Title: "Cost", Width: 10},
		{Title: "Input", Width: 10},
		{Title: "Cache W", Width: 10},
		{Title: "Cache R", Width: 10},
		{Title: "Output", Width: 10},
		{Title: "Models", Width: 22},
		{Title: "Date", Width: 12},
	}
	t := newBaseTable(cols)
	filtered := m.filteredReports()
	rows := make([]table.Row, 0, len(filtered))
	for _, r := range filtered {
		models := make([]string, 0, len(r.Models))
		for _, mm := range r.Models {
			models = append(models, shortenModel(mm.Model))
		}
		date := ""
		if !r.FirstSeen.IsZero() {
			date = r.FirstSeen.Format("2006-01-02")
		}
		title := r.Title
		if title == "" {
			title = r.Project
		}
		rows = append(rows, table.Row{
			truncForCell(title, 50),
			fmt.Sprintf("$%.2f", r.TotalCost.Total),
			fmt.Sprintf("%d", r.TotalInput),
			fmt.Sprintf("%d", r.TotalCacheW),
			fmt.Sprintf("%d", r.TotalCacheR),
			fmt.Sprintf("%d", r.TotalOutput),
			truncForCell(strings.Join(models, ", "), 22),
			date,
		})
	}
	t.SetRows(rows)
	t.SetHeight(20)
	if m.cursorRow >= len(rows) {
		m.cursorRow = 0
	}
	t.SetCursor(m.cursorRow)
	return t
}

func (m *Model) buildModelsTable() table.Model {
	cols := []table.Column{
		{Title: "Model", Width: 28},
		{Title: "Input", Width: 12},
		{Title: "Cache W", Width: 12},
		{Title: "Cache R", Width: 12},
		{Title: "Output", Width: 12},
		{Title: "Turns", Width: 8},
		{Title: "Cost", Width: 12},
	}
	t := newBaseTable(cols)
	rows := make([]table.Row, 0)
	for _, mm := range m.aggregate.SortedModels() {
		rows = append(rows, table.Row{
			mm.Model,
			fmt.Sprintf("%d", mm.Input),
			fmt.Sprintf("%d", mm.CacheWrite),
			fmt.Sprintf("%d", mm.CacheRead),
			fmt.Sprintf("%d", mm.Output),
			fmt.Sprintf("%d", mm.Turns),
			fmt.Sprintf("$%.2f", mm.Cost.Total),
		})
	}
	t.SetRows(rows)
	t.SetHeight(20)
	return t
}

func (m *Model) buildProjectsTable() table.Model {
	cols := []table.Column{
		{Title: "Project", Width: 50},
		{Title: "Sessions", Width: 10},
		{Title: "Tokens", Width: 14},
		{Title: "Cost", Width: 12},
	}
	t := newBaseTable(cols)
	rows := make([]table.Row, 0)
	for _, p := range m.aggregate.SortedProjects() {
		rows = append(rows, table.Row{
			truncForCell(p.Project, 50),
			fmt.Sprintf("%d", p.Sessions),
			fmt.Sprintf("%d", p.Tokens),
			fmt.Sprintf("$%.2f", p.Cost),
		})
	}
	t.SetRows(rows)
	t.SetHeight(20)
	return t
}

func (m *Model) buildDaysTable() table.Model {
	cols := []table.Column{
		{Title: "Date", Width: 14},
		{Title: "Tokens", Width: 16},
		{Title: "Cost", Width: 12},
	}
	t := newBaseTable(cols)
	rows := make([]table.Row, 0)
	for _, d := range m.aggregate.SortedDays() {
		rows = append(rows, table.Row{
			d.Day,
			fmt.Sprintf("%d", d.Tokens),
			fmt.Sprintf("$%.2f", d.Cost),
		})
	}
	t.SetRows(rows)
	t.SetHeight(20)
	return t
}

func shortenModel(m string) string {
	m = strings.TrimPrefix(m, "claude-")
	parts := strings.Split(m, "-")
	if len(parts) >= 3 {
		m = parts[0] + "-" + strings.Join(parts[1:], ".")
	}
	return m
}

func truncForCell(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return ""
	}
	return s[:n-1] + "\u2026"
}

// wrapText word-wraps s into lines no longer than width, preserving leading
// spaces on continuation lines. It splits on any whitespace.
func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() == 0 {
			cur.WriteString(w)
			continue
		}
		// +1 for the space between words
		if cur.Len()+1+len(w) <= width {
			cur.WriteByte(' ')
			cur.WriteString(w)
		} else {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// Init starts the program.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.interval > 0 {
		cmds = append(cmds, tea.Tick(m.interval, func(t time.Time) tea.Msg {
			return tickMsg{at: t}
		}))
	}
	return tea.Batch(cmds...)
}

type tickMsg struct {
	at time.Time
}

// doRefresh is the async command that runs the loader off the main loop.
func (m Model) doRefresh() tea.Cmd {
	return func() tea.Msg {
		if m.loader == nil {
			return refreshResult{at: time.Now()}
		}
		return refreshResult{reports: m.loader(), at: time.Now()}
	}
}

// sortSessions orders reports in place. Sessions without a timestamp sort to
// the bottom regardless of mode.
func sortSessions(reports []report.SessionReport, mode sortMode) {
	sort.SliceStable(reports, func(i, j int) bool {
		switch mode {
		case sortByTime:
			ti, tj := reports[i].LastSeen, reports[j].LastSeen
			if ti.IsZero() {
				return false
			}
			if tj.IsZero() {
				return true
			}
			return ti.After(tj)
		default:
			return reports[i].TotalCost.Total > reports[j].TotalCost.Total
		}
	})
}

// filterByTime returns the subset of reports whose LastSeen falls within the
// filter window. A zero window returns all reports untouched.
func filterByTime(reports []report.SessionReport, f timeFilter) []report.SessionReport {
	w := f.Window()
	if w == 0 {
		return reports
	}
	cutoff := time.Now().Add(-w)
	out := make([]report.SessionReport, 0, len(reports))
	for _, r := range reports {
		if r.LastSeen.IsZero() || r.LastSeen.Before(cutoff) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// filteredReports returns the current session view with the active time
// filter applied.
func (m *Model) filteredReports() []report.SessionReport {
	return filterByTime(m.reports, m.timeRange)
}

// rebuildSessions re-sorts the session data and rebuilds only the sessions
// table, preserving the cursor when possible.
func (m *Model) rebuildSessions() {
	sortSessions(m.reports, m.sortBy)
	m.tables[ViewSessions] = m.buildSessionsTable()
}

func (m *Model) applyRefresh(res refreshResult) {
	reports := res.reports
	sortSessions(reports, m.sortBy)
	agg := report.NewAggregate()
	for i := range reports {
		agg.Add(reports[i])
	}
	m.reports = reports
	m.aggregate = agg
	m.lastUpdate = res.at
	if m.interval > 0 {
		m.nextUpdate = res.at.Add(m.interval)
	} else {
		m.nextUpdate = time.Time{}
	}
	m.buildTables()
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.interval > 0 && !m.refreshing {
			m.refreshing = true
			return m, tea.Batch(m.doRefresh(), m.spinner.Tick)
		}
		// reschedule even if we somehow skipped
		return m, tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg{at: t} })

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.refreshing {
			return m, cmd
		}
		return m, cmd

	case refreshResult:
		m.refreshing = false
		m.applyRefresh(msg)
		cmds := []tea.Cmd{}
		if m.interval > 0 {
			cmds = append(cmds, tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg{at: t} }))
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resizeTables()
		m.detail.Width = msg.Width
		m.detail.Height = msg.Height - 12
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.showDetail {
				m.showDetail = false
				return m, nil
			}
			return m, tea.Quit
		case "esc":
			if m.showDetail {
				m.showDetail = false
				return m, nil
			}
		case "enter":
			if m.active == ViewSessions && !m.showDetail {
				m.cursorRow = m.tables[ViewSessions].Cursor()
				m.showDetail = true
				m.buildDetailText()
				m.detail.GotoTop()
				return m, nil
			}
		case "r":
			if !m.refreshing {
				m.refreshing = true
				return m, tea.Batch(m.doRefresh(), m.spinner.Tick)
			}
		case "tab", "right":
			if m.showDetail {
				return m, nil
			}
			m.active = (m.active + 1) % View(len(views))
			return m, nil
		case "shift+tab", "left":
			if m.showDetail {
				return m, nil
			}
			m.active--
			if m.active < 0 {
				m.active = View(len(views) - 1)
			}
			return m, nil
			case "1", "2", "3", "4":
				idx := int(msg.String()[0] - '1')
				if idx < len(views) {
					m.active = views[idx]
				}
				return m, nil
			case "s":
				if m.showDetail {
					return m, nil
				}
				if m.sortBy == sortByCost {
					m.sortBy = sortByTime
				} else {
					m.sortBy = sortByCost
				}
				m.rebuildSessions()
				return m, nil
			case "f":
				if m.showDetail {
					return m, nil
				}
				switch m.timeRange {
				case filterAll:
					m.timeRange = filter24h
				case filter24h:
					m.timeRange = filter7d
				case filter7d:
					m.timeRange = filterAll
				}
				m.cursorRow = 0
				m.tables[ViewSessions] = m.buildSessionsTable()
				return m, nil
		}
	}

	if m.showDetail {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}

	t := m.tables[m.active]
	var cmd tea.Cmd
	t, cmd = t.Update(msg)
	if m.active == ViewSessions {
		m.cursorRow = t.Cursor()
	}
	m.tables[m.active] = t
	return m, cmd
}

func (m *Model) resizeTables() {
	for v, t := range m.tables {
		t.SetHeight(m.height - 12)
		m.tables[v] = t
	}
}

func (m *Model) buildDetailText() {
	t := m.tables[ViewSessions]
	idx := t.Cursor()
	filtered := m.filteredReports()
	if idx >= len(filtered) {
		m.detailText = "No session selected."
		m.detail.SetContent(m.detailText)
		return
	}
	r := filtered[idx]
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", m.titleStyle.Render("Session Detail"))
	title := r.Title
	if title == "" {
		title = "(untitled)"
	}
	fmt.Fprintf(&b, "  Title:      %s\n", truncForCell(title, 80))
	if r.FirstPrompt != "" && r.FirstPrompt != title {
		fmt.Fprintf(&b, "  %s\n", m.accentStyle.Render("  First prompt:"))
		for _, line := range wrapText(r.FirstPrompt, 76) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	fmt.Fprintf(&b, "  Session ID: %s\n", r.SessionID)
	fmt.Fprintf(&b, "  Project:    %s\n", r.Project)
	if r.GitBranch != "" {
		fmt.Fprintf(&b, "  Branch:     %s\n", r.GitBranch)
	}
	if r.Cwd != "" {
		fmt.Fprintf(&b, "  Cwd:        %s\n", r.Cwd)
	}
	if !r.FirstSeen.IsZero() {
		fmt.Fprintf(&b, "  Started:    %s\n", r.FirstSeen.Format(time.RFC1123))
	}
	if !r.LastSeen.IsZero() {
		fmt.Fprintf(&b, "  Last seen:  %s\n", r.LastSeen.Format(time.RFC1123))
	}
	if r.Duration > 0 {
		fmt.Fprintf(&b, "  Duration:   %s\n", humanDuration(r.Duration))
	}
	fmt.Fprintf(&b, "  Turns:      %d assistant / %d user\n\n", r.AssistantTurns, r.UserTurns)
	fmt.Fprintf(&b, "  %s  $%.4f\n", m.accentStyle.Render("Total Cost:"), r.TotalCost.Total)
	fmt.Fprintf(&b, "    Input tokens:   %12d  ($%.4f)\n", r.TotalInput, r.TotalCost.Input)
	fmt.Fprintf(&b, "    Cache writes:   %12d  ($%.4f)\n", r.TotalCacheW, r.TotalCost.CacheWrite)
	fmt.Fprintf(&b, "    Cache reads:    %12d  ($%.4f)\n", r.TotalCacheR, r.TotalCost.CacheRead)
	fmt.Fprintf(&b, "    Output tokens:  %12d  ($%.4f)\n\n", r.TotalOutput, r.TotalCost.Output)
	fmt.Fprintf(&b, "%s\n", m.accentStyle.Render("  Per-Model Breakdown"))
	for _, mm := range r.Models {
		fmt.Fprintf(&b, "\n  %s\n", mm.Model)
		fmt.Fprintf(&b, "    input  %10d   cache-w %10d   cache-r %10d   output %10d\n", mm.Input, mm.CacheWrite, mm.CacheRead, mm.Output)
		fmt.Fprintf(&b, "    cost: $%.4f (in $%.4f / cw $%.4f / cr $%.4f / out $%.4f)  turns %d\n",
			mm.Cost.Total, mm.Cost.Input, mm.Cost.CacheWrite, mm.Cost.CacheRead, mm.Cost.Output, mm.Turns)
	}
	m.detailText = b.String()
	m.detail.SetContent(m.detailText)
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// View renders the current state.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}
	if m.showDetail {
		return m.detail.View() + "\n" + m.dimStyle.Render("  [esc] back  [q] quit")
	}

	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")
	b.WriteString(m.tabsView())
	b.WriteString("\n\n")
	b.WriteString(m.tables[m.active].View())
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

func (m Model) headerView() string {
	total := m.aggregate.TotalCost.Total
	headline := fmt.Sprintf("Claude Usage Report  \u2014  %d sessions  \u2014  %s total",
		m.aggregate.Sessions, m.accentStyle.Render(fmt.Sprintf("$%.2f", total)))
	tokens := fmt.Sprintf("in %d  cache-w %d  cache-r %d  out %d",
		m.aggregate.TotalInput, m.aggregate.TotalCacheW, m.aggregate.TotalCacheR, m.aggregate.TotalOutput)

	status := m.refreshStatus()
	return m.titleStyle.Render(headline) + "\n" +
		m.subtitleStyle.Render(tokens) + "\n" +
		m.dimStyle.Render(status)
}

func (m Model) refreshStatus() string {
	updated := "updated " + m.lastUpdate.Format("15:04:05")
	if m.refreshing {
		return fmt.Sprintf("  %s refreshing\u2026", m.spinner.View()) + "  " + updated
	}
	if m.interval > 0 {
		remaining := time.Until(m.nextUpdate).Round(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		return fmt.Sprintf("  %s  next refresh in %s  (interval %s)",
			m.okStyle.Render("\u2714"), remaining, m.interval)
	}
	return "  auto-refresh off  \u2014  " + updated
}

func (m Model) tabsView() string {
	var tabs []string
	for i, v := range views {
		label := fmt.Sprintf("%d %s", i+1, v.String())
		if v == m.active {
			tabs = append(tabs, m.activeTab.Render(label))
		} else {
			tabs = append(tabs, m.tabStyle.Render(label))
		}
	}
	return strings.Join(tabs, "")
}

func (m Model) footerView() string {
	auto := ""
	if m.interval > 0 {
		auto = "  \u2014  auto-refresh " + m.interval.String()
	}
	return m.dimStyle.Render("  [tab/1-4] views  [s] sort: "+m.sortBy.String()+
		"  [f] time: "+m.timeRange.String()+
		"  [enter] detail  [r] refresh now  [q] quit"+auto)
}

// Run launches the TUI.
func Run(reports []report.SessionReport, loader func() []report.SessionReport, interval time.Duration) error {
	m := New(reports, loader, interval)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
