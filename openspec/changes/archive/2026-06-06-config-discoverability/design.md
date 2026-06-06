## Context

`claude-usage` runs fully on built-in defaults; a config file is optional. Three gaps make configuration undiscoverable and inconsistent:

- The reference `config.default.yaml` exists only inside the release `.tar.gz` archive, which `install.sh` does not extract — so installed users never see it.
- The binary loads config silently; `--help` (stock pflag autogen) gives no hint about the search chain or which file is active.
- `internal/cache` honors `XDG_CACHE_HOME` (`cache.go:29`), but `internal/config` hardcodes `~/.config` (`config.go:80-85`), ignoring `XDG_CONFIG_HOME`.

## Goals / Non-Goals

**Goals:**

- Provision `config.default.yaml` as a discoverable, version-matched reference at install time without ever touching the user's `config.yaml`.
- Make the config loader XDG-compliant, consistent with the cache loader.
- Surface the search chain and active config in `--help`, and add a `-c` shorthand.
- Document the `config.default.yaml` sync obligation in `AGENTS.md` and the user-facing behavior in `README.md`.

**Non-Goals:**

- No change to config keys, defaults, or the `--status` JSON schema (non-breaking).
- No auto-generation of `config.yaml` from defaults.
- No GoReleaser asset changes (reference is fetched from the repo at the tag).
- No native macOS path (`~/Library/Application Support`) — `~/.config` stays the cross-platform default.

## Decisions

**1. Reference file is overwrite-always; `config.yaml` is never written.**
The reference is regenerated each install to match the installed binary, so a stale reference can never mislead. The user's real config (`config.yaml`) is the only file the loader reads, so leaving it untouched is the safety boundary. Alternative (write `config.yaml` from template) rejected: reinstall would clobber user edits.

**2. Fetch via raw GitHub at the release tag, not a release asset.**
`config.default.yaml` lives in the repo root, and `CLAUDE_USAGE_VERSION` is a git tag (`install.sh:210-214`), so `https://raw.githubusercontent.com/<repo>/${CLAUDE_USAGE_VERSION}/config.default.yaml` resolves with no GoReleaser change. Alternative (publish as release asset) rejected: extra release-pipeline surface for a file already in the repo.

**3. Loader mirrors the existing `XDG_CACHE_HOME` pattern.**
`config.go` reads `XDG_CONFIG_HOME`, falling back to `~/.config`, exactly like `cache.go`. This removes the install-vs-loader divergence when a custom XDG layout is set, and keeps the two loaders idiomatically symmetric.

**4. Single resolution path via a `ResolvePath` helper.**
A new exported helper returns `(path string, found bool)` for the winning entry; `Load` calls it instead of re-walking the chain, and `--help` calls it to render the `Active config:` line. Avoids duplicated chain logic drifting between load and display.

**5. Help footer is append-only (layout 1b).**
Keep stock pflag output; set `flag.Usage` to print defaults then the `Configuration:` block. The chain is shown in literal `$XDG_CONFIG_HOME/...` form (portable/teaching); resolved paths appear only on the `Active config:` / `Reference file:` lines, and `Reference file:` prints only when `config.default.yaml` exists. Approved by the user as scenarios A/B/C.

**6. Uninstall mirrors install; empty-dir cleanup via `rmdir`.**
`install.sh --uninstall` removes only `config.default.yaml` (the file install provisioned), then runs `rmdir "${CFG_DIR}" 2>/dev/null || true` — `rmdir` fails silently on a non-empty dir, so a surviving user `config.yaml` automatically keeps the directory. Alternative (`rm -rf "${CFG_DIR}"`) rejected: would destroy the user's `config.yaml`, violating the safety boundary. Removal is best-effort (`rm -f`), so a missing reference is a no-op.

## Risks / Trade-offs

- **`--help` performs disk I/O (`os.Stat`)** for the active-config and reference lines, a minor break from pure-static help → Mitigation: stat is cheap and bounded (≤3 paths); no network or heavy work.
- **Raw-GitHub fetch can fail** (offline, rate limit, tag not yet pushed) → Mitigation: treat the reference download as best-effort and non-fatal — warn and continue, since the binary works without it. (Contrast the binary download, which is fatal.)
- **`config.default.yaml` drifts from `Default()`** over time → Mitigation: AGENTS.md rule makes syncing mandatory on any config change; it is already aligned today.

## Migration Plan

No data migration. On next install/reinstall, the reference file appears (or refreshes). Existing user `config.yaml` files are unaffected. Rollback is removing the reference file; it has no runtime effect since the loader never reads it.
