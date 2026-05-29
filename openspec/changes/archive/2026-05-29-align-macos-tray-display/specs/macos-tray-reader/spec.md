## MODIFIED Requirements

### Requirement: macOS tray displays utilization from CLI status

The macOS tray binary (`claude-usage-tray`) SHALL call `claude-usage --status`
every 60 seconds and display current (5h) and weekly (7d) utilization in the
macOS menu bar, using a quota glyph and the claudemeter title format.

#### Scenario: Normal display

- **WHEN** `claude-usage --status` returns `c_pct: 42`, `w_pct: 67`, `stale: false`
- **THEN** the menu bar shows `◑ 5h 42% · 7d 67%` (glyph selected from the 5h ladder: `◔` <50, `◑` <75, `◕` <95, `●` ≥95)

#### Scenario: Stale data

- **WHEN** `claude-usage --status` returns `stale: true`
- **THEN** the title retains the glyph + percentages and adds a staleness indicator

#### Scenario: Claude not running

- **WHEN** `claude-usage --status` returns `claude_running: false`
- **THEN** the menu bar shows the idle indicator `◌ —`

#### Scenario: Error state

- **WHEN** the poll fails or the response cannot be parsed
- **THEN** the menu bar shows `⚠ —`

### Requirement: macOS tray shows dropdown detail

The tray binary SHALL display a dropdown menu with a status line and detailed
quota information when clicked.

#### Scenario: Dropdown items

- **WHEN** the user clicks the menu bar item
- **THEN** the dropdown shows, in order: `Status: <state>`, separator, `5h: 42%`, `  resets in 2h15m`, `7d: 67%`, `  resets in 3d01h`, separator, `Refresh now`, `Quit`

#### Scenario: Refresh action

- **WHEN** the user clicks `Refresh now`
- **THEN** the tray calls `claude-usage --status --force-poll` and updates the display immediately

#### Scenario: Reset duration formatting

- **WHEN** a reset value is rendered in the dropdown
- **THEN** it uses the compact form: `<m>m` under an hour, `<h>h<mm>m` under a day, `<d>d<hh>h` otherwise, and `—` when there is no reset
