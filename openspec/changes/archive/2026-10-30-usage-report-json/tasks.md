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

## 6. Cache writes at their recorded rate

- [x] 6.1 Reader: `CacheWrite1hTokens` from `usage.cache_creation.ephemeral_1h_input_tokens`, clamped to the total, 0 without a breakdown.
- [x] 6.2 Pricing: `CacheWrite1h` on every row, `Calculate` taking the 1-hour count, overrides deriving 1.6x `cache_write` when the rate is left out; `cache_write_1h` in `config.ModelPricing` and `config.default.yaml`.
- [x] 6.3 Analyzer and report: `TotalCacheWrite1hTokens`, `tokens.cache_write_1h`, new `cost_basis`.
- [x] 6.4 Tests: breakdown read, absent and clamped; every row at 2x input; override derivation; cost at both rates in pricing, analyzer and report; config key.
- [x] 6.5 README: cost note, report example and field list.

## 7. Haiku 5.5 prompt-length tier

- [x] 7.1 Pricing: `PromptTier` on `ModelPrice`, the Haiku 5.5 row with its over-100K rates, `Calculate` switching to the tier when the request's prompt is over the threshold.
- [x] 7.2 Config: optional `long_prompt` block on an override, mapped in `buildPricingOverrides`; `config.default.yaml` example.
- [x] 7.3 Tests: at and over the threshold, cache reads counted toward it, output at the tier rate, no tier on any other row, override with and without the block, config parse.
- [x] 7.4 README cost note.

## 8. Window of the `all` period

- [x] 8.1 `from` is the earliest request on disk for `all`, omitted when there is none; bounded labels unchanged.
- [x] 8.2 Tests in report and cmd; README and spec.

## 9. Review follow-ups (PR 22)

- [x] 9.1 Dispatch `--report` on the flag's presence so `--report=` fails on the empty period; integration test on the binary.
- [x] 9.2 Resolve only dated variants through the prefix table, so a new minor version is reported unpriced; tests for variants and non-variants.
- [x] 9.3 `reader.LoadEntriesStrict` failing on an unreadable transcript, an unlistable directory or a line over the scanner limit; `--report` uses it; tests in reader and cmd.
- [x] 9.4 Declined: distinguishing an explicit `cache_write_1h: 0` from an omitted key. No model has a free 1-hour cache and every other override field already reads 0 as 0; documented that 0 and omitted both derive.

## 10. Verification

- [x] 9.1 `go vet`, `go test ./...` (the pre-existing `internal/auth` Keychain test is environment-dependent on macOS), golangci-lint v2.14.0 clean.
- [x] 9.2 On a machine with 30 days of transcripts, the report's token totals equal the dashboard's `30d` row.
