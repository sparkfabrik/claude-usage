# Proposal

## Why

The Go toolchain and dependencies on `main` have fallen behind. `make lint-cli` panics under Go 1.27 because the pinned golangci-lint was built with Go 1.26, and `go.mod` declares Go 1.23, which no longer receives security fixes. Several modules are outdated or unmaintained, and lipgloss has a new major version.

## What Changes

- Bump the pinned golangci-lint from 2.12.2 to 2.14.0 so `make lint-cli` runs under Go 1.27.
- Raise the `go` directive in `go.mod` from 1.23 to 1.26, the oldest supported Go release. CI follows `go.mod`, so it builds with a supported toolchain too.
- Bump `golang.org/x/sys` to its latest release (fixes GO-2026-5024, Windows only and not reachable from this code).
- Replace the unmaintained `gopkg.in/yaml.v3` with its maintained fork `go.yaml.in/yaml/v3`.
- Migrate the dashboard from lipgloss v1 to lipgloss v2. lipgloss v2 no longer adapts colors to the terminal on its own, so the dashboard writer now does it explicitly. This keeps today's behavior: plain text when piped, no colors with `NO_COLOR`. With `NO_COLOR` the colors match v1 output, but bold and faint styling are now kept. `CLICOLOR` and `CLICOLOR_FORCE` are normalized to keep v1 semantics.
- No change to the `--status` JSON schema, the config file, or any reader.

## Capabilities

### New Capabilities

- `dashboard-output`: How the terminal dashboard adapts its styling to the output destination (terminal, pipe, `NO_COLOR`).

### Modified Capabilities

None.

## Impact

- **Code:** `internal/dashboard` (lipgloss v2 import paths), `cmd/claude-usage` (dashboard writer), `internal/config` (YAML import path).
- **Dependencies:** `go.mod`/`go.sum`; most indirect `charmbracelet/*` and terminal modules change with lipgloss v2.
- **Tooling:** `Makefile` golangci-lint pin; CI picks up Go 1.26 through `go-version-file: go.mod`.
- **Docs:** `AGENTS.md` (Go version, YAML module name).
- **Users:** building from source needs Go 1.26 or newer. Release binaries are unaffected.
