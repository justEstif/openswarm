---
# openswarm-0v7q
title: 'Phase 5: swarm events, status, prompt commands'
status: completed
type: task
priority: normal
created_at: 2026-03-30T16:14:18Z
updated_at: 2026-04-02T11:31:06Z
---

Implement cmd/swarm/commands/events.go, status.go, prompt.go

## Summary of Changes

All three Phase 5 commands were already fully implemented:
- `cmd/swarm/commands/events.go` — `swarm events tail` with follow/snapshot modes
- `cmd/swarm/commands/status.go` — `swarm status` aggregating agents, tasks, messages, runs, panes
- `cmd/swarm/commands/prompt.go` — `swarm prompt` generating agent-priming prompt from task state

All registered in `cmd/swarm/root.go`. Build and all tests pass.
