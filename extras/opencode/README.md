# openswarm — opencode plugin

Auto-initialises swarm and keeps coordination state alive across compactions.

## What it does

- **Auto-init**: runs `swarm init` when opencode starts (idempotent)
- **Compaction hook**: appends `swarm prompt` output to the compaction context, so agent task/message state survives context resets

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

## Requirements

- opencode with plugin support
- `swarm` binary on `$PATH`
