## Why

Users have no way to discover that `claude-usage` is configurable or where to put a config file. The reference `config.default.yaml` ships only inside the release tarball (which `install.sh` never extracts), the binary gives no hint about which config it loaded, and the config loader ignores `XDG_CONFIG_HOME` even though the cache loader honors `XDG_CACHE_HOME` — an inconsistency that breaks portability for users with a custom XDG layout.

## What Changes

- **`install.sh`** ensures `${XDG_CONFIG_HOME:-${HOME}/.config}/claude-code-usage/` exists and downloads `config.default.yaml` there as a **reference-only** file, fetched via raw GitHub at the chosen release tag. The reference file is always overwritten on install (regenerated to match the installed version); the user's real `config.yaml` is **never** created or touched.
- **`install.sh --uninstall`** removes the `config.default.yaml` reference (symmetric with install) and the config directory only when empty; the user's `config.yaml` is **never** removed.
- **Config loader** (`internal/config`) honors `XDG_CONFIG_HOME` (falling back to `~/.config`), mirroring the existing `XDG_CACHE_HOME` pattern in `internal/cache`. A new exported helper reports which file in the search chain won, so callers can display it.
- **CLI** (`cmd/claude-usage`): `--config` gains a `-c` shorthand, and `--help` gains a `Configuration:` footer block listing the default search chain, the active config (winning path, or "built-in default configuration applied"), and the reference-file path when present.
- **`AGENTS.md`** gains a rule requiring `config.default.yaml` to stay in sync with `config.Default()`/the config struct on every config change.
- **`README.md`** is updated for the new flag shorthand, help output, and install-time config behavior.

## Capabilities

### New Capabilities

- `config-discovery`: How the config file is located (XDG-aware search chain), how the install-time reference file is provisioned, and how the CLI surfaces the active configuration and search chain to the user.

### Modified Capabilities

<!-- None: no existing spec captures config behavior yet. -->

## Impact

- **Code:** `internal/config/config.go` (XDG + resolve helper), `cmd/claude-usage/main.go` (`-c` shorthand, help footer), `install.sh` (config dir + reference download on install, reference removal on uninstall).
- **Docs:** `AGENTS.md` (sync rule + file-path table), `README.md` (flags/help/install).
- **Build/release:** none — `config.default.yaml` is fetched from the repo at the release tag via raw GitHub, so no GoReleaser asset change is required.
- **Compatibility:** macOS-safe (`~/.config` already used on both platforms; install primitives are bash 3.2 safe). No `--status` schema or config-key changes, so non-breaking.
