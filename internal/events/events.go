// Package events provides an append-only event log for openswarm.
//
// Events are persisted as newline-delimited JSON (JSONL) in .swarm/events/events.jsonl.
// Callers use Append to record events and Tail to stream them (with optional filtering).
package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Event is a single entry in the event log.
type Event struct {
	ID     string          `json:"id"`
	Source string          `json:"source"` // "task"|"msg"|"pane"|"run"|"worktree"|"agent"
	Type   string          `json:"type"`
	Ref    string          `json:"ref"`            // ID of the affected resource
	Data   json.RawMessage `json:"data,omitempty"` // optional payload
	At     time.Time       `json:"at"`
}

// ─── Event type constants ─────────────────────────────────────────────────────

const (
	TypeAgentRegistered   = "agent.registered"
	TypeAgentDeregistered = "agent.deregistered"

	TypeTaskCreated   = "task.created"
	TypeTaskAssigned  = "task.assigned"
	TypeTaskClaimed   = "task.claimed"
	TypeTaskDone      = "task.done"
	TypeTaskFailed    = "task.failed"
	TypeTaskCancelled = "task.cancelled"
	TypeTaskBlocked   = "task.blocked"
	TypeTaskUnblocked = "task.unblocked"
	TypeTaskUpdated   = "task.updated"

	TypeMsgSent = "msg.sent"
	TypeMsgRead = "msg.read"

	TypePaneCreated = "pane.created"
	TypePaneExited  = "pane.exited"
	TypePaneKilled  = "pane.killed"

	TypeRunStarted = "run.started"
	TypeRunDone    = "run.done"
	TypeRunFailed  = "run.failed"

	TypeWorktreeCreated = "worktree.created"
	TypeWorktreeMerged  = "worktree.merged"
	TypeWorktreeCleaned = "worktree.cleaned"
)

// ─── Append ───────────────────────────────────────────────────────────────────

// Append writes one Event as a JSON line to events.jsonl.
// It generates an ID via swarmfs.NewID("evt") and sets At to time.Now().
// data is marshalled to json.RawMessage; pass nil for no payload.
func Append(root *swarmfs.Root, eventType, source, ref string, data any) error {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("events: marshal data: %w", err)
		}
		raw = json.RawMessage(b)
	}

	evt := Event{
		ID:     swarmfs.NewID("evt"),
		Source: source,
		Type:   eventType,
		Ref:    ref,
		Data:   raw,
		At:     time.Now().UTC(),
	}

	line, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("events: marshal event: %w", err)
	}

	if err := swarmfs.AppendLine(root.EventsPath(), line); err != nil {
		return fmt.Errorf("events: append line: %w", err)
	}
	return nil
}

// ─── Tail ─────────────────────────────────────────────────────────────────────

const defaultPollInterval = 200 * time.Millisecond

// Tail streams events from events.jsonl.
// If filter is non-empty, only events whose Type contains filter are emitted.
// It reads all existing lines first, then polls for new lines at the given
// interval (0 uses the 200 ms default).
// The returned channel is closed when ctx is cancelled.
func Tail(ctx context.Context, root *swarmfs.Root, filter string, interval ...time.Duration) (<-chan Event, error) {
	pollInterval := defaultPollInterval
	if len(interval) > 0 && interval[0] > 0 {
		pollInterval = interval[0]
	}
	path := root.EventsPath()

	if err := touchFile(path); err != nil {
		return nil, fmt.Errorf("events: tail open %q: %w", path, err)
	}

	ch := make(chan Event, 64)

	go func() {
		defer close(ch)

		// pos tracks how many bytes of the file we have consumed so far.
		// We re-open from pos on each poll so we never mis-parse partial lines.
		var pos int64

		for {
			n, err := drainFrom(path, pos, filter, ch, ctx)
			pos += n
			if err != nil {
				// drainFrom only returns an error on a hard I/O failure or
				// when ctx is done — either way, stop.
				return
			}

			// Reached EOF — wait before polling again.
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollInterval):
			}
		}
	}()

	return ch, nil
}

// Last reads all matching events from the log once (no follow) and returns
// the last n. If n <= 0 all events are returned.
func Last(root *swarmfs.Root, filter string, n int) ([]Event, error) {
	path := root.EventsPath()
	if err := touchFile(path); err != nil {
		return nil, fmt.Errorf("events: last: %w", err)
	}

	ch := make(chan Event, 512)
	ctx := context.Background()
	if _, err := drainFrom(path, 0, filter, ch, ctx); err != nil {
		return nil, fmt.Errorf("events: last drain: %w", err)
	}
	close(ch)

	var all []Event
	for e := range ch {
		all = append(all, e)
	}
	if n > 0 && len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// drainFrom reads complete JSON lines from path starting at byteOffset.
// It sends matching events to ch and returns the number of bytes consumed.
// Returns an error only on hard I/O failure or ctx cancellation.
func drainFrom(path string, offset int64, filter string, ch chan<- Event, ctx context.Context) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("events: open %q: %w", path, err)
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return 0, fmt.Errorf("events: seek: %w", err)
		}
	}

	var consumed int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// scanner.Bytes() is the line without the newline.
		// We account for the '\n' that AppendLine added.
		consumed += int64(len(scanner.Bytes())) + 1

		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		var evt Event
		if err := json.Unmarshal([]byte(text), &evt); err != nil {
			// Skip malformed lines.
			continue
		}

		if filter != "" && !strings.Contains(evt.Type, filter) {
			continue
		}

		select {
		case ch <- evt:
		case <-ctx.Done():
			return consumed, ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		return consumed, fmt.Errorf("events: scan: %w", err)
	}

	return consumed, nil
}

// touchFile ensures path (and its parent directory) exists.
// It uses O_WRONLY|O_CREATE so it works on all platforms.
func touchFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
