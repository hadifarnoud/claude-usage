package report

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// RenderText produces a plain-text report for the non-interactive mode.
func RenderText(reports []SessionReport, agg *Aggregate, limit int) string {
	var b strings.Builder
	sorted := make([]SessionReport, len(reports))
	copy(sorted, reports)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalCost.Total > sorted[j].TotalCost.Total
	})
	reports = sorted
	line := strings.Repeat("\u2550", 63)
	dash90 := strings.Repeat("-", 90)
	dash84 := strings.Repeat("-", 84)
	dash44 := strings.Repeat("-", 44)

	b.WriteString(line + "\n")
	b.WriteString(fmt.Sprintf("  Claude Usage Report \u2014 %d sessions\n", agg.Sessions))
	b.WriteString(line + "\n\n")

	b.WriteString(fmt.Sprintf("  Total cost:     $%.2f\n", agg.TotalCost.Total))
	b.WriteString(fmt.Sprintf("    Input:        $%.2f  (%d tokens)\n", agg.TotalCost.Input, agg.TotalInput))
	b.WriteString(fmt.Sprintf("    Cache writes: $%.2f  (%d tokens)\n", agg.TotalCost.CacheWrite, agg.TotalCacheW))
	b.WriteString(fmt.Sprintf("    Cache reads:  $%.2f  (%d tokens)\n", agg.TotalCost.CacheRead, agg.TotalCacheR))
	b.WriteString(fmt.Sprintf("    Output:       $%.2f  (%d tokens)\n\n", agg.TotalCost.Output, agg.TotalOutput))

	models := agg.SortedModels()
	if len(models) > 0 {
		b.WriteString(strings.Repeat("\u2500", 31) + " By Model " + strings.Repeat("\u2500", 40) + "\n")
		b.WriteString(fmt.Sprintf("  %-28s %12s %12s %12s %12s %10s\n", "Model", "Input", "Cache W", "Cache R", "Output", "Cost"))
		b.WriteString("  " + dash90 + "\n")
		for _, m := range models {
			b.WriteString(fmt.Sprintf("  %-28s %12d %12d %12d %12d %9.2f\n", m.Model, m.Input, m.CacheWrite, m.CacheRead, m.Output, m.Cost.Total))
		}
		b.WriteString("\n")
	}

	projs := agg.SortedProjects()
	if len(projs) > 0 {
		b.WriteString(strings.Repeat("\u2500", 31) + " By Project " + strings.Repeat("\u2500", 42) + "\n")
		b.WriteString(fmt.Sprintf("  %-50s %8s %12s %10s\n", "Project", "Sessions", "Tokens", "Cost"))
		b.WriteString("  " + dash84 + "\n")
		for _, p := range projs {
			b.WriteString(fmt.Sprintf("  %-50s %8d %12d %9.2f\n", truncStr(p.Project, 50), p.Sessions, p.Tokens, p.Cost))
		}
		b.WriteString("\n")
	}

	days := agg.SortedDays()
	if len(days) > 0 {
		b.WriteString(strings.Repeat("\u2500", 31) + " By Day " + strings.Repeat("\u2500", 25) + "\n")
		b.WriteString(fmt.Sprintf("  %-14s %16s %10s\n", "Date", "Tokens", "Cost"))
		b.WriteString("  " + dash44 + "\n")
		for _, d := range days {
			b.WriteString(fmt.Sprintf("  %-14s %16d %9.2f\n", d.Day, d.Tokens, d.Cost))
		}
		b.WriteString("\n")
	}

	if len(reports) > 0 {
		if limit <= 0 || limit > len(reports) {
			limit = len(reports)
		}
		b.WriteString(fmt.Sprintf("%s Top %d Sessions (by cost) %s\n", strings.Repeat("\u2500", 31), limit, strings.Repeat("\u2500", 20)))
		b.WriteString(fmt.Sprintf("  %-50s %10s %22s %12s\n", "Session Title", "Cost", "Models", "Date"))
		b.WriteString("  " + dash90 + "\n")
		for i := 0; i < limit; i++ {
			r := reports[i]
			title := r.Title
			if title == "" {
				title = r.Project
			}
			modelNames := make([]string, 0, len(r.Models))
			for _, mm := range r.Models {
				modelNames = append(modelNames, mm.Model)
			}
			date := ""
			if !r.FirstSeen.IsZero() {
				date = r.FirstSeen.Format("2006-01-02")
			}
			b.WriteString(fmt.Sprintf("  %-50s %10.2f %22s %12s\n",
				truncStr(title, 50),
				r.TotalCost.Total,
				truncStr(strings.Join(modelNames, ","), 22),
				date))
		}
	}

	return b.String()
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return ""
	}
	return s[:n-1] + "\u2026"
}

// FormatDuration is a small helper exposed for external callers.
func FormatDuration(d time.Duration) string {
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
