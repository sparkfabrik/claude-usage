## Context

The CLI already walks the transcripts (`internal/reader`), aggregates entries by period and by model (`internal/analyzer`), prices them (`internal/pricing`) and prints JSON for readers (`--status`). The report is a fourth mode on top of the same pieces, so the design is mostly a set of decisions about shape and semantics.

## Decisions

### A fourth mode, dispatched before credentials

`--report` is a string flag carrying a period label. It is handled right after `--version` and before `auth.Load()`, like the `--status --no-poll` fast path, so it needs no credentials file, no cache directory and no network. `--config` is honoured for pricing overrides. Combining it with `--status`, `--poll-only`, `--period`, `--force-poll` or `--no-cost` is an error naming the offending flag; `--no-poll` is accepted and meaningless.

### Period semantics are the dashboard's

The labels are exactly those `--period` accepts, resolved with `analyzer.FindPeriod`: rolling from the moment the command runs, `now - N days` to `now`. Days for `active_days` are counted on the local calendar, the rule the rest of the tool already follows and documents. The document records `from` and `to` in UTC and the zone (`timezone`, `utc_offset`) so a file is self-describing wherever it is read. Go reports an unnamed local zone as `Local`; the report substitutes the abbreviation in force at generation time.

### The report bypasses the stats cache

`--report` reads the transcripts on every run and never touches `stats.json`. The cache exists for readers polling every minute; a one-off report must reflect the disk now. On a working machine the walk takes seconds.

### One session is a transcript file plus its subagents

Claude Code stores a subagent transcript at `<project>/<session-id>/subagents/agent-<hash>.jsonl`. Every line, in both kinds of file, carries `sessionId` with the id of the parent session. The reader now keys an entry on that field, falling back to the file name for transcripts without it. Consequences: a subagent's requests belong to the session that spawned it, the session count is a count of sessions, and the dashboard's "sessions" column drops the subagent files it used to add. A session can stay open for days when it is resumed; that is the file's lifetime, not the report's concern.

### Synthetic and empty models are not usage

Claude Code writes `model: "<synthetic>"` for messages it produces itself. The reader skips such lines, and lines without a model, next to its existing zero-usage check. Checked on a working machine over 30 days: every `<synthetic>` line carried zero tokens, and no line lacked a model.

### Top-level usage, not `iterations[]`

Newer transcripts add `usage.iterations`, a per-iteration list next to the top-level counts. Checked on 3,257 lines over 7 days: every list had one element and the top-level counts equalled its sum. The reader keeps reading the top-level fields; a test pins the assumption so a multi-iteration format would fail a test rather than skew the numbers.

### Pricing: list prices, cache writes at their recorded rate, flagged unknowns

The table gains the Claude 5 generation from the Anthropic pricing page, read on 2026-10-28, with the longest prefixes first so `claude-opus-5-5` resolves before `claude-opus-5`. One simplification stays and is documented: Haiku 5.5 is priced at its rate for prompts up to 100K tokens. The fallback for an unknown model stays the Sonnet 4 rate, as the dashboard shows; the report adds `priced: false` on the model row and lists the id in `unpriced_models`, so the guess is visible. `pricing` says `bundled` or `config` depending on whether overrides were loaded, and `cost_basis` states the rule in words.

### Cache writes: two rates, read from the transcript

A cache write costs 1.25x input with the 5-minute TTL and 2x input with the 1-hour one. Claude Code requests the 1-hour TTL for the main conversation on a subscription within plan usage, and the 5-minute one for subagents and everything else, and records the split of every request in `usage.cache_creation.ephemeral_5m_input_tokens` and `ephemeral_1h_input_tokens`. Checked on a working machine, deduplicated: the two always sum to `cache_creation_input_tokens`, no line lacks the breakdown, and 56% of the cache-write tokens were 1-hour ones, so pricing everything at the 5-minute rate left the 30-day estimate 10% low.

The reader keeps the 1-hour count on the entry (`CacheWrite1hTokens`, clamped to the total, 0 when the breakdown is absent). `ModelPrice` gains `CacheWrite1h`, set from the pricing page on every row; `Calculate` takes the 1-hour count and prices the remainder at the 5-minute rate. A config override may set `cache_write_1h`; when left out it is derived as 1.6x `cache_write`, the ratio of 2x to 1.25x that every row of the pricing page follows, so existing configs keep working. The report adds `cache_write_1h` under `tokens`, a part of `cache_write` and not added to `total`, and `cost_basis` becomes "list prices, cache writes at their recorded TTL rate". The dashboard shows no new column; its cost simply becomes right.

### Percentiles

Session cost is the sum of the cost of every request in the session. The median is the middle value, or the mean of the two middle values; the p90 is the nearest-rank percentile, the value at rank `ceil(0.9 * n)` of the ascending list. Both are 0 when there are no sessions.

### No identifying data

The document carries counts, costs, model identifiers and the window. No project path, no session id, no timestamps other than the window and generation time, no file names. A test asserts the absence of the identifying keys and that a project directory name and a session id do not appear in the output.

## Alternatives considered

- Calendar-day windows in UTC: would disagree with `--period` on the same machine; rejected for consistency with the dashboard.
- Deriving the 1-hour rate as 1.6x the 5-minute one everywhere, without a column in the table: fewer lines, but the table would no longer mirror the pricing page row by row and a future model with another ratio would be mispriced silently. The column is explicit; the derivation is kept only for config overrides that leave it out.
- Keeping the 5-minute rate for every cache write as a documented simplification: a 10% systematic under-estimate defeats the purpose of a cost survey across a team.
- Cost 0 for unknown models: would make the report disagree with the dashboard; the flag was preferred.
