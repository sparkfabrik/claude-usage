## Context

The scale decision reads every utilization in the payload and returns percent-scaled only when one value exceeds 1. A payload whose values are all 0 or 1 is ambiguous, and today the ambiguity resolves to fractions. See proposal.md for the observed failure.

The raw endpoint response observed on 2026-11-27 carries the flat buckets as floats (`"utilization": 2.0`) and the scoped entries as integers (`"percent": 2`), both percent-scaled.

## Goals / Non-Goals

**Goals:**

- Resolve an ambiguous payload to percent-scaled.
- Keep reading a payload with a value strictly between 0 and 1 and none above 1 as fraction-scaled.

**Non-Goals:**

- Removing fraction support.
- Changing the cache format or the `--status` schema.

## Decisions

**An ambiguous payload is percent-scaled, unless a value strictly between 0 and 1 proves fractions.** Every observed payload is percent-scaled, and an account at a few percent is common, while a fraction-scaled account with every window at exactly 0% or 100% has never been observed. Only a fraction-scaled payload can carry a value strictly between 0 and 1 when no value exceeds 1, so such a value stays decisive.

Alternatives considered:

- **Integer against float literal.** Read an integer `1` as percent and a float `1.0` as a fraction. Rejected: the observed percent-scaled payload sends `utilization` as floats, so a genuine 1% arrives as `1.0`.
- **Scale remembered in the cache.** Reuse the last unambiguous decision when a payload is ambiguous. Rejected: it adds state to the cache, and the first poll after install or after a cache loss is still ambiguous.
- **Percent only when every value equals 1.** The direction proposed in the issue. Rejected: it misses the observed payload, where the session is at 1 and the weekly windows are at 0.

## Risks / Trade-offs

- [A fraction-scaled account with every window at exactly 0% or 100% now reads at most 1%] → No fraction-scaled payload has been observed.
- [A percent-scaled payload under 1% with a decimal value, such as `0.4`, reads as 40%] → This happens with today's rule too. No observed payload carries a decimal value.
