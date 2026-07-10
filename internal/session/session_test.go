package session

import (
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
		{"/root/-Users-foo-Code-bar--claude-worktrees-x/uuid.jsonl", "/root", "Users/foo/Code/bar/.claude/worktrees/x"},
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
