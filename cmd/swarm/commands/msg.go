package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/justEstif/openswarm/internal/msg"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/spf13/cobra"
)

// MsgCmd is the `swarm msg` group command.
var MsgCmd = &cobra.Command{
	Use:   "msg",
	Short: "Send and receive agent messages",
	Long:  "Send messages between agents, inspect inboxes, and watch for new arrivals.",
}

func init() {
	// swarm msg send
	msgSendCmd.Flags().String("subject", "", "Message subject (required)")
	msgSendCmd.Flags().String("body", "", "Message body (required)")
	msgSendCmd.Flags().String("from", defaultFrom(), "Sender agent ID or name")
	_ = msgSendCmd.MarkFlagRequired("subject")
	_ = msgSendCmd.MarkFlagRequired("body")

	// swarm msg inbox
	msgInboxCmd.Flags().Bool("unread", false, "Show only unread messages")

	// swarm msg reply
	msgReplyCmd.Flags().String("body", "", "Reply body (required)")
	_ = msgReplyCmd.MarkFlagRequired("body")

	MsgCmd.AddCommand(
		msgSendCmd,
		msgInboxCmd,
		msgReadCmd,
		msgReplyCmd,
		msgClearCmd,
		msgWatchCmd,
	)
}

// defaultFrom returns $SWARM_AGENT or "cli" if unset.
func defaultFrom() string {
	if v := os.Getenv("SWARM_AGENT"); v != "" {
		return v
	}
	return "cli"
}

// ─── swarm msg send ──────────────────────────────────────────────────────────

var msgSendCmd = &cobra.Command{
	Use:   "send <recipient>",
	Short: "Send a message to an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		recipient := args[0]
		subject, _ := cmd.Flags().GetString("subject")
		body, _ := cmd.Flags().GetString("body")
		from, _ := cmd.Flags().GetString("from")

		result, err := msg.Send(root, from, recipient, subject, body)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(result, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Sent message %s to %s\n", result.ID, recipient)
		return nil
	},
}

// ─── swarm msg inbox ─────────────────────────────────────────────────────────

// msgInboxRow is the subset of Message fields shown in the human-readable table.
type msgInboxRow struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	Subject   string `json:"subject"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

var msgInboxCmd = &cobra.Command{
	Use:   "inbox <agent>",
	Short: "Show an agent's inbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		unread, _ := cmd.Flags().GetBool("unread")

		results, err := msg.Inbox(root, args[0], unread)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(results, true)
		}
		rows := make([]msgInboxRow, len(results))
		for i, m := range results {
			rows[i] = msgInboxRow{
				ID:        m.ID,
				From:      m.From,
				Subject:   m.Subject,
				Read:      m.Read,
				CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
			}
		}
		return output.Print(rows, false)
	},
}

// ─── swarm msg read ───────────────────────────────────────────────────────────

var msgReadCmd = &cobra.Command{
	Use:   "read <agent> <msg-id>",
	Short: "Read a message (marks it as read)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		agentArg, msgID := args[0], args[1]

		result, err := msg.Read(root, agentArg, msgID)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(result, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Subject: %s\n\n%s\n", result.Subject, result.Body)
		return nil
	},
}

// ─── swarm msg reply ──────────────────────────────────────────────────────────

var msgReplyCmd = &cobra.Command{
	Use:   "reply <agent> <msg-id>",
	Short: "Reply to a message",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		agentArg, msgID := args[0], args[1]
		body, _ := cmd.Flags().GetString("body")

		result, err := msg.Reply(root, agentArg, msgID, body)
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			return output.Print(result, true)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Sent reply %s\n", result.ID)
		return nil
	},
}

// ─── swarm msg clear ──────────────────────────────────────────────────────────

var msgClearCmd = &cobra.Command{
	Use:   "clear <agent>",
	Short: "Clear read messages from an agent's inbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)

		n, err := msg.Clear(root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		if jsonFlag(cmd) {
			data, _ := json.Marshal(map[string]int{"cleared": n})
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Cleared %d message(s)\n", n)
		return nil
	},
}

// ─── swarm msg watch ──────────────────────────────────────────────────────────

var msgWatchCmd = &cobra.Command{
	Use:   "watch <agent>",
	Short: "Watch an agent's inbox for new messages (blocks until Ctrl-C)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, _ := mustRoot(cmd)
		ctx := cmd.Context()

		ch, err := msg.Watch(ctx, root, args[0])
		if err != nil {
			output.PrintError(err, jsonFlag(cmd))
			return nil
		}
		for m := range ch {
			if jsonFlag(cmd) {
				data, _ := json.Marshal(m)
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", data)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] from %s: %s\n", m.ID, m.From, m.Subject)
			}
		}
		return nil
	},
}
