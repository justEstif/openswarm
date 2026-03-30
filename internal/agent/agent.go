// Package agent manages the openswarm agent registry.
//
// Agents are persisted as a JSON array in .swarm/agents/registry.json.
// All mutations acquire an exclusive flock on .swarm/agents/.lock before
// reading and writing the registry, ensuring consistency across concurrent
// processes.
//
// Every mutating operation emits a corresponding event via the events package.
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// Agent represents a registered participant in the swarm.
type Agent struct {
	ID         string    `json:"id"`         // e.g. "agent-a3f2k1"
	Name       string    `json:"name"`       // human label, e.g. "alice"
	Role       string    `json:"role"`       // e.g. "researcher", "implementer"
	ProfileRef string    `json:"profile"`    // references config.AgentProfile.Name; may be empty
	CreatedAt  time.Time `json:"created_at"` // UTC
}

// lockPath returns the flock file path for the agent registry.
func lockPath(root *swarmfs.Root) string {
	return filepath.Join(filepath.Dir(root.AgentsPath()), ".lock")
}

// readAll reads the registry file and returns all agents.
// Returns an empty (non-nil) slice if the file does not exist.
func readAll(root *swarmfs.Root) ([]Agent, error) {
	data, err := os.ReadFile(root.AgentsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Agent{}, nil
		}
		return nil, fmt.Errorf("agent: read registry: %w", err)
	}

	var agents []Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, fmt.Errorf("agent: unmarshal registry: %w", err)
	}
	if agents == nil {
		agents = []Agent{}
	}
	return agents, nil
}

// writeAll serialises agents and atomically writes the registry file.
// An empty slice is written as "[]" (not "null").
func writeAll(root *swarmfs.Root, agents []Agent) error {
	if agents == nil {
		agents = []Agent{}
	}
	data, err := json.Marshal(agents)
	if err != nil {
		return fmt.Errorf("agent: marshal registry: %w", err)
	}
	if err := swarmfs.AtomicWrite(root.AgentsPath(), data); err != nil {
		return fmt.Errorf("agent: write registry: %w", err)
	}
	return nil
}

// Register creates a new agent with the given name, role, and optional profile
// reference. It returns a CONFLICT error if an agent with the same name already
// exists. On success it emits a TypeAgentRegistered event.
func Register(root *swarmfs.Root, name, role, profile string) (*Agent, error) {
	var created Agent

	err := swarmfs.WithFileLock(lockPath(root), func() error {
		agents, err := readAll(root)
		if err != nil {
			return err
		}

		// Duplicate name check (case-sensitive, as documented).
		for _, a := range agents {
			if a.Name == name {
				return output.ErrConflict(fmt.Sprintf("agent %q already exists", name))
			}
		}

		created = Agent{
			ID:         swarmfs.NewID("agent"),
			Name:       name,
			Role:       role,
			ProfileRef: profile,
			CreatedAt:  time.Now().UTC(),
		}

		agents = append(agents, created)
		return writeAll(root, agents)
	})
	if err != nil {
		return nil, err
	}

	// Emit event outside the lock — the registry is already consistent.
	if err := events.Append(root, events.TypeAgentRegistered, "agent", created.ID, map[string]string{
		"name": created.Name,
		"role": created.Role,
	}); err != nil {
		// Non-fatal: log the failure in the error message but still return the
		// created agent so the caller can act on the successful registration.
		return &created, fmt.Errorf("agent: event append: %w", err)
	}

	return &created, nil
}

// List returns all registered agents, sorted by CreatedAt ascending.
func List(root *swarmfs.Root) ([]*Agent, error) {
	agents, err := readAll(root)
	if err != nil {
		return nil, err
	}

	// Sort ascending by CreatedAt, then by ID for determinism on equal times.
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].CreatedAt.Equal(agents[j].CreatedAt) {
			return agents[i].ID < agents[j].ID
		}
		return agents[i].CreatedAt.Before(agents[j].CreatedAt)
	})

	result := make([]*Agent, len(agents))
	for i := range agents {
		a := agents[i] // copy to heap
		result[i] = &a
	}
	return result, nil
}

// Get resolves an agent by ID (e.g. "agent-a3f2k1") or by name (e.g. "alice").
// Returns output.ErrNotFound if no match is found.
func Get(root *swarmfs.Root, idOrName string) (*Agent, error) {
	agents, err := readAll(root)
	if err != nil {
		return nil, err
	}

	for i := range agents {
		if agents[i].ID == idOrName || agents[i].Name == idOrName {
			a := agents[i]
			return &a, nil
		}
	}

	return nil, output.ErrNotFound(fmt.Sprintf("agent %q not found", idOrName))
}

// Deregister removes an agent identified by ID or name.
// Returns output.ErrNotFound if no match is found.
// On success it emits a TypeAgentDeregistered event.
func Deregister(root *swarmfs.Root, idOrName string) error {
	var removed Agent
	found := false

	err := swarmfs.WithFileLock(lockPath(root), func() error {
		agents, err := readAll(root)
		if err != nil {
			return err
		}

		idx := -1
		for i := range agents {
			if agents[i].ID == idOrName || agents[i].Name == idOrName {
				idx = i
				break
			}
		}
		if idx < 0 {
			return output.ErrNotFound(fmt.Sprintf("agent %q not found", idOrName))
		}

		removed = agents[idx]
		found = true

		// Remove element at idx, preserving order.
		agents = append(agents[:idx], agents[idx+1:]...)
		return writeAll(root, agents)
	})
	if err != nil {
		return err
	}

	if found {
		if err := events.Append(root, events.TypeAgentDeregistered, "agent", removed.ID, map[string]string{
			"name": removed.Name,
		}); err != nil {
			return fmt.Errorf("agent: event append: %w", err)
		}
	}

	return nil
}
