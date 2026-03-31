//go:build !windows

package swarmfs

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// WithFileLock acquires an exclusive flock on lockPath, calls fn, then releases
// the lock. It creates lockPath if it does not exist.
// Uses syscall.Flock(LOCK_EX) — this is a cooperative advisory lock.
func WithFileLock(lockPath string, fn func() error) error {
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("swarmfs: WithFileLock mkdir %q: %w", dir, err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("swarmfs: WithFileLock open %q: %w", lockPath, err)
	}
	defer func() { _ = f.Close() }()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("swarmfs: WithFileLock acquire %q: %w", lockPath, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}
