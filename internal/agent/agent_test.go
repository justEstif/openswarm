package agent_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/justEstif/openswarm/internal/agent"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// newTestRoot initialises a temporary .swarm/ directory and returns its Root.
func newTestRoot(t *testing.T) *swarmfs.Root {
	t.Helper()
	root, err := swarmfs.InitRoot(t.TempDir())
	if err != nil {
		t.Fatalf("InitRoot: %v", err)
	}
	return root
}

// ─── Register ────────────────────────────────────────────────────────────────

func TestRegister_HappyPath(t *testing.T) {
	root := newTestRoot(t)

	a, err := agent.Register(root, "alice", "researcher", "")
	if err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}

	if a == nil {
		t.Fatal("Register: returned nil agent")
	}
	if !strings.HasPrefix(a.ID, "agent-") {
		t.Errorf("ID %q does not have prefix 'agent-'", a.ID)
	}
	if a.Name != "alice" {
		t.Errorf("Name: got %q, want %q", a.Name, "alice")
	}
	if a.Role != "researcher" {
		t.Errorf("Role: got %q, want %q", a.Role, "researcher")
	}
	if a.ProfileRef != "" {
		t.Errorf("ProfileRef: got %q, want empty", a.ProfileRef)
	}
	if a.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestRegister_WithProfile(t *testing.T) {
	root := newTestRoot(t)

	a, err := agent.Register(root, "bob", "implementer", "default-profile")
	if err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	if a.ProfileRef != "default-profile" {
		t.Errorf("ProfileRef: got %q, want %q", a.ProfileRef, "default-profile")
	}
}

func TestRegister_DuplicateNameReturnsConflict(t *testing.T) {
	root := newTestRoot(t)

	if _, err := agent.Register(root, "alice", "researcher", ""); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, err := agent.Register(root, "alice", "implementer", "")
	if err == nil {
		t.Fatal("expected CONFLICT error, got nil")
	}

	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("error is not *output.SwarmError: %T %v", err, err)
	}
	if se.Code != "CONFLICT" {
		t.Errorf("Code: got %q, want %q", se.Code, "CONFLICT")
	}
}

// ─── List ────────────────────────────────────────────────────────────────────

func TestList_Empty(t *testing.T) {
	root := newTestRoot(t)

	agents, err := agent.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestList_SortedByCreatedAt(t *testing.T) {
	root := newTestRoot(t)

	names := []string{"charlie", "alice", "bob"}
	for _, n := range names {
		if _, err := agent.Register(root, n, "worker", ""); err != nil {
			t.Fatalf("Register %q: %v", n, err)
		}
		// Small sleep so CreatedAt values differ (time.Now() granularity).
		time.Sleep(2 * time.Millisecond)
	}

	agents, err := agent.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}

	// Must be returned in registration order (earliest CreatedAt first).
	wantOrder := []string{"charlie", "alice", "bob"}
	for i, a := range agents {
		if a.Name != wantOrder[i] {
			t.Errorf("agents[%d].Name = %q, want %q", i, a.Name, wantOrder[i])
		}
	}

	// Verify ascending CreatedAt ordering.
	for i := 1; i < len(agents); i++ {
		if agents[i].CreatedAt.Before(agents[i-1].CreatedAt) {
			t.Errorf("agents[%d].CreatedAt (%v) is before agents[%d].CreatedAt (%v)",
				i, agents[i].CreatedAt, i-1, agents[i-1].CreatedAt)
		}
	}
}

// ─── Get ─────────────────────────────────────────────────────────────────────

func TestGet_ByID(t *testing.T) {
	root := newTestRoot(t)

	created, err := agent.Register(root, "alice", "researcher", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := agent.Get(root, created.ID)
	if err != nil {
		t.Fatalf("Get by ID: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}
	if got.Name != "alice" {
		t.Errorf("Name: got %q, want %q", got.Name, "alice")
	}
}

func TestGet_ByName(t *testing.T) {
	root := newTestRoot(t)

	created, err := agent.Register(root, "alice", "researcher", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := agent.Get(root, "alice")
	if err != nil {
		t.Fatalf("Get by name: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}
}

func TestGet_MissingReturnsNotFound(t *testing.T) {
	root := newTestRoot(t)

	_, err := agent.Get(root, "nobody")
	if err == nil {
		t.Fatal("expected NOT_FOUND error, got nil")
	}

	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("error is not *output.SwarmError: %T %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code: got %q, want %q", se.Code, "NOT_FOUND")
	}
}

// ─── Deregister ──────────────────────────────────────────────────────────────

func TestDeregister_HappyPath(t *testing.T) {
	root := newTestRoot(t)

	created, err := agent.Register(root, "alice", "researcher", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := agent.Deregister(root, created.ID); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	// Agent must be gone.
	_, err = agent.Get(root, created.ID)
	if err == nil {
		t.Fatal("expected NOT_FOUND after Deregister, got nil")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) || se.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}

	// Registry must still be consistent (other agents unaffected).
	agents, err := agent.List(root)
	if err != nil {
		t.Fatalf("List after Deregister: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDeregister_ByName(t *testing.T) {
	root := newTestRoot(t)

	if _, err := agent.Register(root, "alice", "researcher", ""); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := agent.Deregister(root, "alice"); err != nil {
		t.Fatalf("Deregister by name: %v", err)
	}

	agents, err := agent.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after deregister, got %d", len(agents))
	}
}

func TestDeregister_LeavesOthersIntact(t *testing.T) {
	root := newTestRoot(t)

	_, _ = agent.Register(root, "alice", "researcher", "")
	bob, _ := agent.Register(root, "bob", "implementer", "")
	_, _ = agent.Register(root, "charlie", "worker", "")

	if err := agent.Deregister(root, bob.ID); err != nil {
		t.Fatalf("Deregister bob: %v", err)
	}

	agents, err := agent.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	for _, a := range agents {
		if a.Name == "bob" {
			t.Error("bob still present after deregister")
		}
	}
}

func TestDeregister_MissingReturnsNotFound(t *testing.T) {
	root := newTestRoot(t)

	err := agent.Deregister(root, "ghost")
	if err == nil {
		t.Fatal("expected NOT_FOUND error, got nil")
	}

	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("error is not *output.SwarmError: %T %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code: got %q, want %q", se.Code, "NOT_FOUND")
	}
}
