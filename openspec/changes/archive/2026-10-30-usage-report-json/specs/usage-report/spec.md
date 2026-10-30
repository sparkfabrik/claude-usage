# usage-report

The `--report` command, its document, and the rules it must keep.

## Command

`claude-usage --report <period>` prints one JSON document on stdout, followed by a newline, and exits 0. `<period>` is one of the labels `--period` accepts: `today`, `7d`, `30d`, `all`. The window is the same the dashboard would show for that label: rolling from the moment the command runs.

The command reads the local transcripts and the price table only. It makes no network call, reads no credentials, and neither reads nor writes the stats cache. `--config` selects the configuration file for pricing overrides. The flags `--status`, `--poll-only`, `--period`, `--force-poll` and `--no-cost` are rejected with an error that names the flag; `--no-poll` is accepted and has no effect.

An unknown period (the empty one included, as with `--report=`), a failure to walk the transcripts, a transcript that cannot be opened or read to its end, or a conflicting flag writes one line on stderr and exits 1 with nothing on stdout. A machine with no transcripts produces a valid document with zero totals.

## Document

Top-level fields: `schema`, `version`, `generated_at`, `window`, `cost_basis`, `pricing`, `totals`, `models`, `sessions`, and `unpriced_models` when it is not empty.

- `schema` is `claude-usage/report/1`. It changes only when a field is renamed or removed; additions keep the value.
- `window` carries `label`, `from` and `to` as RFC 3339 in UTC, `timezone` (the local zone name, or its abbreviation when the zone is unnamed) and `utc_offset` as `+hh:mm`. For `all`, `from` is the earliest request found in the transcripts, and the field is absent when there are none; every other label keeps its rolling bound.
- `totals` carries `tokens` (`input`, `output`, `cache_write`, `cache_write_1h`, `cache_read`, `total`), `cost_usd`, `requests`, `sessions`, `active_days`. `cache_write_1h` is the part of `cache_write` written with the 1-hour cache lifetime; `total` counts it once.
- `models[]` is ordered heaviest first and carries `id` (the identifier found in the transcripts), `name` (display form), `tokens`, `cost_usd`, `requests`, `priced`.
- `sessions` carries `count`, `cost_median_usd`, `cost_p90_usd`.

## Semantics

**A request** is one assistant message with usage, deduplicated as the reader deduplicates everything.

**A session** is a transcript file together with the subagent transcripts spawned from it, identified by the `sessionId` the transcript lines carry; a line without the field belongs to the session named by its file. Session cost is the sum of the cost of its requests. The median is the middle value of the ascending session costs, or the mean of the two middle values. The p90 is the nearest-rank percentile: the value at rank `ceil(0.9 * n)`. Both are 0 when there are no sessions.

**Active days** are the distinct local calendar days with at least one request, the same calendar the dashboard uses.

**Synthetic messages**, which Claude Code records with model `<synthetic>`, and lines without a model are not usage and do not appear.

## Pricing

Every `cost_usd` is computed from the token counts at the list prices of the built-in table, or at the overrides from the configuration file when present, in which case `pricing` is `config` instead of `bundled`. Cache writes are priced at the rate of the cache lifetime the transcript records for each request: the 5-minute rate for `cache_write` less `cache_write_1h`, the 1-hour rate for `cache_write_1h`; a line without the breakdown is all 5-minute writes. An override that leaves `cache_write_1h` out gets 1.6x its `cache_write`. A model priced by prompt length on the pricing page (Haiku 5.5) pays its over-threshold rates on every token of a request whose prompt, input plus cache reads and writes, is over the threshold; an override applies a tier only when it carries a `long_prompt` block. `cost_basis` states the rule in words: list prices, cache writes at their recorded TTL rate.

A model without a row, an override, or a row of which it is a dated variant (the row's id, a hyphen and an eight-digit date) is costed with the fallback price, reported with `priced: false` on its row and listed in `unpriced_models`, so the figure is recognisable as a guess.

## Identifying data

The document never contains a project path, a session id, a file name, a message id, or a timestamp other than the window bounds and the generation time.
