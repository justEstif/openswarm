---
# openswarm-vrt6
title: Investigate richer terminal-agent UI integration (notifications, live team status)
status: draft
type: epic
priority: normal
created_at: 2026-04-02T20:26:55Z
updated_at: 2026-04-02T20:27:17Z
---

Explore how openswarm can surface swarm state as ambient UI inside agent sessions — e.g. a persistent status line, desktop/terminal notifications on task completion, a 'team running' indicator inside Claude Code, etc. Research what hooks each agent platform exposes and design non-intrusive patterns.

## Problem

Agents currently have no ambient awareness of the swarm. They must actively run `swarm status` or `/swarm-status` to see what is happening. There is no push-style feedback when tasks complete, agents join, or runs finish.

## Ideas to investigate

### 1. Persistent status line / HUD

- **Zellij**: inject a plugin pane or use `zellij status-bar` API to show a compact team summary (N agents, M tasks, K runs active). Stays visible without stealing focus.
- **tmux**: `tmux set-option -g status-right "$(swarm status --compact)"` — update via a background refresh loop or `swarm events tail` pipe.
- **WezTerm**: `wezterm.status_update` event / `status.left_attribute` in the config can call an external script.

### 2. Desktop / terminal notifications on key events

- When a run completes (or prints `<promise>COMPLETE</promise>`), fire `notify-send` (Linux) / `osascript -e display notification` (macOS) / `terminal-notifier`.
- Could be a `swarm events tail | swarm notify` pipeline, or a `--notify` flag on `swarm run start --wait`.
- pi already emits notifications via `pi.notify()` — hook into that from the extension.

### 3. "Team running" indicator inside Claude Code

- Claude Code has `SessionStart` / `Stop` hooks but no persistent UI surface.
- Options: write a status file (`.swarm/status.txt`) that a background watcher renders; or use Claude Code's `--system-prompt` injection on resume to include a one-liner swarm summary.
- A `PreToolUse` hook could prepend a compact status block to each tool call context.

### 4. Compact `swarm status --one-line` output format

- A new `--one-line` or `--compact` flag on `swarm status` for embedding in status bars.
- Example: `⚡ 3 agents · 2 tasks active · 1 run`

### 5. `swarm watch` daemon (lightweight)

- A long-running `swarm watch` command that tails events and triggers side-effects (notify, update tmux status, write a status file).
- No persistent process required for the core CLI — this is an opt-in sidecar.

## Questions to answer

- What notification APIs does each platform support cross-OS?
- Can Zellij render a status plugin without recompiling (WASM plugin vs. CLI)?
- Does Claude Code expose a way to inject content into the UI outside of tool responses?
- Is a lightweight watcher daemon acceptable given the no-daemon design principle?
- What is the minimal viable "team running" signal that adds value without cognitive overhead?

## Success criteria

Agents in a swarm session have ambient, non-intrusive awareness of team state without needing to explicitly poll. At minimum: one working notification mechanism and one status-bar integration.

## Child tasks (to be created after investigation)

- [ ] Research notification APIs per OS and agent platform
- [ ] Prototype tmux status-bar integration
- [ ] Prototype Zellij status-bar / plugin integration
- [ ] Design `swarm status --compact` output format
- [ ] Prototype `swarm watch` event→notification pipeline
- [ ] Evaluate Claude Code hook options for ambient status
