package pane

import "sync"

var (
	mu      sync.RWMutex
	drivers = map[string]func() Backend{}
)

// Register makes a backend available under name.
// Called from init() in each driver package (e.g. pane/tmux, pane/zellij).
func Register(name string, factory func() Backend) {
	mu.Lock()
	defer mu.Unlock()
	drivers[name] = factory
}

// New returns a fresh Backend for the given name.
// Returns [ErrNoBackend] if no driver has been registered under that name.
func New(name string) (Backend, error) {
	mu.RLock()
	f, ok := drivers[name]
	mu.RUnlock()
	if !ok {
		return nil, ErrNoBackend(name)
	}
	return f(), nil
}
