## Context

Users currently install claude-usage by building from source or downloading goreleaser binaries manually. Reader installation is a separate manual step per platform. The claudemeter project (Python predecessor) has a working installer that we draw patterns from, adapted for our Go binary distribution model.

The repo is at `sparkfabrik/claude-usage` on GitHub. goreleaser already produces per-platform binaries in "binary" format (no archive wrapper). We need to add a readers archive and write the installer script.

## Goals / Non-Goals

**Goals:**
- One-liner install: `curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | bash`
- Auto-detect OS (Darwin/Linux) and arch (amd64/arm64)
- Auto-detect desktop environment on Linux (GNOME, KDE, or Waybar — mutually exclusive, first match wins)
- Idempotent: re-run is a no-op if version matches
- Upgrade: detects new version, replaces binary and reader assets
- Uninstall: `install.sh --uninstall` removes everything cleanly
- Single install directory (`~/.local/share/claude-usage/`) with symlinks to system locations
- Ansible-friendly output protocol (CHANGED/OK lines)

**Non-Goals:**
- Package manager integration (apt, brew, etc.) — future work
- Auto-update daemon or cron — user re-runs installer manually
- Building from source — installer downloads pre-built binaries
- Supporting multiple readers simultaneously (KDE + Waybar) — first detected wins

## Decisions

### 1. Single install directory with symlinks

**Layout:**
```
~/.local/share/claude-usage/
├── .version
├── bin/
│   ├── claude-usage
│   └── claude-usage-tray        (macOS only)
├── readers/
│   ├── gnome-shell-extension/
│   ├── kde-plasmoid/
│   └── waybar/
└── install.sh                   (copy for uninstall reference)
```

**Symlinks created:**
- `~/.local/bin/claude-usage` → `~/.local/share/claude-usage/bin/claude-usage`
- `~/.local/bin/claude-usage-tray` → (macOS only)
- `~/.local/bin/claude-usage-waybar.sh` → (Waybar only)
- `~/.local/share/gnome-shell/extensions/claude-usage@claude-code-usage` → (GNOME, directory symlink)

**KDE exception:** kpackagetool6 manages its own copy, no symlink needed. Install/upgrade via kpackagetool6.

**Rationale:** Single directory makes uninstall trivial (rm -rf + remove symlinks). Symlinks keep binaries in PATH and extensions in expected locations without duplication.

**Alternative rejected:** Direct install to multiple locations. Makes uninstall complex and version tracking scattered.

### 2. Download binaries + readers archive from GitHub Releases

Installer fetches exactly 2-3 assets per install:
1. `claude-usage_{os}_{arch}` — CLI binary (always)
2. `claude-usage-readers.tar.gz` — reader assets (always)
3. `claude-usage-tray_{os}_{arch}` — tray binary (macOS only)

**Rationale:** All assets come from the same release. No repo access needed, works with private repos if token provided. Binary format (no tar wrapper) means direct download to file.

**Alternative rejected:** Download source tarball and extract readers. Adds unnecessary download size and couples installer to repo structure.

### 3. DE detection order: GNOME → KDE → Waybar

```bash
if command -v gnome-shell >/dev/null 2>&1; then
    READER="gnome"
elif command -v plasmashell >/dev/null 2>&1; then
    READER="kde"
elif command -v waybar >/dev/null 2>&1; then
    READER="waybar"
else
    READER="none"
fi
```

**Rationale:** These are mutually exclusive in practice. GNOME users don't have plasmashell. KDE users don't have gnome-shell. Waybar users (Sway/Hyprland) have neither. First match is sufficient.

On macOS: skip DE detection entirely, always install tray binary.

### 4. Version tracking via .version file

File at `~/.local/share/claude-usage/.version` contains the installed tag (e.g., `v1.2.0`).

- If file matches requested version → skip download (idempotent)
- If file differs or missing → download and replace
- Latest version resolved via GitHub API: `repos/sparkfabrik/claude-usage/releases/latest`

**Override:** `CLAUDE_USAGE_VERSION=v1.0.0 install.sh` pins a specific version.

### 5. goreleaser produces readers.tar.gz via extra archive

Add to `.goreleaser.yaml`:
```yaml
  - id: readers
    formats:
      - tar.gz
    name_template: "claude-usage-readers"
    meta: true
    files:
      - src: readers/gnome-shell-extension/*
        dst: gnome-shell-extension/
      - src: readers/kde-plasmoid/**/*
        dst: kde-plasmoid/
      - src: readers/waybar/*
        dst: waybar/
```

This produces `claude-usage-readers.tar.gz` attached to every release.

## Risks / Trade-offs

- **GitHub API rate limits**: Unauthenticated requests limited to 60/hour. For version resolution. → Accept; user can set CLAUDE_USAGE_VERSION to skip API call. Could add GITHUB_TOKEN support later.
- **Symlink breaks on binary update**: Symlinks point to files that get replaced. → ln -sf overwrites cleanly. Binary replacement is atomic (download to tmp, mv into place).
- **KDE kpackagetool6 not installed**: KDE detected but tool missing. → Fall back to manual copy with instructions printed.
- **~/.local/bin not in PATH**: Common on fresh systems. → Print warning if not in PATH after install.
- **curl vs wget**: Not all systems have curl. → Check for curl first, fall back to wget. Most modern systems have curl.
