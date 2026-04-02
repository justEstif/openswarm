// Package config handles loading and validation of .swarm/config.toml.
// It is the single source of truth for all project-wide settings.
//
// Priority (highest to lowest):
//  1. Environment variables (SWARM_BACKEND, SWARM_DEFAULT_AGENT)
//  2. Values from .swarm/config.toml
//  3. Built-in defaults (returned by Defaults)
package config

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/justEstif/openswarm/internal/swarmfs"
)

// PaneConfig holds pane-spawning defaults.
type PaneConfig struct {
	// Placement is the default location for new panes.
	// "current_tab" (default) | "new_tab" | "new_session"
	Placement string `toml:"placement"`
}

// Config holds the project-level settings read from .swarm/config.toml.
type Config struct {
	TeamName      string         `toml:"team_name"`
	DefaultAgent  string         `toml:"default_agent"`
	Backend       string         `toml:"backend"`       // "auto" | "tmux" | "zellij" | "wezterm" | "kitty"
	PollInterval  string         `toml:"poll_interval"` // e.g. "200ms", "1s"; default "200ms"
	Pane          PaneConfig     `toml:"pane"`
	AgentProfiles []AgentProfile `toml:"agent"`
}

// AgentProfile describes a named agent and how to launch it.
type AgentProfile struct {
	Name    string            `toml:"name"`
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	Env     map[string]string `toml:"env"`
}

// Defaults returns a new Config with all built-in default values applied.
// Callers should start from Defaults rather than a zero Config to ensure
// every field has a well-defined value.
func Defaults() *Config {
	return &Config{
		Backend:      "auto",
		TeamName:     "",
		DefaultAgent: "",
		PollInterval: "200ms",
		Pane:         PaneConfig{Placement: "current_tab"},
	}
}

// Load reads .swarm/config.toml via root.ConfigPath and returns the resulting
// Config with environment-variable overrides applied on top.
//
// If the file does not exist Load returns Defaults() with no error — an absent
// config is valid (the project just uses all defaults).
//
// Environment variables:
//
//	SWARM_BACKEND       overrides Config.Backend
//	SWARM_DEFAULT_AGENT overrides Config.DefaultAgent
func Load(root *swarmfs.Root) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(root.ConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No config file — apply env overrides to defaults and return.
			applyEnv(cfg)
			return cfg, nil
		}
		return nil, err
	}

	// Empty file is valid; skip TOML parsing to avoid "unexpected EOF" errors.
	if len(data) > 0 {
		if _, err := toml.Decode(string(data), cfg); err != nil {
			return nil, err
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

// applyEnv overlays environment-variable values on top of cfg in-place.
func applyEnv(cfg *Config) {
	if v := os.Getenv("SWARM_BACKEND"); v != "" {
		cfg.Backend = v
	}
	if v := os.Getenv("SWARM_DEFAULT_AGENT"); v != "" {
		cfg.DefaultAgent = v
	}
	if v := os.Getenv("SWARM_PANE_PLACEMENT"); v != "" {
		cfg.Pane.Placement = v
	}
}
