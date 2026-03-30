// Package task manages the openswarm task subsystem.
//
// Tasks are persisted as a JSON array in .swarm/tasks/tasks.json.
// All mutations acquire an exclusive flock on .swarm/tasks/.lock before
// reading and writing, ensuring consistency across concurrent processes.
//
// ETag-based optimistic locking is supported via the ifMatch parameter on
// Update. Blocked status is derived at read time from blocked_by references.
//
// Every mutating operation emits a corresponding event via the events package.
package task

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Status represents the lifecycle state of a task.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// terminalStatuses are statuses that cannot be transitioned out of.
var terminalStatuses = map[Status]bool{
	StatusDone:      true,
	StatusFailed:    true,
	StatusCancelled: true,
}

// Priority represents the urgency of a task.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityNormal   Priority = "normal"
	PriorityLow      Priority = "low"
	PriorityDeferred Priority = "deferred"
)

// priorityOrder maps priorities to sort weights (lower = higher priority).
var priorityOrder = map[Priority]int{
	PriorityCritical: 0,
	PriorityHigh:     1,
	PriorityNormal:   2,
	PriorityLow:      3,
	PriorityDeferred: 4,
}

// Task represents a unit of work in the swarm.
type Task struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Status     Status    `json:"status"`
	Priority   Priority  `json:"priority"`
	Tags       []string  `json:"tags,omitempty"`
	AssignedTo string    `json:"assigned_to,omitempty"`
	BlockedBy  []string  `json:"blocked_by,omitempty"`
	Output     string    `json:"output,omitempty"`
	Notes      string    `json:"notes,omitempty"`
	ETag       string    `json:"etag"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Problem describes an integrity issue found by Check.
type Problem struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s", p.TaskID, p.Message)
}

// ListFilter constrains the results returned by List.
type ListFilter struct {
	Status        []Status
	ExcludeStatus []Status
	AssignedTo    string
	Tags          []string
	Ready         bool   // not blocked + not terminal + not in-progress + not deferred
	SortBy        string // "priority" | "created" | "updated" | "status"
}

// AddOpts holds optional fields for Add.
type AddOpts struct {
	Status     Status
	Priority   Priority
	Tags       []string
	AssignedTo string
	BlockedBy  []string
	Notes      string
}

// UpdateOpts holds optional fields for Update.
// Pointer fields: nil means "leave unchanged".
type UpdateOpts struct {
	Title        *string
	Status       *Status
	Priority     *Priority
	Tags         []string // nil = leave unchanged; non-nil (even empty) replaces
	AssignedTo   *string
	Notes        *string
	Output       *string
	AppendOutput bool
	BlockedBy    []string // nil = leave unchanged; non-nil replaces
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// readAll reads tasks.json and returns all tasks.
// Returns an empty slice if the file does not exist.
func readAll(root *swarmfs.Root) ([]Task, error) {
	data, err := os.ReadFile(root.TasksPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Task{}, nil
		}
		return nil, fmt.Errorf("task: read: %w", err)
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("task: unmarshal: %w", err)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

// writeAll serialises tasks and atomically writes tasks.json.
func writeAll(root *swarmfs.Root, tasks []Task) error {
	if tasks == nil {
		tasks = []Task{}
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("task: marshal: %w", err)
	}
	return swarmfs.AtomicWrite(root.TasksPath(), data)
}

// computeETag returns a stable hash of the mutable task fields.
func computeETag(t *Task) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s|%v|%v",
		t.ID, t.Title, t.Status, t.Priority,
		strings.Join(t.Tags, ","),
		t.AssignedTo,
		strings.Join(t.BlockedBy, ","),
		t.Output, t.Notes, t.UpdatedAt.UnixNano())
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// findIdx returns the index of the task with the given ID, or -1.
func findIdx(tasks []Task, id string) int {
	for i := range tasks {
		if tasks[i].ID == id {
			return i
		}
	}
	return -1
}

// idSet returns a set of all task IDs.
func idSet(tasks []Task) map[string]bool {
	s := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		s[t.ID] = true
	}
	return s
}

// ─── Public API ──────────────────────────────────────────────────────────────

// Add creates a new task with the given title and options.
// Emits a task.created event on success.
func Add(root *swarmfs.Root, title string, opts AddOpts) (*Task, error) {
	if strings.TrimSpace(title) == "" {
		return nil, output.ErrValidation("task title must not be empty")
	}

	status := opts.Status
	if status == "" {
		status = StatusTodo
	}
	priority := opts.Priority
	if priority == "" {
		priority = PriorityNormal
	}

	var created Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		created = Task{
			ID:         swarmfs.NewID("task"),
			Title:      title,
			Status:     status,
			Priority:   priority,
			Tags:       opts.Tags,
			AssignedTo: opts.AssignedTo,
			BlockedBy:  opts.BlockedBy,
			Notes:      opts.Notes,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		created.ETag = computeETag(&created)

		tasks = append(tasks, created)
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}

	_ = events.Append(root, events.TypeTaskCreated, "task", created.ID, map[string]string{
		"title":  created.Title,
		"status": string(created.Status),
	})
	return &created, nil
}

// List returns tasks matching the filter, sorted as requested.
func List(root *swarmfs.Root, f ListFilter) ([]*Task, error) {
	tasks, err := readAll(root)
	if err != nil {
		return nil, err
	}

	statusMap := idSet(tasks)
	_ = statusMap // used below via closure

	// Build blockedSet: task IDs that are blocked by an open (non-terminal) task.
	blockedSet := make(map[string]bool)
	openByID := make(map[string]bool)
	for _, t := range tasks {
		if !terminalStatuses[t.Status] {
			openByID[t.ID] = true
		}
	}
	for _, t := range tasks {
		for _, bid := range t.BlockedBy {
			if openByID[bid] {
				blockedSet[t.ID] = true
			}
		}
	}

	// Filter.
	var result []*Task
	for i := range tasks {
		t := &tasks[i]

		if f.Ready {
			if terminalStatuses[t.Status] {
				continue
			}
			if t.Status == StatusDraft {
				continue
			}
			if t.Status == StatusInProgress {
				continue
			}
			if t.Priority == PriorityDeferred {
				continue
			}
			if blockedSet[t.ID] {
				continue
			}
		}

		if len(f.Status) > 0 {
			match := false
			for _, s := range f.Status {
				if t.Status == s {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		if len(f.ExcludeStatus) > 0 {
			skip := false
			for _, s := range f.ExcludeStatus {
				if t.Status == s {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		if f.AssignedTo != "" && t.AssignedTo != f.AssignedTo {
			continue
		}

		if len(f.Tags) > 0 {
			tagSet := make(map[string]bool, len(t.Tags))
			for _, tg := range t.Tags {
				tagSet[tg] = true
			}
			match := false
			for _, tg := range f.Tags {
				if tagSet[tg] {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		tc := *t
		result = append(result, &tc)
	}

	// Sort.
	sortBy := f.SortBy
	if sortBy == "" {
		sortBy = "created"
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		switch sortBy {
		case "priority":
			pa := priorityOrder[a.Priority]
			pb := priorityOrder[b.Priority]
			if pa != pb {
				return pa < pb
			}
			return a.CreatedAt.Before(b.CreatedAt)
		case "updated":
			return a.UpdatedAt.Before(b.UpdatedAt)
		case "status":
			if a.Status != b.Status {
				return a.Status < b.Status
			}
			return a.CreatedAt.Before(b.CreatedAt)
		default: // "created"
			return a.CreatedAt.Before(b.CreatedAt)
		}
	})

	return result, nil
}

// Get returns the task with the given ID.
// Returns output.ErrNotFound if no task has that ID.
func Get(root *swarmfs.Root, id string) (*Task, error) {
	tasks, err := readAll(root)
	if err != nil {
		return nil, err
	}
	idx := findIdx(tasks, id)
	if idx < 0 {
		return nil, output.ErrNotFound(fmt.Sprintf("task %q not found", id))
	}
	t := tasks[idx]
	return &t, nil
}

// Update applies the given opts to the task identified by id.
// If ifMatch is non-empty, it must match the task's current ETag or
// output.ErrConflict is returned.
func Update(root *swarmfs.Root, id string, opts UpdateOpts, ifMatch string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]

		if ifMatch != "" && t.ETag != ifMatch {
			return output.ErrConflict(fmt.Sprintf("etag mismatch: want %q got %q", ifMatch, t.ETag))
		}

		if opts.Title != nil {
			t.Title = *opts.Title
		}
		if opts.Status != nil {
			t.Status = *opts.Status
		}
		if opts.Priority != nil {
			t.Priority = *opts.Priority
		}
		if opts.Tags != nil {
			t.Tags = opts.Tags
		}
		if opts.AssignedTo != nil {
			t.AssignedTo = *opts.AssignedTo
		}
		if opts.Notes != nil {
			t.Notes = *opts.Notes
		}
		if opts.Output != nil {
			if opts.AppendOutput {
				if t.Output != "" {
					t.Output += "\n"
				}
				t.Output += *opts.Output
			} else {
				t.Output = *opts.Output
			}
		}
		if opts.BlockedBy != nil {
			t.BlockedBy = opts.BlockedBy
		}

		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}

	_ = events.Append(root, events.TypeTaskUpdated, "task", updated.ID, map[string]string{
		"title": updated.Title,
	})
	return &updated, nil
}

// Assign sets AssignedTo on the task to agentIDOrName.
func Assign(root *swarmfs.Root, id, agentIDOrName string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]
		t.AssignedTo = agentIDOrName
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskAssigned, "task", updated.ID, map[string]string{
		"assigned_to": updated.AssignedTo,
	})
	return &updated, nil
}

// Claim atomically assigns the task to agentIDOrName and sets status to
// in-progress. Returns output.ErrConflict if the task is already assigned.
func Claim(root *swarmfs.Root, id, agentIDOrName string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]
		if t.AssignedTo != "" && t.AssignedTo != agentIDOrName {
			return output.ErrConflict(fmt.Sprintf("task %q is already assigned to %q", id, t.AssignedTo))
		}
		t.AssignedTo = agentIDOrName
		t.Status = StatusInProgress
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskClaimed, "task", updated.ID, map[string]string{
		"claimed_by": updated.AssignedTo,
	})
	return &updated, nil
}

// Done marks the task as done and records optional output text.
func Done(root *swarmfs.Root, id, outputText string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]
		t.Status = StatusDone
		if outputText != "" {
			t.Output = outputText
		}
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskDone, "task", updated.ID, nil)
	return &updated, nil
}

// Fail marks the task as failed with an optional reason stored in Output.
func Fail(root *swarmfs.Root, id, reason string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]
		t.Status = StatusFailed
		if reason != "" {
			t.Output = reason
		}
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskFailed, "task", updated.ID, map[string]string{
		"reason": reason,
	})
	return &updated, nil
}

// Cancel marks the task as cancelled.
func Cancel(root *swarmfs.Root, id string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]
		t.Status = StatusCancelled
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskCancelled, "task", updated.ID, nil)
	return &updated, nil
}

// Block adds blockerID to the task's BlockedBy list.
func Block(root *swarmfs.Root, id, blockerID string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		// Verify blocker exists.
		if findIdx(tasks, blockerID) < 0 {
			return output.ErrNotFound(fmt.Sprintf("blocker task %q not found", blockerID))
		}
		t := &tasks[idx]
		// Idempotent: only add if not already present.
		for _, bid := range t.BlockedBy {
			if bid == blockerID {
				updated = *t
				return nil
			}
		}
		t.BlockedBy = append(t.BlockedBy, blockerID)
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskBlocked, "task", updated.ID, map[string]string{
		"blocked_by": blockerID,
	})
	return &updated, nil
}

// Unblock removes blockerID from the task's BlockedBy list.
func Unblock(root *swarmfs.Root, id, blockerID string) (*Task, error) {
	var updated Task
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		t := &tasks[idx]
		newList := t.BlockedBy[:0]
		for _, bid := range t.BlockedBy {
			if bid != blockerID {
				newList = append(newList, bid)
			}
		}
		t.BlockedBy = newList
		t.UpdatedAt = time.Now().UTC()
		t.ETag = computeETag(t)
		updated = *t
		return writeAll(root, tasks)
	})
	if err != nil {
		return nil, err
	}
	_ = events.Append(root, events.TypeTaskUnblocked, "task", updated.ID, map[string]string{
		"unblocked_from": blockerID,
	})
	return &updated, nil
}

// Remove deletes a task by ID.
// Returns output.ErrConflict if other tasks depend on this one.
func Remove(root *swarmfs.Root, id string) error {
	return swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		idx := findIdx(tasks, id)
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("task %q not found", id))
		}
		// Check dependents.
		for _, t := range tasks {
			for _, bid := range t.BlockedBy {
				if bid == id {
					return output.ErrConflict(fmt.Sprintf("task %q is blocking %q; unblock first", id, t.ID))
				}
			}
		}
		tasks = append(tasks[:idx], tasks[idx+1:]...)
		return writeAll(root, tasks)
	})
}

// Check validates the integrity of the task store and optionally fixes issues.
// Problems include: references to non-existent blocker IDs.
func Check(root *swarmfs.Root, fix bool) ([]Problem, error) {
	problems := []Problem{}
	err := swarmfs.WithFileLock(root.TasksLockPath(), func() error {
		tasks, err := readAll(root)
		if err != nil {
			return err
		}
		ids := idSet(tasks)

		changed := false
		for i := range tasks {
			t := &tasks[i]
			var validBlockers []string
			for _, bid := range t.BlockedBy {
				if !ids[bid] {
					problems = append(problems, Problem{
						TaskID:  t.ID,
						Message: fmt.Sprintf("blocked_by references missing task %q", bid),
					})
				} else {
					validBlockers = append(validBlockers, bid)
				}
			}
			if fix && len(validBlockers) != len(t.BlockedBy) {
				t.BlockedBy = validBlockers
				t.UpdatedAt = time.Now().UTC()
				t.ETag = computeETag(t)
				changed = true
			}
		}

		if fix && changed {
			return writeAll(root, tasks)
		}
		return nil
	})
	return problems, err
}

// Prompt returns an agent-priming text block describing the current task state.
// This is intended for injection into agent context.
func Prompt(root *swarmfs.Root) (string, error) {
	tasks, err := readAll(root)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Task State\n\n")

	inProgress := filterByStatus(tasks, StatusInProgress)
	ready := readyTasks(tasks)
	blocked := blockedTasks(tasks)

	section := func(heading string, ts []Task) {
		if len(ts) == 0 {
			return
		}
		sb.WriteString("## ")
		sb.WriteString(heading)
		sb.WriteString("\n\n")
		for _, t := range ts {
			sb.WriteString(fmt.Sprintf("- [%s] %s (priority: %s", t.ID, t.Title, t.Priority))
			if t.AssignedTo != "" {
				sb.WriteString(fmt.Sprintf(", assigned: %s", t.AssignedTo))
			}
			sb.WriteString(")\n")
		}
		sb.WriteString("\n")
	}

	section("In Progress", inProgress)
	section("Ready", ready)
	section("Blocked", blocked)

	return sb.String(), nil
}

func filterByStatus(tasks []Task, s Status) []Task {
	var out []Task
	for _, t := range tasks {
		if t.Status == s {
			out = append(out, t)
		}
	}
	return out
}

func readyTasks(tasks []Task) []Task {
	openByID := make(map[string]bool)
	for _, t := range tasks {
		if !terminalStatuses[t.Status] {
			openByID[t.ID] = true
		}
	}
	var out []Task
	for _, t := range tasks {
		if terminalStatuses[t.Status] || t.Status == StatusDraft || t.Status == StatusInProgress || t.Priority == PriorityDeferred {
			continue
		}
		blocked := false
		for _, bid := range t.BlockedBy {
			if openByID[bid] {
				blocked = true
				break
			}
		}
		if !blocked {
			out = append(out, t)
		}
	}
	return out
}

func blockedTasks(tasks []Task) []Task {
	openByID := make(map[string]bool)
	for _, t := range tasks {
		if !terminalStatuses[t.Status] {
			openByID[t.ID] = true
		}
	}
	var out []Task
	for _, t := range tasks {
		if terminalStatuses[t.Status] {
			continue
		}
		for _, bid := range t.BlockedBy {
			if openByID[bid] {
				out = append(out, t)
				break
			}
		}
	}
	return out
}
