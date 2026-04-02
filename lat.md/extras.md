# Extras

Supplementary integrations that live under `extras/` and make openswarm easier to adopt from a coding agent session. These are opt-in — the core CLI works without them.

## Universal Skill

`extras/skills/openswarm/SKILL.md` — teaches any SKILL.md-compatible agent the full `swarm` CLI. Works with Claude Code, pi, and others.

## Config Example

`extras/config.toml` — a fully-commented reference covering every config key.

Copy to `.swarm/config.toml` (project) or `~/.config/swarm/config.toml` (global). Covers `team_name`, `default_agent`, `backend`, `poll_interval`, `[pane] placement`, and `[[agent]]` profiles.

Follows progressive disclosure: agents load only the description at startup and read the full file when relevant. Covers command syntax, workflows, and the `.swarm/` state layout.

> **Note on `msg` commands:** `msg send` requires `--subject` and `--body` flags (no positional text). `msg read` and `msg reply` both take `<agent> <msg-id>` — the agent argument is required for inbox routing.

> **Note on `swarm run`:** Non-blocking by default since PATH-inherit fix (v0.1.2). Use `--wait` to block. PATH is automatically forwarded from the calling process to the spawned pane, so mise/nvm/pyenv tools are available without manual workarounds. Put complex commands (with parens, quotes, `&&`) in a `.sh` file rather than inline args to avoid shell-quoting issues.

## Claude Code

`extras/claude-code/settings.json` — a `SessionStart` hook that runs `swarm init` on every project startup. Drop it into `.claude/settings.json`.

Fires only on `startup` (not `resume` or `compact`). The universal skill can be installed under `.claude/skills/openswarm/` or `~/.claude/skills/openswarm/`.

`extras/claude-code/commands/assign-task.md` — a slash command (`/assign-task <task-id>`) that registers the session as a swarm worker, claims the task, and completes it. Install to `.claude/commands/` (project) or `~/.claude/commands/` (global).

## opencode Plugin

`extras/opencode/index.ts` is an opencode plugin (Bun shell `$` API) that:

1. Runs `swarm init` on plugin load (auto-init on every session start)
2. Hooks into `experimental.session.compacting` to append `swarm prompt` output to the compaction context — agents retain coordination state across context resets

Install by copying to `~/.config/opencode/plugins/` or `.opencode/plugins/`, or reference directly in `opencode.json`.

`extras/opencode/commands/assign-task.md` — an opencode command (`/assign-task <task-id>`) with `subtask: true`. Spawns a subagent that registers, claims the task, completes the work, and marks it done — without polluting the primary context. Install to `.opencode/commands/` or `~/.config/opencode/commands/`.

## pi Extension

`extras/pi/openswarm.ts` is a pi coding agent extension that:

1. Runs `swarm init` on `session_start`
2. Registers `/swarm-status` — shows agents, tasks, unread, active runs in a notification
3. Registers `/swarm-prompt` — runs `swarm prompt` and injects the output as a user message via `pi.sendUserMessage()`, giving the agent full coordination context on demand
4. Registers `/assign-task <task-id> [--provider <p> --model <m>]` — registers this session as a swarm worker, claims the task, then spawns a detached `pi --print` sub-agent (fire-and-forget; notifies on exit)

Install by copying to `~/.pi/agent/extensions/` (global) or `.pi/extensions/` (project-local).

## Static Site

`docs/index.html` — GitHub Pages landing page for the project.

Deployed automatically via `.github/workflows/pages.yml` on every push to `main` that touches `docs/`. Covers hero with install command, subsystems, design principles, and quickstart. Pure HTML + missing.css + vanilla JS; no build step.

## Multiplexer Note

`swarm msg`, `swarm task`, `swarm agent`, `swarm events`, and `swarm status` work without any multiplexer. Only `swarm pane` and `swarm run` require one. [[internal/pane/detect.go#DetectBackend]] returns a clear error if none is detected.
