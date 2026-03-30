package commands

import (
	"fmt"
	"strings"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/pane"
	"github.com/spf13/cobra"
)

// PaneCmd is the `swarm pane` command group.
var PaneCmd = &cobra.Command{
	Use:   "pane",
	Short: "Manage multiplexer panes",
}

func init() {
	PaneCmd.AddCommand(paneSpawnCmd)
	PaneCmd.AddCommand(paneSendCmd)
	PaneCmd.AddCommand(paneCaptureCmd)
	PaneCmd.AddCommand(paneListCmd)
	PaneCmd.AddCommand(paneCloseCmd)
}

func mustBackend(cmd *cobra.Command) (pane.Backend, bool) {
	_, cfg := mustRoot(cmd)
	b, err := pane.DetectBackend(cfg)
	if err != nil {
		output.PrintError(err, jsonFlag(cmd))
		return nil, false
	}
	return b, true
}

// swarm pane spawn <name> [cmd...]
var paneSpawnCmd = &cobra.Command{
	Use:   "spawn <name> [cmd...]",
	Short: "Spawn a new pane",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, ok := mustBackend(cmd)
		if !ok {
			return nil
		}
		name := args[0]
		cmdStr := strings.Join(args[1:], " ")
		id, err := b.Spawn(name, cmdStr, nil)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			type result struct {
				ID string `json:"id"`
			}
			return output.Print(result{ID: string(id)}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Spawned pane %s\n", id)
		return nil
	},
}

// swarm pane send <id> <text>
var paneSendCmd = &cobra.Command{
	Use:   "send <id> <text>",
	Short: "Send text to a pane",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, ok := mustBackend(cmd)
		if !ok {
			return nil
		}
		if err := b.Send(pane.PaneID(args[0]), args[1]); err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			type result struct {
				OK bool `json:"ok"`
			}
			return output.Print(result{OK: true}, true)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Sent")
		return nil
	},
}

// swarm pane capture <id>
var paneCaptureCmd = &cobra.Command{
	Use:   "capture <id>",
	Short: "Capture pane scrollback",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, ok := mustBackend(cmd)
		if !ok {
			return nil
		}
		out, err := b.Capture(pane.PaneID(args[0]))
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	},
}

// swarm pane list
var paneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List panes",
	RunE: func(cmd *cobra.Command, args []string) error {
		b, ok := mustBackend(cmd)
		if !ok {
			return nil
		}
		infos, err := b.List()
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(infos, jsonFlag(cmd))
	},
}

// swarm pane close <id>
var paneCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a pane",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, ok := mustBackend(cmd)
		if !ok {
			return nil
		}
		if err := b.Close(pane.PaneID(args[0])); err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			type result struct {
				OK bool `json:"ok"`
			}
			return output.Print(result{OK: true}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Closed %s\n", args[0])
		return nil
	},
}
