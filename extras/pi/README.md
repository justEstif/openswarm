# openswarm — pi coding agent extension

Auto-initialises swarm and adds `/swarm-status` and `/swarm-prompt` commands.

## What it does

- **Auto-init**: runs `swarm init` on `session_start` (idempotent)
- **`/swarm-status`**: shows agents, tasks, unread messages, and active runs in a notification
- **`/swarm-prompt`**: runs `swarm prompt` and injects the output as a user message, so the agent gets full coordination context

## Install

```bash
# Global (all projects)
cp extras/pi/extension.ts ~/.pi/agent/extensions/openswarm.ts

# Project-local
mkdir -p .pi/extensions
cp extras/pi/extension.ts .pi/extensions/openswarm.ts
```

Extensions in those directories are auto-discovered at startup. Reload without restarting with `/reload`.

## Requirements

- pi coding agent
- `swarm` binary on `$PATH`
