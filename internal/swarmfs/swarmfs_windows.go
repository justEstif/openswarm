//go:build windows

package swarmfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// mu is a process-level fallback lock for Windows, where syscall.Flock is
// unavailable. This does NOT protect against concurrent processes — it only
// serialises goroutines within a single process.
var mu sync.Mutex

// WithFileLock on Windows falls back to a process-local mutex. It still
// creates the lock file so callers can assume its existence.
func WithFileLock(lockPath string, fn func() error) error {
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("swarmfs: WithFileLock mkdir %q: %w", dir, err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("swarmfs: WithFileLock open %q: %w", lockPath, err)
	}
	_ = f.Close()

	mu.Lock()
	defer mu.Unlock()
	return fn()
}
