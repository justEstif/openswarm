package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justEstif/openswarm/internal/config"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newRoot initialises a temporary .swarm/ directory and returns the Root.
func newRoot(t *testing.T) *swarmfs.Root {
	t.Helper()
	tmp := t.TempDir()
	root, err := swarmfs.InitRoot(tmp)
	if err != nil {
		t.Fatalf("InitRoot: %v", err)
	}
	return root
}

// writeConfig writes content to root.ConfigPath.
func writeConfig(t *testing.T, root *swarmfs.Root, content string) {
	t.Helper()
	if err := os.WriteFile(root.ConfigPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
}

// removeConfig deletes root.ConfigPath so Load sees a missing file.
func removeConfig(t *testing.T, root *swarmfs.Root) {
	t.Helper()
	if err := os.Remove(root.ConfigPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("removeConfig: %v", err)
	}
}

// ─── Defaults ────────────────────────────────────────────────────────────────

func TestDefaults(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Backend != "auto" {
		t.Errorf("Backend = %q; want %q", cfg.Backend, "auto")
	}
	if cfg.TeamName != "" {
		t.Errorf("TeamName = %q; want empty string", cfg.TeamName)
	}
	if cfg.DefaultAgent != "" {
		t.Errorf("DefaultAgent = %q; want empty string", cfg.DefaultAgent)
	}
	if cfg.AgentProfiles != nil {
		t.Errorf("AgentProfiles = %v; want nil", cfg.AgentProfiles)
	}
}

// ─── Load: missing file ───────────────────────────────────────────────────────

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	root := newRoot(t)
	removeConfig(t, root)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
	want := config.Defaults()
	if cfg.Backend != want.Backend {
		t.Errorf("Backend = %q; want %q", cfg.Backend, want.Backend)
	}
	if cfg.TeamName != want.TeamName {
		t.Errorf("TeamName = %q; want %q", cfg.TeamName, want.TeamName)
	}
	if cfg.DefaultAgent != want.DefaultAgent {
		t.Errorf("DefaultAgent = %q; want %q", cfg.DefaultAgent, want.DefaultAgent)
	}
}

// ─── Load: empty file ────────────────────────────────────────────────────────

func TestLoad_EmptyFile_ReturnsDefaults(t *testing.T) {
	root := newRoot(t)
	// InitRoot already creates an empty config.toml; nothing extra needed.

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load returned error for empty file: %v", err)
	}
	if cfg.Backend != "auto" {
		t.Errorf("Backend = %q; want %q", cfg.Backend, "auto")
	}
}

// ─── Load: TOML parsing ───────────────────────────────────────────────────────

func TestLoad_TOMLParsing_AllFields(t *testing.T) {
	root := newRoot(t)
	writeConfig(t, root, `
team_name     = "alpha"
default_agent = "coder"
backend       = "tmux"

[[agent]]
name    = "coder"
command = "claude"
args    = ["--model", "sonnet"]

[agent.env]
ANTHROPIC_API_KEY = "sk-test"
`)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.TeamName != "alpha" {
		t.Errorf("TeamName = %q; want %q", cfg.TeamName, "alpha")
	}
	if cfg.DefaultAgent != "coder" {
		t.Errorf("DefaultAgent = %q; want %q", cfg.DefaultAgent, "coder")
	}
	if cfg.Backend != "tmux" {
		t.Errorf("Backend = %q; want %q", cfg.Backend, "tmux")
	}
	if len(cfg.AgentProfiles) != 1 {
		t.Fatalf("len(AgentProfiles) = %d; want 1", len(cfg.AgentProfiles))
	}
	p := cfg.AgentProfiles[0]
	if p.Name != "coder" {
		t.Errorf("AgentProfiles[0].Name = %q; want %q", p.Name, "coder")
	}
	if p.Command != "claude" {
		t.Errorf("AgentProfiles[0].Command = %q; want %q", p.Command, "claude")
	}
	if len(p.Args) != 2 || p.Args[0] != "--model" || p.Args[1] != "sonnet" {
		t.Errorf("AgentProfiles[0].Args = %v; want [--model sonnet]", p.Args)
	}
	if p.Env["ANTHROPIC_API_KEY"] != "sk-test" {
		t.Errorf("AgentProfiles[0].Env[ANTHROPIC_API_KEY] = %q; want %q",
			p.Env["ANTHROPIC_API_KEY"], "sk-test")
	}
}

func TestLoad_TOMLParsing_MultipleAgents(t *testing.T) {
	root := newRoot(t)
	writeConfig(t, root, `
[[agent]]
name    = "writer"
command = "gpt"

[[agent]]
name    = "reviewer"
command = "claude"
`)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.AgentProfiles) != 2 {
		t.Fatalf("len(AgentProfiles) = %d; want 2", len(cfg.AgentProfiles))
	}
	if cfg.AgentProfiles[0].Name != "writer" {
		t.Errorf("AgentProfiles[0].Name = %q; want %q", cfg.AgentProfiles[0].Name, "writer")
	}
	if cfg.AgentProfiles[1].Name != "reviewer" {
		t.Errorf("AgentProfiles[1].Name = %q; want %q", cfg.AgentProfiles[1].Name, "reviewer")
	}
}

func TestLoad_TOMLParsing_InvalidTOML_ReturnsError(t *testing.T) {
	root := newRoot(t)
	writeConfig(t, root, `backend = [not valid toml`)

	_, err := config.Load(root)
	if err == nil {
		t.Fatal("Load returned nil error for invalid TOML; want error")
	}
}

// ─── Load: env var overrides ──────────────────────────────────────────────────

func TestLoad_EnvOverride_Backend(t *testing.T) {
	root := newRoot(t)
	writeConfig(t, root, `backend = "tmux"`)

	t.Setenv("SWARM_BACKEND", "zellij")

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "zellij" {
		t.Errorf("Backend = %q; want %q (env override)", cfg.Backend, "zellij")
	}
}

func TestLoad_EnvOverride_DefaultAgent(t *testing.T) {
	root := newRoot(t)
	writeConfig(t, root, `default_agent = "writer"`)

	t.Setenv("SWARM_DEFAULT_AGENT", "reviewer")

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultAgent != "reviewer" {
		t.Errorf("DefaultAgent = %q; want %q (env override)", cfg.DefaultAgent, "reviewer")
	}
}

func TestLoad_EnvOverride_OnMissingFile(t *testing.T) {
	root := newRoot(t)
	removeConfig(t, root)

	t.Setenv("SWARM_BACKEND", "kitty")
	t.Setenv("SWARM_DEFAULT_AGENT", "dev")

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "kitty" {
		t.Errorf("Backend = %q; want %q", cfg.Backend, "kitty")
	}
	if cfg.DefaultAgent != "dev" {
		t.Errorf("DefaultAgent = %q; want %q", cfg.DefaultAgent, "dev")
	}
}

func TestLoad_EnvOverride_EmptyEnvVarIgnored(t *testing.T) {
	root := newRoot(t)
	writeConfig(t, root, `backend = "wezterm"`)

	// Explicitly set to empty — should NOT override the file value.
	t.Setenv("SWARM_BACKEND", "")

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "wezterm" {
		t.Errorf("Backend = %q; want %q (empty env should not override)", cfg.Backend, "wezterm")
	}
}

// ─── Load: ConfigPath used correctly ─────────────────────────────────────────

func TestLoad_UsesRootConfigPath(t *testing.T) {
	// Create a root and then point ConfigPath to a completely separate file
	// to confirm Load uses root.ConfigPath rather than a hardcoded path.
	root := newRoot(t)

	altPath := filepath.Join(t.TempDir(), "alt.toml")
	if err := os.WriteFile(altPath, []byte(`backend = "kitty"`), 0o644); err != nil {
		t.Fatalf("write alt config: %v", err)
	}

	// Swap the path.
	root.ConfigPath = altPath

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "kitty" {
		t.Errorf("Backend = %q; want %q", cfg.Backend, "kitty")
	}
}
