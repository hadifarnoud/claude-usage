// Package session reads and parses Claude Code session transcript files
// (JSONL) and exposes structured usage data for downstream reporting.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Usage captures the token counters embedded in each assistant message.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// ServerToolUse captures ancillary server-side tool usage.
type ServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests"`
	WebFetchRequests  int `json:"web_fetch_requests"`
}

// Message is the message body shared by user/assistant lines.
type Message struct {
	Model      string          `json:"model"`
	Role       string          `json:"role"`
	Usage      Usage           `json:"usage"`
	ServerTool ServerToolUse   `json:"server_tool_use"`
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
}

// Line is a single JSONL record from a session transcript.
type Line struct {
	Type        string  `json:"type"`
	Timestamp   string  `json:"timestamp"`
	SessionID   string  `json:"sessionId"`
	ParentUUID  *string `json:"parentUuid"`
	IsSidechain bool    `json:"isSidechain"`
	Cwd         string  `json:"cwd"`
	GitBranch   string  `json:"gitBranch"`
	Version     string  `json:"version"`
	Entrypoint  string  `json:"entrypoint"`
	UserType    string  `json:"userType"`
	Message     Message `json:"message"`
	Summary     string          `json:"summary"`
	LeafUUID    string          `json:"leafUuid"`
	LastPrompt  string          `json:"lastPrompt"`
	AITitle     string          `json:"aiTitle"`
	Content     json.RawMessage `json:"content"`
}

// Session is the fully aggregated view of one transcript file.
type Session struct {
	Path       string
	SessionID  string
	Title      string
	Project    string
	GitBranch  string
	Cwd        string
	Entrypoint string
	Version    string
	FirstSeen  time.Time
	LastSeen   time.Time
	Duration   time.Duration
	Summary    string

	Models     map[string]*ModelUsage
	Total      Usage
	ServerTool ServerToolUse

	AssistantTurns int
	UserTurns      int
	IsSidechain    bool
}

// ModelUsage holds per-model aggregated counters.
type ModelUsage struct {
	Model      string
	Input      int
	CacheWrite int
	CacheRead  int
	Output     int
	Turns      int
}

func (m *ModelUsage) add(u Usage) {
	m.Input += u.InputTokens
	m.CacheWrite += u.CacheCreationInputTokens
	m.CacheRead += u.CacheReadInputTokens
	m.Output += u.OutputTokens
	m.Turns++
}

func (u *Usage) add(other Usage) {
	u.InputTokens += other.InputTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
	u.OutputTokens += other.OutputTokens
}

// ParseFile reads a single JSONL transcript and returns an aggregated Session.
// root is the projects directory (e.g. ~/.claude/projects) used to derive the
// project name; pass "" to fall back to path heuristics.
func ParseFile(path, root string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(path, root, f)
}

// Parse reads a JSONL transcript from an io.Reader.
func Parse(path, root string, r io.Reader) (*Session, error) {
	s := &Session{
		Path:    path,
		Models:  make(map[string]*ModelUsage),
		Project: deriveProject(path, root),
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var l Line
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			continue
		}
		s.absorb(l)
	}
	if err := sc.Err(); err != nil {
		return s, fmt.Errorf("scan %s: %w", path, err)
	}

	s.finalize()
	return s, nil
}

func (s *Session) absorb(l Line) {
	if l.SessionID != "" && s.SessionID == "" {
		s.SessionID = l.SessionID
	}
	if l.IsSidechain {
		s.IsSidechain = true
	}
	if l.Cwd != "" && s.Cwd == "" {
		s.Cwd = l.Cwd
	}
	if l.GitBranch != "" && s.GitBranch == "" {
		s.GitBranch = l.GitBranch
	}
	if l.Version != "" && s.Version == "" {
		s.Version = l.Version
	}
	if l.Entrypoint != "" && s.Entrypoint == "" {
		s.Entrypoint = l.Entrypoint
	}
	if l.Summary != "" && s.Summary == "" {
		s.Summary = l.Summary
	}

	if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
		if s.FirstSeen.IsZero() || ts.Before(s.FirstSeen) {
			s.FirstSeen = ts
		}
		if ts.After(s.LastSeen) {
			s.LastSeen = ts
		}
	}

	switch l.Type {
	case "assistant":
		model := l.Message.Model
		if model == "" {
			model = "unknown"
		}
		// skip zero-usage turns (e.g. synthetic/error lines)
		u := l.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
			break
		}
		s.AssistantTurns++
		mu := s.Models[model]
		if mu == nil {
			mu = &ModelUsage{Model: model}
			s.Models[model] = mu
		}
		mu.add(u)
		s.Total.add(u)
		s.ServerTool.WebSearchRequests += l.Message.ServerTool.WebSearchRequests
		s.ServerTool.WebFetchRequests += l.Message.ServerTool.WebFetchRequests
	case "user":
		s.UserTurns++
		// The AI-generated title (ai-title line) is the concise, human-readable
		// session name Claude Code shows in its UI. Prefer it over the raw first
		// prompt; only fall back to the prompt when no ai-title is present.
		if s.Title == "" {
			if prompt := extractUserPrompt(l); prompt != "" {
				s.Title = prompt
			}
		}
	case "ai-title":
		// Claude Code writes ai-title lines as a session evolves; keep the most
		// recent non-empty one so the title reflects the latest name.
		if t := strings.TrimSpace(l.AITitle); t != "" {
			s.Title = t
		}
	case "last-prompt":
		if s.Title == "" && l.LastPrompt != "" {
			s.Title = l.LastPrompt
		}
	}
}

func (s *Session) finalize() {
	if !s.LastSeen.IsZero() && !s.FirstSeen.IsZero() {
		s.Duration = s.LastSeen.Sub(s.FirstSeen)
	}
}

// deriveProject converts a file path into a human-readable project name.
// If root is provided (the projects directory), the project is the first
// path component below root. The encoded dashes are decoded back to slashes.
func deriveProject(path, root string) string {
	var raw string
	if root != "" {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != "" {
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if len(parts) > 0 {
				raw = parts[0]
			}
		}
	}
	if raw == "" {
		// fallback: parent directory name
		raw = filepath.Base(filepath.Dir(path))
	}
	return decodeProjectName(raw)
}

// decodeProjectName reverses Claude Code's encoding of directory paths.
// Claude encodes a cwd like /Users/foo/Code/bar/.claude/worktrees/x as
// -Users-foo-Code-bar--claude-worktrees-x  (slashes and dots both become -).
func decodeProjectName(name string) string {
	name = strings.TrimPrefix(name, "-")
	if strings.HasPrefix(name, "ssh-") {
		return name
	}
	// -- maps to /. (hidden dir boundary), single - maps to /
	name = strings.ReplaceAll(name, "--", "/.")
	name = strings.ReplaceAll(name, "-", "/")
	return name
}

// SortedModels returns the per-model usage sorted by total tokens descending.
func (s *Session) SortedModels() []*ModelUsage {
	out := make([]*ModelUsage, 0, len(s.Models))
	for _, m := range s.Models {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].totalTokens() > out[j].totalTokens()
	})
	return out
}

func (m *ModelUsage) totalTokens() int {
	return m.Input + m.CacheWrite + m.CacheRead + m.Output
}

// Discover finds all session JSONL files under the given root.
func Discover(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// extractUserPrompt pulls the text content from a user message line.
// Content can be a plain string or an array of content blocks.
func extractUserPrompt(l Line) string {
	content := l.Message.Content
	if len(content) == 0 {
		// top-level content field (queue-operation style)
		content = l.Content
	}
	if len(content) == 0 {
		return ""
	}
	// try string first
	var s string
	if json.Unmarshal(content, &s) == nil {
		return cleanTitle(s)
	}
	// try array of blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return cleanTitle(b.Text)
			}
		}
	}
	return ""
}

// cleanTitle normalises a prompt into a concise session title.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	// collapse internal whitespace (newlines, tabs) into spaces
	s = strings.Join(strings.Fields(s), " ")
	return s
}
