package main

import (
	"github.com/spf13/cobra"

	"github.com/justEstif/openswarm/cmd/swarm/commands"

	// Register multiplexer backends.
	_ "github.com/justEstif/openswarm/internal/pane/ghostty"
	_ "github.com/justEstif/openswarm/internal/pane/tmux"
	_ "github.com/justEstif/openswarm/internal/pane/wezterm"
	_ "github.com/justEstif/openswarm/internal/pane/zellij"
)

// rootCmd is the top-level `swarm` command.
// Persistent flags (inherited by all subcommands):
//
//	--json   Output as JSON
var rootCmd = &cobra.Command{
	Use:   "swarm",
	Short: "openswarm — multi-agent task orchestration",
	// Suppress cobra's default error/usage printing — every command manages its
	// own output via the output package so errors are never double-printed.
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the cobra command tree. Called by main.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")

	rootCmd.AddCommand(commands.InitCmd)
	rootCmd.AddCommand(commands.VersionCmd)
	rootCmd.AddCommand(commands.AgentCmd)
	rootCmd.AddCommand(commands.TaskCmd)
	rootCmd.AddCommand(commands.MsgCmd)
	rootCmd.AddCommand(commands.PaneCmd)
	rootCmd.AddCommand(commands.RunCmd)
	rootCmd.AddCommand(commands.WorktreeCmd)
	rootCmd.AddCommand(commands.EventsCmd)
	rootCmd.AddCommand(commands.StatusCmd)
	rootCmd.AddCommand(commands.PromptCmd)
}
