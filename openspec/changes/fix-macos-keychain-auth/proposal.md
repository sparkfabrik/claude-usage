## Why

On macOS, Claude Code stores its OAuth credentials in the **login Keychain**
(generic password, service `Claude Code-credentials`), **not** in
`~/.claude/.credentials.json`. The current `auth.Load()` reads only that file,
so on every Mac it finds nothing and the CLI emits `Error: no credentials found`.
This breaks `--status` and therefore the macOS tray, the statusline reader, and
the dashboard on the platform the tray targets.

The Python reference (`claudemeter.py`) reads the Keychain **first** and falls
back to the file. The Go port must do the same to work on macOS at all.

Verified on this machine:

```
$ ls ~/.claude/.credentials.json        → No such file or directory
$ security find-generic-password -s "Claude Code-credentials" -a "$USER" -w
  → {"claudeAiOauth":{"accessToken":"sk-ant-oat01-...
```

## What Changes

- Add a macOS Keychain credential source to `internal/auth` that runs
  `security find-generic-password -s "Claude Code-credentials" -a <user> -w`.
- `auth.Load()` tries the Keychain first on darwin, then falls back to the
  credentials file. On non-darwin platforms behavior is unchanged (file only).
- The Keychain blob is the same JSON shape as the file
  (`{"claudeAiOauth":{...}}`), so it is parsed by the existing logic — no new
  field handling, expiry (`expiresAt`) still honored.
- No keychain write/prompt; read-only lookup with a short timeout.

## Capabilities

### New Capabilities

- `credentials`: OAuth token resolution from platform-appropriate sources
  (macOS Keychain + credentials file fallback).

### Modified Capabilities

## Impact

- New file: `internal/auth/auth_darwin.go` (Keychain reader, build-tagged
  `darwin`) and `internal/auth/auth_other.go` (no-op stub for other OSes).
- Modified: `internal/auth/auth.go` `Load()` to consult the Keychain source
  before the file.
- No change to the `--status` JSON contract or any reader.
- Fixes the "no credentials found" symptom reported for the macOS tray.
