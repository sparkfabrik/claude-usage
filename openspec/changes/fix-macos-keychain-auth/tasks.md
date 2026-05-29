## 1. Keychain reader (darwin)

- [ ] 1.1 Add `internal/auth/auth_darwin.go` (build tag `darwin`) with `readKeychain() ([]byte, error)` running `security find-generic-password -s "Claude Code-credentials" -a <current user> -w` with a ~10s timeout
- [ ] 1.2 Add `internal/auth/auth_other.go` (build tag `!darwin`) with `readKeychain()` returning `(nil, nil)` so non-mac builds compile and skip the Keychain
- [ ] 1.3 Treat a missing item / non-zero `security` exit as "not found" (return nil, no error), not a hard failure

## 2. Load() integration

- [ ] 2.1 In `auth.Load()`, attempt the Keychain blob first; if it yields a valid `accessToken`, parse and return it (reuse the existing nested/top-level unmarshal path)
- [ ] 2.2 Fall back to reading `~/.claude/.credentials.json` (or `CLAUDE_CONFIG_DIR`) when the Keychain has nothing
- [ ] 2.3 Factor the JSON→`Credentials` parsing into a shared helper so both sources use identical logic (nested `claudeAiOauth`, then top-level)
- [ ] 2.4 Preserve `IsExpired()` semantics for tokens from either source

## 3. Verify

- [ ] 3.1 `go build ./...` on darwin and confirm cross-compile for linux still builds (`GOOS=linux go build ./...`)
- [ ] 3.2 Run `claude-usage --status` on macOS and confirm it returns utilization (no "no credentials found")
- [ ] 3.3 Confirm the macOS tray now shows live percentages instead of the error state
