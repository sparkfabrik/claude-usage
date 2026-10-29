## 1. Reader (internal/reader)

- [x] 1.1 Key entries on the transcript's `sessionId`, falling back to the file name.
- [x] 1.2 Skip lines whose model is empty or `<synthetic>`.
- [x] 1.3 Tests: a subagent file keyed on its parent session, the file-name fallback, synthetic and empty models skipped, a line with `usage.iterations` read from its top-level counts.

## 2. Pricing (internal/pricing)

- [x] 2.1 Add the Claude 5 generation rows and their prefixes, longest first; update the source comment.
- [x] 2.2 Add `Known` reporting whether a model has an override, a row or a prefix.
- [x] 2.3 Tests: every new id and a date-suffixed variant, the longer-prefix rule, `Known`, a Fable 5.1 cost.

## 3. Report (internal/report)

- [x] 3.1 Document types with JSON tags: window, totals, models, sessions, `schema`, `cost_basis`, `pricing`, `unpriced_models`.
- [x] 3.2 `Build` filtering entries to the period, aggregating totals and per-model rows through the analyzer, counting active days on the local calendar, computing session median and nearest-rank p90.
- [x] 3.3 Zone name with the abbreviation fallback and the UTC offset.
- [x] 3.4 Tests: empty window, metadata and window, totals and per-model with out-of-window entries, unpriced flag, config overrides, local versus UTC active days, median and p90 edges, JSON shape without identifying keys.

## 4. CLI (cmd/claude-usage)

- [x] 4.1 `--report` flag, dispatched before `auth.Load()`, honouring `--config`.
- [x] 4.2 Conflict check naming the first incompatible flag.
- [x] 4.3 `runReport` resolving the period, loading entries and printing the document through `outputJSON`.
- [x] 4.4 Tests: conflict matrix, unknown period, empty projects directory, aggregation of a fixture with no identifiers in the output.

## 5. Documentation

- [x] 5.1 README: feature line, CLI example, "JSON usage report" section with the schema and rules, cost-note simplifications.
- [x] 5.2 AGENTS.md: the `--report` contract and the package tree.
- [x] 5.3 This change: proposal, design, tasks and the `usage-report` spec.

## 6. Verification

- [x] 6.1 `go vet`, `go test ./...` (the pre-existing `internal/auth` Keychain test is environment-dependent on macOS), golangci-lint v2.14.0 clean.
- [x] 6.2 On a machine with 30 days of transcripts, the report's token totals equal the dashboard's `30d` row.
