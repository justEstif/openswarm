package main

import (
	"os"

	"github.com/justEstif/openswarm/cmd/swarm/commands"
	"github.com/justEstif/openswarm/internal/config"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
	"github.com/spf13/cobra"

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

// mustRoot is the middleware used by every command handler.
// It calls swarmfs.FindRoot() and config.Load().
// On error it prints via output.PrintError and calls os.Exit(1).
//
// Note: command handlers live in the commands sub-package which cannot import
// package main. Those handlers use the equivalent unexported helpers defined in
// commands/helpers.go. This copy is provided here for any future handlers that
// live directly in package main.
func mustRoot(cmd *cobra.Command) (*swarmfs.Root, *config.Config) {
	root, err := swarmfs.FindRoot()
	if err != nil {
		output.PrintError(err, jsonFlag(cmd))
		os.Exit(1)
	}
	cfg, err := config.Load(root)
	if err != nil {
		output.PrintError(err, jsonFlag(cmd))
		os.Exit(1)
	}
	return root, cfg
}

// jsonFlag returns the --json flag value from cmd.
func jsonFlag(cmd *cobra.Command) bool {
	b, _ := cmd.Root().PersistentFlags().GetBool("json")
	return b
}
