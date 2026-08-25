// Package config loads YAML configuration with defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type API struct {
	Enabled    bool   `yaml:"enabled"`
	StaleAfter int    `yaml:"stale_after"`
	Model      string `yaml:"model"`
	// UsageEndpoint is the OAuth usage endpoint. Empty means the built-in
	// default. It is read first; the 1-token header poll is the fallback.
	UsageEndpoint  string `yaml:"usage_endpoint"`
	OnlyWhenActive *bool  `yaml:"only_when_active"`
}

// IsOnlyWhenActive returns whether polling should only happen when Claude Code
// is running. It defaults to false: the usage endpoint costs nothing, so the
// reason to hold back is gone, and holding back left readers showing stale
// numbers for as long as Claude Code stayed closed.
func (a API) IsOnlyWhenActive() bool {
	if a.OnlyWhenActive == nil {
		return false
	}
	return *a.OnlyWhenActive
}

type Display struct {
	ShowCost bool     `yaml:"show_cost"`
	Periods  []string `yaml:"periods"`
}

type Colors struct {
	GreenBelow  int `yaml:"green_below"`
	OrangeBelow int `yaml:"orange_below"`
}

type ModelPricing struct {
	Input      float64 `yaml:"input"`
	Output     float64 `yaml:"output"`
	CacheWrite float64 `yaml:"cache_write"`
	CacheRead  float64 `yaml:"cache_read"`
}

type Cache struct {
	Path string `yaml:"path"`
}

type Config struct {
	API     API                     `yaml:"api"`
	Display Display                 `yaml:"display"`
	Colors  Colors                  `yaml:"colors"`
	Cache   Cache                   `yaml:"cache"`
	Pricing map[string]ModelPricing `yaml:"pricing,omitempty"`
}

// Default returns configuration with default values.
func Default() *Config {
	onlyWhenActive := true
	return &Config{
		API: API{
			Enabled:        true,
			StaleAfter:     60,
			Model:          "claude-haiku-4-5-20251001",
			OnlyWhenActive: &onlyWhenActive,
		},
		Display: Display{
			ShowCost: true,
			Periods:  []string{"today", "7d", "30d"},
		},
		Colors: Colors{
			GreenBelow:  80,
			OrangeBelow: 90,
		},
	}
}

// configPaths to search in order. Honors XDG_CONFIG_HOME (falling back to
// ~/.config), mirroring the XDG_CACHE_HOME pattern in internal/cache.
func configPaths() []string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Home unknown and XDG unset: fall back to CWD only.
			return []string{"config.yaml"}
		}
		configDir = filepath.Join(home, ".config")
	}
	return []string{
		filepath.Join(configDir, "claude-code-usage", "config.yaml"),
		"config.yaml",
	}
}

// configDir returns the XDG-aware config directory
// (${XDG_CONFIG_HOME:-~/.config}/claude-code-usage) and whether it could be
// determined. It mirrors the search-chain root used by configPaths.
func configDir() (string, bool) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "claude-code-usage"), true
}

// ReferencePath reports the resolved path to the install-provisioned
// reference file (config.default.yaml) and whether it exists on disk.
func ReferencePath() (path string, found bool) {
	dir, ok := configDir()
	if !ok {
		return "", false
	}
	p := filepath.Join(dir, "config.default.yaml")
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// SearchChainDisplay returns the default search chain in portable literal
// form (using $XDG_CONFIG_HOME), suitable for --help output.
func SearchChainDisplay() []string {
	return []string{
		filepath.Join("$XDG_CONFIG_HOME", "claude-code-usage", "config.yaml") + "  (falls back to ~/.config when unset)",
		"./config.yaml",
	}
}

// ResolvePath reports which file in the default search chain is selected.
// It returns the first existing file (found=true) or ("", false) when no
// file in the chain exists. Used by Load and by --help so the resolution
// logic is not duplicated.
func ResolvePath() (path string, found bool) {
	for _, p := range configPaths() {
		// Only a regular file counts as a match; a directory or special
		// file must not "win" the chain and shadow later entries.
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p, true
		}
	}
	return "", false
}

// Load reads config from YAML file. Falls back to defaults.
func Load(path string) *Config {
	cfg := Default()

	if path == "" {
		resolved, found := ResolvePath()
		if !found {
			return cfg
		}
		path = resolved
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	// Unmarshal over defaults — only overrides what's present
	if err := yaml.Unmarshal(data, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config parse error in %s: %v\n", path, err)
	}

	return cfg
}
