## Context

Two projects solve Claude Code usage monitoring:
- **claude-usage** (Monska85): Go CLI + GNOME extension. Has poller, cache, token/cost analysis, `--status` JSON API, YAML config, goreleaser. Backend is production-ready.
- **claudemeter** (stevesibilia): Python menubar + KDE plasmoid + Waybar module + Claude Code hooks. Readers for macOS/KDE/Sway but tightly coupled to a Python poller daemon.

The merged project lives at `sparkfabrik/claude-usage`. The Go backend is the foundation. Readers become thin display layers consuming `claude-usage --status` output.

## Goals / Non-Goals

**Goals:**
- Single repo with Go backend + all platform readers (GNOME, KDE, Waybar, macOS)
- All readers use identical integration pattern: call `claude-usage --status` every 60s, render JSON
- macOS reader as separate Go binary (`claude-usage-tray`), no CGo in core binary
- Complete separation between backend and readers — readers have zero polling/caching logic

**Non-Goals:**
- Rewriting the Go backend (it stays as-is from Monska85)
- Claude Code hooks (readers drive polling via `--status` auto-poll)
- Unified installer (each reader has its own install method appropriate to its platform)
- Reader-side caching or fallback logic (if CLI fails, reader shows error/stale)

## Decisions

### 1. All readers call `claude-usage --status` (not read cache directly)

**Rationale**: Centralizes all logic (staleness, polling decisions, color thresholds, process detection) in the Go binary. Readers are pure renderers. If status JSON schema evolves, only CLI changes — readers just display whatever fields they get.

**Alternative rejected**: Readers read `~/.cache/claude-code-usage/quota.json` directly. Would require each reader to implement freshness checks, color logic, and timestamp parsing. Duplicates logic across 4 languages.

### 2. macOS tray as separate binary in same repo

**Rationale**: macOS system tray requires CGo (Cocoa bindings via systray lib). Keeping it in a separate `cmd/claude-usage-tray/` avoids polluting the core binary with CGo constraints. goreleaser builds core for all platforms (pure Go), builds tray only for darwin.

**Alternative rejected**: Single binary with `tray` subcommand. Would require CGo everywhere or build tags that complicate CI.

### 3. 60s poll interval for all readers

**Rationale**: Backend has `stale_after: 60` default. Calling `--status` every 60s means at most one API poll per minute when Claude is active. Matches the backend's freshness model. Lower than GNOME's current 30s — reduces unnecessary process spawns.

### 4. No hooks

**Rationale**: With `only_when_active: true` in config, `--status` auto-polls only when Claude is running. Any active reader (tray/plasmoid/waybar/extension) triggers polls. No daemon to manage, no lifecycle to track. Clean.

### 5. Readers under `readers/` directory, not top-level

**Rationale**: Clear separation. `cmd/` and `internal/` are Go backend. `readers/` contains platform-specific display code in their native languages (JS, QML, bash, Go). Each reader is self-contained with its own README/install instructions.

## Risks / Trade-offs

- **CGo on macOS**: systray libs need CGo. Cross-compilation from Linux CI won't work. → Use GitHub Actions macOS runner for darwin tray build only.
- **Process spawn overhead**: Calling `claude-usage --status` every 60s spawns a process. Go binary starts fast (~5ms). Acceptable. → No mitigation needed.
- **GNOME extension already exists**: Monska85's GNOME extension is already integrated. Moving it to `readers/` is a reorganization. → Keep same code, just relocate.
- **StatusResponse schema coupling**: All readers depend on `--status` JSON shape. → Document schema. Treat as public API — additions OK, removals need deprecation.
