package commands

import (
	"fmt"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/task"
	"github.com/spf13/cobra"
)

// TaskCmd is the `swarm task` group command.
var TaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage swarm tasks",
	Long:  "Add, list, inspect, update, assign, and lifecycle-manage tasks in the swarm.",
}

func init() {
	// swarm task add
	taskAddCmd.Flags().String("status", "todo", "Initial status")
	taskAddCmd.Flags().String("priority", "normal", "Task priority")
	taskAddCmd.Flags().StringArray("tag", nil, "Tag (repeatable)")
	taskAddCmd.Flags().String("assign", "", "Agent ID or name to assign")
	taskAddCmd.Flags().StringArray("blocked-by", nil, "Blocked-by task ID (repeatable)")
	taskAddCmd.Flags().String("notes", "", "Notes")

	// swarm task list
	taskListCmd.Flags().StringArray("status", nil, "Filter by status (repeatable)")
	taskListCmd.Flags().String("assign", "", "Filter by assigned agent")
	taskListCmd.Flags().StringArray("tag", nil, "Filter by tag (repeatable)")
	taskListCmd.Flags().Bool("ready", false, "Show only ready tasks")
	taskListCmd.Flags().String("sort", "", "Sort by: priority|created|updated|status")

	// swarm task update
	taskUpdateCmd.Flags().String("title", "", "New title")
	taskUpdateCmd.Flags().String("status", "", "New status")
	taskUpdateCmd.Flags().String("priority", "", "New priority")
	taskUpdateCmd.Flags().StringArray("tag", nil, "Replace tags (repeatable)")
	taskUpdateCmd.Flags().String("assign", "", "Assign to agent")
	taskUpdateCmd.Flags().String("notes", "", "Set notes")
	taskUpdateCmd.Flags().String("output", "", "Set output")
	taskUpdateCmd.Flags().Bool("append-output", false, "Append to output instead of replacing")
	taskUpdateCmd.Flags().StringArray("blocked-by", nil, "Replace blocked-by list (repeatable)")
	taskUpdateCmd.Flags().String("if-match", "", "ETag for optimistic locking")

	// swarm task claim
	taskClaimCmd.Flags().String("as", "", "Agent ID or name claiming the task (required)")
	_ = taskClaimCmd.MarkFlagRequired("as")

	// swarm task done
	taskDoneCmd.Flags().String("output", "", "Output text to record")

	// swarm task fail
	taskFailCmd.Flags().String("reason", "", "Failure reason")

	// swarm task block
	taskBlockCmd.Flags().String("by", "", "Blocker task ID (required)")
	_ = taskBlockCmd.MarkFlagRequired("by")

	// swarm task check
	taskCheckCmd.Flags().Bool("fix", false, "Auto-fix detected issues")

	TaskCmd.AddCommand(
		taskAddCmd,
		taskListCmd,
		taskGetCmd,
		taskUpdateCmd,
		taskAssignCmd,
		taskClaimCmd,
		taskDoneCmd,
		taskFailCmd,
		taskCancelCmd,
		taskBlockCmd,
		taskCheckCmd,
		taskPromptCmd,
	)
}

// ─── swarm task add ──────────────────────────────────────────────────────────

var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		statusStr, _ := cmd.Flags().GetString("status")
		priorityStr, _ := cmd.Flags().GetString("priority")
		tags, _ := cmd.Flags().GetStringArray("tag")
		assign, _ := cmd.Flags().GetString("assign")
		blockedBy, _ := cmd.Flags().GetStringArray("blocked-by")
		notes, _ := cmd.Flags().GetString("notes")
		opts := task.AddOpts{
			Status:     task.Status(statusStr),
			Priority:   task.Priority(priorityStr),
			Tags:       tags,
			AssignedTo: assign,
			BlockedBy:  blockedBy,
			Notes:      notes,
		}
		result, err := task.Add(root, args[0], opts)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(result, jsonFlag(cmd))
	},
}

// ─── swarm task list ─────────────────────────────────────────────────────────

// taskListRow is the subset of Task fields shown in the human-readable table.
type taskListRow struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	AssignedTo string `json:"assigned_to"`
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		statusStrs, _ := cmd.Flags().GetStringArray("status")
		assign, _ := cmd.Flags().GetString("assign")
		tags, _ := cmd.Flags().GetStringArray("tag")
		ready, _ := cmd.Flags().GetBool("ready")
		sortBy, _ := cmd.Flags().GetString("sort")

		statuses := make([]task.Status, len(statusStrs))
		for i, s := range statusStrs {
			statuses[i] = task.Status(s)
		}
		results, err := task.List(root, task.ListFilter{
			Status:     statuses,
			AssignedTo: assign,
			Tags:       tags,
			Ready:      ready,
			SortBy:     sortBy,
		})
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(results, true)
		}
		rows := make([]taskListRow, len(results))
		for i, t := range results {
			rows[i] = taskListRow{
				ID:         t.ID,
				Title:      t.Title,
				Status:     string(t.Status),
				Priority:   string(t.Priority),
				AssignedTo: t.AssignedTo,
			}
		}
		return output.Print(rows, false)
	},
}

// ─── swarm task get ──────────────────────────────────────────────────────────

var taskGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a task by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		result, err := task.Get(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(result, jsonFlag(cmd))
	},
}

// ─── swarm task update ───────────────────────────────────────────────────────

var taskUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		opts := task.UpdateOpts{}
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			opts.Title = &v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			s := task.Status(v)
			opts.Status = &s
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			p := task.Priority(v)
			opts.Priority = &p
		}
		if cmd.Flags().Changed("tag") {
			v, _ := cmd.Flags().GetStringArray("tag")
			opts.Tags = v
		}
		if cmd.Flags().Changed("assign") {
			v, _ := cmd.Flags().GetString("assign")
			opts.AssignedTo = &v
		}
		if cmd.Flags().Changed("notes") {
			v, _ := cmd.Flags().GetString("notes")
			opts.Notes = &v
		}
		if cmd.Flags().Changed("output") {
			v, _ := cmd.Flags().GetString("output")
			opts.Output = &v
		}
		if cmd.Flags().Changed("append-output") {
			v, _ := cmd.Flags().GetBool("append-output")
			opts.AppendOutput = v
		}
		if cmd.Flags().Changed("blocked-by") {
			v, _ := cmd.Flags().GetStringArray("blocked-by")
			opts.BlockedBy = v
		}
		ifMatch, _ := cmd.Flags().GetString("if-match")
		result, err := task.Update(root, args[0], opts, ifMatch)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(result, jsonFlag(cmd))
	},
}

// ─── swarm task assign ───────────────────────────────────────────────────────

var taskAssignCmd = &cobra.Command{
	Use:   "assign <id> <agent-id-or-name>",
	Short: "Assign a task to an agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		_, err := task.Assign(root, args[0], args[1])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "assigned_to": args[1]}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Assigned %s to %s\n", args[0], args[1])
		return nil
	},
}

// ─── swarm task claim ────────────────────────────────────────────────────────

var taskClaimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a task as an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		asAgent, _ := cmd.Flags().GetString("as")
		_, err := task.Claim(root, args[0], asAgent)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "claimed_by": asAgent}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Claimed %s\n", args[0])
		return nil
	},
}

// ─── swarm task done ─────────────────────────────────────────────────────────

var taskDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark a task as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		outputText, _ := cmd.Flags().GetString("output")
		_, err := task.Done(root, args[0], outputText)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "status": "done"}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Done: %s\n", args[0])
		return nil
	},
}

// ─── swarm task fail ─────────────────────────────────────────────────────────

var taskFailCmd = &cobra.Command{
	Use:   "fail <id>",
	Short: "Mark a task as failed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		reason, _ := cmd.Flags().GetString("reason")
		_, err := task.Fail(root, args[0], reason)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "status": "failed"}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Failed: %s\n", args[0])
		return nil
	},
}

// ─── swarm task cancel ───────────────────────────────────────────────────────

var taskCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		_, err := task.Cancel(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "status": "cancelled"}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Cancelled: %s\n", args[0])
		return nil
	},
}

// ─── swarm task block ────────────────────────────────────────────────────────

var taskBlockCmd = &cobra.Command{
	Use:   "block <id>",
	Short: "Mark a task as blocked by another",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		blockerID, _ := cmd.Flags().GetString("by")
		_, err := task.Block(root, args[0], blockerID)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "blocked_by": blockerID}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s is now blocked by %s\n", args[0], blockerID)
		return nil
	},
}

// ─── swarm task check ────────────────────────────────────────────────────────

var taskCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check task store integrity",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		fix, _ := cmd.Flags().GetBool("fix")
		problems, err := task.Check(root, fix)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(problems, true)
		}
		if len(problems) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No issues found.\n")
			return nil
		}
		for _, p := range problems {
			fmt.Fprintln(cmd.OutOrStdout(), p)
		}
		return nil
	},
}

// ─── swarm task prompt ───────────────────────────────────────────────────────

var taskPromptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Print agent-priming task context",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		text, err := task.Prompt(root)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), text)
		return nil
	},
}
