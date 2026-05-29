## ADDED Requirements

### Requirement: KDE plasmoid displays utilization from CLI status

The KDE Plasma 6 panel widget SHALL call `claude-usage --status` every 60 seconds and display the current (5h) and weekly (7d) utilization percentages in the system panel.

#### Scenario: Normal display with fresh data
- **WHEN** `claude-usage --status` returns JSON with `c_pct: 42`, `w_pct: 67`, `stale: false`
- **THEN** the plasmoid displays "C:42% W:67%" in the panel with colors from `c_color` and `w_color` fields

#### Scenario: Stale data indication
- **WHEN** `claude-usage --status` returns JSON with `stale: true`
- **THEN** the plasmoid displays the percentages at 50% opacity

#### Scenario: CLI not found or error
- **WHEN** `claude-usage` binary is not in PATH or returns non-zero exit code
- **THEN** the plasmoid displays "C:?" and does not crash

#### Scenario: Claude not running
- **WHEN** `claude-usage --status` returns JSON with `claude_running: false`
- **THEN** the plasmoid hides or shows a dimmed idle indicator

### Requirement: KDE plasmoid shows detail on click

The plasmoid SHALL show a dropdown/tooltip with reset times and status details when clicked.

#### Scenario: Dropdown content
- **WHEN** user clicks the plasmoid panel indicator
- **THEN** a popup displays: 5h utilization with `c_reset`, 7d utilization with `w_reset`, and any error message from the `error` field

### Requirement: KDE plasmoid binary lookup

The plasmoid SHALL locate `claude-usage` by checking `$PATH` first, then falling back to `~/.local/bin/claude-usage`.

#### Scenario: Binary in PATH
- **WHEN** `claude-usage` exists in `$PATH`
- **THEN** the plasmoid uses that binary

#### Scenario: Fallback to local bin
- **WHEN** `claude-usage` is not in `$PATH` but exists at `~/.local/bin/claude-usage`
- **THEN** the plasmoid uses `~/.local/bin/claude-usage`
