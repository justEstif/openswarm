package run_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/pane"
	"github.com/justEstif/openswarm/internal/run"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Mock backend ────────────────────────────────────────────────────────────

// mockBackend implements pane.Backend for testing.
type mockBackend struct {
	spawnID    pane.PaneID
	spawnErr   error
	waitCode   int
	waitErr    error
	captureOut string
	captureErr error
	closeErr   error
	closed     []pane.PaneID
	spawned    []string // commands that were spawned
}

func (m *mockBackend) Spawn(name, cmd string, env map[string]string) (pane.PaneID, error) {
	m.spawned = append(m.spawned, cmd)
	return m.spawnID, m.spawnErr
}

func (m *mockBackend) Send(id pane.PaneID, text string) error { return nil }

func (m *mockBackend) Capture(id pane.PaneID) (string, error) {
	return m.captureOut, m.captureErr
}

func (m *mockBackend) Subscribe(ctx context.Context, id pane.PaneID) (<-chan pane.OutputEvent, error) {
	ch := make(chan pane.OutputEvent)
	close(ch)
	return ch, nil
}

func (m *mockBackend) List() ([]pane.PaneInfo, error) { return nil, nil }

func (m *mockBackend) Close(id pane.PaneID) error {
	m.closed = append(m.closed, id)
	return m.closeErr
}

func (m *mockBackend) Wait(id pane.PaneID) (int, error) {
	return m.waitCode, m.waitErr
}

func (m *mockBackend) Name() string { return "mock" }

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestRoot creates a temporary .swarm/ directory and returns a Root.
func newTestRoot(t *testing.T) *swarmfs.Root {
	t.Helper()
	dir := t.TempDir()
	root, err := swarmfs.InitRoot(dir)
	if err != nil {
		t.Fatalf("InitRoot: %v", err)
	}
	return root
}

// newMock returns a backend that spawns pane "p1" and exits with code 0.
func newMock() *mockBackend {
	return &mockBackend{
		spawnID:    "p1",
		waitCode:   0,
		captureOut: "hello world",
	}
}

// ─── Start tests ─────────────────────────────────────────────────────────────

func TestStart_RecordsRun(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	r, err := run.Start(root, b, "my-build", "go build ./...", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// ID should have "run-" prefix.
	if len(r.ID) < 5 || r.ID[:4] != "run-" {
		t.Errorf("expected ID prefix 'run-', got %q", r.ID)
	}

	if r.Name != "my-build" {
		t.Errorf("Name = %q, want 'my-build'", r.Name)
	}
	if r.Cmd != "go build ./..." {
		t.Errorf("Cmd = %q, want 'go build ./...'", r.Cmd)
	}
	if r.Backend != "mock" {
		t.Errorf("Backend = %q, want 'mock'", r.Backend)
	}
	if r.PaneID != "p1" {
		t.Errorf("PaneID = %q, want 'p1'", r.PaneID)
	}
	if r.Status != run.RunStatusRunning {
		t.Errorf("Status = %q, want 'running'", r.Status)
	}
}

func TestStart_CommandWrappedInShell(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	_, err := run.Start(root, b, "test", "echo hello", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(b.spawned) != 1 {
		t.Fatalf("expected 1 spawned command, got %d", len(b.spawned))
	}
	want := "/bin/sh -c 'echo hello'"
	if b.spawned[0] != want {
		t.Errorf("spawned command = %q, want %q", b.spawned[0], want)
	}
}

func TestStart_SpawnError_NoRunPersisted(t *testing.T) {
	root := newTestRoot(t)
	b := &mockBackend{
		spawnErr: errors.New("tmux not running"),
	}

	_, err := run.Start(root, b, "test", "echo hi", nil)
	if err == nil {
		t.Fatal("expected error from Start when Spawn fails")
	}

	// runs.json should not exist or be empty.
	runs, listErr := run.List(root)
	if listErr != nil {
		t.Fatalf("List: %v", listErr)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

// ─── Wait tests ───────────────────────────────────────────────────────────────

func TestWait_SuccessUpdatesStatus(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	r, err := run.Start(root, b, "build", "make", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	r2, err := run.Wait(root, b, r.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if r2.Status != run.RunStatusDone {
		t.Errorf("Status = %q, want 'done'", r2.Status)
	}
	if r2.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r2.ExitCode)
	}
	if r2.Output != "hello world" {
		t.Errorf("Output = %q, want 'hello world'", r2.Output)
	}
	if r2.EndedAt == nil {
		t.Error("EndedAt should be set")
	}
}

func TestWait_NonZeroExitCodeSetsStatusFailed(t *testing.T) {
	root := newTestRoot(t)
	b := &mockBackend{
		spawnID:    "p2",
		waitCode:   1,
		captureOut: "build failed",
	}

	r, err := run.Start(root, b, "build", "make", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	r2, err := run.Wait(root, b, r.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if r2.Status != run.RunStatusFailed {
		t.Errorf("Status = %q, want 'failed'", r2.Status)
	}
	if r2.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r2.ExitCode)
	}
}

func TestWait_ExitCodePropagation(t *testing.T) {
	for _, code := range []int{0, 1, 2, 127} {
		code := code
		t.Run("exit"+string(rune('0'+code)), func(t *testing.T) {
			root := newTestRoot(t)
			b := &mockBackend{spawnID: "p", waitCode: code, captureOut: ""}
			r, _ := run.Start(root, b, "x", "cmd", nil)
			r2, err := run.Wait(root, b, r.ID)
			if err != nil {
				t.Fatalf("Wait: %v", err)
			}
			if r2.ExitCode != code {
				t.Errorf("ExitCode = %d, want %d", r2.ExitCode, code)
			}
		})
	}
}

func TestWait_NotFound(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	_, err := run.Wait(root, b, "run-nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown run ID")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("expected SwarmError, got %T: %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want 'NOT_FOUND'", se.Code)
	}
}

// ─── COMPLETE signal tests ────────────────────────────────────────────────────

func TestWait_CompleteSignalDetected(t *testing.T) {
	root := newTestRoot(t)
	b := &mockBackend{
		spawnID:    "p3",
		waitCode:   0,
		captureOut: "some output\n<promise>COMPLETE</promise>\nmore output",
	}

	r, err := run.Start(root, b, "agent", "run agent", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	r2, err := run.Wait(root, b, r.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if !r2.Complete {
		t.Error("Complete should be true when <promise>COMPLETE</promise> is in output")
	}
}

func TestWait_CompleteSignalAbsent(t *testing.T) {
	root := newTestRoot(t)
	b := &mockBackend{
		spawnID:    "p4",
		waitCode:   0,
		captureOut: "normal output, no signal here",
	}

	r, _ := run.Start(root, b, "agent", "run agent", nil)
	r2, err := run.Wait(root, b, r.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if r2.Complete {
		t.Error("Complete should be false when signal is absent")
	}
}

// ─── List tests ───────────────────────────────────────────────────────────────

func TestList_NewestFirst(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	// Start three runs with small delays to ensure distinct StartedAt.
	var ids []string
	for i := 0; i < 3; i++ {
		r, err := run.Start(root, b, "run", "cmd", nil)
		if err != nil {
			t.Fatalf("Start[%d]: %v", i, err)
		}
		ids = append(ids, r.ID)
		time.Sleep(time.Millisecond)
	}

	runs, err := run.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	// Last started should appear first.
	if runs[0].ID != ids[2] {
		t.Errorf("first run = %q, want %q (newest)", runs[0].ID, ids[2])
	}
	if runs[2].ID != ids[0] {
		t.Errorf("last run = %q, want %q (oldest)", runs[2].ID, ids[0])
	}
}

func TestList_EmptyWhenNoFile(t *testing.T) {
	root := newTestRoot(t)

	runs, err := run.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

// ─── Get tests ────────────────────────────────────────────────────────────────

func TestGet_ReturnsRun(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	r, _ := run.Start(root, b, "fetch", "curl example.com", nil)

	got, err := run.Get(root, r.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("ID = %q, want %q", got.ID, r.ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	root := newTestRoot(t)

	_, err := run.Get(root, "run-does-not-exist")
	if err == nil {
		t.Fatal("expected error")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("expected *output.SwarmError, got %T: %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want 'NOT_FOUND'", se.Code)
	}
}

// ─── Kill tests ───────────────────────────────────────────────────────────────

func TestKill_MarksRunKilled(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	r, err := run.Start(root, b, "long", "sleep 999", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := run.Kill(root, b, r.ID); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	got, err := run.Get(root, r.ID)
	if err != nil {
		t.Fatalf("Get after Kill: %v", err)
	}
	if got.Status != run.RunStatusKilled {
		t.Errorf("Status = %q, want 'killed'", got.Status)
	}
	if got.EndedAt == nil {
		t.Error("EndedAt should be set after Kill")
	}
}

func TestKill_ClosesPane(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	r, _ := run.Start(root, b, "proc", "cmd", nil)
	_ = run.Kill(root, b, r.ID)

	if len(b.closed) != 1 {
		t.Errorf("expected Close called once, got %d times", len(b.closed))
	}
	if string(b.closed[0]) != r.PaneID {
		t.Errorf("Close called with %q, want %q", b.closed[0], r.PaneID)
	}
}

func TestKill_NotFound(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	err := run.Kill(root, b, "run-ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("expected *output.SwarmError, got %T: %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want 'NOT_FOUND'", se.Code)
	}
}

// ─── Logs tests ───────────────────────────────────────────────────────────────

func TestLogs_CompletedRunReturnsStoredOutput(t *testing.T) {
	root := newTestRoot(t)
	b := &mockBackend{spawnID: "p5", waitCode: 0, captureOut: "stored output"}

	r, _ := run.Start(root, b, "x", "cmd", nil)
	_, _ = run.Wait(root, b, r.ID)

	logs, err := run.Logs(root, b, r.ID)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if logs != "stored output" {
		t.Errorf("Logs = %q, want 'stored output'", logs)
	}
}

func TestLogs_RunningRunCallsCapture(t *testing.T) {
	root := newTestRoot(t)
	b := &mockBackend{spawnID: "p6", captureOut: "live output"}

	r, err := run.Start(root, b, "live", "watch logs", nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	logs, err := run.Logs(root, b, r.ID)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if logs != "live output" {
		t.Errorf("Logs = %q, want 'live output'", logs)
	}
}

// ─── JSON serialisation tests ─────────────────────────────────────────────────

func TestRunsSerialisedToJSON(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	r, _ := run.Start(root, b, "build", "go build", nil)
	_, _ = run.Wait(root, b, r.ID)

	// Read the raw file and verify it is valid JSON.
	data, err := os.ReadFile(root.RunsPath())
	if err != nil {
		t.Fatalf("read runs.json: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("runs.json is empty")
	}
	// Must start with '[' (JSON array).
	if data[0] != '[' {
		t.Errorf("runs.json does not start with '[', got %q", string(data[:min(20, len(data))]))
	}
}

func TestLockFileCreated(t *testing.T) {
	root := newTestRoot(t)
	b := newMock()

	_, _ = run.Start(root, b, "x", "cmd", nil)

	lockFile := filepath.Join(filepath.Dir(root.RunsPath()), ".lock")
	if _, err := os.Stat(lockFile); os.IsNotExist(err) {
		t.Error("lock file should be created on first mutation")
	}
}

// min is available as a builtin in Go 1.21+ but defined here for clarity.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
