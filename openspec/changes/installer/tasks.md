## 1. goreleaser readers archive

- [x] 1.1 Add `readers` archive entry to `.goreleaser.yaml` producing `claude-usage-readers.tar.gz` with gnome-shell-extension/, kde-plasmoid/, and waybar/ contents
- [x] 1.2 Verify goreleaser config is valid (`goreleaser check`)

## 2. Installer script — core

- [x] 2.1 Create `install.sh` with shebang, set -euo pipefail, CHANGED/OK output helpers, die function
- [x] 2.2 Implement `--uninstall` flag handling (remove install dir, symlinks, GNOME symlink, KDE via kpackagetool6, waybar hint)
- [x] 2.3 Implement version resolution (CLAUDE_USAGE_VERSION env or GitHub API latest)
- [x] 2.4 Implement version skip check (.version file comparison)
- [x] 2.5 Implement OS/arch detection (uname -s, uname -m → mapped to goreleaser naming: linux/darwin, amd64/arm64)
- [x] 2.6 Implement binary download (curl/wget to tmp, mv to install dir, chmod +x)
- [x] 2.7 Implement macOS tray binary download (Darwin only)
- [x] 2.8 Implement readers archive download and extraction

## 3. Installer script — symlinks

- [x] 3.1 Create `~/.local/bin/` if needed, symlink claude-usage binary
- [x] 3.2 Symlink claude-usage-tray on macOS
- [x] 3.3 Print PATH warning if `~/.local/bin` not in PATH

## 4. Installer script — reader detection and install

- [x] 4.1 Implement DE detection logic (gnome-shell → KDE → waybar, first match)
- [x] 4.2 Implement GNOME reader install (directory symlink to extensions path)
- [x] 4.3 Implement KDE reader install (kpackagetool6 --install, or --upgrade if already present)
- [x] 4.4 Implement Waybar reader install (symlink script to ~/.local/bin/)
- [x] 4.5 Implement macOS reader (no-op, tray already handled in binary download)
- [x] 4.6 Print reader-specific post-install instructions (enable extension, add widget, waybar config)

## 5. Installer script — idempotency and upgrade

- [x] 5.1 Ensure symlink creation uses `ln -sf` (idempotent)
- [x] 5.2 Ensure KDE plasmoid detects installed version and uses --upgrade when version differs
- [x] 5.3 Write .version file after successful install/upgrade
- [x] 5.4 Copy install.sh into install dir for uninstall reference

## 6. Documentation

- [x] 6.1 Update root README.md with one-liner install command and uninstall instructions
- [x] 6.2 Add INSTALL_DIR and CLAUDE_USAGE_VERSION env var documentation to install.sh header comment
