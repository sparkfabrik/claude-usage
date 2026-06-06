# Claude Code Usage Monitor

CLI dashboard + multi-platform desktop readers for monitoring Claude Code usage — utilization percentages, rate-limit reset times, token counts, and estimated costs.

## Architecture

```
┌─────────────────────────────────────────────────┐
│              claude-usage (Go CLI)               │
│  Poller → Cache → --status JSON → CLI Dashboard │
└─────────────────────┬───────────────────────────┘
                      │ --status (every 60s)
        ┌─────────────┼─────────────┬──────────────┐
        │             │             │              │
   ┌────┴────┐  ┌────┴────┐  ┌────┴────┐  ┌─────┴─────┐
   │  GNOME  │  │   KDE   │  │ Waybar  │  │  macOS    │
   │Extension│  │Plasmoid │  │ Module  │  │  Tray     │
   └─────────┘  └─────────┘  └─────────┘  └───────────┘
                                    │
                             ┌──────┴──────┐
                             │ Statusline  │
                             │  (Bash)     │
                             └─────────────┘
```

All readers call `claude-usage --status` every 60 seconds and render the JSON response. No reader contains polling or caching logic.

## How it works

Claude Code has no public usage API. This tool polls the Anthropic API with a minimal 1-token Haiku request (~$0.000012/poll) and reads the rate-limit headers to determine your current utilization. Token and cost data is parsed from Claude Code's local JSONL conversation logs.

## Features

- **CLI dashboard** — colored tables showing utilization, tokens, and costs across configurable time periods
- **GNOME Shell extension** — panel indicator with current (5h) and weekly (7d) utilization percentages, with detailed stats in dropdown menu
- **KDE Plasma 6 plasmoid** — panel widget with utilization display and click-to-expand details
- **Waybar module** — custom module with glyph indicators and CSS class theming
- **macOS tray app** — menu bar item with dropdown details and color-coded status
- **Terminal statusline** — lightweight Bash script for tmux/shell prompt integration
- **Minimal polling cost** — ~$0.017/day, ~$0.52/month with 60s interval
- **Single static binary** — no runtime dependencies, no Python, no venv
- **Configurable** — YAML config for polling interval, display periods, cost visibility, color thresholds, model pricing overrides

## Requirements

- Claude Code with valid credentials at `~/.claude/.credentials.json`

**Per platform:**

| Platform | Requirement               |
| -------- | ------------------------- |
| GNOME    | GNOME Shell 45–50         |
| KDE      | Plasma 6, `kpackagetool6` |
| Waybar   | Waybar, `jq`              |
| macOS    | macOS 12+                 |

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | bash
```

The installer auto-detects your OS, architecture, and desktop environment, then installs the CLI binary and the appropriate reader (GNOME extension, KDE plasmoid, Waybar module, or macOS tray).

**Pin a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | CLAUDE_USAGE_VERSION=v1.0.0 bash
```

**Upgrade:** Re-run the install command. The installer is idempotent — it skips if already up to date, upgrades if a new version is available.

**Uninstall:**

```bash
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | bash -s -- --uninstall
```

### Options

| Flag / env var               | Default | Effect                                                                                              |
| ---------------------------- | ------- | --------------------------------------------------------------------------------------------------- |
| `--statusline` / `CLAUDE_USAGE_STATUSLINE=1` | off | Register the Claude Code statusLine in `~/.claude/settings.json` (opt-in).                           |
| `--no-reader` / `CLAUDE_USAGE_READER=0`      | reader on | Skip wiring the desktop reader. Reader files are still placed on disk; only the wiring is skipped.   |
| `CLAUDE_USAGE_VERSION=<tag>`                 | latest  | Pin a specific release.                                                                              |
| `INSTALL_DIR=<path>`                         | `~/.local/share/claude-usage` | Override the installation directory.                                              |

```bash
# Enable the Claude Code statusline as well as the desktop reader
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | CLAUDE_USAGE_STATUSLINE=1 bash

# CLI + statusline only, no desktop reader wiring
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | CLAUDE_USAGE_READER=0 CLAUDE_USAGE_STATUSLINE=1 bash
```

When passing flags through a piped install, append them after `bash -s --` (e.g. `bash -s -- --no-reader`).

### What the installer does

1. Downloads the correct binary for your OS/arch from GitHub Releases
2. Installs to `~/.local/share/claude-usage/` with symlinks in `~/.local/bin/`
3. Detects your desktop environment and wires the matching reader (unless `--no-reader`):
   - **GNOME** → symlinks the shell extension, prints enable command
   - **KDE** → installs plasmoid via `kpackagetool6`, prints widget instructions
   - **Waybar** → symlinks the module script, prints config snippet
   - **macOS** → installs the tray binary, prints launch instructions
4. Symlinks the terminal statusline script (always), but only registers it in `~/.claude/settings.json` when `--statusline` / `CLAUDE_USAGE_STATUSLINE=1` is given

## Usage

### CLI

```bash
# Full dashboard with polling
claude-usage

# Skip polling, use cached data
claude-usage --no-poll

# Force poll even if cache is fresh
claude-usage --force-poll

# Hide cost estimates
claude-usage --no-cost

# Show specific period only (today, 7d, 30d, all)
claude-usage --period 7d

# Poll and update cache silently (for scripting/cron)
claude-usage --poll-only

# JSON status output (used by readers)
claude-usage --status

# Force a fresh API poll with status output
claude-usage --status --force-poll

# Cache-only status, no API call
claude-usage --status --no-poll
# The --status JSON includes an "auth" field: "valid" | "expired" | "missing" | "unknown"

# Show version info
claude-usage --version

# Use custom config file
claude-usage --config /path/to/config.yaml
```

### GNOME Shell extension

Once enabled, the extension shows a panel indicator:

```
C:42%  W:67%
```

- **C** — Current period (5h window) utilization
- **W** — Weekly period (7d window) utilization
- Color-coded: green (<80%), orange (80–89%), red (>=90%)
- Faded to 50% opacity when Claude Code is not running
- Auth state when not valid: expired (orange) / missing (red) / unknown (grey)
- Click for dropdown with reset times, status details, and "Refresh Now" button

### KDE Plasma 6 plasmoid

Panel widget showing "C:X% W:Y%" with color-coded text. Click to expand details with reset times and error info. Shows an auth-state line when credentials are expired or missing. Dims at 50% opacity when data is stale, hides when Claude is not running.

### Waybar module

Displays a glyph + percentages: `◑ 5h:42% 7d:67%`

- Glyphs: ◔ (<50%), ◑ (50-74%), ◕ (75-94%), ● (>=95%)
- CSS classes: `normal`, `warning`, `critical`, `error` (`error` also when auth is expired/missing)
- Tooltip shows reset times, plus an auth note when credentials are expired/missing
- Hides when Claude is not running

### macOS tray

Menu bar item showing "C:X% W:Y%". Click for dropdown with reset times, auth state (shown when not valid), "Refresh Now", and "Quit".

## Configuration

Default config location: `~/.config/claude-code-usage/config.yaml`

See [`config.default.yaml`](config.default.yaml) for all options:

```yaml
api:
  enabled: true
  stale_after: 60 # seconds — poll API if cache is older than this
  model: claude-haiku-4-5-20251001
  only_when_active: true # only poll when Claude Code is running

display:
  show_cost: true
  periods:
    - today
    - 7d
    - 30d

colors:
  green_below: 80 # percentage thresholds
  orange_below: 90

cache:
  # path: /custom/path/quota.json  # default: ~/.cache/claude-code-usage/quota.json
```

## How polling works

1. Sends a minimal API request: `max_tokens: 1`, `content: "."`, `temperature: 0` using `claude-haiku-4-5-20251001`
2. Reads `x-ratelimit-*` response headers for utilization and reset times
3. Writes results to a JSON cache at `~/.cache/claude-code-usage/quota.json`
4. Readers call `claude-usage --status` every 60s, which returns JSON (percentages, reset times, colors, stale flag)
5. The CLI handles cache freshness internally — if cache is warm, it returns cached data without polling
6. Readers are pure renderers — they format display text and handle stale indicators locally

## Cost note

Cost values shown are **API-equivalent estimates** based on token counts and published pricing. If you're on a Claude subscription (Pro/Team/Enterprise), usage is included in your plan — the costs shown represent what the equivalent API usage would cost, not actual charges.

## License

[GPL-3.0](LICENSE)
