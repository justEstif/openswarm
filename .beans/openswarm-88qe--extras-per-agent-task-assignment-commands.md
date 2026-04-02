---
# openswarm-88qe
title: 'Extras: per-agent task assignment commands'
status: in-progress
type: feature
priority: normal
created_at: 2026-04-02T13:15:47Z
updated_at: 2026-04-02T13:38:01Z
---

Add agent-specific extras that teach each coding agent (pi, opencode, claude code) how to spawn a sub-agent and assign itself to a swarm task.

Each `extras/<agent>/` directory should gain a command/hook that, when invoked, registers the agent with the swarm, claims the target task, then spawns a sub-agent to do the actual work — using that agent's headless CLI.

## Tasks

- [x] `extras/pi/` — added `/assign-task` command to openswarm.ts (detached `pi --print` sub-agent)
- [x] `extras/opencode/` — added `commands/assign-task.md` (`subtask: true`, Bun shell)
- [x] `extras/claude-code/` — added `commands/assign-task.md` (slash command with `$ARGUMENTS`)
- [x] Update `extras/README.md` with per-agent assignment instructions
- [x] Update `lat.md/extras.md`

## Per-agent headless invocation

### pi

```
---
description: Claim a swarm task and run a sub-agent to complete it
---
# Usage: /assign-task <task-id> [--provider <provider> --model <model>]
#
# Registers you as an agent, claims the given task, then spawns a sub-agent
# via `pi --print` to complete it.

TASK_ID="$1"; shift
AGENT_NAME=$(swarm agent register "$(hostname)" --role worker --json | jq -r .id)
swarm task claim "$TASK_ID" --as "$AGENT_NAME"
pi --print "$@" "You have been assigned swarm task $TASK_ID. Run \`swarm task get $TASK_ID\` to read its description, complete the work, then call \`swarm task done $TASK_ID\`."
```

Use `--provider` and `--model` to override the model, e.g. `--provider anthropic --model claude-opus-4-5`.

### opencode

```
---
description: Claim a swarm task and run a sub-agent to complete it
---
# Usage: /assign-task <task-id> [--model <provider/model>]
#
# Registers you as an agent, claims the given task, then spawns a sub-agent
# via `opencode run` to complete it.

TASK_ID="$1"; shift
AGENT_NAME=$(swarm agent register "$(hostname)" --role worker --json | jq -r .id)
swarm task claim "$TASK_ID" --as "$AGENT_NAME"
opencode run "$@" "You have been assigned swarm task $TASK_ID. Run \`swarm task get $TASK_ID\` to read its description, complete the work, then call \`swarm task done $TASK_ID\`."
```

Use `--model provider/model` to override the model, e.g. `--model anthropic/claude-opus-4-5`.

### claude code

```
---
description: Claim a swarm task and run a sub-agent to complete it
---
# Usage: /assign-task <task-id> [--model <model>]
#
# Registers you as an agent, claims the given task, then spawns a sub-agent
# via `claude -p` to complete it.

TASK_ID="$1"; shift
AGENT_NAME=$(swarm agent register "$(hostname)" --role worker --json | jq -r .id)
swarm task claim "$TASK_ID" --as "$AGENT_NAME"
claude -p "$@" "You have been assigned swarm task $TASK_ID. Run \`swarm task get $TASK_ID\` to read its description, complete the work, then call \`swarm task done $TASK_ID\`."
```

Use `--model` to override the model, e.g. `--model claude-opus-4-5`.
