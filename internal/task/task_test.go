package task_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
	"github.com/justEstif/openswarm/internal/task"
)

func initRoot(t *testing.T) *swarmfs.Root {
	t.Helper()
	root, err := swarmfs.InitRoot(t.TempDir())
	if err != nil {
		t.Fatalf("InitRoot: %v", err)
	}
	return root
}

func TestAdd(t *testing.T) {
	root := initRoot(t)
	tk, err := task.Add(root, "hello world", task.AddOpts{})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if tk.Title != "hello world" {
		t.Errorf("Title = %q; want %q", tk.Title, "hello world")
	}
	if tk.Status != task.StatusTodo {
		t.Errorf("Status = %q; want %q", tk.Status, task.StatusTodo)
	}
	if tk.Priority != task.PriorityNormal {
		t.Errorf("Priority = %q; want %q", tk.Priority, task.PriorityNormal)
	}
	if tk.ETag == "" {
		t.Error("ETag must not be empty")
	}
}

func TestAddEmptyTitle(t *testing.T) {
	root := initRoot(t)
	_, err := task.Add(root, "  ", task.AddOpts{})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestListAndGet(t *testing.T) {
	root := initRoot(t)
	a, _ := task.Add(root, "task A", task.AddOpts{Priority: task.PriorityHigh})
	b, _ := task.Add(root, "task B", task.AddOpts{})

	list, err := task.List(root, task.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len = %d; want 2", len(list))
	}

	got, err := task.Get(root, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != a.ID {
		t.Errorf("Get ID = %q; want %q", got.ID, a.ID)
	}
	_ = b
}

func TestClaimAndDone(t *testing.T) {
	root := initRoot(t)
	tk, _ := task.Add(root, "claimable", task.AddOpts{})

	_, err := task.Claim(root, tk.ID, "alice")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// Claiming again by different agent should fail.
	_, err = task.Claim(root, tk.ID, "bob")
	if err == nil {
		t.Fatal("expected conflict when claiming already-claimed task")
	}

	_, err = task.Done(root, tk.ID, "output text")
	if err != nil {
		t.Fatalf("Done: %v", err)
	}

	got, _ := task.Get(root, tk.ID)
	if got.Status != task.StatusDone {
		t.Errorf("Status = %q; want done", got.Status)
	}
	if got.Output != "output text" {
		t.Errorf("Output = %q; want %q", got.Output, "output text")
	}
}

func TestBlock(t *testing.T) {
	root := initRoot(t)
	a, _ := task.Add(root, "blocker", task.AddOpts{})
	b, _ := task.Add(root, "blocked", task.AddOpts{})

	_, err := task.Block(root, b.ID, a.ID)
	if err != nil {
		t.Fatalf("Block: %v", err)
	}

	// Ready list should exclude b (blocked by a).
	ready, _ := task.List(root, task.ListFilter{Ready: true})
	for _, tk := range ready {
		if tk.ID == b.ID {
			t.Errorf("blocked task %q should not appear in ready list", b.ID)
		}
	}
}

func TestUpdateETag(t *testing.T) {
	root := initRoot(t)
	tk, _ := task.Add(root, "original", task.AddOpts{})

	title := "updated"
	_, err := task.Update(root, tk.ID, task.UpdateOpts{Title: &title}, tk.ETag)
	if err != nil {
		t.Fatalf("Update with valid etag: %v", err)
	}

	// Stale etag should fail.
	_, err = task.Update(root, tk.ID, task.UpdateOpts{Title: &title}, tk.ETag)
	if err == nil {
		t.Fatal("expected conflict with stale etag")
	}
}

func TestCheck(t *testing.T) {
	root := initRoot(t)
	tk, _ := task.Add(root, "needs fixing", task.AddOpts{BlockedBy: []string{"task-nonexistent"}})
	_ = tk

	problems, err := task.Check(root, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(problems) == 0 {
		t.Fatal("expected at least one problem")
	}

	// Fix should remove the bad reference.
	_, err = task.Check(root, true)
	if err != nil {
		t.Fatalf("Check fix: %v", err)
	}
	problems2, _ := task.Check(root, false)
	if len(problems2) != 0 {
		t.Fatalf("after fix, expected 0 problems, got %d", len(problems2))
	}
}

// ─── Additional coverage tests ────────────────────────────────────────────────

// Add with BlockedBy sets the field correctly.
func TestAddWithBlockedBy(t *testing.T) {
	root := initRoot(t)
	blocker, _ := task.Add(root, "blocker", task.AddOpts{})
	blocked, err := task.Add(root, "needs blocker", task.AddOpts{BlockedBy: []string{blocker.ID}})
	if err != nil {
		t.Fatalf("Add with BlockedBy: %v", err)
	}
	if len(blocked.BlockedBy) != 1 || blocked.BlockedBy[0] != blocker.ID {
		t.Errorf("BlockedBy = %v; want [%s]", blocked.BlockedBy, blocker.ID)
	}
}

// List --ready INCLUDES a task whose blocker is done.
func TestReadyIncludesTaskWithDoneBlocker(t *testing.T) {
	root := initRoot(t)
	blocker, _ := task.Add(root, "blocker", task.AddOpts{})
	blocked, _ := task.Add(root, "blocked", task.AddOpts{BlockedBy: []string{blocker.ID}})

	// Before blocker is done, blocked should NOT be in ready list.
	ready, _ := task.List(root, task.ListFilter{Ready: true})
	for _, tk := range ready {
		if tk.ID == blocked.ID {
			t.Error("blocked task should NOT appear in ready list when blocker is open")
		}
	}

	if _, err := task.Done(root, blocker.ID, ""); err != nil {
		t.Fatalf("Done blocker: %v", err)
	}

	// Now blocked should appear in ready list.
	ready2, _ := task.List(root, task.ListFilter{Ready: true})
	found := false
	for _, tk := range ready2 {
		if tk.ID == blocked.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("blocked task SHOULD appear in ready list when blocker is done")
	}
}

// Get returns NOT_FOUND for unknown id.
func TestGetNotFound(t *testing.T) {
	root := initRoot(t)
	_, err := task.Get(root, "task-nonexistent")
	if err == nil {
		t.Fatal("expected NOT_FOUND error for unknown id, got nil")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) || se.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND SwarmError, got: %v", err)
	}
}

// Done is idempotent.
func TestDoneIdempotent(t *testing.T) {
	root := initRoot(t)
	tk, _ := task.Add(root, "idempotent done", task.AddOpts{})
	if _, err := task.Done(root, tk.ID, "output1"); err != nil {
		t.Fatalf("first Done: %v", err)
	}
	if _, err := task.Done(root, tk.ID, "output2"); err != nil {
		t.Fatalf("second Done (idempotent): %v", err)
	}
	got, _ := task.Get(root, tk.ID)
	if got.Status != task.StatusDone {
		t.Errorf("Status = %q; want done", got.Status)
	}
}

// Cancel is idempotent.
func TestCancelIdempotent(t *testing.T) {
	root := initRoot(t)
	tk, _ := task.Add(root, "idempotent cancel", task.AddOpts{})
	if _, err := task.Cancel(root, tk.ID); err != nil {
		t.Fatalf("first Cancel: %v", err)
	}
	if _, err := task.Cancel(root, tk.ID); err != nil {
		t.Fatalf("second Cancel (idempotent): %v", err)
	}
	got, _ := task.Get(root, tk.ID)
	if got.Status != task.StatusCancelled {
		t.Errorf("Status = %q; want cancelled", got.Status)
	}
}

// Block/Unblock modify BlockedBy correctly.
func TestBlockUnblock(t *testing.T) {
	root := initRoot(t)
	blocker, _ := task.Add(root, "blocker", task.AddOpts{})
	blocked, _ := task.Add(root, "blocked", task.AddOpts{})

	if _, err := task.Block(root, blocked.ID, blocker.ID); err != nil {
		t.Fatalf("Block: %v", err)
	}
	got, _ := task.Get(root, blocked.ID)
	if len(got.BlockedBy) != 1 || got.BlockedBy[0] != blocker.ID {
		t.Errorf("BlockedBy after Block = %v; want [%s]", got.BlockedBy, blocker.ID)
	}

	if _, err := task.Unblock(root, blocked.ID, blocker.ID); err != nil {
		t.Fatalf("Unblock: %v", err)
	}
	got2, _ := task.Get(root, blocked.ID)
	if len(got2.BlockedBy) != 0 {
		t.Errorf("BlockedBy after Unblock = %v; want []", got2.BlockedBy)
	}
}

// Prompt returns non-empty markdown string.
func TestPromptNonEmpty(t *testing.T) {
	root := initRoot(t)
	task.Add(root, "a task", task.AddOpts{}) //nolint:errcheck
	text, err := task.Prompt(root)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if strings.TrimSpace(text) == "" {
		t.Error("Prompt returned empty string")
	}
	if !strings.Contains(text, "# Task State") {
		t.Errorf("Prompt output missing header: %q", text)
	}
}

// List --ready excludes draft status.
func TestReadyExcludesDraft(t *testing.T) {
	root := initRoot(t)
	draft, _ := task.Add(root, "draft task", task.AddOpts{Status: task.StatusDraft})
	ready, _ := task.List(root, task.ListFilter{Ready: true})
	for _, tk := range ready {
		if tk.ID == draft.ID {
			t.Error("draft task should NOT appear in ready list")
		}
	}
}

// List --ready excludes deferred priority.
func TestReadyExcludesDeferred(t *testing.T) {
	root := initRoot(t)
	deferred, _ := task.Add(root, "deferred task", task.AddOpts{Priority: task.PriorityDeferred})
	ready, _ := task.List(root, task.ListFilter{Ready: true})
	for _, tk := range ready {
		if tk.ID == deferred.ID {
			t.Error("deferred task should NOT appear in ready list")
		}
	}
}
