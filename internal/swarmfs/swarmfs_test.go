package swarmfs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// tempDir creates a temporary directory and registers cleanup.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "swarmfs-test-*")
	if err != nil {
		t.Fatalf("tempDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// mustInitRoot calls InitRoot and fails the test on error.
func mustInitRoot(t *testing.T, base string) *swarmfs.Root {
	t.Helper()
	root, err := swarmfs.InitRoot(base)
	if err != nil {
		t.Fatalf("InitRoot(%q): %v", base, err)
	}
	return root
}

// ─── Root path methods ───────────────────────────────────────────────────────

func TestRootPaths(t *testing.T) {
	base := tempDir(t)
	root := mustInitRoot(t, base)

	swarmDir := filepath.Join(base, ".swarm")

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Dir", root.Dir, swarmDir},
		{"ConfigPath", root.ConfigPath, filepath.Join(swarmDir, "config.toml")},
		{"TasksPath", root.TasksPath(), filepath.Join(swarmDir, "tasks", "tasks.json")},
		{"TasksLockPath", root.TasksLockPath(), filepath.Join(swarmDir, "tasks", ".lock")},
		{"AgentsPath", root.AgentsPath(), filepath.Join(swarmDir, "agents", "registry.json")},
		{"RunsPath", root.RunsPath(), filepath.Join(swarmDir, "runs", "runs.json")},
		{"WorktreesPath", root.WorktreesPath(), filepath.Join(swarmDir, "worktrees", "worktrees.json")},
		{"EventsPath", root.EventsPath(), filepath.Join(swarmDir, "events", "events.jsonl")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestRootInboxPath(t *testing.T) {
	base := tempDir(t)
	root := mustInitRoot(t, base)

	got := root.InboxPath("alice-123")
	wantPrefix := filepath.Join(root.Dir, "messages", "alice-123", "inbox")
	// InboxPath returns a trailing separator
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("InboxPath got %q, want prefix %q", got, wantPrefix)
	}
}

// ─── InitRoot ────────────────────────────────────────────────────────────────

func TestInitRoot_CreatesDirectoryStructure(t *testing.T) {
	base := tempDir(t)
	mustInitRoot(t, base)

	swarmDir := filepath.Join(base, ".swarm")

	expectedDirs := []string{
		swarmDir,
		filepath.Join(swarmDir, "agents"),
		filepath.Join(swarmDir, "messages"),
		filepath.Join(swarmDir, "tasks"),
		filepath.Join(swarmDir, "runs"),
		filepath.Join(swarmDir, "worktrees"),
		filepath.Join(swarmDir, "events"),
	}

	for _, d := range expectedDirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q exists but is not a directory", d)
		}
	}

	// config.toml must exist (may be empty)
	configPath := filepath.Join(swarmDir, "config.toml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config.toml not created: %v", err)
	}
}

func TestInitRoot_Idempotent(t *testing.T) {
	base := tempDir(t)

	// Call twice — must not error.
	root1 := mustInitRoot(t, base)

	// Write something into the swarm dir to prove it's not wiped.
	marker := filepath.Join(root1.Dir, "tasks", "tasks.json")
	if err := os.WriteFile(marker, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	root2 := mustInitRoot(t, base)
	if root1.Dir != root2.Dir {
		t.Errorf("roots differ: %q vs %q", root1.Dir, root2.Dir)
	}

	// Marker must still be there.
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker gone after second InitRoot: %v", err)
	}
	if string(data) != `{}` {
		t.Errorf("marker contents changed: %q", data)
	}
}

// ─── FindRoot ────────────────────────────────────────────────────────────────

func TestFindRoot_Found(t *testing.T) {
	base := tempDir(t)
	mustInitRoot(t, base)

	// Change into a deeply nested subdirectory — FindRoot must walk up.
	nested := filepath.Join(base, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root, err := swarmfs.FindRoot()
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}

	wantDir := filepath.Join(base, ".swarm")
	if root.Dir != wantDir {
		t.Errorf("Dir got %q, want %q", root.Dir, wantDir)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	// Use a temp dir that has NO .swarm/ and is not below any project dir.
	base := tempDir(t)

	origWd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origWd) }) //nolint:errcheck
	if err := os.Chdir(base); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err := swarmfs.FindRoot()
	if err == nil {
		t.Fatal("FindRoot: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "swarm init") {
		t.Errorf("error message should mention 'swarm init', got: %q", err.Error())
	}
}

// ─── AtomicWrite ─────────────────────────────────────────────────────────────

func TestAtomicWrite_Basic(t *testing.T) {
	base := tempDir(t)
	path := filepath.Join(base, "sub", "data.json")
	data := []byte(`{"hello":"world"}`)

	if err := swarmfs.AtomicWrite(path, data); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestAtomicWrite_Overwrites(t *testing.T) {
	base := tempDir(t)
	path := filepath.Join(base, "data.json")

	if err := swarmfs.AtomicWrite(path, []byte("first")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := swarmfs.AtomicWrite(path, []byte("second")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}

func TestAtomicWrite_NoTempFilesLeft(t *testing.T) {
	base := tempDir(t)
	path := filepath.Join(base, "data.json")

	swarmfs.AtomicWrite(path, []byte("ok")) //nolint:errcheck

	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".swarmfs-tmp-") {
			t.Errorf("temp file leaked: %q", e.Name())
		}
	}
}

// ─── AppendLine ──────────────────────────────────────────────────────────────

func TestAppendLine_CreatesFile(t *testing.T) {
	base := tempDir(t)
	path := filepath.Join(base, "events", "events.jsonl")

	if err := swarmfs.AppendLine(path, []byte(`{"type":"test"}`)); err != nil {
		t.Fatalf("AppendLine: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != `{"type":"test"}`+"\n" {
		t.Errorf("got %q", got)
	}
}

func TestAppendLine_Accumulates(t *testing.T) {
	base := tempDir(t)
	path := filepath.Join(base, "events.jsonl")

	lines := []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}
	for _, l := range lines {
		if err := swarmfs.AppendLine(path, []byte(l)); err != nil {
			t.Fatalf("AppendLine(%q): %v", l, err)
		}
	}

	got, _ := os.ReadFile(path)
	want := strings.Join(lines, "\n") + "\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ─── WithFileLock ─────────────────────────────────────────────────────────────

func TestWithFileLock_Basic(t *testing.T) {
	base := tempDir(t)
	lockPath := filepath.Join(base, ".lock")

	var called bool
	err := swarmfs.WithFileLock(lockPath, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithFileLock: %v", err)
	}
	if !called {
		t.Error("fn was not called")
	}

	// Lock file should exist after the call.
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("lock file not created: %v", err)
	}
}

func TestWithFileLock_PropagatesError(t *testing.T) {
	base := tempDir(t)
	lockPath := filepath.Join(base, ".lock")

	want := "sentinel error"
	err := swarmfs.WithFileLock(lockPath, func() error {
		return fmt.Errorf("%s", want)
	})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("expected error containing %q, got %v", want, err)
	}
}

func TestWithFileLock_ConcurrentAccess(t *testing.T) {
	base := tempDir(t)
	lockPath := filepath.Join(base, ".lock")
	counter := filepath.Join(base, "counter.txt")

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			swarmfs.WithFileLock(lockPath, func() error { //nolint:errcheck
				// Read-increment-write with no other synchronisation.
				// If the lock is broken, the final count will be wrong.
				var n int
				data, err := os.ReadFile(counter)
				if err == nil {
					fmt.Sscanf(string(data), "%d", &n)
				}
				n++
				return os.WriteFile(counter, []byte(fmt.Sprintf("%d", n)), 0o644)
			})
		}()
	}

	wg.Wait()

	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	var got int
	fmt.Sscanf(string(data), "%d", &got)
	if got != goroutines {
		t.Errorf("counter = %d, want %d (lock not exclusive)", got, goroutines)
	}
}

// ─── NewID ───────────────────────────────────────────────────────────────────

func TestNewID_Format(t *testing.T) {
	cases := []string{"task", "msg", "run", "agent"}
	for _, prefix := range cases {
		id := swarmfs.NewID(prefix)
		parts := strings.SplitN(id, "-", 2)
		if len(parts) != 2 {
			t.Errorf("NewID(%q) = %q: expected exactly one dash", prefix, id)
			continue
		}
		if parts[0] != prefix {
			t.Errorf("NewID(%q) prefix = %q, want %q", prefix, parts[0], prefix)
		}
		suffix := parts[1]
		if len(suffix) != 6 {
			t.Errorf("NewID(%q) suffix length = %d, want 6", prefix, len(suffix))
		}
		for _, c := range suffix {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789", c) {
				t.Errorf("NewID(%q) suffix %q contains non-alphanumeric char %q", prefix, suffix, c)
			}
		}
	}
}

func TestNewID_Uniqueness(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := swarmfs.NewID("x")
		if seen[id] {
			t.Fatalf("NewID collision at iteration %d: %q", i, id)
		}
		seen[id] = true
	}
}
