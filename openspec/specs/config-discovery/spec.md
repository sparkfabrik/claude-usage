# config-discovery Specification

## Purpose

Define how `claude-usage` locates, reports, and provisions its configuration file. The capability covers the XDG-aware search chain used by the config loader, the explicit `--config`/`-c` override, the resolve helper that reports the winning path, the `Configuration:` section appended to `--help`, and the installer/uninstaller handling of the reference `config.default.yaml`.

## Requirements

### Requirement: XDG-aware config search chain

The config loader SHALL locate the configuration file using an ordered search chain, honoring the `XDG_CONFIG_HOME` environment variable and selecting the first existing file.

The default search chain SHALL be, in order:

1. `${XDG_CONFIG_HOME}/claude-code-usage/config.yaml`, where `XDG_CONFIG_HOME` falls back to `~/.config` when unset or empty.
2. `./config.yaml` (current working directory).

When `os.UserHomeDir()` fails and `XDG_CONFIG_HOME` is unset, the loader SHALL fall back to `./config.yaml` only.

#### Scenario: XDG_CONFIG_HOME set

- **WHEN** `XDG_CONFIG_HOME` is set to `/custom/cfg` and `/custom/cfg/claude-code-usage/config.yaml` exists
- **THEN** the loader reads `/custom/cfg/claude-code-usage/config.yaml`

#### Scenario: XDG_CONFIG_HOME unset

- **WHEN** `XDG_CONFIG_HOME` is unset and `~/.config/claude-code-usage/config.yaml` exists
- **THEN** the loader reads `~/.config/claude-code-usage/config.yaml`

#### Scenario: First match wins

- **WHEN** both the XDG-resolved path and `./config.yaml` exist
- **THEN** the loader reads the XDG-resolved path and ignores `./config.yaml`

#### Scenario: No config present

- **WHEN** no file in the search chain exists
- **THEN** the loader applies built-in defaults and reports no active config file

### Requirement: Explicit config path override

The CLI SHALL accept an explicit config path that overrides the default search chain, exposed as both `--config <path>` and the shorthand `-c <path>`.

#### Scenario: Explicit path via long flag

- **WHEN** the user runs `claude-usage --config /tmp/my.yaml`
- **THEN** the loader reads `/tmp/my.yaml` and does not consult the default search chain

#### Scenario: Explicit path via shorthand

- **WHEN** the user runs `claude-usage -c /tmp/my.yaml`
- **THEN** the behavior is identical to `--config /tmp/my.yaml`

### Requirement: Resolve helper reports winning path

The config package SHALL expose a helper that reports which file in the search chain was selected, distinguishing "a file was found" from "no file found, defaults applied". This helper SHALL be used by `Load` so resolution logic is not duplicated.

#### Scenario: Winning path reported

- **WHEN** a config file in the search chain exists
- **THEN** the helper returns that file's path and a found indicator that is true

#### Scenario: No file found

- **WHEN** no config file in the search chain exists
- **THEN** the helper returns a found indicator that is false

### Requirement: Help output surfaces configuration state

The `--help` output SHALL append a `Configuration:` section after the standard flag list (the standard pflag layout is preserved). The section SHALL contain:

- The default search chain, displayed with the literal `$XDG_CONFIG_HOME/...` form (not the resolved path).
- An `Active config:` line showing the winning resolved path, or `none — built-in default configuration applied` when no file is found.
- A `Reference file:` line showing the resolved path to `config.default.yaml`, printed ONLY when that file exists.

When an explicit `--config`/`-c` path is supplied, the `Active config:` line SHALL show that explicit path.

#### Scenario: Custom config active

- **WHEN** `--help` is shown and a `config.yaml` in the search chain exists
- **THEN** the `Active config:` line shows that file's resolved path

#### Scenario: Defaults active, reference present

- **WHEN** `--help` is shown, no `config.yaml` exists, but `config.default.yaml` exists in the config dir
- **THEN** the `Active config:` line shows `none — built-in default configuration applied`
- **AND** a `Reference file:` line shows the resolved `config.default.yaml` path

#### Scenario: Defaults active, no reference

- **WHEN** `--help` is shown and neither `config.yaml` nor `config.default.yaml` exists
- **THEN** the `Active config:` line shows `none — built-in default configuration applied`
- **AND** no `Reference file:` line is printed

### Requirement: Install provisions reference config

The installer SHALL ensure the config directory `${XDG_CONFIG_HOME:-${HOME}/.config}/claude-code-usage/` exists and SHALL download `config.default.yaml` into it as a reference file. The file SHALL be fetched from raw GitHub at the chosen release tag (`https://raw.githubusercontent.com/<repo>/${CLAUDE_USAGE_VERSION}/config.default.yaml`). The installer SHALL always overwrite `config.default.yaml` and SHALL NEVER create or modify `config.yaml`.

#### Scenario: Fresh install

- **WHEN** the installer runs and the config dir does not exist
- **THEN** the dir is created and `config.default.yaml` is downloaded into it
- **AND** no `config.yaml` is created

#### Scenario: Reinstall refreshes reference only

- **WHEN** the installer runs and a user-edited `config.yaml` already exists
- **THEN** `config.default.yaml` is overwritten with the version for the installed release
- **AND** the existing `config.yaml` is left untouched

### Requirement: Uninstall removes reference only

The uninstaller (`install.sh --uninstall`) SHALL remove `${XDG_CONFIG_HOME:-${HOME}/.config}/claude-code-usage/config.default.yaml` and SHALL NEVER remove or modify the user's `config.yaml`. After removing the reference, the uninstaller SHALL remove the config directory only if it is empty (i.e. the user kept no `config.yaml`).

#### Scenario: Uninstall with user config present

- **WHEN** the user runs `install.sh --uninstall` and both `config.default.yaml` and a user `config.yaml` exist
- **THEN** `config.default.yaml` is removed
- **AND** `config.yaml` and its directory are left untouched

#### Scenario: Uninstall with no user config

- **WHEN** the user runs `install.sh --uninstall`, `config.default.yaml` exists, and no `config.yaml` exists
- **THEN** `config.default.yaml` is removed
- **AND** the now-empty `claude-code-usage` config directory is removed

#### Scenario: Uninstall with no reference present

- **WHEN** the user runs `install.sh --uninstall` and no `config.default.yaml` exists
- **THEN** the uninstaller makes no change to the config directory and reports no error
