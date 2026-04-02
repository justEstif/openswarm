---
# openswarm-hlen
title: Run result persistence and task linkage
status: todo
type: feature
priority: normal
created_at: 2026-04-02T20:51:13Z
updated_at: 2026-04-02T20:51:13Z
parent: openswarm-jf3x
blocked_by:
    - openswarm-pgko
---

Extend Task schema with result_run_id (optional). swarm task done <id> --run <run-id> sets the field. swarm task show <id> inlines the run result when result_run_id is set. swarm prompt --mode coordinator includes a section listing tasks with pending/completed results for session-resume awareness.
