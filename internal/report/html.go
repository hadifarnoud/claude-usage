package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// htmlPayload is the JSON document embedded in the exported page. The page
// renders itself from this data, so the export stays a single file with no
// network requests.
type htmlPayload struct {
	GeneratedAt string        `json:"generatedAt"`
	Totals      htmlTotals    `json:"totals"`
	Models      []htmlModel   `json:"models"`
	Projects    []htmlProject `json:"projects"`
	Days        []htmlDay     `json:"days"`
	Sessions    []htmlSession `json:"sessions"`
}

type htmlTotals struct {
	Sessions   int     `json:"sessions"`
	Cost       float64 `json:"cost"`
	CostIn     float64 `json:"costIn"`
	CostOut    float64 `json:"costOut"`
	CostCacheW float64 `json:"costCacheW"`
	CostCacheR float64 `json:"costCacheR"`
	Input      int     `json:"input"`
	Output     int     `json:"output"`
	CacheW     int     `json:"cacheW"`
	CacheR     int     `json:"cacheR"`
}

type htmlModel struct {
	Model  string  `json:"model"`
	Input  int     `json:"input"`
	CacheW int     `json:"cacheW"`
	CacheR int     `json:"cacheR"`
	Output int     `json:"output"`
	Turns  int     `json:"turns"`
	Cost   float64 `json:"cost"`
}

type htmlProject struct {
	Project  string  `json:"project"`
	Sessions int     `json:"sessions"`
	Tokens   int     `json:"tokens"`
	Cost     float64 `json:"cost"`
}

type htmlDay struct {
	Day    string  `json:"day"`
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

type htmlSession struct {
	Title     string         `json:"title"`
	Project   string         `json:"project"`
	Branch    string         `json:"branch"`
	Date      string         `json:"date"`
	Started   string         `json:"started"`
	Duration  string         `json:"duration"`
	Cost      float64        `json:"cost"`
	Input     int            `json:"input"`
	CacheW    int            `json:"cacheW"`
	CacheR    int            `json:"cacheR"`
	Output    int            `json:"output"`
	Turns     int            `json:"turns"`
	Models    []htmlModel    `json:"models"`
	Subagents []htmlSubagent `json:"subagents"`
	SubCost   float64        `json:"subCost"`
	Prompt    string         `json:"prompt"`
}

type htmlSubagent struct {
	AgentType   string      `json:"agentType"`
	Description string      `json:"description"`
	Cost        float64     `json:"cost"`
	Turns       int         `json:"turns"`
	Models      []htmlModel `json:"models"`

	// Workflow provenance, empty for plain Task-tool subagents.
	Workflow string `json:"workflow,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Label    string `json:"label,omitempty"`
}

// buildPayload converts reports and their aggregate into the export document.
func buildPayload(reports []SessionReport, agg *Aggregate) htmlPayload {
	p := htmlPayload{
		GeneratedAt: time.Now().Format("2006-01-02 15:04"),
		Totals: htmlTotals{
			Sessions:   agg.Sessions,
			Cost:       agg.TotalCost.Total,
			CostIn:     agg.TotalCost.Input,
			CostOut:    agg.TotalCost.Output,
			CostCacheW: agg.TotalCost.CacheWrite,
			CostCacheR: agg.TotalCost.CacheRead,
			Input:      agg.TotalInput,
			Output:     agg.TotalOutput,
			CacheW:     agg.TotalCacheW,
			CacheR:     agg.TotalCacheR,
		},
	}
	for _, m := range agg.SortedModels() {
		p.Models = append(p.Models, toHTMLModel(m))
	}
	for _, pr := range agg.SortedProjects() {
		p.Projects = append(p.Projects, htmlProject{
			Project:  pr.Project,
			Sessions: pr.Sessions,
			Tokens:   pr.Tokens,
			Cost:     pr.Cost,
		})
	}
	for _, d := range agg.SortedDays() {
		p.Days = append(p.Days, htmlDay{Day: d.Day, Tokens: d.Tokens, Cost: d.Cost})
	}
	for _, r := range reports {
		s := htmlSession{
			Title:    clean(firstNonEmpty(r.Title, r.Summary, r.Project), 80),
			Project:  r.Project,
			Branch:   r.GitBranch,
			Duration: FormatDuration(r.Duration),
			Cost:     r.TotalCost.Total,
			Input:    r.TotalInput,
			CacheW:   r.TotalCacheW,
			CacheR:   r.TotalCacheR,
			Output:   r.TotalOutput,
			Turns:    r.AssistantTurns,
			SubCost:  r.SubagentCost.Total,
			Prompt:   clean(r.FirstPrompt, 700),
		}
		if !r.FirstSeen.IsZero() {
			s.Date = r.FirstSeen.Format("2006-01-02")
			s.Started = r.FirstSeen.Format("2006-01-02 15:04")
		}
		for _, m := range r.Models {
			s.Models = append(s.Models, toHTMLModel(m))
		}
		for _, sa := range r.Subagents {
			row := htmlSubagent{
				AgentType:   sa.AgentType,
				Description: clean(sa.Description, 160),
				Cost:        sa.Cost.Total,
				Turns:       sa.Turns,
				Workflow:    sa.WorkflowName,
				Phase:       sa.Phase,
				Label:       clean(sa.Label, 80),
			}
			for _, m := range sa.Models {
				row.Models = append(row.Models, toHTMLModel(m))
			}
			s.Subagents = append(s.Subagents, row)
		}
		p.Sessions = append(p.Sessions, s)
	}
	return p
}

func toHTMLModel(m ModelRow) htmlModel {
	return htmlModel{
		Model:  m.Model,
		Input:  m.Input,
		CacheW: m.CacheWrite,
		CacheR: m.CacheRead,
		Output: m.Output,
		Turns:  m.Turns,
		Cost:   m.Cost.Total,
	}
}

// clean collapses whitespace and caps the length of free-form text so one
// runaway title or prompt cannot stretch the exported table.
func clean(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "\u2026"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// RenderHTML builds a single self-contained HTML page from the reports. The
// page has no external references, so it works offline and can be shared as
// one file.
func RenderHTML(reports []SessionReport, agg *Aggregate) (string, error) {
	data, err := json.Marshal(buildPayload(reports, agg))
	if err != nil {
		return "", fmt.Errorf("encode report data: %w", err)
	}
	// json.Marshal escapes <, > and & to \u003c, \u003e and \u0026, so the
	// payload can never close the surrounding <script> tag.
	var b bytes.Buffer
	b.WriteString(htmlHead)
	b.WriteString(`<script id="usage-data" type="application/json">`)
	b.Write(data)
	b.WriteString("</script>\n")
	b.WriteString(htmlScript)
	b.WriteString(htmlTail)
	return b.String(), nil
}

// WriteHTML renders the report and writes it to path, creating parent
// directories as needed.
func WriteHTML(reports []SessionReport, agg *Aggregate, path string) error {
	out, err := RenderHTML(reports, agg)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
