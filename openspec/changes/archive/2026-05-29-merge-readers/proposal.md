## Why

The claudemeter project (Python, macOS/KDE/Waybar) and claude-usage project (Go, CLI/GNOME) solve the same problem — monitoring Claude Code utilization — but target different platforms. Merging them into a single repo with a Go backend and multiple platform readers eliminates duplication, provides a single polling/caching core, and covers all major desktop environments.

## What Changes

- Import Monska85/claude-usage Go backend as the core (poller, cache, CLI dashboard, `--status` JSON output)
- Add KDE Plasma 6 plasmoid reader that calls `claude-usage --status` every 60s
- Add Waybar custom module reader that calls `claude-usage --status` every 60s
- Add macOS system tray reader (Go, separate binary) that calls `claude-usage --status` every 60s
- Remove hooks dependency — readers drive polling via `--status` (auto-polls when cache is stale and Claude is running)
- GNOME extension already exists; adapt interval to 60s if needed

## Capabilities

### New Capabilities
- `kde-reader`: KDE Plasma 6 panel widget that renders `--status` JSON as a system tray indicator
- `waybar-reader`: Waybar custom module script that renders `--status` JSON in Waybar format
- `macos-tray-reader`: macOS system tray Go binary that renders `--status` JSON as a menu bar item

### Modified Capabilities

## Impact

- New directory `readers/` with subdirectories per platform
- New Go binary `cmd/claude-usage-tray/` for macOS (darwin-only build, CGo for systray)
- goreleaser config updated to build both binaries
- Makefile updated with per-reader install targets
- No changes to existing Go backend or GNOME extension logic
