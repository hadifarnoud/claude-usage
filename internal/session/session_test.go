package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleTranscript = `
{"type":"user","sessionId":"abc-123","timestamp":"2026-01-01T10:00:00Z","cwd":"/home/user/proj","gitBranch":"main","version":"2.0.0","message":{"role":"user","content":"hello"}}
{"type":"assistant","sessionId":"abc-123","timestamp":"2026-01-01T10:00:05Z","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":100,"cache_creation_input_tokens":5000,"cache_read_input_tokens":2000,"output_tokens":300}}}
{"type":"assistant","sessionId":"abc-123","timestamp":"2026-01-01T10:00:10Z","message":{"model":"claude-opus-4-8","role":"assistant","usage":{"input_tokens":50,"cache_creation_input_tokens":3000,"cache_read_input_tokens":1000,"output_tokens":200}}}
{"type":"assistant","sessionId":"abc-123","timestamp":"2026-01-01T10:00:15Z","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}
`

func TestParse(t *testing.T) {
	s, err := Parse("test.jsonl", "/root", strings.NewReader(sampleTranscript))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want abc-123", s.SessionID)
	}
	if s.Title != "hello" {
		t.Errorf("Title = %q, want \"hello\"", s.Title)
	}
	if s.Cwd != "/home/user/proj" {
		t.Errorf("Cwd = %q", s.Cwd)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q", s.GitBranch)
	}
	// 3 assistant lines but 1 is zero-usage, so only 2 real turns
	if s.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2", s.AssistantTurns)
	}
	if s.UserTurns != 1 {
		t.Errorf("UserTurns = %d, want 1", s.UserTurns)
	}

	// totals: 100+50 input, 5000+3000 cache-w, 2000+1000 cache-r, 300+200 output
	if s.Total.InputTokens != 150 {
		t.Errorf("Total input = %d, want 150", s.Total.InputTokens)
	}
	if s.Total.CacheCreationInputTokens != 8000 {
		t.Errorf("Total cache-w = %d, want 8000", s.Total.CacheCreationInputTokens)
	}
	if s.Total.OutputTokens != 500 {
		t.Errorf("Total output = %d, want 500", s.Total.OutputTokens)
	}

	models := s.SortedModels()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	// sorted by total tokens descending: sonnet has more
	if models[0].Model != "claude-sonnet-5" {
		t.Errorf("top model = %q, want claude-sonnet-5", models[0].Model)
	}
}

func TestDeriveProject(t *testing.T) {
	cases := []struct {
		path, root, want string
	}{
		{"/root/-Users-foo-Code-bar/uuid.jsonl", "/root", "Users/foo/Code/bar"},
		{"/root/-Users-foo-Code-bar--claude-worktrees-x/uuid.jsonl", "/root", "Users/foo/Code/bar"},
		{"/root/-Users-foo-Code-bar--claude-worktrees-brave-nobel-52d39f/uuid.jsonl", "/root", "Users/foo/Code/bar"},
		{"/root/ssh-abc-def/uuid.jsonl", "/root", "ssh-abc-def"},
		{"/root/-Users-foo-Code-bar/uuid/subagents/a.jsonl", "/root", "Users/foo/Code/bar"},
	}
	for _, c := range cases {
		got := deriveProject(c.path, c.root)
		if got != c.want {
			t.Errorf("deriveProject(%q, %q) = %q, want %q", c.path, c.root, got, c.want)
		}
	}
}

func TestExtractUserPromptArray(t *testing.T) {
	transcript := `{"type":"user","sessionId":"x","message":{"role":"user","content":[{"type":"text","text":"build a web app"}]}}`
	s, err := Parse("t.jsonl", "/root", strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "build a web app" {
		t.Errorf("Title = %q, want \"build a web app\"", s.Title)
	}
}

func TestCleanTitle(t *testing.T) {
	got := cleanTitle("  hello\n  world  \t foo  ")
	if got != "hello world foo" {
		t.Errorf("cleanTitle = %q", got)
	}
}

func TestAITitlePreferredOverPrompt(t *testing.T) {
	// The user prompt appears first, then Claude writes an ai-title line.
	// The concise ai-title should win over the raw prompt.
	transcript := `{"type":"user","sessionId":"x","timestamp":"2026-01-01T10:00:00Z","message":{"role":"user","content":"Hestudio is a small startup studio. we want to redesign the site"}}
{"type":"assistant","sessionId":"x","timestamp":"2026-01-01T10:00:05Z","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"ai-title","sessionId":"x","aiTitle":"Implement Hestudio Redesign"}
{"type":"ai-title","sessionId":"x","aiTitle":"Implement Hestudio Redesign design file"}
`
	s, err := Parse("t.jsonl", "/root", strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Implement Hestudio Redesign design file" {
		t.Errorf("Title = %q, want the latest ai-title \"Implement Hestudio Redesign design file\"", s.Title)
	}
}

func TestPromptFallbackWhenNoAITitle(t *testing.T) {
	// No ai-title line: fall back to the first user prompt.
	transcript := `{"type":"user","sessionId":"x","message":{"role":"user","content":"just a prompt"}}
{"type":"assistant","sessionId":"x","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":10,"output_tokens":5}}}
`
	s, err := Parse("t.jsonl", "/root", strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "just a prompt" {
		t.Errorf("Title = %q, want \"just a prompt\"", s.Title)
	}
}

func TestSidechainRoutedToSubagent(t *testing.T) {
	// A sidechain assistant turn must NOT count toward the parent totals; it
	// lands in the subagent bucket keyed by agentId.
	transcript := `{"type":"user","sessionId":"s1","timestamp":"2026-01-01T10:00:00Z","message":{"role":"user","content":"hi"}}
{"type":"assistant","sessionId":"s1","timestamp":"2026-01-01T10:00:05Z","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"assistant","sessionId":"s1","timestamp":"2026-01-01T10:00:10Z","isSidechain":true,"agentId":"abc123","message":{"model":"claude-haiku-4-5","role":"assistant","usage":{"input_tokens":1000,"cache_creation_input_tokens":5000,"output_tokens":200}}}
`
	s, err := Parse("test.jsonl", "/root", strings.NewReader(transcript))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// parent totals only include the main turn
	if s.Total.InputTokens != 100 || s.Total.OutputTokens != 50 {
		t.Errorf("parent total = in %d out %d, want in 100 out 50", s.Total.InputTokens, s.Total.OutputTokens)
	}
	if len(s.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(s.Subagents))
	}
	sa := s.Subagents["abc123"]
	if sa == nil {
		t.Fatal("subagent abc123 missing")
	}
	if sa.Total.InputTokens != 1000 || sa.Total.OutputTokens != 200 || sa.Total.CacheCreationInputTokens != 5000 {
		t.Errorf("subagent total = %+v", sa.Total)
	}
	if sa.AssistantTurns != 1 {
		t.Errorf("subagent turns = %d, want 1", sa.AssistantTurns)
	}
	if len(sa.Models) != 1 || sa.Models["claude-haiku-4-5"] == nil {
		t.Errorf("subagent models = %+v", sa.Models)
	}
}

func TestLoadSubagents(t *testing.T) {
	// Build a fake projects dir:
	//   root/-proj/sid.jsonl          (parent)
	//   root/-proj/sid/subagents/agent-deadbeef.jsonl
	//   root/-proj/sid/subagents/agent-deadbeef.meta.json
	dir := t.TempDir()
	proj := filepath.Join(dir, "-proj")
	sid := "sid-123"
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	parentPath := filepath.Join(proj, sid+".jsonl")
	parent := `{"type":"user","sessionId":"` + sid + `","message":{"role":"user","content":"do stuff"}}
{"type":"assistant","sessionId":"` + sid + `","message":{"model":"claude-sonnet-5","role":"assistant","usage":{"input_tokens":10,"output_tokens":5}}}
`
	if err := os.WriteFile(parentPath, []byte(parent), 0o644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(proj, sid, "subagents")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	agentID := "deadbeef"
	subJSONL := `{"type":"assistant","sessionId":"` + sid + `","isSidechain":true,"agentId":"` + agentID + `","timestamp":"2026-01-01T10:00:00Z","message":{"model":"claude-haiku-4-5","role":"assistant","usage":{"input_tokens":300,"cache_creation_input_tokens":7000,"output_tokens":80}}}
{"type":"assistant","sessionId":"` + sid + `","isSidechain":true,"agentId":"` + agentID + `","timestamp":"2026-01-01T10:00:05Z","message":{"model":"claude-haiku-4-5","role":"assistant","usage":{"input_tokens":200,"output_tokens":40}}}
`
	if err := os.WriteFile(filepath.Join(subDir, "agent-"+agentID+".jsonl"), []byte(subJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := `{"agentType":"Explore","description":"find the config file","toolUseId":"toolu_x","spawnDepth":1}`
	if err := os.WriteFile(filepath.Join(subDir, "agent-"+agentID+".meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := ParseFile(parentPath, dir)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if err := s.LoadSubagents(); err != nil {
		t.Fatalf("LoadSubagents: %v", err)
	}
	if len(s.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(s.Subagents))
	}
	sa := s.Subagents[agentID]
	if sa == nil {
		t.Fatal("subagent missing")
	}
	if sa.AgentType != "Explore" {
		t.Errorf("AgentType = %q, want Explore", sa.AgentType)
	}
	if sa.Description != "find the config file" {
		t.Errorf("Description = %q", sa.Description)
	}
	// 300 + 200 input across two turns
	if sa.Total.InputTokens != 500 {
		t.Errorf("subagent input = %d, want 500", sa.Total.InputTokens)
	}
	if sa.AssistantTurns != 2 {
		t.Errorf("subagent turns = %d, want 2", sa.AssistantTurns)
	}
	// parent totals unaffected by subagent
	if s.Total.InputTokens != 10 {
		t.Errorf("parent input = %d, want 10", s.Total.InputTokens)
	}
}

func TestDiscoverSkipsSubagents(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "-proj")
	sid := "sid-1"
	os.MkdirAll(filepath.Join(proj, sid, "subagents"), 0o755)
	os.WriteFile(filepath.Join(proj, sid+".jsonl"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(proj, sid, "subagents", "agent-aaa.jsonl"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(proj, sid, "subagents", "agent-aaa.meta.json"), []byte("{}"), 0o644)

	files, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (parent only), got %d: %v", len(files), files)
	}
	if !strings.HasSuffix(files[0], "sid-1.jsonl") {
		t.Errorf("expected parent file, got %q", files[0])
	}
}
