---
# openswarm-zgnu
title: internal/events — append-only event log
status: completed
type: task
priority: normal
created_at: 2026-03-30T12:54:57Z
updated_at: 2026-03-30T12:57:31Z
---

Implement internal/events package with Append and Tail functions. Depends on internal/swarmfs only.

## Summary of Changes

Delivered `internal/events/events.go` and `internal/events/events_test.go`.

### events.go
- `Event` struct with JSON tags (`id`, `source`, `type`, `ref`, `data`, `at`)
- 22 event type constants covering agent/task/msg/pane/run/worktree domains
- `Append(root, eventType, source, ref, data)` — marshals optional payload, generates `evt-xxxxxx` ID via `swarmfs.NewID`, sets UTC timestamp, delegates to `swarmfs.AppendLine`
- `Tail(ctx, root, filter)` — goroutine-based streaming; reads existing bytes via `drainFrom` (offset-tracked, bufio.Scanner), polls every 200 ms for new lines, closes channel on ctx cancel; filter is `strings.Contains` on `Type`
- `touchFile` helper ensures events.jsonl and parent dir exist before opening

### events_test.go (12 tests, all pass with -race)
- `TestAppend_WritesValidJSONLine` — valid JSON, correct fields, evt- prefix
- `TestAppend_WithData` — payload round-trips through json.RawMessage
- `TestAppend_MultipleLines` — 3 events, 3 lines, correct order
- `TestAppend_IDsAreUnique` — 20 IDs, no duplicates
- `TestTail_ReadsExistingEvents` — pre-written events consumed in order
- `TestTail_StreamsNewEvents` — events appended after Tail starts are received
- `TestTail_ExistingAndNew` — both existing and new events delivered
- `TestTail_Filter` — only matching events emitted
- `TestTail_ChannelClosedOnCancel` — channel closes promptly on ctx cancel
- `TestTail_EmptyFile` — Tail on empty log, event appended later is received
- `TestEventTypeConstants` — all 22 constants non-empty, dot-separated, unique
- `TestEventsPath` — path under temp dir, filename = events.jsonl
