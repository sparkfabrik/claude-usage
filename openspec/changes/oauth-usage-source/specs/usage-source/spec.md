# usage-source

Where utilization comes from, how windows are normalized, and what reaches the readers.

## Sources

Utilization is read from Anthropic's OAuth usage endpoint, `GET https://api.anthropic.com/api/oauth/usage`, authenticated with the `accessToken` from the credentials file and the `anthropic-beta: oauth-2025-04-20` header. It consumes no quota.

The endpoint is undocumented and ships behind a beta header, so the previous method remains as a permanent fallback: a 1-token request to `/v1/messages` whose `anthropic-ratelimit-unified-5h-*` and `-7d-*` response headers carry the session and weekly utilization. The fallback reports no model-scoped windows.

The active source is reported as `source`, either `oauth` or `headers`. A response body is never included in an error, because it can echo account details.

## Windows

A window has a stable machine-readable `key`, a display-ready `title`, an optional `model`, a `utilization` fraction in `[0,1]`, and an optional `resets_at`.

- The session and weekly windows are always present, keyed `session` and `weekly`. The weekly figure is read from `seven_day_oauth_apps` when present, otherwise `seven_day`.
- Model-scoped windows come only from the payload's `limits[]`, keyed `<model>:<window>` in lower case. One model can hold several windows, so identity is the model and kind together; a repeated pair is reported once, keeping the first.

**Titles are resolved by the producer, never parsed by consumers.** The producer knows the window kind; a reader seeing a model named `Opus 5 (1M context)` would read it as a one-minute window.

Window names derive from the kind: a kind containing `month` is `Monthly`, one containing `week` or `day` is `Weekly`, one containing `hour` or `session` is `Session`. An unrecognized kind is dropped rather than mislabelled.

## Utilization scale

The endpoint reports utilization either as a percentage (`37.0`) or as a fraction (`0.37`). A single payload is internally consistent, but accounts differ.

**The scale is decided once per payload**, looking at the flat buckets and the scoped entries together: if any value exceeds 1, the payload is percent-scaled. Deciding per value would render a genuine `1.0` percent as fully exhausted.

A value that is absent, null, negative or unparseable yields 0.

## Reset times and expiry

`resets_at` is normalized to RFC 3339. An ISO string is accepted, as is an epoch: below `1e12` it is read as seconds, otherwise as milliseconds.

A cached window survives a failed refresh, **but only until its own reset passes**. Showing 78% against a period that has already restarted is worse than showing nothing. A window with no reset time, or an unparseable one, is always considered open: absent information must not silently drop data.

## Local aggregation

Token totals come from the local JSONL transcripts: seven days of daily totals, per-model totals, and a summary of the current day.

Days follow the **local** calendar. Grouping by UTC files an evening's work under the following day for anyone west of Greenwich, and the reverse to the east. Every date in one aggregation is resolved in the same zone.

Transcript entries deduplicate on the assistant message id, which is stable when a session is resumed or forked and the same message is rewritten under a fresh uuid. Entries without a message id fall back to `uuid + requestId`.

The aggregation is cached on disk with a 15-minute TTL, because a working machine holds hundreds of megabytes of transcripts and the readers poll every minute. An explicit refresh bypasses the TTL.

## Reader contract

`--status` carries the original ten fields unchanged and always present, so readers written against the previous schema keep working. Added to them: `source`, `limits[]` (open windows only), `recent_days[]` (seven days, oldest first), `models[]` (heaviest first) and `today`.
