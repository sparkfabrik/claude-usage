# Tasks

## 1. Toolchain

- [x] 1.1 Bump `GOLANGCI_LINT_VERSION` in `Makefile` to 2.14.0, fix or justify any new findings, and verify `make lint-cli` passes under Go 1.27
- [ ] 1.2 Raise the `go` directive in `go.mod` to 1.26, update the Go version in `AGENTS.md` (tech stack and package management rule), and verify `go build ./...` and `make test-cli` pass

## 2. Dependencies

- [ ] 2.1 Bump `golang.org/x/sys` to its latest release, run `go mod tidy` and `go mod verify`, and verify govulncheck no longer reports GO-2026-5024
- [ ] 2.2 Replace `gopkg.in/yaml.v3` with `go.yaml.in/yaml/v3` in `internal/config`, update the module name in `AGENTS.md`, run `go mod tidy` and `go mod verify`, and verify the config tests pass

## 3. lipgloss v2

- [ ] 3.1 Switch `internal/dashboard` to `charm.land/lipgloss/v2` and its `table` subpackage, run `go mod tidy` and `go mod verify`, and verify `go build ./...` and the dashboard tests pass
- [ ] 3.2 Wrap the writer in `runDashboard` with a colorprofile writer, and verify with new tests that dashboard output written to a non-terminal contains no ANSI escape sequences, including when `TERM` advertises color
- [ ] 3.3 Run the dashboard in a color terminal, piped through `cat`, and with `NO_COLOR=1`, and verify each matches `specs/dashboard-output/spec.md`

## 4. Integration

- [ ] 4.1 Run `go vet ./...`, `make lint-cli`, `make test-cli`, `make build-cli`, and `make test-cli-cover`, and verify all pass

## Workflow follow-up

- Archive the change after the PR merges.
