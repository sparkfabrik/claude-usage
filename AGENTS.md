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

- Single static binary, no runtime dependencies.
- **CLI must work on both Linux and macOS.** GNOME extension and KDE plasmoid are Linux-only.
- Process detection uses `pgrep -x claude` (available on both Linux and macOS). Never use Linux-only mechanisms like `/proc` scanning.
- All readers call the CLI (`claude-usage --status`) as their sole API — no direct file I/O, no formatting logic.
- Readers are pure renderers: CLI returns raw data (percentages, reset times, colors, stale flag), readers format display text.
- GNOME extension binary lookup: `$PATH` first, then `~/.local/bin/` fallback (GNOME Shell's PATH often excludes `~/.local/bin`).
- Reader errors (binary not found, spawn/parse failures) shown in UI with error styling.
- Config lives at `~/.config/claude-code-usage/config.yaml` (YAML, loaded with `gopkg.in/yaml.v3`).
- Cache at `~/.cache/claude-code-usage/quota.json` (XDG Base Directory spec, configurable via `cache.path`).
- Cache directory created with `0700` permissions.
- Credentials read from `~/.claude/.credentials.json` (Claude Code OAuth, read-only — this file is managed by Claude Code).
- Never write to `~/.claude/` — that directory belongs to Claude Code.

### `--status` JSON API

The `--status` flag outputs a JSON object consumed by readers and other tools.

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
  "error": ""
}
```

- `c_pct`/`w_pct` — current (5h) and weekly (7d) utilization percentage (0–100).
- `c_reset`/`w_reset` — human-readable time until rate-limit window resets.
- `c_color`/`w_color` — hex color based on config thresholds.
- `stale` — true if cached data is older than the configured freshness window.
- `claude_running` — true if a Claude Code process is currently running.
- `auth` — credential state: `"valid"`, `"expired"`, `"missing"`, or `"unknown"`.
- `error` — non-empty on failure (no credentials, poll error, no cached data).

Flag combinations:

- `--status` — returns cached data if fresh, polls API if stale and Claude Code is running (or `only_when_active: false`).
- `--status --force-poll` — always polls API regardless of cache freshness or Claude Code state.
- `--status --no-poll` — returns cached data only, never polls. Error `"no cached data available"` if no cache exists.

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
  poller/                 1-token Haiku API polling, rate-limit header parsing
  pricing/                Per-model pricing table with prefix fallback
  process/                Claude Code process detection (pgrep-based, cross-platform)
  reader/                 JSONL conversation log parser (filepath.WalkDir)
readers/
  gnome-shell-extension/  GNOME Shell panel indicator (extension.js)
  kde-plasmoid/           KDE Plasma 6 widget (QML)
  macos-tray/             macOS menu bar app (Objective-C + Go)
  waybar/                 Waybar custom module (Bash + CSS)
  statusline/             Terminal statusline script (Bash)
openspec/                 Change management specs
```

## Code Style

- **Go**: standard `gofmt` formatting. Tabs for indentation.
- **JavaScript** (GNOME extension): 4-space indent, GJS/GNOME Shell API conventions.
- **QML** (KDE plasmoid): 2-space indent.
- **Bash** (Waybar/statusline): 2-space indent.
- **YAML/JSON**: 2-space indent.
- `.editorconfig` enforces these settings.
- Linting: `make lint-cli` runs golangci-lint (auto-downloaded to `.bin/`, pinned version). Config in `.golangci.yml` (v2 format).

## Git Workflow

### Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<scope>): <description>
```

**Types:** `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf`, `build`.
**Scope** is optional — use the affected component (e.g., `poller`, `dashboard`, `extension`, `waybar`, `installer`).

Keep the description lowercase, imperative, no period.

### Branching

- Branch naming: `feat/`, `fix/`, `chore/`, `test/`, `docs/` prefix + kebab-case description
  (e.g., `feat/add-export-csv`, `fix/broken-auth-redirect`).
- **Never push directly to `main`.** Always create a feature branch and open a pull request.

### Rebasing

- Always rebase onto `main` before pushing. No merge commits.
- Use `--force-with-lease` (never `--force`) after rebasing.
- Rebase before the first push, before opening a PR, and whenever the base branch advances.

## OpenSpec Change Management

Spec artifacts live in `openspec/changes/<name>/`, archived in `openspec/changes/archive/YYYY-MM-DD-<name>/`.

### Git workflow for specs

OpenSpec itself has no opinion on git — it is a local file workflow. We add these conventions:

1. **Always commit spec artifacts to git** — never leave proposals, designs, specs, or tasks untracked. Commit them as soon as they are created or updated.

2. **Non-trivial changes: spec-first PR** — for changes that span multiple files, involve architectural decisions, or require infrastructure work:
   - Create a branch (e.g., `docs/<issue>-<name>-spec`)
   - Commit the proposal, design, specs, and tasks
   - Open a PR for review ("is this the right plan?")
   - Merge the spec PR **before** starting implementation
   - This creates a review checkpoint and prevents building on a wrong design

3. **Trivial changes: spec + implementation in one PR** — for small, well-scoped changes (single module, clear scope), spec and code can go in the same PR.

4. **Archive on merge** — when the implementation is complete, archive the change (`openspec/changes/<name>/` → `openspec/changes/archive/YYYY-MM-DD-<name>/`) as part of that PR or as an immediate follow-up. Do not leave completed changes in the active directory.

## Package Management

### Go

- Add: `go get <package>@latest`
- Tidy: `go mod tidy`
- Verify: `go mod verify`

### Dependency Safety

Before adding or upgrading any dependency, follow these rules:

1. **Never assume you know the latest version.** Your training data is outdated. Always verify against the live registry before adding or upgrading any package.

2. **Check the live registry:**

   ```bash
   curl -s "https://proxy.golang.org/<module>/@latest" | jq .
   ```

3. **Use the newest stable major version** compatible with Go 1.23. Check actual compatibility metadata.

4. **Avoid releases published within the last 5 days** to reduce supply chain attack risk. Check the release date from the registry response.

5. **Always run `go mod tidy`** after changing dependencies, then verify with `go mod verify`.

## Testing

`make test-cli` runs all tests (unit + integration). `make test-cli-short` runs unit tests only (skips integration).

- Go tests live in `cmd/claude-usage/main_test.go` (and alongside code in `internal/*/`).
- Run with `go test ./...` or via Make targets.
- Coverage: `make test-cli-cover` shows per-function coverage.

## CI/CD

The project uses GitHub Actions with GoReleaser for releases.

### Release workflow

- Triggered by pushing a `v*` tag to `main`.
- Runs on `macos-latest` (needed for macOS tray CGO build).
- GoReleaser builds CLI for linux/darwin (amd64/arm64), macOS tray (darwin only), and bundles the GNOME extension.
- Pre-release hooks: `go mod tidy`, `go test -short ./...`, tar readers bundle.
- Archives: `.tar.gz` (CLI + docs), raw binary (for curl install), GNOME extension `.zip`.

### Key artifacts

| Artifact        | Format     | Contents                                   |
| --------------- | ---------- | ------------------------------------------ |
| CLI archive     | tar.gz     | Binary + README + LICENSE + config         |
| CLI binary      | raw        | Direct download for `install.sh`           |
| macOS tray      | raw binary | Tray app (darwin only)                     |
| GNOME extension | zip        | extension.js + metadata.json + sparkle.svg |
| Readers bundle  | tar.gz     | All reader source files                    |

### Releasing

Releases are fully automated via GoReleaser. **Never create GitHub releases manually** (no `gh release create`, no API calls to create releases). GoReleaser owns the entire release lifecycle: building binaries, creating the GitHub release, generating the changelog, and uploading artifacts.

To release a new version:

1. Ensure `main` is up to date and CI is green.
2. Tag the commit: `git tag v<major>.<minor>.<patch>`.
3. Push the tag: `git push origin v<major>.<minor>.<patch>`.
4. The `release.yml` workflow triggers automatically and GoReleaser handles everything.

Follow [Semantic Versioning](https://semver.org/):
- `patch`: bug fixes, dependency updates, internal refactors.
- `minor`: new features, new CLI flags, new reader support.
- `major`: breaking changes to `--status` JSON schema or config format.

Pre-release tags (e.g., `v1.0.0-rc.1`) are detected automatically (`prerelease: auto` in `.goreleaser.yaml`).

**Never delete or overwrite an existing tag.** If a release is broken, fix forward with a new patch version.

## Command Safety

### Safe (run autonomously)

- `make build-cli` — compile the binary
- `make test-cli` — run all tests
- `make test-cli-short` — run unit tests only
- `make test-cli-cover` — show test coverage
- `make lint-cli` — run linter (auto-downloads if needed)
- `go vet ./...` — static analysis
- `go build ./...` — verify compilation
- `go mod verify` — verify dependency checksums
- `git status`, `git log`, `git diff`

### Dangerous (ask user first)

- `make install-cli` — writes binary to `~/.local/bin`
- `make install-gnome-extension` — copies files to GNOME extensions directory
- `make install-kde` — installs KDE plasmoid
- `make install-waybar` — installs Waybar module script
- `make install-macos-tray` — builds and installs macOS tray app
- `make reload-gnome-extension` — disables and re-enables the GNOME extension
- `make test-gnome-extension` — launches a nested GNOME Shell session
- `go get <package>` — modifies `go.mod` and `go.sum`
- `git push`, `git commit`

### Destructive (never run)

- `make uninstall-cli` — deletes binary from `~/.local/bin`
- `make uninstall-gnome-extension` — runs `rm -rf` on extension directory
- `make clean` — removes build artifacts
- `git push --force`

## Important Rules

- Every new CLI feature or bug fix must include tests. Place `*_test.go` files alongside the code they test. Run `make test-cli` to verify all tests pass before considering the work complete. Then run `make test-cli-cover` and report the coverage result to the user.
- After any Go code change, always run `make build-cli` to verify compilation. Then ask the user if they want to install the binary (`make install-cli`).
- After any reader change (extension JS, QML, Bash scripts), ask the user if they want to install the reader.
- Never discard `os.UserHomeDir()` errors — propagate or handle them.
- Cache directory permissions must be `0700`, not `0755`.
- Extension JS errors must be logged (`log()`), never silently swallowed in catch blocks.
- Use `filepath.WalkDir` (not `filepath.Walk`) for filesystem traversal.
- Use `strings.HasPrefix` for prefix matching, not manual slice comparison.
- Pre-allocate slices when the capacity is known or estimable.
- Run `go vet ./...` before every commit.
- Follow conventional commits.
- Verify dependency versions against the live Go module proxy before adding.
- All readers must use `claude-usage --status` as their sole data source — no direct API calls or file parsing in readers.
- Never write to `~/.claude/` — that directory belongs to Claude Code.
