---
# openswarm-g5y7
title: swarm run wait --all for parallel fan-out/join
status: todo
type: feature
priority: high
created_at: 2026-04-02T20:51:25Z
updated_at: 2026-04-02T20:51:25Z
parent: openswarm-ab7c
blocked_by:
    - openswarm-pgko
---

swarm run wait --all <id1> <id2> ... blocks until all named runs complete; prints JSON array of Result records in completion order. swarm run wait --any <id1> <id2> ... (bonus) returns first completion. --timeout <duration> flag on all wait variants. Implementation: goroutine per run, channel-merge, context cancellation. Coordinators pipe to jq to extract individual results. Closes the gap with Claude Code's implicit parallel AgentTool tracking.
