package events_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newRoot creates a temporary .swarm/ tree and returns a *swarmfs.Root.
func newRoot(t *testing.T) *swarmfs.Root {
	t.Helper()
	tmp := t.TempDir()
	root, err := swarmfs.InitRoot(tmp)
	if err != nil {
		t.Fatalf("InitRoot: %v", err)
	}
	return root
}

// collect reads up to n events from ch within timeout, then cancels and returns
// whatever was collected.
func collect(t *testing.T, ch <-chan events.Event, n int, timeout time.Duration) []events.Event {
	t.Helper()
	var got []events.Event
	deadline := time.After(timeout)
	for len(got) < n {
		select {
		case evt, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, evt)
		case <-deadline:
			t.Logf("collect: timed out after %v (got %d/%d)", timeout, len(got), n)
			return got
		}
	}
	return got
}

// ─── Append tests ────────────────────────────────────────────────────────────

func TestAppend_WritesValidJSONLine(t *testing.T) {
	root := newRoot(t)

	if err := events.Append(root, events.TypeTaskCreated, "task", "task-abc", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}

	data, err := os.ReadFile(root.EventsPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}

	var evt events.Event
	if err := json.Unmarshal([]byte(lines[0]), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if evt.ID == "" {
		t.Error("ID must not be empty")
	}
	if !strings.HasPrefix(evt.ID, "evt-") {
		t.Errorf("ID should have prefix 'evt-', got %q", evt.ID)
	}
	if evt.Type != events.TypeTaskCreated {
		t.Errorf("Type: want %q, got %q", events.TypeTaskCreated, evt.Type)
	}
	if evt.Source != "task" {
		t.Errorf("Source: want %q, got %q", "task", evt.Source)
	}
	if evt.Ref != "task-abc" {
		t.Errorf("Ref: want %q, got %q", "task-abc", evt.Ref)
	}
	if evt.Data != nil {
		t.Errorf("Data: want nil, got %s", evt.Data)
	}
	if evt.At.IsZero() {
		t.Error("At must not be zero")
	}
}

func TestAppend_WithData(t *testing.T) {
	root := newRoot(t)

	payload := map[string]string{"agent": "worker-1"}
	if err := events.Append(root, events.TypeTaskAssigned, "task", "task-xyz", payload); err != nil {
		t.Fatalf("Append: %v", err)
	}

	data, err := os.ReadFile(root.EventsPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var evt events.Event
	if err := json.Unmarshal(bytes(t, string(data)), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if evt.Data == nil {
		t.Fatal("Data must not be nil when payload is provided")
	}

	var decoded map[string]string
	if err := json.Unmarshal(evt.Data, &decoded); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if decoded["agent"] != "worker-1" {
		t.Errorf("Data: want agent=worker-1, got %v", decoded)
	}
}

func TestAppend_MultipleLines(t *testing.T) {
	root := newRoot(t)

	types := []string{
		events.TypeTaskCreated,
		events.TypeTaskAssigned,
		events.TypeTaskDone,
	}
	for _, ty := range types {
		if err := events.Append(root, ty, "task", "task-1", nil); err != nil {
			t.Fatalf("Append %q: %v", ty, err)
		}
	}

	data, err := os.ReadFile(root.EventsPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var evt events.Event
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Errorf("line %d: unmarshal: %v", i, err)
		}
		if evt.Type != types[i] {
			t.Errorf("line %d: want type %q, got %q", i, types[i], evt.Type)
		}
	}
}

func TestAppend_IDsAreUnique(t *testing.T) {
	root := newRoot(t)

	const n = 20
	for i := 0; i < n; i++ {
		if err := events.Append(root, events.TypeMsgSent, "msg", "msg-1", nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	data, err := os.ReadFile(root.EventsPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	ids := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		var evt events.Event
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, dup := ids[evt.ID]; dup {
			t.Errorf("duplicate ID: %s", evt.ID)
		}
		ids[evt.ID] = struct{}{}
	}
}

// ─── Tail tests ───────────────────────────────────────────────────────────────

func TestTail_ReadsExistingEvents(t *testing.T) {
	root := newRoot(t)

	// Write events before starting Tail.
	for _, ty := range []string{events.TypeRunStarted, events.TypeRunDone} {
		if err := events.Append(root, ty, "run", "run-1", nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := events.Tail(ctx, root, "")
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	got := collect(t, ch, 2, 2*time.Second)
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if got[0].Type != events.TypeRunStarted {
		t.Errorf("[0] want %q, got %q", events.TypeRunStarted, got[0].Type)
	}
	if got[1].Type != events.TypeRunDone {
		t.Errorf("[1] want %q, got %q", events.TypeRunDone, got[1].Type)
	}
}

func TestTail_StreamsNewEvents(t *testing.T) {
	root := newRoot(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := events.Tail(ctx, root, "")
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	// Give the goroutine a moment to start reading.
	time.Sleep(50 * time.Millisecond)

	// Append after Tail has started.
	if err := events.Append(root, events.TypeWorktreeCreated, "worktree", "wt-1", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got := collect(t, ch, 1, 2*time.Second)
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d", len(got))
	}
	if got[0].Type != events.TypeWorktreeCreated {
		t.Errorf("want %q, got %q", events.TypeWorktreeCreated, got[0].Type)
	}
}

func TestTail_ExistingAndNew(t *testing.T) {
	root := newRoot(t)

	// Write one event before Tail starts.
	if err := events.Append(root, events.TypeAgentRegistered, "agent", "agent-1", nil); err != nil {
		t.Fatalf("pre-Append: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := events.Tail(ctx, root, "")
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	// Collect the existing event first.
	pre := collect(t, ch, 1, 2*time.Second)
	if len(pre) != 1 || pre[0].Type != events.TypeAgentRegistered {
		t.Fatalf("existing event: want %q, got %v", events.TypeAgentRegistered, pre)
	}

	// Now append a new one.
	if err := events.Append(root, events.TypeAgentDeregistered, "agent", "agent-1", nil); err != nil {
		t.Fatalf("post-Append: %v", err)
	}

	post := collect(t, ch, 1, 2*time.Second)
	if len(post) != 1 || post[0].Type != events.TypeAgentDeregistered {
		t.Fatalf("new event: want %q, got %v", events.TypeAgentDeregistered, post)
	}
}

func TestTail_Filter(t *testing.T) {
	root := newRoot(t)

	if err := events.Append(root, events.TypeTaskCreated, "task", "task-1", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := events.Append(root, events.TypeMsgSent, "msg", "msg-1", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := events.Append(root, events.TypeTaskDone, "task", "task-1", nil); err != nil {
		t.Fatalf("Append: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Only stream "task.*" events.
	ch, err := events.Tail(ctx, root, "task")
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	got := collect(t, ch, 2, 2*time.Second)
	if len(got) != 2 {
		t.Fatalf("want 2 task events, got %d", len(got))
	}
	for _, evt := range got {
		if !strings.Contains(evt.Type, "task") {
			t.Errorf("filter leak: got %q", evt.Type)
		}
	}
}

func TestTail_ChannelClosedOnCancel(t *testing.T) {
	root := newRoot(t)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := events.Tail(ctx, root, "")
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}

	cancel()

	// Channel should close promptly.
	select {
	case _, ok := <-ch:
		if ok {
			// Drain any buffered events.
			for range ch {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after ctx cancel")
	}
}

func TestTail_EmptyFile(t *testing.T) {
	root := newRoot(t)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := events.Tail(ctx, root, "")
	if err != nil {
		t.Fatalf("Tail on empty log: %v", err)
	}

	// Append something small after a short delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = events.Append(root, events.TypePaneCreated, "pane", "pane-1", nil)
	}()

	got := collect(t, ch, 1, 2*time.Second)
	cancel()

	if len(got) != 1 {
		t.Fatalf("want 1 event from empty-start Tail, got %d", len(got))
	}
	if got[0].Type != events.TypePaneCreated {
		t.Errorf("want %q, got %q", events.TypePaneCreated, got[0].Type)
	}
}

// ─── Constants coverage ───────────────────────────────────────────────────────

func TestEventTypeConstants(t *testing.T) {
	// Verify all constants are non-empty and contain a dot (source.action format).
	consts := []string{
		events.TypeAgentRegistered,
		events.TypeAgentDeregistered,
		events.TypeTaskCreated,
		events.TypeTaskAssigned,
		events.TypeTaskClaimed,
		events.TypeTaskDone,
		events.TypeTaskFailed,
		events.TypeTaskCancelled,
		events.TypeTaskBlocked,
		events.TypeTaskUnblocked,
		events.TypeTaskUpdated,
		events.TypeMsgSent,
		events.TypeMsgRead,
		events.TypePaneCreated,
		events.TypePaneExited,
		events.TypePaneKilled,
		events.TypeRunStarted,
		events.TypeRunDone,
		events.TypeRunFailed,
		events.TypeWorktreeCreated,
		events.TypeWorktreeMerged,
		events.TypeWorktreeCleaned,
	}

	if len(consts) != 22 {
		t.Errorf("expected 22 constants, got %d", len(consts))
	}

	seen := make(map[string]bool)
	for _, c := range consts {
		if c == "" {
			t.Error("found empty constant")
		}
		if !strings.Contains(c, ".") {
			t.Errorf("constant %q missing dot separator", c)
		}
		if seen[c] {
			t.Errorf("duplicate constant value %q", c)
		}
		seen[c] = true
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// bytes trims whitespace and returns the byte slice of s.
func bytes(_ *testing.T, s string) []byte {
	return []byte(strings.TrimSpace(s))
}

// Ensure EventsPath is inside the temp dir (smoke-test path construction).
func TestEventsPath(t *testing.T) {
	tmp := t.TempDir()
	root, err := swarmfs.InitRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}
	p := root.EventsPath()
	if !strings.HasPrefix(p, tmp) {
		t.Errorf("EventsPath %q should be under %q", p, tmp)
	}
	if filepath.Base(p) != "events.jsonl" {
		t.Errorf("EventsPath base should be events.jsonl, got %q", filepath.Base(p))
	}
}
