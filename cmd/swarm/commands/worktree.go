package commands

import (
	"fmt"
	"time"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/worktree"
	"github.com/spf13/cobra"
)

// WorktreeCmd is the `swarm worktree` group command.
var WorktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage git worktrees for agents",
}

func init() {
	// swarm worktree new
	worktreeNewCmd.Flags().String("branch", "", "Branch name (required)")
	worktreeNewCmd.Flags().String("agent", "", "Agent ID to associate with the worktree")

	// swarm worktree merge
	worktreeMergeCmd.Flags().Bool("squash", false, "Squash commits before merging")
	worktreeMergeCmd.Flags().Bool("delete-branch", false, "Delete the branch after merging")

	WorktreeCmd.AddCommand(worktreeNewCmd)
	WorktreeCmd.AddCommand(worktreeListCmd)
	WorktreeCmd.AddCommand(worktreeGetCmd)
	WorktreeCmd.AddCommand(worktreeMergeCmd)
	WorktreeCmd.AddCommand(worktreeCleanCmd)
	WorktreeCmd.AddCommand(worktreeCleanAllCmd)
}

// ─── swarm worktree new ───────────────────────────────────────────────────────

var worktreeNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a git worktree for an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		branch, _ := cmd.Flags().GetString("branch")
		agentID, _ := cmd.Flags().GetString("agent")
		if branch == "" {
			output.PrintError(fmt.Errorf("--branch is required"), jsonFlag(cmd))
			return nil
		}
		wt, err := worktree.New(root, branch, agentID)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(wt, jsonFlag(cmd))
	},
}

// ─── swarm worktree list ──────────────────────────────────────────────────────

// worktreeListRow is the subset of Worktree fields shown in the human-readable table.
type worktreeListRow struct {
	ID        string `json:"id"`
	Branch    string `json:"branch"`
	Status    string `json:"status"`
	Agent     string `json:"agent"`
	CreatedAt string `json:"created_at"`
}

func toWorktreeRow(wt *worktree.Worktree) worktreeListRow {
	return worktreeListRow{wt.ID, wt.Branch, string(wt.Status), wt.AgentID, wt.CreatedAt.Format(time.RFC3339)}
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List git worktrees",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		wts, err := worktree.List(root)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(wts, true)
		}
		rows := make([]worktreeListRow, len(wts))
		for i, wt := range wts {
			rows[i] = toWorktreeRow(wt)
		}
		return output.Print(rows, false)
	},
}

// ─── swarm worktree get ───────────────────────────────────────────────────────

var worktreeGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a worktree by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		wt, err := worktree.Get(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(wt, jsonFlag(cmd))
	},
}

// ─── swarm worktree merge ─────────────────────────────────────────────────────

var worktreeMergeCmd = &cobra.Command{
	Use:   "merge <id>",
	Short: "Merge a worktree branch into the current branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		squash, _ := cmd.Flags().GetBool("squash")
		deleteBranch, _ := cmd.Flags().GetBool("delete-branch")
		wt, err := worktree.Get(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		opts := worktree.MergeOpts{Squash: squash, DeleteBranch: deleteBranch}
		if err := worktree.Merge(root, args[0], opts); err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0], "branch": wt.Branch}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Merged worktree %s (branch: %s)\n", args[0], wt.Branch)
		return nil
	},
}

// ─── swarm worktree clean ─────────────────────────────────────────────────────

var worktreeCleanCmd = &cobra.Command{
	Use:   "clean <id>",
	Short: "Remove a worktree and clean up git state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		if err := worktree.Clean(root, args[0]); err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]string{"id": args[0]}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Cleaned worktree %s\n", args[0])
		return nil
	},
}

// ─── swarm worktree clean-all ─────────────────────────────────────────────────

var worktreeCleanAllCmd = &cobra.Command{
	Use:   "clean-all",
	Short: "Clean all merged and abandoned worktrees",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		cleaned, err := worktree.CleanAll(root)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(map[string]int{"count": len(cleaned)}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d worktrees\n", len(cleaned))
		return nil
	},
}
