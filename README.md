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
- **HTML export** — `--html report.html` writes one self-contained page you can
  share. It has no external files, works offline, and includes a daily cost chart,
  a searchable session table, and per-session model and subagent detail.
  Press `e` in the TUI to export the current data.
- **Fast** — parallel file parsing across CPU cores; handles 1,000+ session files
  in a few seconds.

## Install

### Download a binary

Each release publishes binaries for macOS (Apple Silicon and Intel), Windows
(x64 and ARM64), and Linux (x64 and ARM64) on the
[Releases page](https://github.com/hadifarnoud/claude-usage/releases).

Download the archive for your platform, unpack it, and run the binary:

```bash
# macOS (Apple Silicon)
tar -xzf claude-usage_*_darwin_arm64.tar.gz
./claude-usage_*_darwin_arm64/claude-usage
```

macOS marks downloaded binaries as quarantined. If Gatekeeper blocks the
binary, remove the quarantine flag:

```bash
xattr -d com.apple.quarantine ./claude-usage
```

On Windows, unpack the `.zip` and run `claude-usage.exe` from a terminal.

`checksums.txt` in each release holds the SHA-256 of every archive.

### Install with Go

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

# Single-file HTML report you can share
claude-usage --html usage.html
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
| `--html`      |                | Write a self-contained HTML report to this file and exit |
| `--version`   | `false`        | Print the version and exit                          |

### TUI Keybindings

| Key         | Action                          |
|-------------|---------------------------------|
| `tab` / `→` | Next view                       |
`shift+tab` / `←` | Previous view              |
| `1`–`4`     | Jump to view                    |
| `↑` / `↓`   | Navigate rows                   |
| `enter`     | Session detail (on Sessions tab) |
| `s`         | Toggle sort: cost ⇄ time (Sessions) |
| `f`         | Cycle time filter: all → 24h → 7d → all (Sessions) |
| `e`         | Export a self-contained HTML report to the working directory |
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

Cache write is 1.25x the input rate and cache read is 0.1x the input rate.

| Model family | Input | Output | Cache write | Cache read |
|---|---|---|---|---|
| Fable 5, Mythos 5 | $10 | $50 | $12.50 | $1.00 |
| Opus 5, Opus 4.8 / 4.7 / 4.6 | $5 | $25 | $6.25 | $0.50 |
| Opus 4.5 and older | $15 | $75 | $18.75 | $1.50 |
| Sonnet 5 | $2 | $10 | $2.50 | $0.20 |
| Sonnet 4.6 and older | $3 | $15 | $3.75 | $0.30 |
| Haiku 4.5 | $1 | $5 | $1.25 | $0.10 |
| Haiku 3.5 | $0.80 | $4 | $1.00 | $0.08 |

Opus dropped from $15/$75 to $5/$25 at version 4.6, so Opus 4.5 and older keep
the old rate. An unknown model falls back to the current Opus rate, so a new
release is never reported as free.

Prices are Anthropic first-party API list rates. Costs are estimates: they do
not include Batch API discounts, 1-hour cache TTL rates, fast mode, or
partner pricing on Bedrock, Vertex AI, or Foundry.

## Project structure

```
cmd/claude-usage/   Entry point — CLI flags, file discovery, parallel parsing
internal/session/   JSONL transcript parser and session aggregation
internal/pricing/   Model → price mapping and cost calculation
internal/report/    Cost aggregation, text and HTML rendering, grouping
internal/tui/       Bubble Tea interactive UI with auto-refresh
```
