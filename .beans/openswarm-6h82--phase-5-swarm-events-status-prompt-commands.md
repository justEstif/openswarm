---
# openswarm-6h82
title: 'Phase 5: swarm events, status, prompt commands'
status: completed
type: task
priority: normal
created_at: 2026-03-30T16:14:21Z
updated_at: 2026-03-30T16:15:18Z
---

Implement cmd/swarm/commands/events.go, status.go, prompt.go

## Summary of Changes\n\n- cmd/swarm/commands/events.go: EventsCmd + eventsTailCmd with --filter, --n, --follow flags\n- cmd/swarm/commands/status.go: StatusCmd aggregating agents/tasks/unread/runs, graceful pane fallback\n- cmd/swarm/commands/prompt.go: PromptCmd delegating to task.Prompt()\n\nBoth go build and go vet pass clean.
