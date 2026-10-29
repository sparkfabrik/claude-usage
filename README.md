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

Utilization comes from Anthropic's OAuth usage endpoint, authenticated with the OAuth token Claude Code already stores. It consumes no quota, and it reports **model-scoped windows** such as "Opus Weekly" alongside the session and weekly figures. That matters because a single model can be nearly exhausted while the overall weekly number still looks calm.

If the endpoint is unreachable, the tool falls back to its original method: a 1-token Haiku request (~$0.000012/poll) whose rate-limit response headers carry the session and weekly utilization. The fallback reports no model-scoped windows.

Token and cost data is parsed from Claude Code's local JSONL conversation logs. That scan is cached, because a working machine can hold hundreds of megabytes of transcripts and the readers poll every minute.

## Features

- **CLI dashboard** — colored tables showing utilization, tokens, and costs across configurable time periods
- **GNOME Shell extension** — panel indicator with current (5h) and weekly (7d) utilization percentages, with detailed stats in dropdown menu
- **KDE Plasma 6 plasmoid** — panel widget with utilization display and click-to-expand details
- **Waybar module** — custom module with glyph indicators and CSS class theming
- **macOS tray app** — menu bar item with dropdown details and color-coded status
- **Terminal statusline** — lightweight Bash script for tmux/shell prompt integration
- **Free by default** — the usage endpoint costs nothing; the paid 1-token poll is only a fallback (~$0.017/day if it ever kicks in)
- **Model-scoped limits** — see "Opus Weekly" or "Fable Weekly" separately from the overall weekly window
- **Single static binary** — no runtime dependencies, no Python, no venv
- **Configurable** — YAML config for polling interval, display periods, cost visibility, color thresholds, model pricing overrides
- **JSON usage report** — `--report 30d` prints tokens, cost and session statistics for a period as one JSON document, offline and without identifying data

## Requirements

- Claude Code with valid credentials at `~/.claude/.credentials.json`

**Per platform:**

| Platform | Requirement               |
| -------- | ------------------------- |
| GNOME    | GNOME Shell 45–51         |
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

| Flag / env var                               | Default                       | Effect                                                                                             |
| -------------------------------------------- | ----------------------------- | -------------------------------------------------------------------------------------------------- |
| `--statusline` / `CLAUDE_USAGE_STATUSLINE=1` | off                           | Register the Claude Code statusLine in `~/.claude/settings.json` (opt-in).                         |
| `--tray` / `CLAUDE_USAGE_TRAY=1`             | off                           | macOS only: register the session hooks that start and stop the tray with Claude Code (opt-in).     |
| `--no-reader` / `CLAUDE_USAGE_READER=0`      | reader on                     | Skip wiring the desktop reader. Reader files are still placed on disk; only the wiring is skipped. |
| `CLAUDE_USAGE_VERSION=<tag>`                 | latest                        | Pin a specific release.                                                                            |
| `INSTALL_DIR=<path>`                         | `~/.local/share/claude-usage` | Override the installation directory.                                                               |

```bash
# Enable the Claude Code statusline as well as the desktop reader
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | CLAUDE_USAGE_STATUSLINE=1 bash

# macOS: start the tray automatically with Claude Code sessions
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | CLAUDE_USAGE_TRAY=1 bash

# CLI + statusline only, no desktop reader wiring
curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | CLAUDE_USAGE_READER=0 CLAUDE_USAGE_STATUSLINE=1 bash
```

When passing flags through a piped install, append them after `bash -s --` (e.g. `bash -s -- --no-reader`).

### Verifying the installer

The installer verifies every binary and archive it downloads against the release `checksums.txt`. Piping `curl … | bash` cannot verify the script itself, so to close that gap download `install.sh`, check it, then run it:

```bash
TAG=v1.0.0   # or the tag you want
base="https://github.com/sparkfabrik/claude-usage/releases/download/${TAG}"
curl -fsSLO "${base}/install.sh"
curl -fsSLO "${base}/checksums.txt"
# Linux ships sha256sum, macOS ships shasum — use whichever exists:
if command -v sha256sum >/dev/null; then
  sha256sum --ignore-missing -c checksums.txt   # expect: install.sh: OK
else
  shasum -a 256 --ignore-missing -c checksums.txt
fi
bash install.sh
```

`install.sh`, the CLI/tray binaries, and `claude-usage-readers.tar.gz` all have entries in `checksums.txt`.

> **Trust model:** verification roots trust in HTTPS and GitHub Releases — `checksums.txt` is fetched over TLS but is not independently signed (no GPG/cosign). This protects against accidental corruption and a tampered mirror, but not against an attacker who can replace both the artifacts and `checksums.txt` at the source. Checksums for releases predating this feature only cover the CLI/tray binaries.

### What the installer does

1. Downloads the correct binary for your OS/arch from GitHub Releases, then verifies it against the release `checksums.txt` (SHA-256) before installing — the CLI binary, the macOS tray binary, and the readers archive are all checked
2. Installs to `~/.local/share/claude-usage/` with symlinks in `~/.local/bin/`
3. Detects your desktop environment and wires the matching reader (unless `--no-reader`):
   - **GNOME** → symlinks the shell extension, prints enable command
   - **KDE** → installs plasmoid via `kpackagetool6`, prints widget instructions
   - **Waybar** → symlinks the module script, prints config snippet
   - **macOS** → installs the tray binary, prints launch instructions; registers the auto-start session hooks only with `--tray` / `CLAUDE_USAGE_TRAY=1`, and removes them otherwise
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

# JSON usage report for a period, offline (see "JSON usage report" below)
claude-usage --report 30d

# Show version info
claude-usage --version

# Use custom config file (overrides the default search chain)
claude-usage --config /path/to/config.yaml
claude-usage -c /path/to/config.yaml      # shorthand

# Show where config is loaded from (search chain, active config, reference file)
claude-usage --help
```

The dashboard adapts its colors to the output:

- **Piped or redirected:** plain text, unless `CLICOLOR_FORCE` is set to a non-empty value other than `0`.
- **`NO_COLOR`:** any non-empty value turns colors off, even with `CLICOLOR_FORCE`. On a terminal, bold and faint text stay.
- **`CLICOLOR=0`:** turns colors off like `NO_COLOR`; on a terminal, bold and faint text stay. Unlike `NO_COLOR`, a `CLICOLOR_FORCE` value that is non-empty and not `0` overrides it.
- **Older terminals:** colors are reduced to what the terminal supports.

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

The tray does not start on its own. Launch it with `claude-usage-tray`, or install with `--tray` / `CLAUDE_USAGE_TRAY=1` to start and stop it with Claude Code sessions.

Menu bar item showing "C:X% W:Y%". Click for dropdown with reset times, auth state (shown when not valid), "Refresh Now", and "Quit".

## JSON usage report

`claude-usage --report <period>` prints one JSON document with the usage of a period and exits. It reads the local transcripts and the price table only: no API call, no credentials, no cache. The period labels are the ones of `--period`: `today`, `7d`, `30d`, `all`. The flag cannot be combined with `--status`, `--poll-only`, `--period`, `--force-poll` or `--no-cost`; `--config` is honoured for pricing overrides.

```bash
claude-usage --report 30d > usage-30d.json
```

```json
{
  "schema": "claude-usage/report/1",
  "version": "v0.9.0",
  "generated_at": "2026-10-29T15:15:36Z",
  "window": {
    "label": "30d",
    "from": "2026-09-29T15:15:36Z",
    "to": "2026-10-29T15:15:36Z",
    "timezone": "CET",
    "utc_offset": "+01:00"
  },
  "cost_basis": "list prices, cache writes at their recorded TTL rate",
  "pricing": "bundled",
  "totals": {
    "tokens": {
      "input": 193290,
      "output": 8342708,
      "cache_write": 56185967,
      "cache_write_1h": 31206114,
      "cache_read": 3329432006,
      "total": 3394153971
    },
    "cost_usd": 1692.08,
    "requests": 10116,
    "sessions": 62,
    "active_days": 23
  },
  "models": [
    {
      "id": "claude-fable-5-1",
      "name": "Fable 5.1",
      "tokens": {
        "input": 1,
        "output": 1,
        "cache_write": 1,
        "cache_write_1h": 1,
        "cache_read": 1,
        "total": 4
      },
      "cost_usd": 1324.5,
      "requests": 7000,
      "priced": true
    }
  ],
  "sessions": { "count": 62, "cost_median_usd": 7.07, "cost_p90_usd": 81.5 }
}
```

- `window` — the period is rolling from the moment the command runs, like `--period`; `from` and `to` are UTC. `active_days` counts local calendar days, like the dashboard, so `timezone` and `utc_offset` say which calendar was used.
- `totals` and `models[]` — tokens by type, cost at list prices, number of requests (assistant messages) and, for the totals, sessions and active days. `cache_write_1h` is the part of `cache_write` written with the 1-hour cache, which costs more; `total` counts it once. Models are listed heaviest first; `id` is the identifier found in the transcripts, `name` its display form.
- `sessions` — a session is a transcript file together with the subagent transcripts it spawned; `cost_median_usd` and `cost_p90_usd` are computed on the cost of each session (the p90 is the nearest-rank percentile). Both are `0` when the period has no sessions.
- `priced` and `unpriced_models` — a model missing from the price table is costed with the fallback price (the Sonnet 4 rate) and flagged, so the figure is recognisable as a guess; add the model under `pricing:` in the config to price it, which sets `pricing` to `config`.
- The document carries no project path, session id or other identifying field. The report covers what is still on disk: Claude Code keeps transcripts for `cleanupPeriodDays`, 30 by default.

## Configuration

The config file is **optional** — `claude-usage` runs fully on built-in defaults.

**Search chain** (first existing file wins, XDG-aware):

1. `$XDG_CONFIG_HOME/claude-code-usage/config.yaml` (falls back to `~/.config` when unset)
2. `./config.yaml` (current directory)

Override the chain entirely with `--config <path>` / `-c <path>`. Run `claude-usage --help`
to see the search chain, the active config, and the reference-file path.

**Reference file:** the installer provisions a version-matched
`config.default.yaml` alongside your config dir
(`~/.config/claude-code-usage/config.default.yaml`), overwritten on every
install/upgrade. The tool never loads it — don't edit it; copy it to
`config.yaml` to start customizing:

```bash
cp ~/.config/claude-code-usage/config.default.yaml ~/.config/claude-code-usage/config.yaml
```

See [`config.default.yaml`](config.default.yaml) for all options:

```yaml
api:
  enabled: true
  stale_after: 60 # seconds — poll API if cache is older than this
  model: claude-haiku-4-5-20251001
  usage_endpoint: "" # OAuth usage endpoint; empty uses the built-in default
  only_when_active: false # poll even while Claude Code is closed (polling is free)

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

Cache writes are priced at the rate of the cache lifetime each request used: Claude Code records the split between 5-minute writes (1.25x input) and 1-hour writes (2x input) in every transcript line, and the main conversation on a subscription uses the 1-hour cache. One simplification applies to every estimate, in the dashboard and in `--report`: Haiku 5.5 is priced at its rate for prompts up to 100K tokens. The built-in table follows the [Anthropic pricing page](https://platform.claude.com/docs/en/about-claude/pricing); a model that is not in it is priced at the Sonnet 4 rate and reported as unpriced by `--report`. A pricing override in the config file may set `cache_write_1h`; when it is left out, it is 1.6x `cache_write`.

## License

[GPL-3.0](LICENSE)
