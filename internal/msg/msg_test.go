package msg_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/justEstif/openswarm/internal/agent"
	"github.com/justEstif/openswarm/internal/msg"
	"github.com/justEstif/openswarm/internal/output"
	"github.com/justEstif/openswarm/internal/swarmfs"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestRoot initialises a temporary .swarm/ directory and returns its Root.
func newTestRoot(t *testing.T) *swarmfs.Root {
	t.Helper()
	root, err := swarmfs.InitRoot(t.TempDir())
	if err != nil {
		t.Fatalf("InitRoot: %v", err)
	}
	return root
}

// registerAgent is a shorthand to register a named agent and return its ID.
func registerAgent(t *testing.T, root *swarmfs.Root, name string) string {
	t.Helper()
	a, err := agent.Register(root, name, "tester", "")
	if err != nil {
		t.Fatalf("Register %q: %v", name, err)
	}
	return a.ID
}

// ─── Send ─────────────────────────────────────────────────────────────────────

func TestSend_CreatesFileInRecipientInbox(t *testing.T) {
	root := newTestRoot(t)
	bobID := registerAgent(t, root, "bob")

	m, err := msg.Send(root, "alice", "bob", "hello", "world")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if m == nil {
		t.Fatal("Send returned nil message")
	}
	if !strings.HasPrefix(m.ID, "msg-") {
		t.Errorf("ID %q does not have prefix 'msg-'", m.ID)
	}
	if m.From != "alice" {
		t.Errorf("From: got %q, want %q", m.From, "alice")
	}
	if m.To != bobID {
		t.Errorf("To: got %q, want %q", m.To, bobID)
	}
	if m.Subject != "hello" {
		t.Errorf("Subject: got %q, want %q", m.Subject, "hello")
	}
	if m.Body != "world" {
		t.Errorf("Body: got %q, want %q", m.Body, "world")
	}
	if m.Read {
		t.Error("message should start unread")
	}
	if m.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Verify the file exists at the correct inbox path.
	msgs, err := msg.Inbox(root, bobID, false)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in inbox, got %d", len(msgs))
	}
	if msgs[0].ID != m.ID {
		t.Errorf("inbox message ID: got %q, want %q", msgs[0].ID, m.ID)
	}
}

func TestSend_UnknownRecipientReturnsNotFound(t *testing.T) {
	root := newTestRoot(t)

	_, err := msg.Send(root, "alice", "nobody", "hi", "there")
	if err == nil {
		t.Fatal("expected NOT_FOUND error, got nil")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("error is not *output.SwarmError: %T %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code: got %q, want NOT_FOUND", se.Code)
	}
}

func TestSend_ResolvesByName(t *testing.T) {
	root := newTestRoot(t)
	aliceID := registerAgent(t, root, "alice")

	m, err := msg.Send(root, "bot", "alice", "ping", "pong")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if m.To != aliceID {
		t.Errorf("To: got %q, want agent ID %q", m.To, aliceID)
	}
}

// ─── Inbox ────────────────────────────────────────────────────────────────────

func TestInbox_ReturnsSortedByCreatedAt(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "carol")

	// Send three messages with a small sleep to ensure distinct CreatedAt.
	for _, subj := range []string{"first", "second", "third"} {
		if _, err := msg.Send(root, "bot", "carol", subj, ""); err != nil {
			t.Fatalf("Send %q: %v", subj, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	msgs, err := msg.Inbox(root, "carol", false)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Subject != "first" || msgs[1].Subject != "second" || msgs[2].Subject != "third" {
		t.Errorf("order wrong: %v", []string{msgs[0].Subject, msgs[1].Subject, msgs[2].Subject})
	}
}

func TestInbox_UnreadOnlyFiltersRead(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "dave")

	m1, _ := msg.Send(root, "bot", "dave", "unread1", "")
	m2, _ := msg.Send(root, "bot", "dave", "unread2", "")

	// Mark m1 as read.
	if _, err := msg.Read(root, "dave", m1.ID); err != nil {
		t.Fatalf("Read: %v", err)
	}

	all, err := msg.Inbox(root, "dave", false)
	if err != nil {
		t.Fatalf("Inbox(all): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 total, got %d", len(all))
	}

	unread, err := msg.Inbox(root, "dave", true)
	if err != nil {
		t.Fatalf("Inbox(unreadOnly): %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("expected 1 unread, got %d", len(unread))
	}
	if unread[0].ID != m2.ID {
		t.Errorf("unread message ID: got %q, want %q", unread[0].ID, m2.ID)
	}
}

func TestInbox_EmptyWhenNoMessages(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "eve")

	msgs, err := msg.Inbox(root, "eve", false)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func TestRead_MarksMessageAsRead(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "frank")

	sent, _ := msg.Send(root, "bot", "frank", "subject", "body")

	m, err := msg.Read(root, "frank", sent.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !m.Read {
		t.Error("Read should have set Read=true")
	}

	// Verify persistence: re-fetch from disk.
	all, err := msg.Inbox(root, "frank", false)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(all) != 1 || !all[0].Read {
		t.Error("message should be marked read on disk")
	}
}

func TestRead_IdempotentWhenAlreadyRead(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "grace")

	sent, _ := msg.Send(root, "bot", "grace", "s", "b")
	msg.Read(root, "grace", sent.ID) //nolint:errcheck

	// Second Read should not error.
	m, err := msg.Read(root, "grace", sent.ID)
	if err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if !m.Read {
		t.Error("still should be read")
	}
}

func TestRead_NotFoundReturnsError(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "heidi")

	_, err := msg.Read(root, "heidi", "msg-nonexistent")
	if err == nil {
		t.Fatal("expected NOT_FOUND error, got nil")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("error is not *output.SwarmError: %T %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code: got %q, want NOT_FOUND", se.Code)
	}
}

// ─── Reply ────────────────────────────────────────────────────────────────────

func TestReply_SetsReplyToAndGoesBackToSender(t *testing.T) {
	root := newTestRoot(t)
	aliceID := registerAgent(t, root, "alice")
	registerAgent(t, root, "bob")

	// alice sends to bob
	orig, err := msg.Send(root, aliceID, "bob", "question", "what time is it?")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// bob replies
	reply, err := msg.Reply(root, "bob", orig.ID, "it's noon")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if reply.ReplyTo != orig.ID {
		t.Errorf("ReplyTo: got %q, want %q", reply.ReplyTo, orig.ID)
	}
	if reply.To != aliceID {
		t.Errorf("To: got %q (should go back to alice=%q)", reply.To, aliceID)
	}
	if reply.Body != "it's noon" {
		t.Errorf("Body: got %q", reply.Body)
	}
	if !strings.HasPrefix(reply.Subject, "Re: ") {
		t.Errorf("Subject should start with 'Re: ', got %q", reply.Subject)
	}

	// The reply should appear in alice's inbox.
	aliceInbox, err := msg.Inbox(root, "alice", false)
	if err != nil {
		t.Fatalf("Inbox(alice): %v", err)
	}
	found := false
	for _, m := range aliceInbox {
		if m.ID == reply.ID {
			found = true
		}
	}
	if !found {
		t.Error("reply not found in alice's inbox")
	}
}

func TestReply_OriginalMessageNotFound(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "ivan")

	_, err := msg.Reply(root, "ivan", "msg-ghost", "hello?")
	if err == nil {
		t.Fatal("expected NOT_FOUND error, got nil")
	}
	var se *output.SwarmError
	if !errors.As(err, &se) {
		t.Fatalf("error is not *output.SwarmError: %T %v", err, err)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code: got %q, want NOT_FOUND", se.Code)
	}
}

// ─── Clear ────────────────────────────────────────────────────────────────────

func TestClear_RemovesOnlyReadMessages(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "judy")

	m1, _ := msg.Send(root, "bot", "judy", "read-me", "")
	m2, _ := msg.Send(root, "bot", "judy", "keep-me", "")

	msg.Read(root, "judy", m1.ID) //nolint:errcheck

	n, err := msg.Clear(root, "judy")
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if n != 1 {
		t.Errorf("Clear deleted %d, want 1", n)
	}

	// m2 should still be in the inbox.
	remaining, err := msg.Inbox(root, "judy", false)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining message, got %d", len(remaining))
	}
	if remaining[0].ID != m2.ID {
		t.Errorf("remaining message ID: got %q, want %q", remaining[0].ID, m2.ID)
	}
}

func TestClear_EmptyInboxReturnsZero(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "kate")

	n, err := msg.Clear(root, "kate")
	if err != nil {
		t.Fatalf("Clear on empty inbox: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestClear_NoReadMessagesReturnsZero(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "leo")

	msg.Send(root, "bot", "leo", "unread", "") //nolint:errcheck
	msg.Send(root, "bot", "leo", "unread2", "") //nolint:errcheck

	n, err := msg.Clear(root, "leo")
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 deletions, got %d", n)
	}

	all, _ := msg.Inbox(root, "leo", false)
	if len(all) != 2 {
		t.Errorf("expected 2 messages to remain, got %d", len(all))
	}
}

// ─── UnreadCount ──────────────────────────────────────────────────────────────

func TestUnreadCount(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "mia")

	if n, err := msg.UnreadCount(root, "mia"); err != nil || n != 0 {
		t.Fatalf("initial UnreadCount: got (%d, %v), want (0, nil)", n, err)
	}

	m1, _ := msg.Send(root, "bot", "mia", "s1", "")
	msg.Send(root, "bot", "mia", "s2", "") //nolint:errcheck
	msg.Send(root, "bot", "mia", "s3", "") //nolint:errcheck

	if n, err := msg.UnreadCount(root, "mia"); err != nil || n != 3 {
		t.Fatalf("UnreadCount after 3 sends: got (%d, %v), want (3, nil)", n, err)
	}

	msg.Read(root, "mia", m1.ID) //nolint:errcheck

	if n, err := msg.UnreadCount(root, "mia"); err != nil || n != 2 {
		t.Fatalf("UnreadCount after 1 read: got (%d, %v), want (2, nil)", n, err)
	}
}

// ─── Watch ────────────────────────────────────────────────────────────────────

func TestWatch_DetectsNewMessages(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "nina")

	// Send a message BEFORE calling Watch — it should NOT be emitted.
	msg.Send(root, "bot", "nina", "old", "pre-watch") //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := msg.Watch(ctx, root, "nina")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Send two messages AFTER Watch is established.
	time.Sleep(50 * time.Millisecond) // slight pause so Watch snapshots the existing file
	want1, _ := msg.Send(root, "bot", "nina", "new1", "body1")
	want2, _ := msg.Send(root, "bot", "nina", "new2", "body2")

	got := make(map[string]bool)
	for len(got) < 2 {
		select {
		case m, ok := <-ch:
			if !ok {
				t.Fatal("Watch channel closed before receiving all messages")
			}
			got[m.ID] = true
		case <-ctx.Done():
			t.Fatalf("Watch timed out; received %d/2 messages", len(got))
		}
	}

	if !got[want1.ID] {
		t.Errorf("did not receive message %q", want1.ID)
	}
	if !got[want2.ID] {
		t.Errorf("did not receive message %q", want2.ID)
	}
}

func TestWatch_ChannelClosesOnContextCancel(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "omar")

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := msg.Watch(ctx, root, "omar")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Cancel immediately and verify the channel closes.
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after ctx cancel")
		}
	case <-time.After(2 * time.Second):
		t.Error("channel did not close within 2 seconds after ctx cancel")
	}
}

func TestWatch_IgnoresPreExistingMessages(t *testing.T) {
	root := newTestRoot(t)
	registerAgent(t, root, "petra")

	// Send a message before Watch.
	msg.Send(root, "bot", "petra", "pre", "existing") //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	ch, err := msg.Watch(ctx, root, "petra")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Don't send any new messages. Channel should drain without any messages.
	var received []*msg.Message
	for m := range ch {
		received = append(received, m)
	}

	if len(received) != 0 {
		t.Errorf("Watch emitted %d pre-existing messages, want 0", len(received))
	}
}
