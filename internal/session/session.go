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
	AgentID     string  `json:"agentId"`
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
	FirstPrompt string
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

	// Subagents spawned via the Task tool live in separate transcript files
	// under {project}/{sessionId}/subagents/. They are loaded by LoadSubagents
	// and kept out of the main Models/Total so the session's top-level cost
	// reflects only the parent conversation.
	Subagents     map[string]*Subagent
	SubagentOrder []string // agentIDs in first-seen order

	AssistantTurns int
	UserTurns      int
	IsSidechain    bool
}

// Subagent is the aggregated usage of one Task-tool dispatch.
type Subagent struct {
	AgentID        string
	AgentType      string // from agent-*.meta.json ("Explore", "general-purpose", ...)
	Description    string // task prompt from agent-*.meta.json
	Models         map[string]*ModelUsage
	Total          Usage
	AssistantTurns int
	FirstSeen      time.Time
	LastSeen       time.Time
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
		Path:      path,
		Models:    make(map[string]*ModelUsage),
		Subagents: make(map[string]*Subagent),
		Project:   deriveProject(path, root),
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
		if l.IsSidechain && l.AgentID != "" {
			sa := s.subagent(l.AgentID)
			if sa.FirstSeen.IsZero() || ts.Before(sa.FirstSeen) {
				sa.FirstSeen = ts
			}
			if ts.After(sa.LastSeen) {
				sa.LastSeen = ts
			}
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
		// Sidechain (subagent) turns are routed into their own bucket and kept
		// out of the parent session's totals.
		if l.IsSidechain && l.AgentID != "" {
			sa := s.subagent(l.AgentID)
			sa.AssistantTurns++
			mu := sa.Models[model]
			if mu == nil {
				mu = &ModelUsage{Model: model}
				sa.Models[model] = mu
			}
			mu.add(u)
			sa.Total.add(u)
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
		if l.IsSidechain {
			// subagent user turns are tracked separately if needed; skip here.
			break
		}
		s.UserTurns++
		// The AI-generated title (ai-title line) is the concise, human-readable
		// session name Claude Code shows in its UI. Prefer it over the raw first
		// prompt; only fall back to the prompt when no ai-title is present.
		if prompt := extractUserPrompt(l); prompt != "" {
			// Always keep the first user prompt for the detail view, even when
			// an ai-title later overrides the session Title.
			if s.FirstPrompt == "" {
				s.FirstPrompt = prompt
			}
			if s.Title == "" {
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

// subagent returns the subagent bucket for agentID, creating and ordering it
// on first reference.
func (s *Session) subagent(agentID string) *Subagent {
	if sa, ok := s.Subagents[agentID]; ok {
		return sa
	}
	sa := &Subagent{AgentID: agentID, Models: make(map[string]*ModelUsage)}
	if s.Subagents == nil {
		s.Subagents = make(map[string]*Subagent)
	}
	s.Subagents[agentID] = sa
	s.SubagentOrder = append(s.SubagentOrder, agentID)
	return sa
}

// agentMeta is the structure of the agent-*.meta.json sibling file.
type agentMeta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	ToolUseID   string `json:"toolUseId"`
	SpawnDepth  int    `json:"spawnDepth"`
}

// LoadSubagents discovers and parses the subagent transcripts that belong to
// this session. Subagents live under {project}/{sessionId}/subagents/ and are
// not returned by Discover, so this must be called explicitly after Parse.
// It is a no-op when the session has no subagents directory (or no SessionID).
func (s *Session) LoadSubagents() error {
	if s.SessionID == "" || s.Path == "" {
		return nil
	}
	dir := filepath.Join(filepath.Dir(s.Path), s.SessionID, "subagents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		path := filepath.Join(dir, name)
		agentID := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")

		sa, err := parseSubagent(path, agentID)
		if err != nil || sa == nil {
			continue
		}
		// Prefer an already-created bucket (from inline sidechain lines) so
		// ordering is stable; otherwise register the freshly parsed one.
		if existing, ok := s.Subagents[agentID]; ok {
			*existing = *sa
		} else {
			if s.Subagents == nil {
				s.Subagents = make(map[string]*Subagent)
			}
			s.Subagents[agentID] = sa
			s.SubagentOrder = append(s.SubagentOrder, agentID)
		}
	}
	return nil
}

// parseSubagent reads one subagent transcript and its sibling .meta.json.
func parseSubagent(path, agentID string) (*Subagent, error) {
	sa := &Subagent{AgentID: agentID, Models: make(map[string]*ModelUsage)}

	// metadata (agent type + task description) lives in a sibling .meta.json
	metaPath := strings.TrimSuffix(path, ".jsonl") + ".meta.json"
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta agentMeta
		if json.Unmarshal(data, &meta) == nil {
			sa.AgentType = meta.AgentType
			sa.Description = meta.Description
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var l Line
		if json.Unmarshal([]byte(line), &l) != nil {
			continue
		}
		if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
			if sa.FirstSeen.IsZero() || ts.Before(sa.FirstSeen) {
				sa.FirstSeen = ts
			}
			if ts.After(sa.LastSeen) {
				sa.LastSeen = ts
			}
		}
		if l.Type != "assistant" {
			continue
		}
		u := l.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
			continue
		}
		model := l.Message.Model
		if model == "" {
			model = "unknown"
		}
		sa.AssistantTurns++
		mu := sa.Models[model]
		if mu == nil {
			mu = &ModelUsage{Model: model}
			sa.Models[model] = mu
		}
		mu.add(u)
		sa.Total.add(u)
	}
	if err := sc.Err(); err != nil {
		return sa, err
	}
	// skip subagents with no real usage (e.g. empty/aborted transcripts)
	if sa.Total.InputTokens == 0 && sa.Total.OutputTokens == 0 &&
		sa.Total.CacheCreationInputTokens == 0 && sa.Total.CacheReadInputTokens == 0 {
		return nil, nil
	}
	return sa, nil
}

// SortedSubagents returns the session's subagents ordered by first-seen time,
// which matches the order they were spawned.
func (s *Session) SortedSubagents() []*Subagent {
	out := make([]*Subagent, 0, len(s.SubagentOrder))
	for _, id := range s.SubagentOrder {
		if sa, ok := s.Subagents[id]; ok {
			out = append(out, sa)
		}
	}
	return out
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
	// Collapse git worktrees under their parent repo. Claude stores each
	// worktree as its own project directory, encoding a cwd like
	// /Users/foo/Code/bar/.claude/worktrees/x as -Users-foo-Code-bar--claude-worktrees-x.
	// Strip the --claude-worktrees-* suffix so all worktrees of a repo share
	// one project identity with the repo's main checkout.
	if idx := strings.Index(raw, "--claude-worktrees-"); idx >= 0 {
		raw = raw[:idx]
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

// Discover finds all session JSONL files under the given root. Subagent
// transcripts (under a "subagents" directory) are excluded — they belong to a
// parent session and are loaded via Session.LoadSubagents.
func Discover(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// prune subagent directories so their transcripts aren't picked up
			// as standalone sessions
			if info.Name() == "subagents" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".jsonl") {
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
