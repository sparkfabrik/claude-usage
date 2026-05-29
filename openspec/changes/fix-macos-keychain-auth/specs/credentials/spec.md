## ADDED Requirements

### Requirement: macOS Keychain credential source

On macOS, OAuth credentials SHALL be read from the login Keychain generic
password whose service is `Claude Code-credentials`, before any file-based
source is consulted.

#### Scenario: Keychain holds the token

- **WHEN** running on darwin and `security find-generic-password -s "Claude Code-credentials" -a <current user> -w` returns a JSON blob containing `accessToken`
- **THEN** `auth.Load()` returns credentials parsed from that blob
- **AND** the credentials file is not required to exist

#### Scenario: Keychain item absent

- **WHEN** running on darwin and the Keychain has no matching item (or `security` exits non-zero)
- **THEN** the Keychain source is treated as "not found" (no hard error)
- **AND** `auth.Load()` falls back to the credentials file

### Requirement: File fallback and cross-platform behavior

Credential resolution SHALL fall back to the credentials file and SHALL remain
file-only on non-macOS platforms.

#### Scenario: File fallback on macOS

- **WHEN** the Keychain yields nothing but `~/.claude/.credentials.json` (or `$CLAUDE_CONFIG_DIR/.credentials.json`) exists with a valid `accessToken`
- **THEN** `auth.Load()` returns credentials parsed from the file

#### Scenario: Non-macOS unchanged

- **WHEN** running on a non-darwin platform
- **THEN** no Keychain lookup is attempted and only the credentials file is read

### Requirement: Shared parsing and expiry

Credentials from either source SHALL be parsed by identical logic and honor
token expiry.

#### Scenario: Nested and top-level shapes

- **WHEN** a blob is either `{"claudeAiOauth":{"accessToken":...}}` or a top-level object with `accessToken`
- **THEN** both parse to the same `Credentials` value

#### Scenario: Expired token still surfaces expiry

- **WHEN** the resolved credentials have an `expiresAt` in the past
- **THEN** `IsExpired()` returns true regardless of which source provided them
