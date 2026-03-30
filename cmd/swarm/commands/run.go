package commands

import (
	"fmt"
	"strings"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/pane"
	"github.com/justEstif/openswarm/internal/run"
	"github.com/spf13/cobra"
)

// RunCmd is the `swarm run` command.
// With no subcommand it spawns + waits (blocking). Subcommands manage existing runs.
var RunCmd = &cobra.Command{
	Use:   "run [--name n] [--no-wait] -- <cmd...>",
	Short: "Spawn a command in a pane and (optionally) wait for it",
	RunE:  runStartWait,
}

func init() {
	RunCmd.Flags().String("name", "", "Human label for the run (default: run ID)")
	RunCmd.Flags().Bool("no-wait", false, "Return immediately after spawning")

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
	noWait, _ := cmd.Flags().GetBool("no-wait")
	cmdStr := strings.Join(args, " ")

	r, err := run.Start(root, b, name, cmdStr, nil)
	if err != nil {
		output.PrintError(err, jsonFlag(cmd))
		return nil
	}
	if !noWait {
		r, err = run.Wait(root, b, r.ID)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
	}
	return output.Print(r, jsonFlag(cmd))
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
