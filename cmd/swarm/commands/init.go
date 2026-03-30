package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
	"github.com/spf13/cobra"
)

// InitCmd implements `swarm init`.
var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .swarm/ project root in the current directory",
	Long: `Initialize a new openswarm project in the current directory.

Creates .swarm/ and all required subdirectories. Safe to run multiple times —
subsequent runs are a no-op.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			output.PrintError(output.ErrIO(err.Error()), jsonFlag(cmd))
			return nil
		}

		// Check whether .swarm/ already exists before calling InitRoot so we
		// can produce the correct human-readable message.
		swarmDir := filepath.Join(cwd, ".swarm")
		_, statErr := os.Stat(swarmDir)
		alreadyExists := statErr == nil

		root, err := swarmfs.InitRoot(cwd)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}

		if jsonFlag(cmd) {
			type initResult struct {
				Path string `json:"path"`
			}
			return output.Print(initResult{Path: root.Dir}, true)
		}

		if alreadyExists {
			fmt.Fprintln(cmd.OutOrStdout(), "Already initialized")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized .swarm/ in %s\n", cwd)
		}
		return nil
	},
}
