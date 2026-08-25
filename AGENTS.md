# AGENTS.md

## Project Overview

CLI dashboard + multi-platform desktop readers for monitoring Claude Code usage. Polls the Anthropic API with minimal 1-token requests, reads rate-limit headers, and parses local JSONL conversation logs to show utilization percentages, token counts, reset times, and estimated costs.

**Architecture:** Core Go CLI handles all polling, caching, and data logic. Multiple "readers" (GNOME Shell extension, KDE plasmoid, Waybar module, macOS tray, terminal statusline) call `claude-usage --status` and render the JSON response. No reader contains polling or caching logic.

**Tech stack:** Go 1.23, lipgloss (TUI), pflag (CLI flags), GNOME Shell JS, KDE QML, Bash (Waybar/statusline), Objective-C (macOS tray), YAML config, GoReleaser, GitHub Actions.

## Setup

Local Go toolchain required. No Docker.

```bash
make build-cli       # Build the binary
make install-cli     # Build + install binary to ~/.local/bin
```

Ensure `~/.local/bin` is in `PATH`.

The project should transition from Makefile to a Justfile.

## Key Conventions

- Single static binary, no runtime dependencies. **Must work on both Linux and macOS.**
- Process detection uses `pgrep -x claude` (available on both Linux and macOS). Never use Linux-only mechanisms like `/proc` scanning.
- All readers are pure renderers calling `claude-usage --status` as their sole data source. No direct API calls, file I/O, or formatting logic in readers.
- GNOME extension binary lookup: `$PATH` first, then `~/.local/bin/` fallback (GNOME Shell's PATH often excludes `~/.local/bin`).
- Reader errors (binary not found, spawn/parse failures) shown in UI with error styling.

### File Paths

| Path                                              | Purpose      | Notes                                                              |
| ------------------------------------------------- | ------------ | ------------------------------------------------------------------ |
| `~/.config/claude-code-usage/config.yaml`         | Config       | YAML, `gopkg.in/yaml.v3`. XDG-aware (`XDG_CONFIG_HOME`)            |
| `~/.config/claude-code-usage/config.default.yaml` | Config (ref) | Read-only reference; install-provisioned, never loaded by the tool |
| `~/.cache/claude-code-usage/quota.json`           | Cache        | XDG, configurable via `cache.path`, dir `0700`                     |
| `~/.claude/.credentials.json`                     | Credentials  | Read-only. **Never write to `~/.claude/`.**                        |

### `--status` JSON API

> **Source of truth:** The `StatusResponse` struct in `cmd/claude-usage/main.go` defines the schema. When modifying that struct, verify this section still matches and update it if needed.

```json
{
  "c_pct": 42,
  "c_reset": "3h12m",
  "c_color": "#32c850",
  "w_pct": 67,
  "w_reset": "5d02h",
  "w_color": "#e6961e",
  "stale": false,
  "claude_running": true,
  "auth": "valid",
  "error": "",
  "source": "oauth",
  "limits": [
    {
      "key": "session",
      "title": "Session (5-hour)",
      "pct": 42,
      "reset": "3h12m",
      "color": "#32c850"
    },
    {
      "key": "weekly",
      "title": "Weekly (7-day)",
      "pct": 67,
      "reset": "5d02h",
      "color": "#e6961e"
    },
    {
      "key": "opus:weekly",
      "title": "Opus Weekly",
      "model": "Opus",
      "pct": 73,
      "reset": "5d02h",
      "color": "#e6961e"
    }
  ],
  "recent_days": [
    {
      "date": "2026-08-25",
      "weekday": "Tue",
      "tokens": 40024791,
      "today": true
    }
  ],
  "models": [{ "name": "Opus 4.8", "tokens": 7118000000 }],
  "today": {
    "tokens": 40024791,
    "messages": 84,
    "sessions": 3,
    "cost_usd": 15.85
  }
}
```

The first ten fields are the original contract and are always present, so readers written against it keep working untouched. Everything after them is additive and may be absent.

- `c_pct`/`w_pct` — current (5h) / weekly (7d) utilization (0-100).
- `c_reset`/`w_reset` — human-readable time until window resets.
- `c_color`/`w_color` — hex color from config thresholds.
- `stale` — cached data older than freshness window.
- `claude_running` — Claude Code process detected.
- `auth` — `"valid"`, `"expired"`, `"missing"`, or `"unknown"`.
- `error` — non-empty on failure.
- `source` — `"oauth"` or `"headers"`. Model-scoped windows exist only with `"oauth"`.
- `limits` — every window that has not yet reset. Entries with a `model` field are model-scoped. `title` is resolved by the CLI and must not be parsed by readers: a model named `Opus 5 (1M context)` would read as a one-minute window.
- `recent_days` — seven days of token totals, oldest first, on local calendar dates.
- `models` — per-model token totals, heaviest first.
- `today` — the current local day.

Flag combinations: `--status` (cached or poll), `--status --force-poll` (always poll), `--status --no-poll` (cache only).

### Project Structure

> **Keep this up to date.** When adding, removing, or renaming packages/files, update this tree.

```
cmd/claude-usage/         CLI entry point (main.go, main_test.go)
internal/
  analyzer/               Token aggregation, time periods, per-model breakdown
  auth/                   OAuth credential loading from ~/.claude/
  cache/                  Atomic JSON cache read/write
  config/                 YAML config with defaults
  dashboard/              Lipgloss TUI rendering (tables, bars, panels)
  poller/                 1-token Haiku API polling, rate-limit header parsing (fallback source)
  pricing/                Per-model pricing table with prefix fallback
  process/                Claude Code process detection (pgrep-based, cross-platform)
  reader/                 JSONL conversation log parser (filepath.WalkDir)
  stats/                  Local transcript aggregation (per day, per model) with a disk cache
  usage/                  OAuth usage endpoint client, window normalization, fallback orchestration
readers/
  gnome-shell-extension/  GNOME Shell panel indicator (extension.js)
  kde-plasmoid/           KDE Plasma 6 widget (QML)
  macos-tray/             macOS menu bar app (Objective-C + Go)
  waybar/                 Waybar custom module (Bash + CSS)
  statusline/             Terminal statusline script (Bash)
openspec/                 Change management specs
```

## Code Style

- **Go**: `gofmt`. **JS** (GNOME): 4-space. **QML**: 2-space. **Bash**: 2-space. **YAML/JSON**: 2-space.
- `.editorconfig` enforces indentation settings.
- Linting: `make lint-cli` runs golangci-lint (auto-downloaded to `.bin/`, pinned version). Config in `.golangci.yml` (v2 format).

## Git Workflow

### Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/): `<type>(<scope>): <description>`

**Types:** `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf`, `build`.
**Scope** is optional (e.g., `poller`, `dashboard`, `extension`). Description: lowercase, imperative, no period.

### Branching

- `feat/`, `fix/`, `chore/`, `test/`, `docs/` prefix + kebab-case (e.g., `feat/add-export-csv`).
- **Never push directly to `main`.** Always feature branch + PR.

### Rebasing

- Always rebase onto `main` before pushing. No merge commits.
- Use `--force-with-lease` (never `--force`) after rebasing.

## OpenSpec Change Management

Spec artifacts live in `openspec/changes/<name>/`, archived in `openspec/changes/archive/YYYY-MM-DD-<name>/`.

- Always commit spec artifacts to git immediately.
- **Non-trivial changes:** spec-first PR, merge before implementation.
- **Trivial changes:** spec + implementation in one PR.
- **Archive on merge** to `openspec/changes/archive/YYYY-MM-DD-<name>/`. Do not leave completed changes in the active directory.

## Package Management

Before adding or upgrading any dependency:

1. **Never assume you know the latest version.** Check the live registry:
   ```bash
   curl -s "https://proxy.golang.org/<module>/@latest" | jq .
   ```
2. Use the newest stable major version compatible with Go 1.23.
3. Avoid releases published within the last 5 days (supply chain risk).
4. Always run `go mod tidy` then `go mod verify` after changes.

## Testing

- `make test-cli` — all tests (unit + integration). `make test-cli-short` — unit only (skips integration).
- `make test-cli-cover` — per-function coverage.
- Tests live alongside code in `*_test.go` files.

## CI/CD

GitHub Actions with GoReleaser. CI runs on PRs to main (ubuntu + macos matrix).

### Releasing

Releases are fully automated via GoReleaser. **Never create GitHub releases manually** (no `gh release create`, no API calls). GoReleaser owns the entire lifecycle: building, creating the release, changelog, and uploading artifacts.

To release:

1. Ensure `main` is up to date and CI is green.
2. `git tag v<major>.<minor>.<patch>` then `git push origin v<major>.<minor>.<patch>`.
3. `release.yml` triggers automatically. Runs on `macos-latest` (needed for macOS tray CGO).

Semver: `patch` (fixes/refactors), `minor` (features/flags/readers), `major` (breaking `--status` schema or config changes). Pre-release tags auto-detected (`prerelease: auto`).

**Never delete or overwrite an existing tag.** Fix forward with a new patch version.

### Key artifacts

| Artifact        | Format     | Contents                                   |
| --------------- | ---------- | ------------------------------------------ |
| CLI archive     | tar.gz     | Binary + README + LICENSE + config         |
| CLI binary      | raw        | Direct download for `install.sh`           |
| macOS tray      | raw binary | Tray app (darwin only)                     |
| GNOME extension | zip        | extension.js + metadata.json + sparkle.svg |
| Readers bundle  | tar.gz     | All reader source files                    |

## Command Safety

### Safe (run autonomously)

`make build-cli`, `make test-cli`, `make test-cli-short`, `make test-cli-cover`, `make lint-cli`, `go vet ./...`, `go build ./...`, `go mod verify`, `git status`, `git log`, `git diff`

### Dangerous (ask user first)

`make install-cli`, `make install-gnome-extension`, `make install-kde`, `make install-waybar`, `make install-macos-tray`, `make reload-gnome-extension`, `make test-gnome-extension`, `go get <package>`, `git push`, `git commit`

### Destructive (never run)

`make uninstall-cli`, `make uninstall-gnome-extension`, `make clean`, `git push --force`

## Important Rules

- **README.md updates are mandatory** whenever a change alters information documented in it (install flags/env vars, usage, CLI options, reader behavior, requirements, `--status` schema, etc.). Update `README.md` in the same change as the implementation — never let it drift from actual behavior.
- **`config.default.yaml` MUST stay in sync with `config.Default()`/the `Config` struct** on every config change. It is the install-provisioned reference (`~/.config/claude-code-usage/config.default.yaml`), regenerated on every install/upgrade — the tool never loads it, so any drift silently misleads users. Update it in the same change that touches config keys or defaults.
- **`install.sh` toggles must expose both a `--flag` and a `CLAUDE_USAGE_*` env var** (e.g. `--statusline`/`CLAUDE_USAGE_STATUSLINE`, `--no-reader`/`CLAUDE_USAGE_READER`). The env var is the only ergonomic toggle through `curl … | bash` (flags require `bash -s -- …`); the flag serves local `./install.sh` runs. Normalize env-var values with a `case` (no bash 4 `,,`) so macOS bash 3.2 keeps working.
- **Always format Markdown documents when touched.** If no formatting tool is available, say formatting is not possible.
- **In shell scripts, brace all variable expansions** (`${var}`, not `$var`) — including positionals (`${1}`, `${0}`). The bare special params (`$@`, `$#`, `$?`, `$$`, `$!`, `$*`) may stay unbraced, and escaped literals in output strings (`\$HOME`) must not be braced. `${var}` is POSIX and safe on macOS bash 3.2.
- Every new CLI feature or bug fix must include tests. Run `make test-cli` to verify, then `make test-cli-cover` and report coverage to user.
- After any Go code change, run `make build-cli`. Ask user about `make install-cli`.
- After any reader change, ask user about installing the reader.
- Never discard `os.UserHomeDir()` errors — propagate or handle them.
- Cache directory permissions: `0700`, not `0755`.
- Extension JS errors must be logged (`log()`), never silently swallowed.
- Use `filepath.WalkDir` (not `filepath.Walk`).
- Use `strings.HasPrefix` for prefix matching, not manual slice comparison.
- Pre-allocate slices when capacity is known or estimable.
- Run `go vet ./...` before every commit.
