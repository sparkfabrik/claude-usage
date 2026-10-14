# Design

## Context

All lipgloss usage lives in `internal/dashboard/dashboard.go`: styles built with `NewStyle`, `Color`, borders, alignment, and the `table` subpackage. `runDashboard` in `cmd/claude-usage` writes the rendered strings to an `io.Writer`; production passes `os.Stdout`, tests pass a `bytes.Buffer`.

With lipgloss v1, a global renderer inspected `os.Stdout` and the environment, then rendered plain text for pipes and dropped colors for `NO_COLOR`. lipgloss v2 removes that renderer: `Style.Render` always emits full-color ANSI sequences, and adapting them to the output is left to the caller.

The repository has a single Go module, which also builds the macOS tray (no lipgloss there). CI and the release workflow select the Go toolchain from `go.mod`.

## Goals / Non-Goals

**Goals:**

- Keep the dashboard's visible behavior identical on a color terminal, in a pipe, and with `NO_COLOR` (see `specs/dashboard-output/spec.md`).
- Keep each upgrade in its own commit so a regression can be bisected to one item.

**Non-Goals:**

- Restyling the dashboard or adopting new lipgloss v2 features (light/dark adaptive colors, new table options).
- Moving to `go.yaml.in/yaml/v4`, which is still a release candidate.

## Decisions

### Adapt colors at the writer, inside `runDashboard`

`runDashboard` wraps the writer it receives with `colorprofile.NewWriter(w, os.Environ())` before writing anything. The colorprofile writer detects the profile from the destination and the environment: no TTY strips every escape sequence, `NO_COLOR` drops colors, and limited terminals get colors downgraded.

Wrapping inside `runDashboard` rather than at the `os.Stdout` call site means the tests that pass a `bytes.Buffer` exercise the same path as production. A buffer is not a TTY, so tests see plain text, as they did with v1.

Alternatives considered:

- `lipgloss.Fprintln` for every write: works, but every `fmt.Fprintln` in `runDashboard` and in `pollOrReadCache` warnings would have to change, and a missed call would leak escape codes.
- Detect the profile once and pass it into the dashboard renderers: more plumbing for the same result.

### `NO_COLOR` keeps text decorations

With v1, `NO_COLOR` produced fully plain text. colorprofile maps `NO_COLOR` to its ASCII profile, which drops colors but keeps bold and faint. This matches the `NO_COLOR` convention (no color, other styling allowed), so the spec requires only that color sequences are absent.

### Go 1.26 as the minimum

Go supports the two newest releases, currently 1.27 and 1.26. lipgloss v2 requires Go 1.25 and recent `golang.org/x/sys` releases require a newer directive as well, so 1.26 covers every upgrade while staying one release behind the newest. No `toolchain` line is added; CI installs the version in the `go` directive.

### `go.yaml.in/yaml/v3` instead of `gopkg.in/yaml.v3`

The fork is maintained by the YAML organization and keeps the same API, so the change is an import path swap in `internal/config`. Config parsing tests cover it.

## Risks / Trade-offs

- [lipgloss v2 table rendering differs in width or padding] → Existing dashboard tests compare rendered content; review the dashboard output by hand on a terminal before merging.
- [Contributors on Go older than 1.26 can no longer build] → Documented in `AGENTS.md`; release binaries are unaffected.
- [golangci-lint 2.14.0 enables new checks that flag existing code] → Fix the findings in the golangci-lint commit, or exclude a rule with a reason in `.golangci.yml`.
- [Color behavior on a real TTY cannot be checked by `go test`] → Unit tests cover the non-TTY path; a manual run in a terminal and with `NO_COLOR=1` covers the rest.
