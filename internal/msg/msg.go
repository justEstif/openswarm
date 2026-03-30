// Package msg implements the openswarm messaging subsystem.
//
// Messages are persisted as individual JSON files under:
//
//	.swarm/messages/<agentID>/inbox/<msgID>.json
//
// # Design
//
// Sends are lock-free: each Send is a pure atomic file write of a unique new
// file. No flock is acquired on Send. Concurrent senders never contend with
// each other or with readers. Only destructive operations (Clear) acquire a
// per-agent flock to prevent races between concurrent deletions.
//
// Reads (Inbox, UnreadCount, Read, Reply, Watch) enumerate the inbox directory
// and parse each .json file. Sorting by CreatedAt makes ordering deterministic.
//
// Every mutating operation emits a corresponding event via the events package.
package msg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/agent"
	"github.com/justEstif/openswarm/internal/events"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Message is a single message in an agent's inbox.
type Message struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`               // sender agent ID or name
	To        string    `json:"to"`                 // recipient agent ID
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	ReplyTo   string    `json:"reply_to,omitempty"` // ID of message being replied to
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// resolveAgentID resolves agentIDOrName to an Agent.ID using agent.Get.
func resolveAgentID(root *swarmfs.Root, agentIDOrName string) (string, error) {
	a, err := agent.Get(root, agentIDOrName)
	if err != nil {
		return "", err
	}
	return a.ID, nil
}

// inboxDir returns the inbox directory for a given agent ID (no trailing sep).
func inboxDir(root *swarmfs.Root, agentID string) string {
	// root.InboxPath returns the path with a trailing separator; trim it so
	// filepath functions work consistently.
	return strings.TrimRight(root.InboxPath(agentID), string(filepath.Separator))
}

// msgPath returns the full path to a single message file.
func msgPath(root *swarmfs.Root, agentID, msgID string) string {
	return filepath.Join(inboxDir(root, agentID), msgID+".json")
}

// lockPath returns the per-agent flock file path (sibling of inbox/).
func lockPath(root *swarmfs.Root, agentID string) string {
	return filepath.Join(filepath.Dir(inboxDir(root, agentID)), ".lock")
}

// writeMsg marshals msg and atomically writes it to its canonical path.
func writeMsg(root *swarmfs.Root, agentID string, m *Message) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("msg: marshal message: %w", err)
	}
	if err := swarmfs.AtomicWrite(msgPath(root, agentID, m.ID), data); err != nil {
		return fmt.Errorf("msg: write message: %w", err)
	}
	return nil
}

// readMsg reads and unmarshals the message file for msgID from agentID's inbox.
// Returns output.ErrNotFound if the file does not exist.
func readMsg(root *swarmfs.Root, agentID, msgID string) (*Message, error) {
	path := msgPath(root, agentID, msgID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, output.ErrNotFound(fmt.Sprintf("message %q not found", msgID))
		}
		return nil, fmt.Errorf("msg: read message %q: %w", msgID, err)
	}
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("msg: unmarshal message %q: %w", msgID, err)
	}
	return &m, nil
}

// listMessages returns all messages in agentID's inbox, sorted by CreatedAt.
// If unreadOnly is true, only messages with Read==false are returned.
func listMessages(root *swarmfs.Root, agentID string, unreadOnly bool) ([]*Message, error) {
	dir := inboxDir(root, agentID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Message{}, nil
		}
		return nil, fmt.Errorf("msg: read inbox dir %q: %w", dir, err)
	}

	var msgs []*Message
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		msgID := strings.TrimSuffix(e.Name(), ".json")
		m, err := readMsg(root, agentID, msgID)
		if err != nil {
			// Skip corrupt files rather than aborting the whole Inbox call.
			continue
		}

		if unreadOnly && m.Read {
			continue
		}
		msgs = append(msgs, m)
	}

	sort.Slice(msgs, func(i, j int) bool {
		if msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
			return msgs[i].ID < msgs[j].ID
		}
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})

	if msgs == nil {
		msgs = []*Message{}
	}
	return msgs, nil
}

// ─── Public API ───────────────────────────────────────────────────────────────

// Send delivers a message to recipient's inbox as an atomic file write.
// recipient may be an agent ID or name; it is resolved to an ID via agent.Get.
// Send is lock-free — each message occupies its own file, so concurrent sends
// never contend. Emits events.TypeMsgSent on success.
func Send(root *swarmfs.Root, from, recipient, subject, body string) (*Message, error) {
	toID, err := resolveAgentID(root, recipient)
	if err != nil {
		return nil, err
	}

	m := &Message{
		ID:        swarmfs.NewID("msg"),
		From:      from,
		To:        toID,
		Subject:   subject,
		Body:      body,
		Read:      false,
		CreatedAt: time.Now().UTC(),
	}

	// Ensure inbox dir exists before writing (AtomicWrite also does MkdirAll,
	// but being explicit is clearer).
	dir := inboxDir(root, toID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("msg: create inbox dir: %w", err)
	}

	if err := writeMsg(root, toID, m); err != nil {
		return nil, err
	}

	if err := events.Append(root, events.TypeMsgSent, "msg", m.ID, map[string]string{
		"from":    m.From,
		"to":      m.To,
		"subject": m.Subject,
	}); err != nil {
		// Non-fatal: message was delivered; report the event failure.
		return m, fmt.Errorf("msg: event append: %w", err)
	}

	return m, nil
}

// Inbox returns all messages in agentIDOrName's inbox, sorted by CreatedAt
// ascending. If unreadOnly is true, only unread messages are returned.
// agentIDOrName is resolved to an agent ID via agent.Get.
func Inbox(root *swarmfs.Root, agentIDOrName string, unreadOnly bool) ([]*Message, error) {
	agentID, err := resolveAgentID(root, agentIDOrName)
	if err != nil {
		return nil, err
	}
	return listMessages(root, agentID, unreadOnly)
}

// Read marks a message as read (sets Read=true) and atomically rewrites the
// file. Returns output.ErrNotFound if the message does not exist.
// agentIDOrName is resolved to an agent ID via agent.Get.
// Emits events.TypeMsgRead on success.
func Read(root *swarmfs.Root, agentIDOrName, msgID string) (*Message, error) {
	agentID, err := resolveAgentID(root, agentIDOrName)
	if err != nil {
		return nil, err
	}

	m, err := readMsg(root, agentID, msgID)
	if err != nil {
		return nil, err
	}

	if m.Read {
		// Already read — return without writing (idempotent).
		return m, nil
	}

	m.Read = true
	if err := writeMsg(root, agentID, m); err != nil {
		return nil, err
	}

	if err := events.Append(root, events.TypeMsgRead, "msg", m.ID, map[string]string{
		"to": agentID,
	}); err != nil {
		return m, fmt.Errorf("msg: event append: %w", err)
	}

	return m, nil
}

// Reply sends a new message with ReplyTo set to the original msgID.
// The reply goes back to the original sender (msg.From).
// agentIDOrName identifies the agent whose inbox contains msgID.
func Reply(root *swarmfs.Root, agentIDOrName, msgID, body string) (*Message, error) {
	agentID, err := resolveAgentID(root, agentIDOrName)
	if err != nil {
		return nil, err
	}

	orig, err := readMsg(root, agentID, msgID)
	if err != nil {
		return nil, err
	}

	// Build reply: from=agentID (the replier), to=orig.From (original sender).
	m := &Message{
		ID:        swarmfs.NewID("msg"),
		From:      agentID,
		To:        orig.From,
		Subject:   "Re: " + orig.Subject,
		Body:      body,
		ReplyTo:   msgID,
		Read:      false,
		CreatedAt: time.Now().UTC(),
	}

	// Deliver to the original sender's inbox.
	// First resolve orig.From to an agent ID (it might already be an ID, but
	// agent.Get handles both cases gracefully).
	toID, err := resolveAgentID(root, orig.From)
	if err != nil {
		// If orig.From can't be resolved (e.g. sender was deregistered), still
		// attempt delivery using orig.From as-is for the path — best-effort.
		toID = orig.From
	}
	m.To = toID

	dir := inboxDir(root, toID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("msg: create inbox dir for reply: %w", err)
	}

	if err := writeMsg(root, toID, m); err != nil {
		return nil, err
	}

	if err := events.Append(root, events.TypeMsgSent, "msg", m.ID, map[string]string{
		"from":     m.From,
		"to":       m.To,
		"reply_to": m.ReplyTo,
	}); err != nil {
		return m, fmt.Errorf("msg: event append: %w", err)
	}

	return m, nil
}

// Clear deletes all read messages from agentIDOrName's inbox.
// It acquires a per-agent flock to prevent concurrent partial deletions.
// Returns the count of deleted messages.
func Clear(root *swarmfs.Root, agentIDOrName string) (int, error) {
	agentID, err := resolveAgentID(root, agentIDOrName)
	if err != nil {
		return 0, err
	}

	lock := lockPath(root, agentID)
	dir := inboxDir(root, agentID)

	var deleted int
	err = swarmfs.WithFileLock(lock, func() error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("msg: read inbox dir: %w", err)
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}

			msgID := strings.TrimSuffix(e.Name(), ".json")
			m, err := readMsg(root, agentID, msgID)
			if err != nil {
				continue // skip unreadable files
			}

			if !m.Read {
				continue
			}

			if err := os.Remove(msgPath(root, agentID, msgID)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("msg: remove message %q: %w", msgID, err)
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return deleted, nil
}

// UnreadCount returns the number of unread messages for agentIDOrName.
func UnreadCount(root *swarmfs.Root, agentIDOrName string) (int, error) {
	msgs, err := Inbox(root, agentIDOrName, true /* unreadOnly */)
	if err != nil {
		return 0, err
	}
	return len(msgs), nil
}

// Watch streams new messages for agentIDOrName by polling the inbox directory
// every 200 ms. Only messages that arrive AFTER Watch is called are emitted
// (the initial set of files is snapshotted and ignored). The returned channel
// is closed when ctx is cancelled.
func Watch(ctx context.Context, root *swarmfs.Root, agentIDOrName string) (<-chan *Message, error) {
	agentID, err := resolveAgentID(root, agentIDOrName)
	if err != nil {
		return nil, err
	}

	dir := inboxDir(root, agentID)

	// Ensure the inbox dir exists so we can read it immediately.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("msg: create inbox dir for watch: %w", err)
	}

	// Snapshot the set of files already present.
	seen := make(map[string]struct{})
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			seen[e.Name()] = struct{}{}
		}
	}

	ch := make(chan *Message, 16)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				entries, err := os.ReadDir(dir)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return
				}

				for _, e := range entries {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}
					if _, already := seen[e.Name()]; already {
						continue
					}

					seen[e.Name()] = struct{}{}

					msgID := strings.TrimSuffix(e.Name(), ".json")
					m, err := readMsg(root, agentID, msgID)
					if err != nil {
						continue
					}

					select {
					case ch <- m:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}
