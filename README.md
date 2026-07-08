# claude-usage

A terminal application that reads Claude Code session transcript files (JSONL)
and reports token usage and cost breakdowns per session, per model, per project,
and per day.

## Features

- **Interactive TUI** — sortable tables with four views: Sessions, Models, Projects, Daily.
- **Auto-refresh** — re-scans session files on a configurable interval (default 15 min).
  Press `r` to refresh manually. The header shows a countdown to the next refresh.
- **Cost calculation** — per-model pricing for input, output, cache writes (1.25x),
  and cache reads (0.1x), matching Anthropic's published rates.
- **Text mode** — `--no-tui` prints a full report to stdout for scripting / piping.
- **Fast** — parallel file parsing across CPU cores; handles 1,000+ session files
  in a few seconds.

## Install

```bash
go install github.com/hadifarnoud/claude-usage/cmd/claude-usage@latest
```

Or build from source:

```bash
git clone <repo> && cd claude-usage
go build -o claude-usage ./cmd/claude-usage
```

## Usage

```bash
# Interactive TUI with default 15-min auto-refresh
claude-usage

# Custom refresh interval (5 minutes)
claude-usage --interval 5m

# Disable auto-refresh
claude-usage --interval 0

# Text report (no TUI), top 30 sessions
claude-usage --no-tui --top 30

# Analyse a single session file
claude-usage --path ~/.claude/projects/-Users-foo-bar/abc123.jsonl

# Custom projects directory
claude-usage --dir /path/to/projects
```

### Flags

| Flag          | Default        | Description                                          |
|---------------|----------------|-----------------------------------------------------|
| `--interval`  | `15m`          | TUI auto-refresh interval (`0` disables; e.g. `30s`) |
| `--no-tui`    | `false`        | Print text report instead of launching the TUI       |
| `--top`       | `20`           | Number of top sessions in text mode (`0` = all)     |
| `--path`      |                | Analyse a single `.jsonl` session file              |
| `--dir`       | `~/.claude/projects` | Custom Claude projects directory               |
| `--quiet`     | `false`        | Suppress progress output in text mode               |

### TUI Keybindings

| Key         | Action                          |
|-------------|---------------------------------|
| `tab` / `→` | Next view                       |
`shift+tab` / `←` | Previous view              |
| `1`–`4`     | Jump to view                    |
| `↑` / `↓`   | Navigate rows                   |
| `enter`     | Session detail (on Sessions tab) |
| `r`         | Refresh now                     |
| `esc`       | Back / close detail             |
| `q` / `ctrl+c` | Quit                        |

## How it works

Claude Code stores session transcripts as JSONL files under `~/.claude/projects/`.
Each line is a JSON record. Assistant messages contain a `usage` block with
`input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`, and
`output_tokens`. The tool parses these, aggregates per-session totals, and
applies model-specific pricing to compute dollar costs.

### Pricing tiers (per 1M tokens)

| Model family | Input | Output | Cache write | Cache read |
|---|---|---|---|---|
| Opus 4.x    | $15  | $75  | $18.75 | $1.50 |
| Sonnet 4–5  | $3   | $15  | $3.75  | $0.30 |
| Sonnet 3.x  | $3   | $15  | $3.75  | $0.30 |
| Haiku 4.x   | $1   | $5   | $1.25  | $0.10 |
| Haiku 3.x   | $0.80| $4   | $1.00  | $0.08 |
| Fable       | $5   | $25  | $6.25  | $0.50 |

## Project structure

```
cmd/claude-usage/   Entry point — CLI flags, file discovery, parallel parsing
internal/session/   JSONL transcript parser and session aggregation
internal/pricing/   Model → price mapping and cost calculation
internal/report/    Cost aggregation, text rendering, multi-dimensional grouping
internal/tui/       Bubble Tea interactive UI with auto-refresh
```
