# Tasks

## 1. Toolchain

- [x] 1.1 Bump `GOLANGCI_LINT_VERSION` in `Makefile` to 2.14.0, fix or justify any new findings, and verify `make lint-cli` passes under Go 1.27
- [x] 1.2 Raise the `go` directive in `go.mod` to 1.26, update the Go version in `AGENTS.md` (tech stack and package management rule), and verify `go build ./...` and `make test-cli` pass
- [x] 1.3 Install golangci-lint under a versioned binary name so a pin bump downloads the new release, and remove older binaries after the download

## 2. Dependencies

- [x] 2.1 Bump `golang.org/x/sys` to its latest release, run `go mod tidy` and `go mod verify`, and verify govulncheck no longer reports GO-2026-5024
- [x] 2.2 Replace `gopkg.in/yaml.v3` with `go.yaml.in/yaml/v3` in `internal/config`, update the module name in `AGENTS.md`, run `go mod tidy` and `go mod verify`, and verify the config tests pass

## 3. lipgloss v2

- [x] 3.1 Switch `internal/dashboard` to `charm.land/lipgloss/v2` and its `table` subpackage, run `go mod tidy` and `go mod verify`, and verify `go build ./...` and the dashboard tests pass
- [x] 3.2 Wrap the writer in `runDashboard` with a colorprofile writer, and verify with new tests that dashboard output written to a non-terminal contains no ANSI escape sequences (including when `TERM` advertises color), that `NO_COLOR` with any non-empty value drops colors even with `CLICOLOR_FORCE`, and that colors are downgraded to the terminal depth
- [x] 3.3 Run the dashboard in a color terminal, piped through `cat`, and with `NO_COLOR=1`, and verify each matches `specs/dashboard-output/spec.md`
- [x] 3.4 Normalize `CLICOLOR` and `CLICOLOR_FORCE` with `NO_COLOR` in `normalizeColorEnv` to keep v1 semantics, and verify with table tests of the escape-sequence classes written
- [x] 3.5 Drop `TTY_FORCE` in `normalizeColorEnv`, and verify that it does not force escapes on a non-terminal
- [x] 3.6 Describe the dashboard color behavior in `README.md`

## 4. Integration

- [x] 4.1 Run `go vet ./...`, `make lint-cli`, `make test-cli`, `make build-cli`, and `make test-cli-cover`, and verify all pass
