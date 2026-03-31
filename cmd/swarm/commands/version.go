package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justEstif/openswarm/internal/output"
)

// Populated by goreleaser ldflags at build time.
// Build with:
//
//	-X github.com/justEstif/openswarm/cmd/swarm/commands.version={{.Version}}
//	-X github.com/justEstif/openswarm/cmd/swarm/commands.commit={{.Commit}}
//	-X github.com/justEstif/openswarm/cmd/swarm/commands.date={{.Date}}
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// VersionCmd implements `swarm version`.
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print swarm version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag(cmd) {
			type versionResult struct {
				Version string `json:"version"`
				Commit  string `json:"commit"`
				Date    string `json:"date"`
			}
			return output.Print(versionResult{
				Version: version,
				Commit:  commit,
				Date:    date,
			}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "swarm %s (%s, %s)\n", version, commit, date)
		return nil
	},
}
