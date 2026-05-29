## 1. Bootstrap repo with Monska85 backend

- [x] 1.1 Import Monska85/claude-usage source as initial commit (cmd/, internal/, config.default.yaml, go.mod, go.sum, Makefile, .goreleaser.yaml, .github/workflows/, LICENSE)
- [x] 1.2 Move gnome-shell-extension/ to readers/gnome-shell-extension/
- [x] 1.3 Update Makefile install targets to reflect new readers/ path
- [x] 1.4 Verify `go build ./cmd/claude-usage` works and `--status` outputs valid JSON

## 2. KDE Plasma 6 reader

- [x] 2.1 Create `readers/kde-plasmoid/metadata.json` with Plasma 6 metadata
- [x] 2.2 Create `readers/kde-plasmoid/contents/ui/main.qml` — Timer every 60s calls `claude-usage --status`, parses JSON, renders panel text "C:X% W:Y%"
- [x] 2.3 Implement click dropdown showing reset times, error field
- [x] 2.4 Implement binary lookup (PATH then ~/.local/bin/claude-usage)
- [x] 2.5 Implement stale indicator (50% opacity) and claude_running hide logic
- [x] 2.6 Add `readers/kde-plasmoid/README.md` with install instructions

## 3. Waybar reader

- [x] 3.1 Create `readers/waybar/claude-usage-waybar.sh` — calls `claude-usage --status`, outputs Waybar JSON format
- [x] 3.2 Implement glyph selection based on c_pct (◔/◑/◕/●)
- [x] 3.3 Implement CSS class logic (normal/warning/critical/error)
- [x] 3.4 Implement tooltip with reset times
- [x] 3.5 Handle CLI missing/error (output nothing)
- [x] 3.6 Add `readers/waybar/README.md` with Waybar config example

## 4. macOS tray reader

- [x] 4.1 Create `readers/macos-tray/` with Go module (can share go.mod or separate)
- [x] 4.2 Implement main.go using fyne.io/systray — 60s timer calls `claude-usage --status`
- [x] 4.3 Implement menu bar title rendering with color from c_color
- [x] 4.4 Implement dropdown menu: detail items, "Refresh Now" (--force-poll), "Quit"
- [x] 4.5 Implement binary lookup (PATH → ~/.local/bin → /usr/local/bin)
- [x] 4.6 Implement stale/idle display states
- [x] 4.7 Add `readers/macos-tray/README.md` with build and install instructions

## 5. Build and release

- [x] 5.1 Update .goreleaser.yaml to build both `claude-usage` (all platforms) and `claude-usage-tray` (darwin only)
- [x] 5.2 Update GitHub Actions workflow for macOS runner (tray build needs CGo)
- [x] 5.3 Add Makefile targets: `install-kde`, `install-waybar`, `install-macos-tray`
- [x] 5.4 Update root README.md with project overview, architecture, and per-reader install docs
