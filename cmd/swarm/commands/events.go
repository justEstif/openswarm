package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
)

// EventsCmd is the `swarm events` command group.
var EventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream the openswarm event log",
}

func init() {
	EventsCmd.AddCommand(eventsTailCmd)

	eventsTailCmd.Flags().String("filter", "", "Only emit events whose Type contains this string (e.g. task, run.done)")
	eventsTailCmd.Flags().Int("n", 0, "Print last N events then exit (0 = follow forever)")
}

var eventsTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Stream or tail the event log",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		filter, _ := cmd.Flags().GetString("filter")
		n, _ := cmd.Flags().GetInt("n")

		// --n: snapshot mode — read existing events once, print last N, exit.
		if n > 0 {
			evts, err := events.Last(root, filter, n)
			if err != nil {
				output.PrintError(err, jsonFlag(cmd))
				return nil
			}
			return output.Print(evts, jsonFlag(cmd))
		}

		// Follow mode: stream until SIGINT/SIGTERM.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		ch, err := events.Tail(ctx, root, filter)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}

		asJSON := jsonFlag(cmd)
		for e := range ch {
			if asJSON {
				_ = output.Print(e, true)
			} else {
				fmt.Printf("%s [%s] %s\n", e.At.Format(time.RFC3339), e.Type, e.Ref)
			}
		}
		return nil
	},
}
