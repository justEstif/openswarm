package pane

import "fmt"

// ErrNotSupported is returned by backends that do not implement an operation.
// Ghostty returns this for all methods until issue #4625 is resolved.
var ErrNotSupported = fmt.Errorf("operation not supported by this backend")

// ErrPaneNotFound is returned when a PaneID cannot be resolved.
func ErrPaneNotFound(id PaneID) error {
	return fmt.Errorf("pane not found: %s", id)
}

// ErrNoBackend is returned by [New] when no driver has been registered for name.
// Fix: blank-import the driver package (e.g. _ "github.com/justEstif/openswarm/internal/pane/tmux").
func ErrNoBackend(name string) error {
	return fmt.Errorf("no backend registered for %q; did you blank-import the driver?", name)
}
