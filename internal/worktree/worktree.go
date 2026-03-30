// Package worktree manages git worktrees for the openswarm project.
//
// Worktrees are persisted as a JSON array in .swarm/worktrees/worktrees.json.
// All mutations acquire an exclusive flock on .swarm/worktrees/.lock before
// reading and writing, ensuring consistency across concurrent processes.
//
// Worktrees are created as siblings to the project directory (git forbids
// nested worktrees): /parent/<projectname>-<sanitized-branch>.
//
// Every mutating operation emits a corresponding event via the events package.
package worktree

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Status represents the lifecycle state of a worktree.
type Status string

const (
	StatusActive    Status = "active"
	StatusMerged    Status = "merged"
	StatusAbandoned Status = "abandoned"
)

// Worktree represents a git worktree created for a branch.
type Worktree struct {
	ID        string    `json:"id"`
	Branch    string    `json:"branch"`
	Path      string    `json:"path"`              // absolute path to worktree directory
	AgentID   string    `json:"agent_id,omitempty"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// MergeOpts controls how a worktree branch is merged.
type MergeOpts struct {
	Squash       bool // use --squash instead of --no-ff
	DeleteBranch bool // delete local branch after merge
}

// ─── Path helpers ─────────────────────────────────────────────────────────────

// sanitizeRe matches any sequence of characters that are not alphanumeric.
var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// sanitizeBranch replaces / and other non-alphanumeric chars with -.
func sanitizeBranch(b string) string {
	return strings.Trim(sanitizeRe.ReplaceAllString(b, "-"), "-")
}

// worktreePath computes the canonical sibling path for a worktree.
// Worktrees must live outside the project directory because git forbids
// nested worktrees.
func worktreePath(root *swarmfs.Root, branch string) string {
	projectRoot := filepath.Dir(root.Dir)      // e.g. /home/user/projects/openswarm
	projectName := filepath.Base(projectRoot)  // e.g. openswarm
	parentDir := filepath.Dir(projectRoot)     // e.g. /home/user/projects
	return filepath.Join(parentDir, projectName+"-"+sanitizeBranch(branch))
}

// projectRoot returns the absolute path of the project directory
// (the parent of the .swarm/ directory).
func projectRoot(root *swarmfs.Root) string {
	return filepath.Dir(root.Dir)
}

// ─── Storage helpers ─────────────────────────────────────────────────────────

// lockPath returns the flock file path for the worktrees store.
func lockPath(root *swarmfs.Root) string {
	return filepath.Join(filepath.Dir(root.WorktreesPath()), ".lock")
}

// readAll reads the worktrees JSON file and returns all worktrees.
// Returns an empty (non-nil) slice if the file does not exist.
func readAll(root *swarmfs.Root) ([]*Worktree, error) {
	data, err := os.ReadFile(root.WorktreesPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []*Worktree{}, nil
		}
		return nil, fmt.Errorf("worktree: read store: %w", err)
	}

	var wts []*Worktree
	if err := json.Unmarshal(data, &wts); err != nil {
		return nil, fmt.Errorf("worktree: unmarshal store: %w", err)
	}
	if wts == nil {
		wts = []*Worktree{}
	}
	return wts, nil
}

// writeAll serialises worktrees and atomically writes the store file.
// An empty slice is written as "[]" (not "null").
func writeAll(root *swarmfs.Root, wts []*Worktree) error {
	if wts == nil {
		wts = []*Worktree{}
	}
	data, err := json.Marshal(wts)
	if err != nil {
		return fmt.Errorf("worktree: marshal store: %w", err)
	}
	if err := swarmfs.AtomicWrite(root.WorktreesPath(), data); err != nil {
		return fmt.Errorf("worktree: write store: %w", err)
	}
	return nil
}

// ─── Git helper ───────────────────────────────────────────────────────────────

// runGit executes a git command in dir with the given arguments.
// It returns combined stdout+stderr output on success, or a wrapped error
// containing the output on failure.
func runGit(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return out, nil
}

// ─── Public API ───────────────────────────────────────────────────────────────

// New creates a new git worktree for the given branch, records it in
// worktrees.json, and emits a TypeWorktreeCreated event.
//
// The worktree is created as a sibling of the project directory (git forbids
// nested worktrees). If the branch already exists, the worktree is checked
// out at the existing branch instead of creating a new one.
func New(root *swarmfs.Root, branch, agentID string) (*Worktree, error) {
	projRoot := projectRoot(root)
	path := worktreePath(root, branch)

	// Try to create the worktree with a new branch first.
	_, err := runGit(projRoot, "worktree", "add", path, "-b", branch)
	if err != nil {
		// If the branch already exists, fall back to checking it out.
		if strings.Contains(err.Error(), "already exists") {
			if _, err2 := runGit(projRoot, "worktree", "add", path, branch); err2 != nil {
				return nil, fmt.Errorf("worktree: add (existing branch): %w", err2)
			}
		} else {
			return nil, fmt.Errorf("worktree: add: %w", err)
		}
	}

	wt := &Worktree{
		ID:        swarmfs.NewID("wt"),
		Branch:    branch,
		Path:      path,
		AgentID:   agentID,
		Status:    StatusActive,
		CreatedAt: time.Now().UTC(),
	}

	if err := swarmfs.WithFileLock(lockPath(root), func() error {
		wts, err := readAll(root)
		if err != nil {
			return err
		}
		wts = append(wts, wt)
		return writeAll(root, wts)
	}); err != nil {
		return nil, err
	}

	if err := events.Append(root, events.TypeWorktreeCreated, "worktree", wt.ID, wt); err != nil {
		return wt, fmt.Errorf("worktree: event append: %w", err)
	}

	return wt, nil
}

// List returns all worktrees, sorted newest-first (by CreatedAt descending).
// Returns an empty slice (not an error) if worktrees.json does not exist.
func List(root *swarmfs.Root) ([]*Worktree, error) {
	wts, err := readAll(root)
	if err != nil {
		return nil, err
	}

	sort.Slice(wts, func(i, j int) bool {
		return wts[i].CreatedAt.After(wts[j].CreatedAt)
	})

	return wts, nil
}

// Get returns the worktree with the given ID.
// Returns output.ErrNotFound if no matching worktree is found.
func Get(root *swarmfs.Root, id string) (*Worktree, error) {
	wts, err := readAll(root)
	if err != nil {
		return nil, err
	}

	for _, wt := range wts {
		if wt.ID == id {
			return wt, nil
		}
	}

	return nil, output.ErrNotFound(fmt.Sprintf("worktree %q not found", id))
}

// Merge merges the worktree's branch into the current branch of the project,
// marks the worktree as merged in worktrees.json, and emits a
// TypeWorktreeMerged event.
func Merge(root *swarmfs.Root, id string, opts MergeOpts) error {
	wt, err := Get(root, id)
	if err != nil {
		return err
	}

	projRoot := projectRoot(root)

	if opts.Squash {
		if _, err := runGit(projRoot, "merge", "--squash", wt.Branch); err != nil {
			return fmt.Errorf("worktree: merge --squash: %w", err)
		}
		if _, err := runGit(projRoot, "commit", "-m", "Squash merge "+wt.Branch); err != nil {
			return fmt.Errorf("worktree: commit after squash: %w", err)
		}
	} else {
		if _, err := runGit(projRoot, "merge", "--no-ff", wt.Branch, "--no-edit"); err != nil {
			return fmt.Errorf("worktree: merge --no-ff: %w", err)
		}
	}

	if opts.DeleteBranch {
		if _, err := runGit(projRoot, "branch", "-D", wt.Branch); err != nil {
			return fmt.Errorf("worktree: delete branch: %w", err)
		}
	}

	if err := swarmfs.WithFileLock(lockPath(root), func() error {
		wts, err := readAll(root)
		if err != nil {
			return err
		}
		for _, w := range wts {
			if w.ID == id {
				w.Status = StatusMerged
				break
			}
		}
		return writeAll(root, wts)
	}); err != nil {
		return err
	}

	return events.Append(root, events.TypeWorktreeMerged, "worktree", wt.ID, wt)
}

// Clean removes the git worktree from disk, prunes the git worktree list,
// removes the record from worktrees.json, and emits a TypeWorktreeCleaned event.
// It is idempotent: if the worktree path no longer exists the git remove is
// skipped without error.
func Clean(root *swarmfs.Root, id string) error {
	wt, err := Get(root, id)
	if err != nil {
		return err
	}

	projRoot := projectRoot(root)

	// Remove the worktree directory. Ignore errors if the path is already gone.
	if _, statErr := os.Stat(wt.Path); statErr == nil {
		if _, err := runGit(projRoot, "worktree", "remove", wt.Path, "--force"); err != nil {
			// Only ignore if the path is truly gone now; otherwise propagate.
			if _, statErr2 := os.Stat(wt.Path); statErr2 == nil {
				return fmt.Errorf("worktree: remove: %w", err)
			}
		}
	}

	// Prune stale worktree references regardless.
	if _, err := runGit(projRoot, "worktree", "prune"); err != nil {
		return fmt.Errorf("worktree: prune: %w", err)
	}

	if err := swarmfs.WithFileLock(lockPath(root), func() error {
		wts, err := readAll(root)
		if err != nil {
			return err
		}
		filtered := make([]*Worktree, 0, len(wts))
		for _, w := range wts {
			if w.ID != id {
				filtered = append(filtered, w)
			}
		}
		return writeAll(root, filtered)
	}); err != nil {
		return err
	}

	return events.Append(root, events.TypeWorktreeCleaned, "worktree", wt.ID, wt)
}

// CleanAll cleans all worktrees whose status is Merged or Abandoned.
// It returns the list of worktrees that were cleaned.
func CleanAll(root *swarmfs.Root) ([]*Worktree, error) {
	wts, err := List(root)
	if err != nil {
		return nil, err
	}

	var cleaned []*Worktree
	for _, wt := range wts {
		if wt.Status != StatusMerged && wt.Status != StatusAbandoned {
			continue
		}
		if err := Clean(root, wt.ID); err != nil {
			return cleaned, fmt.Errorf("worktree: clean %s: %w", wt.ID, err)
		}
		cleaned = append(cleaned, wt)
	}

	return cleaned, nil
}
