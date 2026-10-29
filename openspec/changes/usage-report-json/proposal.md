## Why

`claude-usage` cannot export usage aggregates. `--status` gives readers seven days of daily totals, per-model totals and today's cost, and the dashboard prints tables for a terminal. Anyone who needs the numbers of a period in a form a script can read has to parse the dashboard or reach for another tool, and the other tools pull a runtime with them.

Two things in the way stop an export from being correct today. The price table ends at the previous model generation, so every current model is priced at the Sonnet fallback without any sign of it. And a subagent transcript, stored as its own file under the session that spawned it, is counted as a session of its own, which inflates the session count in the dashboard as well.

## What Changes

- **New `--report <period>`** prints one JSON document for `today`, `7d`, `30d` or `all` and exits: tokens by type (input, output, cache read, cache write) and cost, in total and per model; request count; session count with median and p90 cost per session; active days; the window with its time zone. It reads the local transcripts and the price table only: no network, no credentials, no cache file. It outputs no project path, session id or other identifying data.
- **New `internal/report`** holds the aggregation and the document types; `schema` is `claude-usage/report/1`.
- **Price table** gains the Claude 5 generation (Fable 5.1 and 5, Mythos 5.1 and 5, Opus 5.5 and 5, Sonnet 5.5 and 5, Haiku 5.5) with prefixes for date-suffixed variants, and a 1-hour cache write rate for every row. `pricing.Known` tells whether a model has a price of its own; the report flags the ones that do not.
- **Cache writes** are priced at the rate of the cache lifetime each request used, read from the `usage.cache_creation` breakdown the transcripts carry, instead of the 5-minute rate for all of them. The config override format gains an optional `cache_write_1h`.
- **Reader** keys sessions on the transcript's `sessionId` field, falling back to the file name, so a subagent's requests count toward the session that spawned it. It skips the `<synthetic>` model Claude Code writes for its own messages.

## Capabilities

### New Capabilities

- `usage-report`: the `--report` command, its document, its rules on identifying data and pricing.

### Modified Capabilities

<!-- None: no existing spec covers the reader or the price table. -->

## Impact

- `--status` and its schema are untouched; readers are unaffected.
- The dashboard's session count stops counting subagent transcripts as sessions; token totals do not change.
- Costs of current models change from the fallback rate to their list price, in the dashboard and in `--status`'s `today.cost_usd`.
- Costs rise where the 1-hour cache was used: the main conversation on a subscription. On a machine checked over 30 days, 56% of the cache writes were 1-hour ones and the estimate was 10% low.
- Closes sparkfabrik/claude-usage#21.
