# openswarm — opencode plugin

Auto-initialises swarm, keeps coordination state alive across compactions, and adds an `/assign-task` command for spawning sub-agents.

## What it does

- **Auto-init**: runs `swarm init` when opencode starts (idempotent)
- **Compaction hook**: appends `swarm prompt` output to the compaction context, so agent task/message state survives context resets
- **`/assign-task <task-id>`**: registers the current session as a swarm worker, claims the task, and completes it as a subagent (does not pollute primary context)

## Install

### Option A — local plugin

```bash
mkdir -p ~/.config/opencode/plugins
cp extras/opencode/index.ts ~/.config/opencode/plugins/openswarm.ts
```

### Option B — project plugin

```bash
mkdir -p .opencode/plugins
cp extras/opencode/index.ts .opencode/plugins/openswarm.ts
```

No config change needed — files in the plugin directory are auto-discovered.

### Option C — reference in config

Add to `opencode.json` or `~/.config/opencode/opencode.json`:

```json
{
  "plugin": ["./extras/opencode/index.ts"]
}
```

## Install the assign-task command

```bash
# Per-project
mkdir -p .opencode/commands
cp extras/opencode/commands/assign-task.md .opencode/commands/

# Global
mkdir -p ~/.config/opencode/commands
cp extras/opencode/commands/assign-task.md ~/.config/opencode/commands/
```

Then use it inside opencode:

```
/assign-task task-abc123
```

opencode spawns a subagent (`subtask: true`) that registers itself, claims the task, completes the work, and marks it done — without touching the primary context.

## Requirements

- opencode with plugin support
- `swarm` binary on `$PATH`
- `jq` (used in the assign-task prompt)
