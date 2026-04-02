package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/pane"
	"github.com/justEstif/openswarm/internal/run"
)

// RunCmd is the `swarm run` command.
// With no subcommand it spawns and returns immediately (fire-and-forget).
// Use --wait to block until the pane exits, or `swarm run wait <id>` later.
var RunCmd = &cobra.Command{
	Use:   "run [--name n] [--wait] -- <cmd...>",
	Short: "Spawn a command in a managed pane (non-blocking by default)",
	RunE:  runStartWait,
}

func init() {
	RunCmd.Flags().String("name", "", "Human label for the run (default: run ID)")
	RunCmd.Flags().Bool("wait", false, "Block until the pane exits (default: fire-and-forget)")
	RunCmd.Flags().String("placement", "", "Where to open the pane: current_tab (default), new_tab, new_session")

	runStartCmd.Flags().String("name", "", "Human label for the run (default: run ID)")
	runStartCmd.Flags().Bool("wait", false, "Block until the pane exits (default: fire-and-forget)")
	runStartCmd.Flags().String("placement", "", "Where to open the pane: current_tab (default), new_tab, new_session")

	RunCmd.AddCommand(runStartCmd)
	RunCmd.AddCommand(runListCmd)
	RunCmd.AddCommand(runGetCmd)
	RunCmd.AddCommand(runWaitCmd)
	RunCmd.AddCommand(runKillCmd)
	RunCmd.AddCommand(runLogsCmd)
}

func runStartWait(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		_ = cmd.Help()
		return nil
	}
	root, cfg := mustRoot(cmd)
	b, err := pane.DetectBackend(cfg)
	if err != nil {
		output.PrintError(err, jsonFlag(cmd))
		return nil
	}
	name, _ := cmd.Flags().GetString("name")
	wait, _ := cmd.Flags().GetBool("wait")
	cmdStr := strings.Join(args, " ")

	// Resolve placement: CLI flag overrides config, config overrides default.
	placement := pane.Placement(cfg.Pane.Placement)
	if p, _ := cmd.Flags().GetString("placement"); p != "" {
		placement = pane.Placement(p)
	}

	opts := pane.SpawnOptions{
		// Pass the caller's PATH so mise/nvm/pyenv tools are available in the pane.
		Env:       map[string]string{"PATH": os.Getenv("PATH")},
		Placement: placement,
		// Close on exit for fire-and-forget; keep pane open when --wait is used
		// so output can be captured after the command finishes.
		CloseOnExit: !wait,
	}

	r, err := run.Start(root, b, name, cmdStr, opts)
	if err != nil {
		output.PrintError(err, jsonFlag(cmd))
		return nil
	}
	if wait {
		r, err = run.Wait(root, b, r.ID)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
	}
	return output.Print(r, jsonFlag(cmd))
}

// swarm run start (alias for the root command — fire-and-forget by default)
var runStartCmd = &cobra.Command{
	Use:   "start [--name n] [--wait] -- <cmd...>",
	Short: "Spawn a command in a managed pane (non-blocking by default)",
	RunE:  runStartWait,
}

// swarm run list
var runListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		runs, err := run.List(root)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(runs, jsonFlag(cmd))
	},
}

// swarm run get <id>
var runGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		r, err := run.Get(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(r, jsonFlag(cmd))
	},
}

// swarm run wait <id>
var runWaitCmd = &cobra.Command{
	Use:   "wait <id>",
	Short: "Wait for a run to complete",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg := mustRoot(cmd)
		b, err := pane.DetectBackend(cfg)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		r, err := run.Wait(root, b, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(r, jsonFlag(cmd))
	},
}

// swarm run kill <id>
var runKillCmd = &cobra.Command{
	Use:   "kill <id>",
	Short: "Kill a running pane",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg := mustRoot(cmd)
		b, err := pane.DetectBackend(cfg)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if err := run.Kill(root, b, args[0]); err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			type result struct {
				OK bool `json:"ok"`
			}
			return output.Print(result{OK: true}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Killed %s\n", args[0])
		return nil
	},
}

// swarm run logs <id>
var runLogsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "Show captured output of a run",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg := mustRoot(cmd)
		b, err := pane.DetectBackend(cfg)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		logs, err := run.Logs(root, b, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), logs)
		return nil
	},
}
