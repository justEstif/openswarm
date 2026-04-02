---
# openswarm-pgko
title: Structured task-completion envelope
status: todo
type: feature
priority: critical
created_at: 2026-04-02T20:50:28Z
updated_at: 2026-04-02T20:50:28Z
parent: openswarm-swkh
---

Add a Result struct to the run package carrying status, summary, output, tokens, tool_uses, duration_ms. Workers emit <swarm-result status="completed">…</swarm-result> before exit; run.Wait() parses it. swarm run wait returns the full Result as JSON. Results stored in .swarm/runs/results/<run-id>.json (lock-free, immutable). Closes the gap with Claude Code's <task-notification> XML envelope.
