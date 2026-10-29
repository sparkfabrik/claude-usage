package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubKeychain makes Load see the given Keychain content instead of the
// developer's real one, and restores the lookup when the test ends. A nil
// data with a nil error is "no item", which is what every non-macOS build
// and every Mac without a Claude Code login report.
func stubKeychain(t *testing.T, data []byte, err error) {
	t.Helper()
	prev := keychainReader
	keychainReader = func() ([]byte, error) { return data, err }
	t.Cleanup(func() { keychainReader = prev })
}

func TestIsExpired_ZeroValue(t *testing.T) {
	c := &Credentials{ExpiresAt: 0}
	if c.IsExpired() {
		t.Error("zero ExpiresAt should not be expired")
	}
}

func TestIsExpired_Future(t *testing.T) {
	c := &Credentials{ExpiresAt: time.Now().Add(1 * time.Hour).UnixMilli()}
	if c.IsExpired() {
		t.Error("future ExpiresAt should not be expired")
	}
}

func TestIsExpired_Past(t *testing.T) {
	c := &Credentials{ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli()}
	if !c.IsExpired() {
		t.Error("past ExpiresAt should be expired")
	}
}

func TestCredentialsPath_Default(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	path, err := CredentialsPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path should be absolute, got %q", path)
	}
	if filepath.Base(path) != ".credentials.json" {
		t.Errorf("expected .credentials.json, got %q", filepath.Base(path))
	}
}

func TestCredentialsPath_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/test-claude")
	path, err := CredentialsPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/tmp/test-claude/.credentials.json"
	if path != want {
		t.Errorf("got %q, want %q", path, want)
	}
}

func TestParseCredentials_Nested(t *testing.T) {
	data := []byte(`{"claudeAiOauth":{"accessToken":"tok123","refreshToken":"ref456","expiresAt":9999999999999}}`)
	creds, err := parseCredentials(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "tok123" {
		t.Error("AccessToken is not the one in the fixture (tok123)")
	}
	if creds.RefreshToken != "ref456" {
		t.Errorf("RefreshToken = %q, want ref456", creds.RefreshToken)
	}
}

func TestParseCredentials_TopLevel(t *testing.T) {
	data := []byte(`{"accessToken":"tok789","expiresAt":9999999999999}`)
	creds, err := parseCredentials(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "tok789" {
		t.Error("AccessToken is not the one in the fixture (tok789)")
	}
}

func TestParseCredentials_NoAccessToken(t *testing.T) {
	data := []byte(`{"refreshToken":"ref","expiresAt":123}`)
	creds, err := parseCredentials(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil for no accessToken, got %v", creds)
	}
}

func TestParseCredentials_InvalidJSON(t *testing.T) {
	_, err := parseCredentials([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseCredentials_WithSubscription(t *testing.T) {
	data := []byte(`{"accessToken":"tok","subscriptionType":"pro","rateLimitTier":"t5","expiresAt":9999999999999}`)
	creds, err := parseCredentials(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.SubscriptionType != "pro" {
		t.Errorf("SubscriptionType = %q, want pro", creds.SubscriptionType)
	}
	if creds.RateLimitTier != "t5" {
		t.Errorf("RateLimitTier = %q, want t5", creds.RateLimitTier)
	}
}

func TestParseCredentials_NestedBadInnerJSON(t *testing.T) {
	// Valid outer wrapper but inner claudeAiOauth is not valid credential JSON
	data := []byte(`{"claudeAiOauth":"not-an-object"}`)
	_, err := parseCredentials(data)
	if err == nil {
		t.Error("expected error for bad inner JSON")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	stubKeychain(t, nil, nil)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	creds, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil for missing file, got %v", creds)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	stubKeychain(t, nil, nil)
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	data := []byte(`{"claudeAiOauth":{"accessToken":"file-tok","expiresAt":9999999999999}}`)
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	creds, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "file-tok" {
		t.Error("AccessToken is not the one in the file (file-tok)")
	}
}

func TestLoad_MalformedFile(t *testing.T) {
	stubKeychain(t, nil, nil)
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("expected error for malformed credentials file")
	}
}

func TestLoad_FileWithNoAccessToken(t *testing.T) {
	stubKeychain(t, nil, nil)
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	data := []byte(`{"claudeAiOauth":{"refreshToken":"ref","expiresAt":123}}`)
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	creds, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Errorf("expected nil for no accessToken, got %v", creds)
	}
}

// The Keychain wins over the file when it holds credentials.
func TestLoad_KeychainWins(t *testing.T) {
	stubKeychain(t, []byte(`{"claudeAiOauth":{"accessToken":"kc-tok","expiresAt":9999999999999}}`), nil)
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	data := []byte(`{"claudeAiOauth":{"accessToken":"file-tok","expiresAt":9999999999999}}`)
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	creds, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil || creds.AccessToken != "kc-tok" {
		t.Error("expected the Keychain credentials (kc-tok)")
	}
}

// Keychain content that does not parse is ignored and the file is read.
func TestLoad_KeychainUnparsableFallsBackToFile(t *testing.T) {
	stubKeychain(t, []byte("not json"), nil)
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	data := []byte(`{"claudeAiOauth":{"accessToken":"file-tok","expiresAt":9999999999999}}`)
	if err := os.WriteFile(filepath.Join(dir, ".credentials.json"), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	creds, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil || creds.AccessToken != "file-tok" {
		t.Error("expected the file credentials (file-tok)")
	}
}

// A Keychain error is returned as is, before the file is tried.
func TestLoad_KeychainError(t *testing.T) {
	want := errors.New("keychain locked")
	stubKeychain(t, nil, want)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if _, err := Load(); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// Printing credentials with %v or %+v, by value or by pointer, must never
// show a token.
func TestCredentials_StringRedacts(t *testing.T) {
	c := &Credentials{AccessToken: "sk-ant-oat01-SECRET", RefreshToken: "sk-ant-ort01-SECRET", ExpiresAt: 42, SubscriptionType: "team", RateLimitTier: "max_5x"}
	for _, s := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%v", *c), fmt.Sprintf("%+v", *c), c.String()} {
		if strings.Contains(s, "SECRET") {
			t.Errorf("token leaked: %s", s)
		}
		if !strings.Contains(s, "redacted") || !strings.Contains(s, "team") || !strings.Contains(s, "42") {
			t.Errorf("redacted form should keep the non-secret fields: %s", s)
		}
	}
}
