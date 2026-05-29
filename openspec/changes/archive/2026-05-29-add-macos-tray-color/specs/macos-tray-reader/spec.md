## MODIFIED Requirements

### Requirement: macOS tray color coding

The tray SHALL render the menu bar item in a color derived from the `c_color`
hex returned by `--status`, defaulting to Claude orange in the healthy state,
using a color-capable render path (since `systray` title text is monochrome).

#### Scenario: Healthy state is Claude orange

- **WHEN** `--status` returns a healthy `c_color` (the Claude-orange default `#D97757`)
- **THEN** the menu bar content is rendered in Claude orange `0xD9,0x77,0x57`, not black

#### Scenario: Green state

- **WHEN** `c_color` is `#32c850`
- **THEN** the menu bar content is displayed in green

#### Scenario: Red state

- **WHEN** `c_color` is `#dc3232`
- **THEN** the menu bar content is displayed in red

#### Scenario: Error state

- **WHEN** the poll fails or returns an error
- **THEN** the menu bar content is rendered in system red
