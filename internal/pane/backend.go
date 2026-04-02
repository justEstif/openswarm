// Package pane is the multiplexer abstraction layer.
//
// Callers import only this package — never a concrete driver.
// Drivers (tmux, zellij, wezterm, ghostty) register via [Register] in their init().
//
// Typical usage:
//
//	import (
//	    _ "github.com/justEstif/openswarm/internal/pane/tmux" // blank-import to register
//	    "github.com/justEstif/openswarm/internal/pane"
//	)
//
//	b, err := pane.DetectBackend(cfg)
//	id, err := b.Spawn("build", "go build ./...", nil)
package pane

import "context"

// PaneID is the backend-specific pane identifier (e.g. "%42", "terminal_5", "3").
type PaneID string

// OutputEvent is one chunk of output from a pane.
type OutputEvent struct {
	PaneID PaneID
	Text   string
	Exited bool // true on the last event when pane exits
	Code   int  // exit code (only meaningful when Exited==true)
}

// PaneInfo describes a live pane.
type PaneInfo struct {
	ID      PaneID
	Name    string
	Running bool
	Command string
}

// Placement controls where a new pane is created within the multiplexer.
type Placement string

const (
	// PlacementCurrentTab creates a split pane inside the active tab/window (default).
	// Zellij: new-pane. tmux: split-window.
	PlacementCurrentTab Placement = "current_tab"

	// PlacementNewTab opens the pane in a dedicated new tab/window.
	// Zellij: new-tab + new-pane. tmux: new-window.
	PlacementNewTab Placement = "new_tab"

	// PlacementNewSession opens the pane in a brand-new session.
	// Zellij: new session (via ZELLIJ_SESSION_NAME self-cleanup). tmux: new-session.
	PlacementNewSession Placement = "new_session"
)

// SpawnOptions configures optional behaviour when creating a pane.
type SpawnOptions struct {
	// Env holds extra environment variables injected into the pane.
	Env map[string]string

	// Placement controls where the pane is created. Defaults to PlacementCurrentTab.
	Placement Placement

	// CloseOnExit closes the pane (and its container tab/session) when the command exits.
	// For ephemeral background runs set this to true; for interactive panes leave it false.
	// When true each backend injects the appropriate cleanup trailer for the given Placement.
	CloseOnExit bool
}

// Backend is the multiplexer abstraction. Callers import only "internal/pane".
// Drivers (tmux, zellij, wezterm, ghostty) register via pane.Register() in their init().
type Backend interface {
	// Spawn creates a new pane running cmd.
	// Returns the pane ID once the pane shell is ready.
	Spawn(name, cmd string, opts SpawnOptions) (PaneID, error)

	// Send delivers text to a pane's stdin (no implicit newline).
	Send(id PaneID, text string) error

	// Capture returns the current scrollback+viewport of a pane as plain text.
	Capture(id PaneID) (string, error)

	// Subscribe streams output from a pane until it exits or ctx is cancelled.
	// Callers must drain the channel; a full channel blocks the driver.
	Subscribe(ctx context.Context, id PaneID) (<-chan OutputEvent, error)

	// List returns all panes known to this backend in the current session.
	List() ([]PaneInfo, error)

	// Close terminates a pane. Idempotent — no error if already gone.
	Close(id PaneID) error

	// Wait blocks until the pane exits and returns its exit code.
	// Returns -1 if exit code is unavailable (e.g. WezTerm).
	Wait(id PaneID) (int, error)

	// Name returns the backend's registered name ("tmux", "zellij", "wezterm", "ghostty").
	Name() string
}
