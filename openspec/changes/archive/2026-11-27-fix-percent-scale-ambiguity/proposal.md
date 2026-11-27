## Why

An account at 1% is shown at 100%. The utilization scale is decided per payload, and a payload is read as percent-scaled only when some value exceeds 1. When every window reads 1 or 0, for example a session at 1% with the weekly windows at 0%, no value exceeds 1, so the payload is read as fractions and the 1 becomes 100%. Readers then show the account as exhausted until usage passes 1%.

The raw endpoint response observed on 2026-11-27 is percent-scaled: `five_hour.utilization` is `2.0` and `limits[].percent` is `2` at 2% usage. No fraction-scaled payload has been observed.

## What Changes

- A payload with no value above 1 is read as percent-scaled, unless it holds a value strictly between 0 and 1, which only a fraction-scaled payload can carry.
- A payload with a value above 1 stays percent-scaled, and a payload with a value strictly between 0 and 1 and none above 1 stays fraction-scaled, as today.

## Capabilities

### New Capabilities

- `usage-source`: how utilization read from the OAuth usage endpoint is normalized. This change adds only the utilization scale requirement. It supersedes the "Utilization scale" rule in the spec of the unarchived `oauth-usage-source` change.

### Modified Capabilities

<!-- None: `usage-source` has no main spec under openspec/specs/ yet. -->

## Impact

- `--status` reports 1% instead of 100% for a payload whose values are only 0 and 1. The `--status` schema does not change.
- A fraction-scaled payload whose values are only 0 and exactly 1 would now read as 0% and 1%. No such payload has been observed.
- Closes sparkfabrik/claude-usage#17.
