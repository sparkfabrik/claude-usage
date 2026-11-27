# Spec Delta

## Purpose

Defines how utilization read from Anthropic's OAuth usage endpoint is normalized into windows with a utilization fraction in [0,1] for `--status` and the readers.

## ADDED Requirements

### Requirement: Utilization scale is decided once per payload

The system SHALL decide once per payload, over the flat buckets and the scoped entries together, whether utilization is a percentage or a fraction. A payload SHALL be percent-scaled when any value exceeds 1, fraction-scaled when no value exceeds 1 and any value is strictly between 0 and 1, and percent-scaled otherwise. Absent, null, negative or unparseable values SHALL NOT take part in the decision.

#### Scenario: One window at 1% and the others at 0%

- **WHEN** the session utilization is `1.0`, the weekly utilization is `0.0` and a scoped weekly entry reads `0`
- **THEN** the session window reports 1% and the other windows report 0%

#### Scenario: Every window at exactly 1%

- **WHEN** the session and weekly utilizations are both `1.0`
- **THEN** both windows report 1%

#### Scenario: Fraction-scaled payload

- **WHEN** the session utilization is `0.37` and the weekly utilization is `1.0`
- **THEN** the session window reports 37% and the weekly window reports 100%

#### Scenario: Percent-scaled payload

- **WHEN** the session utilization is `42.0` and the weekly utilization is `0.5`
- **THEN** the session window reports 42% and the weekly window reports 0.5%

#### Scenario: Every window at 0%

- **WHEN** every utilization is `0`
- **THEN** every window reports 0%
