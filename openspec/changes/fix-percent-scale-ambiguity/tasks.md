# Tasks

## 1. Scale decision

- [ ] 1.1 Change the payload scale decision so that a payload with no value above 1 is percent-scaled unless a value lies strictly between 0 and 1, and update its doc comment; verify with `go vet ./...` and `make build-cli`
- [ ] 1.2 Add `ParseResponse` tests in `internal/usage/usage_test.go` for the spec scenarios (1/0/0 with a scoped entry, every window at 1, a true fraction such as 0.37, percent values above 1, every window at 0) and verify `make test-cli` passes with the existing scale tests unchanged
- [ ] 1.3 Run `make lint-cli` and `make test-cli-cover`, and verify that lint is clean and the coverage of the scale decision is reported

## Workflow follow-up

- Archive the change to `openspec/changes/archive/YYYY-MM-DD-fix-percent-scale-ambiguity/` on merge.
- After the release, run `sjust claude-usage-install` and check `claude-usage --status`.
