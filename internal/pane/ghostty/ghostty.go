// Package ghostty is a placeholder backend for the Ghostty terminal.
//
// Ghostty has no remote-control API at this time.  All methods return
// pane.ErrNotSupported.  This package exists so that code can blank-import it
// without build errors, and so that the driver is registered in the backend
// registry for forward-compatibility.
//
// Upstream tracking issue: https://github.com/ghostty-org/ghostty/issues/4625
package ghostty

import (
	"context"
	"fmt"

	"github.com/justEstif/openswarm/internal/pane"
)

func init() {
	pane.Register("ghostty", func() pane.Backend { return &GhosttyBackend{} })
}

// GhosttyBackend is a stub that satisfies pane.Backend.
// Every method returns ErrNotSupported until Ghostty exposes a remote-control API.
type GhosttyBackend struct{}

// notSupported returns a wrapped ErrNotSupported with a reference to the upstream issue.
func notSupported() error {
	return fmt.Errorf("%w: Ghostty remote control is not yet available (see https://github.com/ghostty-org/ghostty/issues/4625)",
		pane.ErrNotSupported)
}

// Name returns the registered backend name.
func (g *GhosttyBackend) Name() string { return "ghostty" }

// Spawn is not supported.
func (g *GhosttyBackend) Spawn(name, cmd string, env map[string]string) (pane.PaneID, error) {
	return "", notSupported()
}

// Send is not supported.
func (g *GhosttyBackend) Send(id pane.PaneID, text string) error {
	return notSupported()
}

// Capture is not supported.
func (g *GhosttyBackend) Capture(id pane.PaneID) (string, error) {
	return "", notSupported()
}

// Subscribe is not supported.
func (g *GhosttyBackend) Subscribe(ctx context.Context, id pane.PaneID) (<-chan pane.OutputEvent, error) {
	return nil, notSupported()
}

// List is not supported.
func (g *GhosttyBackend) List() ([]pane.PaneInfo, error) {
	return nil, notSupported()
}

// Close is not supported.
func (g *GhosttyBackend) Close(id pane.PaneID) error {
	return notSupported()
}

// Wait is not supported.
func (g *GhosttyBackend) Wait(id pane.PaneID) (int, error) {
	return -1, notSupported()
}
