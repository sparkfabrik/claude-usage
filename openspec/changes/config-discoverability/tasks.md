## 1. Config loader (internal/config)

- [ ] 1.1 Add a `ResolvePath() (path string, found bool)` helper that honors `XDG_CONFIG_HOME` (fallback `~/.config`), builds the chain `[<xdg>/claude-code-usage/config.yaml, ./config.yaml]`, and returns the first existing file (found=true) or ("", false).
- [ ] 1.2 Refactor `configPaths()`/`Load()` to use the XDG-aware chain and `ResolvePath` (no duplicated walk); preserve the `os.UserHomeDir()` failure fallback to `./config.yaml` only.
- [ ] 1.3 Add/extend unit tests using `t.Setenv("XDG_CONFIG_HOME", ...)`: XDG set, XDG unset (home fallback), first-match-wins, no-config-found, explicit-path override.

## 2. CLI flag + help footer (cmd/claude-usage)

- [ ] 2.1 Change `--config` to `flag.StringP("config", "c", "", "...")` to add the `-c` shorthand; update the flag usage string.
- [ ] 2.2 Set `flag.Usage` to print stock `flag.PrintDefaults()` output, then append the `Configuration:` block: literal `$XDG_CONFIG_HOME/...` search chain, `Active config:` line (resolved winner via `ResolvePath`, explicit `--config` path when set, else `none — built-in default configuration applied`), and a `Reference file:` line printed only when `config.default.yaml` exists in the config dir.
- [ ] 2.3 Add a test asserting the help footer renders the three scenarios (custom active / defaults+reference / defaults+no-reference).

## 3. Installer (install.sh)

- [ ] 3.1 Compute `CFG_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/claude-code-usage"` and `mkdir -p` it (braced expansions, bash 3.2 safe).
- [ ] 3.2 Download `https://raw.githubusercontent.com/${REPO}/${CLAUDE_USAGE_VERSION}/config.default.yaml` to `${CFG_DIR}/config.default.yaml`, always overwriting; never create/touch `config.yaml`.
- [ ] 3.3 Make the reference download best-effort (warn + continue on failure), and print a hint pointing users to copy `config.default.yaml` → `config.yaml` to customize.
- [ ] 3.4 In the `--uninstall` block, `rm -f "${CFG_DIR}/config.default.yaml"` (best-effort), then `rmdir "${CFG_DIR}" 2>/dev/null || true` to drop the dir only when empty; never remove `config.yaml`. Emit a `changed` line when the reference was removed.

## 4. Documentation

- [ ] 4.1 Add an AGENTS.md rule: `config.default.yaml` MUST stay in sync with `config.Default()`/the config struct on every config change; update the file-paths table to mention the reference file.
- [ ] 4.2 Update README.md for the `-c` shorthand, the new `--help` Configuration block, and install-time reference provisioning.
- [ ] 4.3 Verify `config.default.yaml` is still aligned with `config.Default()` (currently aligned); fix if drift found.

## 5. Verify

- [ ] 5.1 `make build-cli`, `make lint-cli`, `go vet ./...`.
- [ ] 5.2 `make test-cli`, then `make test-cli-cover` and report coverage.
- [ ] 5.3 Manually confirm `claude-usage --help` and `-c <path>` match the approved scenarios on Linux; confirm install.sh logic is bash 3.2 safe (no bash-4 constructs).
- [ ] 5.4 Manually verify uninstall: with a user `config.yaml` present, `--uninstall` removes only `config.default.yaml` and keeps `config.yaml` + dir; with no `config.yaml`, it also removes the empty dir.
