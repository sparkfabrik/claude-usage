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
- **Minimal polling cost** — ~$0.017/day, ~$0.52/month with 60s interval
- **Single static binary** — no runtime dependencies, no Python, no venv
- **Configurable** — YAML config for polling interval, display periods, cost visibility, color thresholds, model pricing overrides

## Requirements

- Go 1.23+ (build only)
- Claude Code with valid credentials at `~/.claude/.credentials.json`
- GNOME Shell 45–50 (for the extension)

## Installation

### Build and install everything

```bash
make install
```

This builds the binary, installs it to `~/.local/bin/claude-usage`, and copies the GNOME Shell extension.

### Binary only

```bash
make install-binary
```

### Extension only

```bash
make install-gnome-extension
gnome-extensions enable claude-usage@claude-code-usage
```

> **Note:** On Wayland, you must log out and back in (or use a nested shell) for extension changes to take effect.

### KDE Plasma 6 plasmoid

```bash
make install-kde
```

Then add "Claude Usage" widget to your panel. See [readers/kde-plasmoid/README.md](readers/kde-plasmoid/README.md).

### Waybar module

```bash
make install-waybar
```

Add to your Waybar config. See [readers/waybar/README.md](readers/waybar/README.md).

### macOS tray app

```bash
make install-macos-tray
```

Requires macOS and CGo. See [readers/macos-tray/README.md](readers/macos-tray/README.md).

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

# JSON status output (used by GNOME extension and other consumers)
claude-usage --status

# Force a fresh API poll with status output
claude-usage --status --force-poll

# Cache-only status, no API call (returns stale data if cache exists)
claude-usage --status --no-poll

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
- Labels dimmed at 50% opacity when data is stale
- Click for a dropdown with:
  - Current/Weekly stats with reset times (same colors as panel)
  - Stale data warning (orange) when applicable
  - Claude Code process state: running (green) / not running (orange)
  - "Refresh Now" button
  - Disclaimer noting data is estimated
- Calls `claude-usage --status` every 30s; all logic lives in the CLI
- "Refresh Now" uses `--status --force-poll` to trigger an API call
- Binary lookup: checks `$PATH` first, falls back to `~/.local/bin/claude-usage`

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

## Testing the extension

Requires `mutter-devkit` package (Arch: `mutter`):

```bash
make test-gnome-extension
```

This installs the extension and launches a nested GNOME Shell via `dbus-run-session gnome-shell --devkit --wayland`.

## Uninstall

```bash
make uninstall
```

## How polling works

1. Sends a minimal API request: `max_tokens: 1`, `content: "."`, `temperature: 0` using `claude-haiku-4-5-20251001`
2. Reads `x-ratelimit-*` response headers for utilization and reset times
3. Writes results to a JSON cache at `~/.cache/claude-code-usage/quota.json`
4. GNOME extension calls `claude-usage --status` every 30s, which returns JSON with raw data (percentages, reset times, colors, stale flag)
5. The CLI handles cache freshness internally — if cache is warm, it returns cached data without polling
6. The extension is a pure renderer — it formats display text and handles the stale indicator locally

## Cost note

Cost values shown are **API-equivalent estimates** based on token counts and published pricing. If you're on a Claude subscription (Pro/Team/Enterprise), usage is included in your plan — the costs shown represent what the equivalent API usage would cost, not actual charges.

## License

[GPL-3.0](LICENSE)
