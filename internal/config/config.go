// Package config loads YAML configuration with defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type API struct {
	Enabled        bool   `yaml:"enabled"`
	StaleAfter     int    `yaml:"stale_after"`
	Model          string `yaml:"model"`
	OnlyWhenActive *bool  `yaml:"only_when_active"`
}

// IsOnlyWhenActive returns whether polling should only happen when Claude Code is running.
// Defaults to true if not explicitly set.
func (a API) IsOnlyWhenActive() bool {
	if a.OnlyWhenActive == nil {
		return true
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

// configPaths to search in order.
func configPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{"config.yaml"}
	}
	return []string{
		filepath.Join(home, ".config", "claude-code-usage", "config.yaml"),
		"config.yaml",
	}
}

// Load reads config from YAML file. Falls back to defaults.
func Load(path string) *Config {
	cfg := Default()

	paths := configPaths()
	if path != "" {
		paths = []string{path}
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		// Unmarshal over defaults — only overrides what's present
		if err := yaml.Unmarshal(data, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: config parse error in %s: %v\n", p, err)
		}
		break
	}

	return cfg
}
