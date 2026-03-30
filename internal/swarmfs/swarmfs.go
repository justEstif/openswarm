// Package swarmfs provides file system primitives for the openswarm project.
// It is the deepest internal module — no other internal package may be imported here.
//
// All .swarm/ path construction is centralised here. Callers never construct
// .swarm/ path strings by hand; they use Root methods instead.
package swarmfs

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"syscall"
)

// ─── Root ────────────────────────────────────────────────────────────────────

// Root represents a resolved .swarm/ project root.
// Every command that touches project state needs one.
// Construct via FindRoot or InitRoot — never directly.
type Root struct {
	Dir        string // absolute path to .swarm/
	ConfigPath string // .swarm/config.toml
}

// TasksPath returns the path to the tasks store.
func (r *Root) TasksPath() string { return filepath.Join(r.Dir, "tasks", "tasks.json") }

// TasksLockPath returns the path to the tasks flock file.
func (r *Root) TasksLockPath() string { return filepath.Join(r.Dir, "tasks", ".lock") }

// AgentsPath returns the path to the agent registry.
func (r *Root) AgentsPath() string { return filepath.Join(r.Dir, "agents", "registry.json") }

// InboxPath returns the path to an agent's inbox directory.
func (r *Root) InboxPath(agentID string) string {
	return filepath.Join(r.Dir, "messages", agentID, "inbox") + string(filepath.Separator)
}

// RunsPath returns the path to the runs store.
func (r *Root) RunsPath() string { return filepath.Join(r.Dir, "runs", "runs.json") }

// WorktreesPath returns the path to the worktrees store.
func (r *Root) WorktreesPath() string { return filepath.Join(r.Dir, "worktrees", "worktrees.json") }

// EventsPath returns the path to the append-only event log.
func (r *Root) EventsPath() string { return filepath.Join(r.Dir, "events", "events.jsonl") }

// ─── Root discovery ──────────────────────────────────────────────────────────

// FindRoot walks up the directory tree from the current working directory,
// looking for a .swarm/ directory. It stops at the filesystem root.
// Returns a descriptive error if no .swarm/ directory is found.
func FindRoot() (*Root, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("swarmfs: cannot determine working directory: %w", err)
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, ".swarm")
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return rootAt(candidate), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root — not found.
			break
		}
		dir = parent
	}

	return nil, fmt.Errorf("no .swarm/ found; run `swarm init`")
}

// InitRoot creates .swarm/ and all required subdirectories at base.
// It is idempotent — safe to call even when already initialised.
func InitRoot(base string) (*Root, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("swarmfs: cannot resolve base path %q: %w", base, err)
	}

	swarmDir := filepath.Join(abs, ".swarm")

	dirs := []string{
		swarmDir,
		filepath.Join(swarmDir, "agents"),
		filepath.Join(swarmDir, "messages"),
		filepath.Join(swarmDir, "tasks"),
		filepath.Join(swarmDir, "runs"),
		filepath.Join(swarmDir, "worktrees"),
		filepath.Join(swarmDir, "events"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("swarmfs: cannot create directory %q: %w", d, err)
		}
	}

	// Touch config.toml (empty) so it exists from day one.
	configPath := filepath.Join(swarmDir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("swarmfs: cannot create config.toml: %w", err)
		}
		f.Close()
	}

	return rootAt(swarmDir), nil
}

// rootAt builds a Root from an absolute .swarm/ path.
func rootAt(swarmDir string) *Root {
	return &Root{
		Dir:        swarmDir,
		ConfigPath: filepath.Join(swarmDir, "config.toml"),
	}
}

// ─── File I/O primitives ─────────────────────────────────────────────────────

// AtomicWrite writes data to path using a temp file + os.Rename.
// It creates any missing parent directories before writing.
// The rename guarantees that readers never see a partially-written file.
func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("swarmfs: AtomicWrite mkdir %q: %w", dir, err)
	}

	// Write to a temp file in the same directory so that os.Rename is always
	// on the same filesystem (cross-device rename would fail).
	tmp, err := os.CreateTemp(dir, ".swarmfs-tmp-*")
	if err != nil {
		return fmt.Errorf("swarmfs: AtomicWrite create temp: %w", err)
	}
	tmpName := tmp.Name()

	// Clean up temp file on any error path.
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("swarmfs: AtomicWrite write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("swarmfs: AtomicWrite sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("swarmfs: AtomicWrite close temp: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("swarmfs: AtomicWrite rename: %w", err)
	}

	ok = true
	return nil
}

// AppendLine appends a newline-terminated line to path.
// If the file does not exist it is created. Used for events.jsonl.
func AppendLine(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("swarmfs: AppendLine mkdir %q: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("swarmfs: AppendLine open %q: %w", path, err)
	}
	defer f.Close()

	line := append(data, '\n')
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("swarmfs: AppendLine write %q: %w", path, err)
	}
	return nil
}

// ─── File locking ────────────────────────────────────────────────────────────

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
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("swarmfs: WithFileLock acquire %q: %w", lockPath, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

// ─── ID generation ───────────────────────────────────────────────────────────

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// NewID generates a short random ID with the given prefix.
// Format: "<prefix>-<6 random alphanumeric chars>", e.g. "task-a3f2k1".
// Uses crypto/rand — no external library.
func NewID(prefix string) string {
	const length = 6
	b := make([]byte, length)
	alphabetLen := big.NewInt(int64(len(idAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			// crypto/rand failure is extremely unlikely; fall back to a fixed char
			// rather than panicking, so callers still get a unique-ish ID.
			b[i] = idAlphabet[0]
			continue
		}
		b[i] = idAlphabet[n.Int64()]
	}
	return prefix + "-" + string(b)
}
