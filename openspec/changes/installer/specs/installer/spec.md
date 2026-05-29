## ADDED Requirements

### Requirement: Installer downloads and installs correct binary for platform

The installer SHALL detect the current OS (Darwin/Linux) and architecture (amd64/arm64) and download the correct pre-built binary from GitHub Releases.

#### Scenario: Install on Linux amd64
- **WHEN** installer runs on Linux x86_64
- **THEN** it downloads `claude-usage_linux_amd64` and places it at `~/.local/share/claude-usage/bin/claude-usage`

#### Scenario: Install on macOS arm64
- **WHEN** installer runs on Darwin arm64
- **THEN** it downloads `claude-usage_darwin_arm64` and `claude-usage-tray_darwin_arm64`

#### Scenario: Unsupported platform
- **WHEN** installer runs on an unsupported OS or architecture
- **THEN** it exits with a clear error message

### Requirement: Installer creates symlinks to standard locations

The installer SHALL create symlinks from `~/.local/bin/` to the install directory binaries.

#### Scenario: CLI symlink on Linux
- **WHEN** installer completes on Linux
- **THEN** `~/.local/bin/claude-usage` is a symlink to `~/.local/share/claude-usage/bin/claude-usage`

#### Scenario: Tray symlink on macOS
- **WHEN** installer completes on macOS
- **THEN** `~/.local/bin/claude-usage-tray` is a symlink to `~/.local/share/claude-usage/bin/claude-usage-tray`

#### Scenario: PATH warning
- **WHEN** `~/.local/bin` is not in the user's PATH
- **THEN** installer prints a warning with instructions to add it

### Requirement: Installer auto-detects desktop environment on Linux

The installer SHALL detect the active desktop environment and install the appropriate reader. Detection order: GNOME, KDE, Waybar. First match wins.

#### Scenario: GNOME detected
- **WHEN** `gnome-shell` is found in PATH
- **THEN** installer installs the GNOME Shell extension via directory symlink to `~/.local/share/gnome-shell/extensions/claude-usage@claude-code-usage`

#### Scenario: KDE detected
- **WHEN** `plasmashell` is found in PATH
- **THEN** installer installs the KDE plasmoid via `kpackagetool6 --install`

#### Scenario: Waybar detected
- **WHEN** `waybar` is found in PATH and neither gnome-shell nor plasmashell are present
- **THEN** installer installs the waybar script and creates symlink at `~/.local/bin/claude-usage-waybar.sh`

#### Scenario: No DE detected
- **WHEN** none of gnome-shell, plasmashell, or waybar are found
- **THEN** installer installs only the CLI binary and prints a note that no reader was detected

#### Scenario: macOS (no DE detection)
- **WHEN** installer runs on macOS
- **THEN** it skips DE detection and installs the tray binary as the reader

### Requirement: Installer is idempotent

The installer SHALL be safe to re-run. If the installed version matches the target version, no changes are made.

#### Scenario: Same version already installed
- **WHEN** `~/.local/share/claude-usage/.version` contains `v1.2.0` and target version is `v1.2.0`
- **THEN** installer outputs "OK: already up to date" and exits 0 with no file changes

#### Scenario: Symlink already correct
- **WHEN** symlinks already point to the correct targets
- **THEN** installer does not recreate them

### Requirement: Installer supports upgrades

The installer SHALL detect when a newer version is available and replace the installed binary and reader assets.

#### Scenario: Upgrade from older version
- **WHEN** installed version is `v1.1.0` and target version is `v1.2.0`
- **THEN** installer downloads new binary, replaces existing, updates readers, and writes new version to `.version`

#### Scenario: Version resolution via GitHub API
- **WHEN** `CLAUDE_USAGE_VERSION` environment variable is not set
- **THEN** installer queries GitHub API for latest release tag

#### Scenario: Pinned version
- **WHEN** `CLAUDE_USAGE_VERSION=v1.0.0` is set
- **THEN** installer installs that specific version without querying GitHub API

### Requirement: Installer supports uninstall

The installer SHALL support `--uninstall` flag to cleanly remove all installed files and symlinks.

#### Scenario: Uninstall removes install directory
- **WHEN** user runs `install.sh --uninstall`
- **THEN** `~/.local/share/claude-usage/` is removed

#### Scenario: Uninstall removes symlinks
- **WHEN** user runs `install.sh --uninstall`
- **THEN** all symlinks in `~/.local/bin/` pointing to the install directory are removed

#### Scenario: Uninstall removes GNOME extension
- **WHEN** user runs `install.sh --uninstall` and GNOME extension symlink exists
- **THEN** the extension symlink at `~/.local/share/gnome-shell/extensions/claude-usage@claude-code-usage` is removed

#### Scenario: Uninstall removes KDE plasmoid
- **WHEN** user runs `install.sh --uninstall` and KDE plasmoid is installed
- **THEN** `kpackagetool6 --remove` is called to uninstall the plasmoid

#### Scenario: Uninstall prints Waybar hint
- **WHEN** user runs `install.sh --uninstall` and waybar config references claude-usage
- **THEN** installer prints a note to manually remove the module from Waybar config

### Requirement: Installer supports one-liner curl pipe

The installer SHALL work when piped from curl without requiring local clone.

#### Scenario: curl pipe install
- **WHEN** user runs `curl -fsSL https://raw.githubusercontent.com/sparkfabrik/claude-usage/main/install.sh | bash`
- **THEN** installer executes successfully and installs latest version

#### Scenario: curl pipe with version pin
- **WHEN** user runs `curl -fsSL ... | CLAUDE_USAGE_VERSION=v1.0.0 bash`
- **THEN** installer installs the specified version

### Requirement: Installer outputs machine-readable status

The installer SHALL output structured status lines for automation integration.

#### Scenario: Change made
- **WHEN** installer modifies the system (install, upgrade, reader install)
- **THEN** it prints `CHANGED: <description>` for each mutation

#### Scenario: No change needed
- **WHEN** installer detects everything is up to date
- **THEN** it prints `OK: already up to date`

#### Scenario: Error
- **WHEN** installer encounters a fatal error
- **THEN** it prints `ERROR: <description>` to stderr and exits with code 1
