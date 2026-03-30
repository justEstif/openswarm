package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

// newTestRepo creates a temporary git repository with an initial commit and
// initialises a swarmfs root inside it.
func newTestRepo(t *testing.T) (string, *swarmfs.Root) {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	root, err := swarmfs.InitRoot(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	return repoDir, root
}

// skipIfNoGit skips the test if git is not in PATH.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
}

// ─── Unit tests ───────────────────────────────────────────────────────────────

// TestSanitizeBranch verifies that sanitizeBranch converts branch names with
// slashes and other non-alphanumeric characters into a safe directory segment.
func TestSanitizeBranch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"feature/my-work", "feature-my-work"},
		{"bugfix/issue-42", "bugfix-issue-42"},
		{"main", "main"},
		{"release/1.2.3", "release-1-2-3"},
		{"---hello---", "hello"},
		{"a//b//c", "a-b-c"},
	}
	for _, tc := range cases {
		got := sanitizeBranch(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeBranch(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestList_empty verifies that List returns an empty slice (not an error) when
// worktrees.json does not exist.
func TestList_empty(t *testing.T) {
	// We don't need git for this — just a valid swarmfs root.
	tmpDir := t.TempDir()
	root, err := swarmfs.InitRoot(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	wts, err := List(root)
	if err != nil {
		t.Fatalf("List returned unexpected error: %v", err)
	}
	if wts == nil {
		t.Fatal("List returned nil slice, want empty non-nil slice")
	}
	if len(wts) != 0 {
		t.Fatalf("List returned %d items, want 0", len(wts))
	}
}

// ─── Integration tests (require git) ─────────────────────────────────────────

// TestNew creates a worktree and verifies that:
//   - the record appears in List()
//   - the worktree directory exists on disk
//   - the worktree has the correct branch and status
func TestNew(t *testing.T) {
	skipIfNoGit(t)
	_, root := newTestRepo(t)

	const branch = "feature/test-branch"
	wt, err := New(root, branch, "agent-test")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(wt.Path) })

	// Verify returned values.
	if wt.Branch != branch {
		t.Errorf("Branch = %q, want %q", wt.Branch, branch)
	}
	if wt.Status != StatusActive {
		t.Errorf("Status = %q, want %q", wt.Status, StatusActive)
	}
	if wt.AgentID != "agent-test" {
		t.Errorf("AgentID = %q, want %q", wt.AgentID, "agent-test")
	}
	if wt.ID == "" {
		t.Error("ID is empty")
	}

	// Verify path exists on disk.
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Errorf("worktree path %q does not exist on disk", wt.Path)
	}

	// Verify record appears in List().
	list, err := List(root)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	found := false
	for _, w := range list {
		if w.ID == wt.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("worktree %q not found in List()", wt.ID)
	}
}

// TestClean creates a worktree, then cleans it, and verifies that:
//   - the record is removed from List()
//   - the worktree directory no longer exists on disk
func TestClean(t *testing.T) {
	skipIfNoGit(t)
	_, root := newTestRepo(t)

	wt, err := New(root, "feature/clean-me", "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Register cleanup in case the test fails before Clean succeeds.
	t.Cleanup(func() { os.RemoveAll(wt.Path) })

	if err := Clean(root, wt.ID); err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	// Verify path is gone.
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Errorf("worktree path %q still exists after Clean()", wt.Path)
	}

	// Verify record is removed from List().
	list, err := List(root)
	if err != nil {
		t.Fatalf("List() after Clean() error: %v", err)
	}
	for _, w := range list {
		if w.ID == wt.ID {
			t.Errorf("worktree %q still present in List() after Clean()", wt.ID)
		}
	}
}

// TestGet verifies that Get returns the correct worktree and returns
// ErrNotFound for an unknown ID.
func TestGet(t *testing.T) {
	skipIfNoGit(t)
	_, root := newTestRepo(t)

	wt, err := New(root, "feature/get-test", "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() {
		Clean(root, wt.ID) //nolint:errcheck
		os.RemoveAll(wt.Path)
	})

	// Happy path.
	got, err := Get(root, wt.ID)
	if err != nil {
		t.Fatalf("Get(%q) error: %v", wt.ID, err)
	}
	if got.ID != wt.ID {
		t.Errorf("Get returned ID %q, want %q", got.ID, wt.ID)
	}

	// Not found.
	_, err = Get(root, "wt-notexist")
	if err == nil {
		t.Error("Get(unknown id) expected error, got nil")
	}
}
