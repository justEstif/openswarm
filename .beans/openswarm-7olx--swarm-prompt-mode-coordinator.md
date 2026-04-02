---
# openswarm-7olx
title: swarm prompt --mode coordinator
status: todo
type: feature
priority: high
created_at: 2026-04-02T20:50:44Z
updated_at: 2026-04-02T20:50:44Z
parent: openswarm-wevr
---

Extend swarm prompt to emit a coordinator system prompt covering: role definition, tool inventory from agent profiles, worker spawn/continue/stop semantics, research-then-synthesize workflow, parallel fan-out pattern, verification discipline, and Active Team section from swarm agent list. swarm prompt (no flag) continues to emit worker task-state. Extras hooks call --mode coordinator on SessionStart for coordinator sessions. Closes the gap with Claude Code's getCoordinatorSystemPrompt().
