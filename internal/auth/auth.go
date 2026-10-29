// Package auth reads OAuth credentials from Claude Code's local storage.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Credentials holds parsed OAuth credential data.
type Credentials struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresAt        int64  `json:"expiresAt"` // Unix timestamp ms
	SubscriptionType string `json:"subscriptionType"`
	RateLimitTier    string `json:"rateLimitTier"`
}

// String describes the credentials without their secrets, so a %v or %+v in
// a log, an error or a test failure never prints a token.
func (c Credentials) String() string {
	return fmt.Sprintf("Credentials{AccessToken:<redacted %d bytes> RefreshToken:<redacted %d bytes> ExpiresAt:%d SubscriptionType:%q RateLimitTier:%q}",
		len(c.AccessToken), len(c.RefreshToken), c.ExpiresAt, c.SubscriptionType, c.RateLimitTier)
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixMilli() > c.ExpiresAt
}

// credentialsFile is the raw JSON structure.
type credentialsFile struct {
	ClaudeAiOauth json.RawMessage `json:"claudeAiOauth"`
}

// CredentialsPath returns the path to Claude Code's credentials file.
func CredentialsPath() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, ".credentials.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude", ".credentials.json"), nil
}

// parseCredentials parses a JSON blob into Credentials.
// It handles both the nested {"claudeAiOauth":{...}} shape and a top-level object.
// Returns nil if no accessToken is present.
func parseCredentials(data []byte) (*Credentials, error) {
	var wrapper credentialsFile
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	raw := wrapper.ClaudeAiOauth
	if raw == nil {
		// Fall back to top-level
		raw = data
	}

	var creds Credentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil, err
	}

	if creds.AccessToken == "" {
		return nil, nil
	}

	return &creds, nil
}

// keychainReader is the Keychain lookup Load uses: readKeychain, which the
// darwin build implements and the others stub out. It is a variable so tests
// can run Load without touching the developer's real Keychain.
var keychainReader = readKeychain

// Load reads OAuth credentials. On macOS it tries the Keychain first, then
// falls back to the credentials file. On other platforms only the file is read.
// Returns nil if not found.
func Load() (*Credentials, error) {
	// Try macOS Keychain first (no-op on other platforms)
	if data, err := keychainReader(); err != nil {
		return nil, err
	} else if data != nil {
		if creds, err := parseCredentials(data); err == nil && creds != nil {
			return creds, nil
		}
		// If keychain data doesn't parse, fall through to file
	}

	// Fall back to credentials file
	credPath, err := CredentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return parseCredentials(data)
}
