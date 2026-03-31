package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/task"
)

// PromptCmd is the `swarm prompt` command.
var PromptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Generate an agent-priming prompt from current swarm state",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		text, err := task.Prompt(root)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), text)
		return nil
	},
}
