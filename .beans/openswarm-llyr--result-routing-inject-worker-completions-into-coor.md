---
# openswarm-llyr
title: 'Result routing: inject worker completions into coordinator inbox'
status: todo
type: feature
priority: high
created_at: 2026-04-02T20:50:34Z
updated_at: 2026-04-02T20:50:34Z
parent: openswarm-swkh
blocked_by:
    - openswarm-pgko
---

Add --notify <agent> to swarm run start. When run.Wait() resolves, auto-call msg.Send with subject 'run.complete:<run-id>' and a JSON body matching the Result struct. Coordinators filter inbox by subject; no polling required. Extras skill passes --notify $SWARM_AGENT_ID automatically. Closes the gap with Claude Code's user-role <task-notification> delivery.
