## ADDED Requirements

### Requirement: macOS tray displays utilization from CLI status

The macOS tray binary (`claude-usage-tray`) SHALL call `claude-usage --status` every 60 seconds and display current (5h) and weekly (7d) utilization in the macOS menu bar.

#### Scenario: Normal display
- **WHEN** `claude-usage --status` returns `c_pct: 42`, `w_pct: 67`, `stale: false`
- **THEN** the menu bar shows "C:42% W:67%"

#### Scenario: Stale data
- **WHEN** `claude-usage --status` returns `stale: true`
- **THEN** the menu bar title appends " ?" to indicate staleness

#### Scenario: Claude not running
- **WHEN** `claude-usage --status` returns `claude_running: false`
- **THEN** the menu bar shows a dimmed or idle indicator (e.g., "C:--")

### Requirement: macOS tray shows dropdown detail

The tray binary SHALL display a dropdown menu with detailed quota information when clicked.

#### Scenario: Dropdown items
- **WHEN** user clicks the menu bar item
- **THEN** dropdown shows: "5h: 42% (resets in 2h 15m)", "7d: 67% (resets in 3d 1h 30m)", separator, "Refresh Now", "Quit"

#### Scenario: Refresh action
- **WHEN** user clicks "Refresh Now"
- **THEN** the tray calls `claude-usage --status --force-poll` and updates display immediately

### Requirement: macOS tray binary lookup

The tray binary SHALL locate `claude-usage` by checking `$PATH` first, then `~/.local/bin/claude-usage`, then `/usr/local/bin/claude-usage`.

#### Scenario: Binary in PATH
- **WHEN** `claude-usage` exists in `$PATH`
- **THEN** the tray uses that binary

#### Scenario: Fallback paths
- **WHEN** `claude-usage` is not in `$PATH`
- **THEN** the tray checks `~/.local/bin/claude-usage` then `/usr/local/bin/claude-usage`

### Requirement: macOS tray is a separate binary

The macOS tray SHALL be built as `claude-usage-tray`, separate from the core `claude-usage` binary. It SHALL have no polling or caching logic — all data comes from calling the core binary.

#### Scenario: Independence from core internals
- **WHEN** the tray binary runs
- **THEN** it imports no packages from `internal/` — it only shells out to `claude-usage --status`

### Requirement: macOS tray color coding

The tray SHALL color the menu bar text based on the `c_color` hex value returned by `--status`.

#### Scenario: Green state
- **WHEN** `c_color` is "#32c850"
- **THEN** the menu bar text is displayed in green

#### Scenario: Red state
- **WHEN** `c_color` is "#dc3232"
- **THEN** the menu bar text is displayed in red
