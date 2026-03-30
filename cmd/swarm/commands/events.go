package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/spf13/cobra"
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
	eventsTailCmd.Flags().BoolP("follow", "f", false, "Keep streaming after printing existing events (ignored when --n=0)")
}

var eventsTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Stream or tail the event log",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		filter, _ := cmd.Flags().GetString("filter")
		n, _ := cmd.Flags().GetInt("n")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		ch, err := events.Tail(ctx, root, filter)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}

		if n > 0 {
			// Collect all existing events, then print last N.
			var buf []events.Event
			for e := range ch {
				buf = append(buf, e)
			}
			if len(buf) > n {
				buf = buf[len(buf)-n:]
			}
			return output.Print(buf, jsonFlag(cmd))
		}

		// Follow mode: stream until cancelled.
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
