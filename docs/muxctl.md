# muxctl — Universal Terminal Multiplexer CLI

## What

A CLI tool (`muxctl`) that provides a single, consistent interface for managing terminal multiplexers and terminal apps. It abstracts away the differences between **tmux**, **Zellij**, **Kitty**, and **Ghostty** behind a common set of verbs:

```
muxctl pane list
muxctl pane new --cmd "npm test"
muxctl pane send <id> "echo hello"
muxctl pane read <id>
muxctl tab new --name "build"
muxctl tab list
muxctl session list
muxctl session new --name "dev"
```

It auto-detects the active backend (or uses config) and translates commands to the native API of whichever multiplexer is running.

**Primary use cases:**

1. Developers scripting workflows across terminal panes without being locked to one multiplexer
2. AI coding agents (Claude Code, Codex, etc.) discovering, controlling, and reading from terminal panes via a single tool/MCP server
3. Power users switching between terminal environments without rewriting automation

## Why

- **No unified abstraction exists.** Every agent orchestration tool (NTM, agent-deck, agtx, tmuxcc) is hardcoded to tmux. Users of Zellij, Kitty, or Ghostty are left out.
- **Each multiplexer has its own incompatible API** — tmux uses `send-keys`/`capture-pane`, Zellij uses `zellij action`, Kitty uses `kitten @` JSON over Unix sockets. Writing cross-multiplexer scripts means learning 3-4 APIs.
- **AI agents need terminal access.** The rise of coding agents that run in terminals creates demand for a programmatic way to list panes, send commands, and read output — without assuming tmux.
- **Personal pain point.** Zellij users (and others) can't easily integrate with the growing agent tooling ecosystem that assumes tmux.

## Competitors & Landscape

### Closest to muxctl

| Tool | What it does | How close | Gap |
| --- | --- | --- | --- |
| **[workmux](https://github.com/raine/workmux)** | Auto-detects tmux/zellij/kitty/wezterm, creates worktree+window pairs for parallel agent work | **Closest structurally** — has multi-backend detection | Git worktree manager, not general pane control. No async task tracking, no structured output, no ghostty. |
| **[KILD](https://github.com/Wirasm/kild)** | Rust daemon managing PTY sessions with a tmux shim for Claude Code agent teams | Has a daemon + task lifecycle concept | tmux-shim only, not a multiplexer abstraction. Tightly coupled to Claude Code. |
| **[CustomPaneBackend proposal](https://github.com/anthropics/claude-code/issues/26572)** | JSON-RPC 2.0 protocol spec for multiplexer-agnostic pane control (spawn, write, capture, kill, list) | **Closest conceptually** — defines the exact abstraction we want | A proposal, not a shipped tool. Lives inside Claude Code's issue tracker. |

### Agent orchestrators (all tmux-locked)

| Tool | What it does | Limitation |
| --- | --- | --- |
| **[NTM](https://github.com/Dicklesworthstone/ntm)** | Go CLI to tile AI agents across tmux panes with TUI command palette | tmux-only |
| **[agent-deck](https://github.com/asheshgoplani/agent-deck)** | Go + Bubble Tea TUI dashboard for managing AI agents | tmux-only |
| **[agtx](https://github.com/fynnfluegge/agtx)** | Multi-session AI agent manager with TOML plugin framework | tmux-only |
| **[tmuxcc](https://github.com/nyanko3141592/tmuxcc)** | TUI dashboard for AI coding agents in tmux | tmux-only |
| **[dmux](https://github.com/standardagents/dmux)** | Git worktree + agent launcher | tmux-only |
| **tmux MCP servers** ([jonrad](https://github.com/jonrad/tmux-mcp), [nickgnd](https://github.com/nickgnd/tmux-mcp), [bnomei](https://github.com/bnomei/tmux-mcp)) | Let AI agents control tmux panes via MCP | tmux-only |

### Terminal-native solutions (single backend or proprietary)

| Tool | What it does | Limitation |
| --- | --- | --- |
| **[cmux](https://github.com/manaflow-ai/cmux)** | Ghostty-based macOS terminal app with CLI + socket API for agents | Ghostty-only, macOS-only, GUI not CLI |
| **[FrankenTerm](https://github.com/Dicklesworthstone/frankenterm)** | Agent-swarm platform with JSON API | WezTerm-only |
| **[Warp 2.0](https://www.warp.dev/agents)** | Agentic dev environment with orchestration | Proprietary, closed ecosystem |
| **[TmuxAI](https://tmuxai.dev/)** | AI assistant that observes tmux pane content | tmux-only, passive |

### Demand signals

- Claude Code [#24122](https://github.com/anthropics/claude-code/issues/24122) — "Add zellij support" for agent teams
- Claude Code [#24189](https://github.com/anthropics/claude-code/issues/24189) — "Add Ghostty as split-pane backend"
- Claude Code [#26572](https://github.com/anthropics/claude-code/issues/26572) — CustomPaneBackend protocol proposal (50+ upvotes)

### The gap

No existing tool provides all three:
1. **Multiplexer-agnostic pane control** — a single CLI that works across tmux, zellij, kitty, ghostty
2. **Async task tracking** — spawn work, check status later, get notified on completion
3. **Agent-friendly output** — structured JSON for machine consumption

## MVP Scope

### Core abstraction (the "driver" model)

```
┌─────────────┐
│   muxctl    │  ← unified CLI / API
├─────────────┤
│  Core Layer │  ← common types: Session, Tab, Pane
├──┬──┬──┬────┤
│tm│ze│ki│gh  │  ← backend drivers (tmux, zellij, kitty, ghostty)
└──┴──┴──┴────┘
```

### MVP commands

Resource hierarchy: **session > tab > pane**

Shared verbs (`new`, `list`, `focus`, `close`, `rename`) work across all three resource types. Pane-only verbs (`read`, `send`, `status`) apply where terminal content lives.

#### Global

| Command         | Description                |
| --------------- | -------------------------- |
| `muxctl detect` | Auto-detect active backend |

#### Pane commands

| Command                        | Description                                                             |
| ------------------------------ | ----------------------------------------------------------------------- |
| `muxctl pane list`             | List panes in current tab                                               |
| `muxctl pane new [--cmd]`      | Open a new pane, optionally run a command                               |
| `muxctl pane focus <id>`       | Focus a specific pane                                                   |
| `muxctl pane close <id>`       | Close a pane                                                            |
| `muxctl pane rename <id> <n>`  | Rename a pane                                                           |
| `muxctl pane send <id> <text>` | Send keystrokes/text to a pane                                          |
| `muxctl pane read <id>`        | Capture current output of a pane                                        |
| `muxctl pane status <id>`      | Check if the process in a pane is still running or exited (+ exit code) |

#### Tab commands

| Command                      | Description                  |
| ---------------------------- | ---------------------------- |
| `muxctl tab list`            | List tabs in current session |
| `muxctl tab new [--name]`    | Create a new tab             |
| `muxctl tab focus <id>`      | Focus a specific tab         |
| `muxctl tab close <id>`      | Close a tab (and its panes)  |
| `muxctl tab rename <id> <n>` | Rename a tab                 |

#### Session commands

| Command                          | Description                        |
| -------------------------------- | ---------------------------------- |
| `muxctl session list`            | List sessions                      |
| `muxctl session new [--name]`    | Create a new session               |
| `muxctl session focus <id>`      | Attach/switch to a session         |
| `muxctl session close <id>`      | Close a session (and all contents) |
| `muxctl session rename <id> <n>` | Rename a session                   |

#### Task commands (background work)

| Command                          | Description                                                          |
| -------------------------------- | -------------------------------------------------------------------- |
| `muxctl run --bg [--name] <cmd>` | Spawn a command in a new pane, return a task handle                  |
| `muxctl task list`               | List tracked background tasks and their status (running/done/failed) |
| `muxctl task wait <id>`          | Block until a task finishes, return exit code + captured output      |
| `muxctl task read <id>`          | Read the latest output of a tracked task (non-blocking)              |

### MVP backends (pick 2 for v0.1)

1. **Zellij** — personal use, less tooling exists
2. **tmux** — largest user base, validates the abstraction

### Config

```toml
# ~/.config/muxctl/config.toml
backend = "auto"  # or "zellij", "tmux", "kitty"

[zellij]
session = "default"

[tmux]
socket = ""
```

### Background tasks & async work

```
# Agent A spawns a research task in a new pane
$ muxctl run --bg --name "research" "claude 'research anthropic'"
→ task:research (pane:3) started

# Agent A continues its own work, checks later
$ muxctl task list
  ID        PANE   STATUS    CMD
  research  3      running   claude 'research anthropic'

# Non-blocking read of what it's produced so far
$ muxctl task read research
→ (latest pane output)

# Or block until it's done
$ muxctl task wait research
→ task:research exited(0)
→ (final output)
```

**How "done" is detected:**

- **Process exit** — the spawned process exits, muxctl captures the exit code. This is the primary signal and works for any command.
- **Shell prompt return** — for interactive shells, detect when the prompt reappears after a command finishes (configurable prompt pattern).
- **Output pattern** (optional) — user can specify a regex to match against output as an early completion signal (e.g. `--done-when "✓|DONE|error:"`).

**How notification works:**

- `muxctl task wait` — blocking, for scripts and agents that want to await a result
- `muxctl task list --json` — polling, for agents that check periodically
- `--on-done <cmd>` flag — fire-and-forget callback when a task completes (e.g. `--on-done "notify-send 'research done'"`)

This makes muxctl the missing link for agent-to-agent delegation: Agent A tells muxctl to run Agent B in a pane, goes back to work, and picks up the result when it's ready.

### Output format

- Default: human-readable table
- `--json` flag for machine consumption (agent-friendly)

## Non-goals for MVP

- GUI / TUI dashboard
- Built-in AI agent orchestration
- Kitty/Ghostty drivers (post-MVP)
- MCP server (post-MVP, but the CLI is designed to make this trivial)
- Plugin system

## Tech stack

- **Go** with **Cobra** for CLI framework
- Single binary, no runtime dependencies
- Backend drivers implement a shared Go interface

## Success criteria

1. `muxctl pane list` returns consistent output whether running in tmux or Zellij
2. `muxctl pane send` + `muxctl pane read` round-trip works for both backends
3. A coding agent (e.g. Claude Code) can use `muxctl` to discover and interact with terminal panes without knowing which multiplexer is running
