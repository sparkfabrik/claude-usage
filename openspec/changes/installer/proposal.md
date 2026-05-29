## Why

The project has no installer. Users must manually build from source or download binaries and place them correctly, then separately install the appropriate reader for their platform. A single `curl | bash` installer that auto-detects OS and desktop environment would make adoption frictionless and support idempotent upgrades.

## What Changes

- Add `install.sh` — a self-contained installer script supporting install, upgrade, and uninstall
- Add `claude-usage-readers.tar.gz` as a goreleaser release asset bundling all reader files
- Installer downloads platform-correct binary + readers archive from GitHub Releases
- Auto-detects macOS vs Linux, then GNOME vs KDE vs Waybar on Linux
- Installs to `~/.local/share/claude-usage/` with symlinks to standard locations
- Supports `--uninstall` to cleanly remove everything
- Idempotent: re-running with same version is a no-op, new version triggers upgrade

## Capabilities

### New Capabilities

- `installer`: Shell-based installer supporting install, upgrade, uninstall with OS/DE auto-detection

### Modified Capabilities

## Impact

- New file: `install.sh` at repo root
- goreleaser config updated to produce `claude-usage-readers.tar.gz` asset
- No changes to existing binaries or reader code
