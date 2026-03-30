package pane

import (
	"fmt"
	"os"

	"github.com/justEstif/openswarm/internal/config"
)

// DetectBackend selects and instantiates a Backend using a 5-level cascade:
//
//  1. $SWARM_BACKEND env var (highest priority — explicit override)
//  2. cfg.Backend if not "auto" or ""
//  3. $TMUX — running inside tmux
//  4. $WEZTERM_PANE — running inside WezTerm
//  5. $ZELLIJ — running inside Zellij
//
// Returns an error if no supported multiplexer is detected and no override is set.
// The selected driver must have been registered via [Register] (typically by
// blank-importing the driver package).
func DetectBackend(cfg *config.Config) (Backend, error) {
	name := cfg.Backend
	if env := os.Getenv("SWARM_BACKEND"); env != "" {
		name = env
	}
	if name != "" && name != "auto" {
		return New(name)
	}
	switch {
	case os.Getenv("TMUX") != "":
		return New("tmux")
	case os.Getenv("WEZTERM_PANE") != "":
		return New("wezterm")
	case os.Getenv("ZELLIJ") != "":
		return New("zellij")
	default:
		return nil, fmt.Errorf("no supported multiplexer detected; set $SWARM_BACKEND or run inside tmux/zellij/wezterm")
	}
}
