package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if !cfg.API.Enabled {
		t.Error("API.Enabled should default to true")
	}
	if cfg.API.StaleAfter != 60 {
		t.Errorf("StaleAfter = %d, want 60", cfg.API.StaleAfter)
	}
	if cfg.API.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q, want claude-haiku-4-5-20251001", cfg.API.Model)
	}
	if !cfg.Display.ShowCost {
		t.Error("ShowCost should default to true")
	}
	if len(cfg.Display.Periods) != 3 {
		t.Errorf("Periods = %v, want 3 items", cfg.Display.Periods)
	}
	if cfg.Colors.GreenBelow != 80 {
		t.Errorf("GreenBelow = %d, want 80", cfg.Colors.GreenBelow)
	}
	if cfg.Colors.OrangeBelow != 90 {
		t.Errorf("OrangeBelow = %d, want 90", cfg.Colors.OrangeBelow)
	}
}

func TestIsOnlyWhenActive_Default(t *testing.T) {
	cfg := Default()
	if !cfg.API.IsOnlyWhenActive() {
		t.Error("IsOnlyWhenActive should default to true")
	}
}

func TestIsOnlyWhenActive_ExplicitFalse(t *testing.T) {
	f := false
	api := API{OnlyWhenActive: &f}
	if api.IsOnlyWhenActive() {
		t.Error("expected false when explicitly set to false")
	}
}

func TestIsOnlyWhenActive_Nil(t *testing.T) {
	api := API{OnlyWhenActive: nil}
	if !api.IsOnlyWhenActive() {
		t.Error("expected true when nil (default)")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	cfg := Load("/nonexistent/config.yaml")
	// Should return defaults
	if !cfg.API.Enabled {
		t.Error("should fall back to defaults")
	}
	if cfg.API.StaleAfter != 60 {
		t.Errorf("StaleAfter = %d, want 60", cfg.API.StaleAfter)
	}
}

func TestLoad_CustomConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
api:
  enabled: false
  stale_after: 120
  model: "claude-sonnet-4"
display:
  show_cost: false
  periods: ["today", "7d"]
colors:
  green_below: 50
  orange_below: 75
cache:
  path: "/tmp/test-cache.json"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Load(path)

	if cfg.API.Enabled {
		t.Error("API.Enabled should be false")
	}
	if cfg.API.StaleAfter != 120 {
		t.Errorf("StaleAfter = %d, want 120", cfg.API.StaleAfter)
	}
	if cfg.API.Model != "claude-sonnet-4" {
		t.Errorf("Model = %q, want claude-sonnet-4", cfg.API.Model)
	}
	if cfg.Display.ShowCost {
		t.Error("ShowCost should be false")
	}
	if len(cfg.Display.Periods) != 2 {
		t.Errorf("Periods = %v, want 2 items", cfg.Display.Periods)
	}
	if cfg.Colors.GreenBelow != 50 {
		t.Errorf("GreenBelow = %d, want 50", cfg.Colors.GreenBelow)
	}
	if cfg.Cache.Path != "/tmp/test-cache.json" {
		t.Errorf("Cache.Path = %q, want /tmp/test-cache.json", cfg.Cache.Path)
	}
}

func TestLoad_PartialOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
api:
  stale_after: 30
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Load(path)

	// Overridden
	if cfg.API.StaleAfter != 30 {
		t.Errorf("StaleAfter = %d, want 30", cfg.API.StaleAfter)
	}
	// Defaults preserved
	if cfg.Colors.GreenBelow != 80 {
		t.Errorf("GreenBelow = %d, want 80 (default)", cfg.Colors.GreenBelow)
	}
	if !cfg.Display.ShowCost {
		t.Error("ShowCost should remain true (default)")
	}
}

func TestLoad_WithPricing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
pricing:
  claude-sonnet-4:
    input: 5.0
    output: 20.0
    cache_write: 6.0
    cache_read: 0.5
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Load(path)

	if len(cfg.Pricing) != 1 {
		t.Fatalf("Pricing len = %d, want 1", len(cfg.Pricing))
	}
	p, ok := cfg.Pricing["claude-sonnet-4"]
	if !ok {
		t.Fatal("expected claude-sonnet-4 in pricing")
	}
	if p.Input != 5.0 {
		t.Errorf("Input = %f, want 5.0", p.Input)
	}
	if p.Output != 20.0 {
		t.Errorf("Output = %f, want 20.0", p.Output)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{{not valid yaml"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should not panic, returns defaults
	cfg := Load(path)
	if cfg == nil {
		t.Fatal("should return non-nil config even on parse error")
	}
}

func TestConfigPaths(t *testing.T) {
	paths := configPaths()
	if len(paths) == 0 {
		t.Fatal("expected at least one config path")
	}
	// Last entry should be local fallback
	if paths[len(paths)-1] != "config.yaml" {
		t.Errorf("last path = %q, want config.yaml", paths[len(paths)-1])
	}
}
