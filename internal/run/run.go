// Package run tracks background command executions spawned through a pane.Backend.
//
// Runs are persisted as a JSON array in .swarm/runs/runs.json.
// All mutations acquire an exclusive flock on .swarm/runs/.lock before
// reading and writing, ensuring consistency across concurrent processes.
//
// The <promise>COMPLETE</promise> signal is detected in captured output and
// recorded on the Run as Run.Complete = true.
//
// Every mutating operation emits a corresponding event via the events package.
package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/pane"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// RunStatus represents the lifecycle state of a run.
type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusDone    RunStatus = "done"
	RunStatusFailed  RunStatus = "failed"
	RunStatusKilled  RunStatus = "killed"
)

// Run represents a single background command execution.
type Run struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Cmd       string     `json:"cmd"`               // full command string as passed
	Backend   string     `json:"backend"`           // b.Name()
	PaneID    string     `json:"pane_id"`           // string form of pane.PaneID
	Status    RunStatus  `json:"status"`
	ExitCode  int        `json:"exit_code"`
	Output    string     `json:"output,omitempty"`  // captured via Capture() at exit
	Complete  bool       `json:"complete"`          // true if <promise>COMPLETE</promise> in output
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// completeSignal is the magic string that marks a successful promise.
const completeSignal = "<promise>COMPLETE</promise>"

// ─── Storage helpers ─────────────────────────────────────────────────────────

// lockPath returns the flock path for runs.json.
func lockPath(root *swarmfs.Root) string {
	return filepath.Join(filepath.Dir(root.RunsPath()), ".lock")
}

// readAll reads runs.json and returns all runs.
// Returns an empty slice if the file does not exist.
func readAll(root *swarmfs.Root) ([]*Run, error) {
	data, err := os.ReadFile(root.RunsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []*Run{}, nil
		}
		return nil, fmt.Errorf("run: read: %w", err)
	}
	var runs []*Run
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, fmt.Errorf("run: unmarshal: %w", err)
	}
	if runs == nil {
		runs = []*Run{}
	}
	return runs, nil
}

// writeAll serialises runs and atomically writes runs.json.
func writeAll(root *swarmfs.Root, runs []*Run) error {
	if runs == nil {
		runs = []*Run{}
	}
	data, err := json.Marshal(runs)
	if err != nil {
		return fmt.Errorf("run: marshal: %w", err)
	}
	return swarmfs.AtomicWrite(root.RunsPath(), data)
}

// findIdx returns the index of the run with the given ID, or -1.
func findIdx(runs []*Run, id string) int {
	for i, r := range runs {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// ─── Public API ──────────────────────────────────────────────────────────────

// Start spawns a new pane via b.Spawn, records the run in runs.json, and emits
// a run.started event. name is the human label; cmd is the full command string
// (wrapped as "/bin/sh -c '<cmd>'" before being passed to Spawn); env is passed
// through to Spawn.
func Start(root *swarmfs.Root, b pane.Backend, name, cmd string, env map[string]string) (*Run, error) {
	shellCmd := "/bin/sh -c '" + cmd + "'"
	pid, err := b.Spawn(name, shellCmd, env)
	if err != nil {
		return nil, fmt.Errorf("run: spawn: %w", err)
	}

	run := &Run{
		ID:        swarmfs.NewID("run"),
		Name:      name,
		Cmd:       cmd,
		Backend:   b.Name(),
		PaneID:    string(pid),
		Status:    RunStatusRunning,
		StartedAt: time.Now().UTC(),
	}

	if err := swarmfs.WithFileLock(lockPath(root), func() error {
		runs, err := readAll(root)
		if err != nil {
			return err
		}
		runs = append(runs, run)
		return writeAll(root, runs)
	}); err != nil {
		// Best-effort close the pane since we can't record the run.
		_ = b.Close(pid)
		return nil, fmt.Errorf("run: persist: %w", err)
	}

	if err := events.Append(root, events.TypeRunStarted, "run", run.ID, map[string]string{
		"name":    run.Name,
		"cmd":     run.Cmd,
		"backend": run.Backend,
		"pane_id": run.PaneID,
	}); err != nil {
		// Non-fatal: the run is already persisted.
		_ = err
	}

	return run, nil
}

// Wait blocks until the run's pane exits, captures final output, checks for the
// COMPLETE signal, updates runs.json, and emits run.done or run.failed.
// Returns the updated Run. Returns output.ErrNotFound if the run ID is unknown.
func Wait(root *swarmfs.Root, b pane.Backend, id string) (*Run, error) {
	// Fetch the run to get its PaneID.
	run, err := Get(root, id)
	if err != nil {
		return nil, err
	}

	pid := pane.PaneID(run.PaneID)

	// Block until the pane exits.
	exitCode, err := b.Wait(pid)
	if err != nil {
		return nil, fmt.Errorf("run: wait: %w", err)
	}

	// Capture final output.
	capturedOutput, captureErr := b.Capture(pid)
	if captureErr != nil {
		// Non-fatal: record what we have.
		capturedOutput = ""
	}

	now := time.Now().UTC()

	// Determine final status.
	status := RunStatusDone
	if exitCode != 0 {
		status = RunStatusFailed
	}

	// Check for COMPLETE signal.
	complete := strings.Contains(capturedOutput, completeSignal)

	// Determine event type.
	eventType := events.TypeRunDone
	if exitCode != 0 {
		eventType = events.TypeRunFailed
	}

	// Persist the update under the lock.
	if err := swarmfs.WithFileLock(lockPath(root), func() error {
		runs, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(runs, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("run %q not found", id))
		}
		r := runs[idx]
		r.Status = status
		r.ExitCode = exitCode
		r.Output = capturedOutput
		r.Complete = complete
		r.EndedAt = &now
		return writeAll(root, runs)
	}); err != nil {
		return nil, fmt.Errorf("run: update: %w", err)
	}

	// Re-read to return the freshest copy.
	run, err = Get(root, id)
	if err != nil {
		return nil, err
	}

	_ = events.Append(root, eventType, "run", run.ID, map[string]any{
		"exit_code": exitCode,
		"complete":  complete,
	})

	return run, nil
}

// List returns all runs, newest first.
func List(root *swarmfs.Root) ([]*Run, error) {
	runs, err := readAll(root)
	if err != nil {
		return nil, err
	}
	// Sort newest first (by StartedAt descending).
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs, nil
}

// Get returns a single run by ID.
// Returns output.ErrNotFound if no run with that ID exists.
func Get(root *swarmfs.Root, id string) (*Run, error) {
	runs, err := readAll(root)
	if err != nil {
		return nil, err
	}
	idx := findIdx(runs, id)
	if idx < 0 {
		return nil, output.ErrNotFound(fmt.Sprintf("run %q not found", id))
	}
	// Return a copy so callers cannot mutate the in-memory slice.
	r := *runs[idx]
	return &r, nil
}

// Kill sends Close to the pane, marks the run as killed in runs.json, and emits
// run.failed. Returns output.ErrNotFound if the run ID is unknown.
func Kill(root *swarmfs.Root, b pane.Backend, id string) error {
	run, err := Get(root, id)
	if err != nil {
		return err
	}

	pid := pane.PaneID(run.PaneID)

	// Close the pane — idempotent if already gone.
	if closeErr := b.Close(pid); closeErr != nil {
		// Log but don't abort; we still want to mark the run as killed.
		_ = closeErr
	}

	now := time.Now().UTC()

	if err := swarmfs.WithFileLock(lockPath(root), func() error {
		runs, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(runs, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("run %q not found", id))
		}
		r := runs[idx]
		r.Status = RunStatusKilled
		r.EndedAt = &now
		return writeAll(root, runs)
	}); err != nil {
		return fmt.Errorf("run: kill update: %w", err)
	}

	_ = events.Append(root, events.TypeRunFailed, "run", id, map[string]string{
		"reason": "killed",
	})

	return nil
}

// Logs returns the captured output of a run.
// For completed/failed/killed runs, returns the stored Output field.
// For a still-running run, calls b.Capture() live.
// Returns output.ErrNotFound if the run ID is unknown.
func Logs(root *swarmfs.Root, b pane.Backend, id string) (string, error) {
	run, err := Get(root, id)
	if err != nil {
		return "", err
	}

	if run.Status != RunStatusRunning {
		return run.Output, nil
	}

	// Live capture for running panes.
	pid := pane.PaneID(run.PaneID)
	text, err := b.Capture(pid)
	if err != nil {
		return "", fmt.Errorf("run: capture: %w", err)
	}
	return text, nil
}
