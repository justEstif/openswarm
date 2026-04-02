# openswarm — Claude Code integration

Two pieces: a **skill** that teaches Claude how to use `swarm`, and a **hook** that auto-initialises swarm at session start.

## Skill

The universal `extras/skills/openswarm/SKILL.md` works as a Claude Code skill. Copy or symlink it into your project or user skill directory:

```bash
# Project-level (committed, shared with team)
mkdir -p .claude/skills/openswarm
cp extras/skills/openswarm/SKILL.md .claude/skills/openswarm/

# User-level (all your projects)
mkdir -p ~/.claude/skills/openswarm
cp extras/skills/openswarm/SKILL.md ~/.claude/skills/openswarm/
```

Claude will auto-load the skill description at session start and read the full instructions when it needs to use `swarm`.

## Hook — auto-init

The `settings.json` in this directory drops a `SessionStart` hook that runs `swarm init` whenever Claude Code starts in a project. This is idempotent — if `.swarm/` already exists, it does nothing.

```bash
# Project-level (committed)
mkdir -p .claude
cp extras/claude-code/settings.json .claude/settings.json

# Or merge with an existing .claude/settings.json manually.
```

The hook fires only on `startup` (not `resume` or `compact`), so it won't re-run on every context window reload.

## Command — /assign-task

`extras/claude-code/commands/assign-task.md` is a Claude Code slash command that registers the current session as a swarm worker, claims a task, completes the work, and marks it done.

```bash
# Per-project
mkdir -p .claude/commands
cp extras/claude-code/commands/assign-task.md .claude/commands/

# User-level (all projects)
mkdir -p ~/.claude/commands
cp extras/claude-code/commands/assign-task.md ~/.claude/commands/
```

Then inside Claude Code:

```
/assign-task task-abc123
```
