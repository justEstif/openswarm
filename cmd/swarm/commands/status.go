package commands

import (
	"fmt"

	"github.com/justEstif/openswarm/internal/agent"
	"github.com/justEstif/openswarm/internal/msg"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/pane"
	"github.com/justEstif/openswarm/internal/run"
	"github.com/justEstif/openswarm/internal/task"
	"github.com/spf13/cobra"
)

// SwarmStatus holds the aggregated swarm state for `swarm status`.
type SwarmStatus struct {
	Agents     int `json:"agents"`
	Tasks      int `json:"tasks"`
	TasksDone  int `json:"tasks_done"`
	Unread     int `json:"unread"`
	RunsActive int `json:"runs_active"`
}

// StatusCmd is the `swarm status` command.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current swarm state at a glance",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg := mustRoot(cmd)

		var st SwarmStatus

		// Agents.
		agents, _ := agent.List(root)
		st.Agents = len(agents)

		// Tasks: total and done.
		tasks, _ := task.List(root, task.ListFilter{})
		st.Tasks = len(tasks)
		for _, t := range tasks {
			if t.Status == task.StatusDone {
				st.TasksDone++
			}
		}

		// Unread messages: sum across all agents.
		for _, a := range agents {
			n, _ := msg.UnreadCount(root, a.ID)
			st.Unread += n
		}

		// Active runs.
		runs, _ := run.List(root)
		for _, r := range runs {
			if r.Status == run.RunStatusRunning {
				st.RunsActive++
			}
		}

		// Pane count: best-effort, ignore errors.
		_ = cfg // used by DetectBackend
		if b, err := pane.DetectBackend(cfg); err == nil {
			if _, lerr := b.List(); lerr != nil {
				// backend detected but List failed — ignore silently
			}
		}

		if jsonFlag(cmd) {
			return output.Print(st, true)
		}

		fmt.Printf("Agents: %d  Tasks: %d (%d done)  Unread: %d  Active runs: %d\n",
			st.Agents, st.Tasks, st.TasksDone, st.Unread, st.RunsActive)
		return nil
	},
}
