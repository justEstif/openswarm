---
# openswarm-b095
title: internal/msg — messaging subsystem
status: completed
type: task
priority: normal
created_at: 2026-03-30T14:33:16Z
updated_at: 2026-03-30T14:35:22Z
---

Implement internal/msg package with Send, Inbox, Read, Reply, Clear, UnreadCount, Watch functions. Lock-free sends, one file per message.

## Summary of Changes

Implemented internal/msg package:

- **msg.go**: Message struct + 7 public functions (Send, Inbox, Read, Reply, Clear, UnreadCount, Watch)
  - Lock-free sends via atomic file write (one file per message)
  - Clear uses swarmfs.WithFileLock per-agent
  - Watch polls inbox dir every 200ms, snapshots initial state, emits only new arrivals
  - agent.Get used throughout to resolve agentIDOrName → Agent.ID

- **msg_test.go**: 18 tests covering all functions
  - Send: creates file in correct inbox, resolves by name, unknown recipient error
  - Inbox: sorted order, unreadOnly filter, empty inbox
  - Read: marks read, idempotent, not found error
  - Reply: ReplyTo set, routes back to sender, not found error
  - Clear: removes only read messages, empty inbox, no-read-messages
  - UnreadCount: count updates correctly
  - Watch: detects new files, closes on ctx cancel, ignores pre-existing
