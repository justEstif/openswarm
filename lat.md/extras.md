# Extras

Supplementary integrations that live under `extras/` and make openswarm easier to adopt from a coding agent session. These are opt-in — the core CLI works without them.

## Universal Skill

`extras/skills/openswarm/SKILL.md` — teaches any SKILL.md-compatible agent the full `swarm` CLI. Works with Claude Code, pi, and others.

Follows progressive disclosure: agents load only the description at startup and read the full file when relevant. Covers command syntax, workflows, and the `.swarm/` state layout.

## Claude Code

`extras/claude-code/settings.json` — a `SessionStart` hook that runs `swarm init` on every project startup. Drop it into `.claude/settings.json`.

Fires only on `startup` (not `resume` or `compact`). The universal skill can be installed under `.claude/skills/openswarm/` or `~/.claude/skills/openswarm/`.

## opencode Plugin

`extras/opencode/index.ts` is an opencode plugin that:

1. Runs `swarm init` on plugin load (auto-init on every session start)
2. Hooks into `experimental.session.compacting` to append `swarm prompt` output to the compaction context — agents retain coordination state across context resets

Install by copying to `~/.config/opencode/plugins/` or `.opencode/plugins/`, or reference directly in `opencode.json`.

## pi Extension

`extras/pi/extension.ts` is a pi coding agent extension that:

1. Runs `swarm init` on `session_start`
2. Registers `/swarm-status` — shows agents, tasks, unread, active runs in a notification
3. Registers `/swarm-prompt` — runs `swarm prompt` and injects the output as a user message via `pi.sendUserMessage()`, giving the agent full coordination context on demand

Install by copying to `~/.pi/agent/extensions/` (global) or `.pi/extensions/` (project-local).

## Multiplexer Note

`swarm msg`, `swarm task`, `swarm agent`, `swarm events`, and `swarm status` work without any multiplexer. Only `swarm pane` and `swarm run` require one. [[internal/pane/detect.go#DetectBackend]] returns a clear error if none is detected.
