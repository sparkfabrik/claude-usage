## 1. Usage source (internal/usage)

- [x] 1.1 Add `Fetch` calling the OAuth usage endpoint with the beta header, bounding the response body and never surfacing it in errors.
- [x] 1.2 Add `ParseResponse` normalizing flat and model-scoped windows, exported so normalization is testable without a network.
- [x] 1.3 Decide the percent-vs-fraction scale once per payload, including the scoped entries.
- [x] 1.4 Resolve window titles from the kind, dropping unrecognized kinds rather than mislabelling them.
- [x] 1.5 Add `Collect` preferring the endpoint and falling back to the header poll, reporting both failures when neither works.
- [x] 1.6 Tests: both scales, the mixed-value case, scoped windows, dedup, the OAuth-apps weekly preference, malformed and empty payloads, header assertions, error opacity, and both fallback directions.

## 2. Window storage (internal/cache)

- [x] 2.1 Add the `Window` type with `IsOpen`, plus `windows` and `source` on `QuotaCache`.
- [x] 2.2 Add `OpenWindows` dropping windows past their reset, and `SyncWindows` rebuilding from flat fields for older caches.
- [x] 2.3 Tests: open, expired, absent and unparseable reset times; rebuild; and that a populated array is never overwritten.

## 3. Local aggregation (internal/stats)

- [x] 3.1 Aggregate seven days of token totals, per-model totals and today, all on local calendar dates.
- [x] 3.2 Add `FriendlyModelName` turning `claude-opus-4-8` into `Opus 4.8`.
- [x] 3.3 Cache to disk beside the quota cache with an atomic write and a TTL; `force` bypasses it.
- [x] 3.4 Tests: the fixed seven-day window, local-date attribution across a UTC rollover, model ordering and naming, today's totals, freshness, round-trip, and both cache paths.

## 4. Status contract (cmd/claude-usage)

- [x] 4.1 Extend `StatusResponse` with `source`, `limits[]`, `recent_days[]`, `models[]` and `today`, leaving the original fields in place.
- [x] 4.2 Populate limits from the open windows, and attach the local aggregation on every status path.
- [x] 4.3 Route polling through `usage.Collect`.
- [x] 4.4 Update the existing tests for the new `runStatus` signature.

## 5. Readers

- [x] 5.1 Waybar: list every window in the tooltip, plus today's totals and the top models.
- [x] 5.2 Statusline: append the fullest model window when it exceeds both flat windows.
- [x] 5.3 GNOME: add model rows and a token line, rebuilt on each update.
- [x] 5.4 KDE: repeat model rows and show today's totals.
- [x] 5.5 macOS tray: parse limits, surface the binding model window in the title and status line without new native menu items.

## 6. Correctness and housekeeping

- [x] 6.1 Deduplicate transcripts on the assistant message id with a uuid fallback; test the forked-session case.
- [x] 6.2 Move day boundaries to the local calendar.
- [x] 6.3 Sort `SummarizeByModel` deterministically.
- [x] 6.4 Rename the module path to `github.com/sparkfabrik/claude-usage`.
- [x] 6.5 Default `only_when_active` to false and update its test.

## 7. Documentation

- [x] 7.1 README: how it works, features, config keys.
- [x] 7.2 AGENTS.md: the `--status` schema and the package tree.
- [x] 7.3 `config.default.yaml`: `usage_endpoint` and the new `only_when_active` default.
