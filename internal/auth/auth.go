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

// Load reads OAuth credentials. On macOS it tries the Keychain first, then
// falls back to the credentials file. On other platforms only the file is read.
// Returns nil if not found.
func Load() (*Credentials, error) {
	// Try macOS Keychain first (no-op on other platforms)
	if data, err := readKeychain(); err != nil {
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
