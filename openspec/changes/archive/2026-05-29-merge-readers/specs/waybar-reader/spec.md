## ADDED Requirements

### Requirement: Waybar module outputs JSON from CLI status

The Waybar reader script SHALL call `claude-usage --status` and output JSON in Waybar's custom module format (`{"text": "...", "tooltip": "...", "class": "..."}`).

#### Scenario: Normal output
- **WHEN** `claude-usage --status` returns `c_pct: 42`, `w_pct: 67`, `c_reset: "2h 15m"`, `w_reset: "3d 1h 30m"`, `stale: false`
- **THEN** the script outputs `{"text": "◑ 5h:42% 7d:67%", "tooltip": "Claude Code Quota\n━━━━━━━━━━━━━━━━\n5h: 42% (resets in 2h 15m)\n7d: 67% (resets in 3d 1h 30m)", "class": "normal"}`

#### Scenario: Stale data
- **WHEN** `claude-usage --status` returns `stale: true`
- **THEN** the script appends " ?" to text and sets class to "error"

#### Scenario: CLI unavailable
- **WHEN** `claude-usage` is not found or exits non-zero
- **THEN** the script outputs nothing (empty output hides the Waybar module)

### Requirement: Waybar module uses color thresholds from status

The script SHALL derive CSS class from `c_pct` value: "normal" (<75%), "warning" (75-94%), "critical" (>=95%), "error" (stale or non-allow status).

#### Scenario: Warning threshold
- **WHEN** `c_pct` is 80 and `stale` is false
- **THEN** class is "warning"

#### Scenario: Critical threshold
- **WHEN** `c_pct` is 96 and `stale` is false
- **THEN** class is "critical"

### Requirement: Waybar module glyph reflects utilization

The script SHALL display a glyph based on `c_pct`: ◔ (<50%), ◑ (50-74%), ◕ (75-94%), ● (>=95%).

#### Scenario: Low utilization glyph
- **WHEN** `c_pct` is 30
- **THEN** text starts with "◔"

#### Scenario: High utilization glyph
- **WHEN** `c_pct` is 97
- **THEN** text starts with "●"

### Requirement: Waybar integration via interval

Waybar SHALL call the script via its `interval` mechanism. The recommended interval is 60 seconds configured in user's Waybar config.

#### Scenario: Waybar config example
- **WHEN** user adds the module to Waybar config
- **THEN** config block specifies `"exec": "/path/to/claude-usage-waybar.sh"`, `"return-type": "json"`, `"interval": 60`
