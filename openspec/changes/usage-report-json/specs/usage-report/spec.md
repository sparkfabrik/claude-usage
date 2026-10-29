# usage-report

The `--report` command, its document, and the rules it must keep.

## Command

`claude-usage --report <period>` prints one JSON document on stdout, followed by a newline, and exits 0. `<period>` is one of the labels `--period` accepts: `today`, `7d`, `30d`, `all`. The window is the same the dashboard would show for that label: rolling from the moment the command runs.

The command reads the local transcripts and the price table only. It makes no network call, reads no credentials, and neither reads nor writes the stats cache. `--config` selects the configuration file for pricing overrides. The flags `--status`, `--poll-only`, `--period`, `--force-poll` and `--no-cost` are rejected with an error that names the flag; `--no-poll` is accepted and has no effect.

An unknown period, a failure to walk the transcripts, or a conflicting flag writes one line on stderr and exits 1 with nothing on stdout. A machine with no transcripts produces a valid document with zero totals.

## Document

Top-level fields: `schema`, `version`, `generated_at`, `window`, `cost_basis`, `pricing`, `totals`, `models`, `sessions`, and `unpriced_models` when it is not empty.

- `schema` is `claude-usage/report/1`. It changes only when a field is renamed or removed; additions keep the value.
- `window` carries `label`, `from` and `to` as RFC 3339 in UTC, `timezone` (the local zone name, or its abbreviation when the zone is unnamed) and `utc_offset` as `+hh:mm`.
- `totals` carries `tokens` (`input`, `output`, `cache_write`, `cache_read`, `total`), `cost_usd`, `requests`, `sessions`, `active_days`.
- `models[]` is ordered heaviest first and carries `id` (the identifier found in the transcripts), `name` (display form), `tokens`, `cost_usd`, `requests`, `priced`.
- `sessions` carries `count`, `cost_median_usd`, `cost_p90_usd`.

## Semantics

**A request** is one assistant message with usage, deduplicated as the reader deduplicates everything.

**A session** is a transcript file together with the subagent transcripts spawned from it, identified by the `sessionId` the transcript lines carry; a line without the field belongs to the session named by its file. Session cost is the sum of the cost of its requests. The median is the middle value of the ascending session costs, or the mean of the two middle values. The p90 is the nearest-rank percentile: the value at rank `ceil(0.9 * n)`. Both are 0 when there are no sessions.

**Active days** are the distinct local calendar days with at least one request, the same calendar the dashboard uses.

**Synthetic messages**, which Claude Code records with model `<synthetic>`, and lines without a model are not usage and do not appear.

## Pricing

Every `cost_usd` is computed from the token counts at the list prices of the built-in table, or at the overrides from the configuration file when present, in which case `pricing` is `config` instead of `bundled`. `cost_basis` states the rule in words: list prices, cache writes at the 5-minute rate.

A model without a row, a prefix or an override is costed with the fallback price, reported with `priced: false` on its row and listed in `unpriced_models`, so the figure is recognisable as a guess.

## Identifying data

The document never contains a project path, a session id, a file name, a message id, or a timestamp other than the window bounds and the generation time.
