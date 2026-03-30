// Package commands contains all subcommand implementations for the swarm CLI.
// Helper functions jsonFlag and mustRoot are available to every command in this
// package so that handlers can follow the NON-NEGOTIABLE pattern without
// importing package main (Go prohibits that).
package commands

import (
	"os"

	"github.com/justEstif/openswarm/internal/config"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
	"github.com/spf13/cobra"
)

// jsonFlag returns the value of the persistent --json flag from the root
// command. Returns false if the flag is not registered.
func jsonFlag(cmd *cobra.Command) bool {
	b, _ := cmd.Root().PersistentFlags().GetBool("json")
	return b
}

// mustRoot calls swarmfs.FindRoot() and config.Load().
// On error it prints via output.PrintError and calls os.Exit(1).
// Use this in any command that requires an initialised project root.
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
