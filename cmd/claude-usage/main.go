// Command claude-usage reads Claude Code session transcripts and reports
// per-session token usage and cost breakdowns.
//
// Usage:
//
//	claude-usage                 interactive TUI (default), auto-refreshes
//	claude-usage --no-tui        print a text summary and exit
//	claude-usage --html FILE     write a single-file HTML report and exit
//	claude-usage --top 20        limit text report to top 20 sessions
//	claude-usage --interval 5m   set auto-refresh interval (default 15m, 0 disables)
//	claude-usage --path FILE     analyse a single session file
//	claude-usage --dir DIR       scan a custom projects directory
//	claude-usage --version       print the version and exit
//
// By default it scans ~/.claude/projects.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hadifarnoud/claude-usage/internal/report"
	"github.com/hadifarnoud/claude-usage/internal/session"
	"github.com/hadifarnoud/claude-usage/internal/tui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		noTUI       bool
		top         int
		pathArg     string
		dirArg      string
		quiet       bool
		htmlPath    string
		intervalStr string
	)
	flag.BoolVar(&noTUI, "no-tui", false, "print a text summary instead of launching the TUI")
	flag.IntVar(&top, "top", 20, "number of top sessions to show in text mode (0 = all)")
	flag.StringVar(&pathArg, "path", "", "analyse a single session .jsonl file")
	flag.StringVar(&dirArg, "dir", "", "custom Claude projects directory (default: ~/.claude/projects)")
	flag.BoolVar(&quiet, "quiet", false, "suppress progress output in non-TUI mode")
	flag.StringVar(&htmlPath, "html", "", "write a self-contained HTML report to this file and exit")
	flag.StringVar(&intervalStr, "interval", "15m", "TUI auto-refresh interval (0 disables, e.g. 5m, 30s, 1h)")
	flag.BoolVar(&showVersion, "version", false, "print the version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("claude-usage " + version)
		return
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --interval %q: %v\n", intervalStr, err)
		os.Exit(1)
	}

	root, files, err := collect(pathArg, dirArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no session files found.")
		os.Exit(1)
	}

	if !quiet && (noTUI || htmlPath != "") {
		fmt.Fprintf(os.Stderr, "parsing %d session files...\n", len(files))
	}

	reports := parseFiles(files, root)
	if len(reports) == 0 {
		fmt.Fprintln(os.Stderr, "no parseable session data found.")
		os.Exit(1)
	}

	if htmlPath != "" {
		agg := report.NewAggregate()
		for _, r := range reports {
			agg.Add(r)
		}
		if err := report.WriteHTML(reports, agg, htmlPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d sessions)\n", htmlPath, agg.Sessions)
		return
	}

	if noTUI {
		agg := report.NewAggregate()
		for _, r := range reports {
			agg.Add(r)
		}
		fmt.Print(report.RenderText(reports, agg, top))
		return
	}

	loader := func() []report.SessionReport {
		_, f2, _ := collect(pathArg, dirArg)
		return parseFiles(f2, root)
	}

	if err := tui.Run(reports, loader, interval); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func defaultProjectsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

func collect(pathArg, dirArg string) (root string, files []string, err error) {
	if pathArg != "" {
		if !strings.HasSuffix(pathArg, ".jsonl") {
			return "", nil, fmt.Errorf("--path must point to a .jsonl file")
		}
		return "", []string{pathArg}, nil
	}

	root = dirArg
	if root == "" {
		root = defaultProjectsDir()
	}
	if root == "" {
		return "", nil, fmt.Errorf("could not determine projects directory; pass --dir")
	}
	if _, statErr := os.Stat(root); statErr != nil {
		return "", nil, fmt.Errorf("projects dir %s: %w", root, statErr)
	}
	files, err = session.Discover(root)
	return root, files, err
}

func parseFiles(files []string, root string) []report.SessionReport {
	type result struct {
		r  report.SessionReport
		ok bool
	}

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 2 {
		workers = 2
	}

	jobs := make(chan string, len(files))
	results := make(chan result, len(files))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				s, err := session.ParseFile(f, root)
				if err != nil || s == nil {
					results <- result{}
					continue
				}
				// Subagent transcripts live in separate files alongside the
				// parent; fold them into the session before reporting.
				_ = s.LoadSubagents()
				results <- result{r: report.FromSession(s), ok: true}
			}
		}()
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]report.SessionReport, 0, len(files))
	for res := range results {
		if res.ok {
			out = append(out, res.r)
		}
	}
	return out
}

var _ = time.Now
