package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justEstif/openswarm/internal/agent"
	"github.com/justEstif/openswarm/internal/output"
)

// AgentCmd is the `swarm agent` group command.
var AgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage swarm agents",
	Long:  "Register, list, inspect, and deregister agents in the swarm.",
}

func init() {
	// register flags
	agentRegisterCmd.Flags().String("role", "agent", "Agent role")
	agentRegisterCmd.Flags().String("profile", "", "Agent profile reference")

	AgentCmd.AddCommand(agentRegisterCmd)
	AgentCmd.AddCommand(agentListCmd)
	AgentCmd.AddCommand(agentGetCmd)
	AgentCmd.AddCommand(agentDeregisterCmd)
}

var agentRegisterCmd = &cobra.Command{
	Use:   "register <name>",
	Short: "Register a new agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		role, _ := cmd.Flags().GetString("role")
		profile, _ := cmd.Flags().GetString("profile")
		result, err := agent.Register(root, args[0], role, profile)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(result, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Registered agent %s (%s)\n", result.Name, result.ID)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered agents",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		results, err := agent.List(root)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(results, jsonFlag(cmd))
	},
}

var agentGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get a registered agent by ID or name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		result, err := agent.Get(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		return output.Print(result, jsonFlag(cmd))
	},
}

var agentDeregisterCmd = &cobra.Command{
	Use:   "deregister <id-or-name>",
	Short: "Deregister an agent by ID or name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		err := agent.Deregister(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			type okResult struct {
				OK bool `json:"ok"`
			}
			return output.Print(okResult{OK: true}, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deregistered agent %s\n", args[0])
		return nil
	},
}
