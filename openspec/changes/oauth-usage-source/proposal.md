## Why

Utilization is read by sending a 1-token request to `/v1/messages` and parsing the `anthropic-ratelimit-unified-5h-*` and `-7d-*` response headers. That costs roughly $0.52 per month per user and yields two numbers.

Anthropic exposes a dedicated endpoint for the same purpose, authenticated with the OAuth token already stored in the credentials file. It consumes no quota and returns model-scoped windows, which the headers cannot express. A model can be nearly exhausted while the overall weekly figure still looks calm, and today nothing surfaces that.

Separately, the readers cannot show tokens even though the CLI computes them: `--status` returns nine scalar fields, so every reader is limited to two percentages. The constraint is the contract, not the readers.

## What Changes

- **New `internal/usage`** calls `GET https://api.anthropic.com/api/oauth/usage` with the `anthropic-beta: oauth-2025-04-20` header, normalizes the payload into windows, and falls back to the existing header poll on any failure. The endpoint is undocumented, so the fallback is permanent, not transitional.
- **Utilization scale** is decided once per payload, across the flat buckets and the scoped entries together. The endpoint reports either percentages or fractions, and deciding per value would render a genuine 1.0% as 100%.
- **Window titles** are resolved by the producer, which knows the window kind, and shipped as an explicit `title`. A model named `Opus 5 (1M context)` would otherwise be parsed by a reader as a one-minute window.
- **`internal/cache`** gains a `Window` type, a `windows` array and a `source` field. `OpenWindows` drops any window whose `resets_at` has passed, so a cached percentage survives a failed refresh but never outlives its own period. `SyncWindows` rebuilds the array from the flat fields for caches written by older versions.
- **New `internal/stats`** aggregates the local transcripts into seven days of token totals, per-model totals and a summary of today, cached on disk with a 15-minute TTL. A working machine holds hundreds of megabytes of transcripts and the readers poll every minute, so the scan cannot run on every poll. `--force-poll` bypasses the TTL.
- **`--status`** gains `source`, `limits[]`, `recent_days[]`, `models[]` and `today`. The original ten fields are untouched and always present, so existing readers keep working.
- **Readers** surface the new data: Waybar in its tooltip, the statusline as a suffix when a model window is the binding constraint, GNOME and KDE as extra rows, the macOS tray in its title and status line.
- **Two correctness fixes.** Transcript entries deduplicate on the assistant message id, which is stable across resumed and forked sessions, falling back to `uuid + requestId`. Day boundaries follow the local calendar rather than UTC midnight.
- **`only_when_active` now defaults to false.** It existed to avoid paying for polls; polling is free now, and holding back left readers showing stale numbers for as long as Claude Code stayed closed.
- **Housekeeping**: the module path moves from `github.com/Monska85/claude-usage` to `github.com/sparkfabrik/claude-usage`, and `SummarizeByModel` sorts its output instead of returning map order.

## Capabilities

### New Capabilities

- `usage-source`: where utilization comes from, how windows are normalized and titled, how a cached window expires, and what the readers receive.

### Modified Capabilities

<!-- None: no existing spec covers the usage source. -->

## Impact

- `--status` grows additively. Readers built against the old schema are unaffected.
- The default polling behavior changes: quota now refreshes while Claude Code is closed.
- Users on a plan without model-scoped limits see exactly what they see today.
