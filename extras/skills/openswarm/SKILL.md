---
name: openswarm
description: Coordinate multi-agent terminal sessions with the `swarm` CLI. Use when spawning agents, assigning tasks, routing messages between agents, or monitoring swarm state across a project.
---

# openswarm

openswarm is a file-backed CLI (`swarm`) for multi-agent coordination — no daemon, all state under `.swarm/`. Every command accepts `--json` for machine-readable output.

## Setup

```bash
swarm init          # create .swarm/ in project root (idempotent)
swarm version       # verify install
```

Pane/run commands require tmux, Zellij, or WezTerm. Backend is auto-detected from `$TMUX`, `$ZELLIJ`, or `$WEZTERM_PANE`. Override with `$SWARM_BACKEND`.

## Agents

```bash
swarm agent register <name> [--role <role>]  # register yourself (role defaults to "agent")
swarm agent list
swarm agent get <id-or-name>
swarm agent deregister <id-or-name>
```

## Tasks

```bash
swarm task list
swarm task add "description"
swarm task assign <id> <agent-id-or-name>
swarm task claim <id> --as <agent>
swarm task update <id> [--status <s>] [--assignee <a>]
swarm task done <id>
swarm task fail <id>
swarm task cancel <id>
swarm task block <id> --by <other-id>
swarm task get <id>
swarm task prompt          # agent-priming context for current tasks
swarm task check           # integrity check
```

## Messaging

```bash
swarm msg send <recipient> --subject "subj" --body "text"
swarm msg inbox <agent>
swarm msg read <agent> <msg-id>
swarm msg reply <agent> <msg-id> --body "text"
swarm msg watch <agent>    # block and stream new messages until Ctrl-C
swarm msg clear <agent>    # remove read messages
```

## Panes & runs

```bash
# Interactive panes — stay open after command exits
swarm pane spawn <name> [cmd...] [--placement <p>]
swarm pane list
swarm pane send <pane-id> "text"
swarm pane capture <pane-id>
swarm pane close <pane-id>

# Managed runs — tracked in runs.json
swarm run [start] [--name <n>] [--placement <p>] -- <cmd...>   # fire-and-forget (default)
swarm run [start] [--name <n>] [--placement <p>] --wait -- <cmd...>  # block until done
swarm run wait <run-id>
swarm run list
swarm run logs <run-id>
swarm run kill <run-id>
```

`--placement` options: `current_tab` (default), `new_tab`, `new_session`.  
`new_tab` and `new_session` open in the **background** — focus stays on the current tab.  
Run panes **close automatically** on exit. Interactive panes (`pane spawn`) stay open.

The caller's `PATH` is forwarded automatically, so mise/nvm/pyenv tools work without setup.

Signal completion from inside a run: print `<promise>COMPLETE</promise>`.

### Config

Set the default placement in `.swarm/config.toml`:

```toml
[pane]
placement = "new_tab"   # or "current_tab" (default), "new_session"
```

Override per-invocation with `--placement`, or globally with `$SWARM_PANE_PLACEMENT`.

### Quoting tip — use script files for complex commands

Long prompts with parens, quotes, or `&&` cause shell-parsing errors as inline args.
Write them to a file instead:

```bash
# ✗ breaks — shell parses parens before swarm sees them
swarm run start -- pi --print "Research X (latest trends)"

# ✓ works
cat > /tmp/task.sh << 'EOF'
#!/bin/sh
pi --print "Research X (latest trends)"
EOF
swarm run start --name my-task -- sh /tmp/task.sh
```

## Worktrees

```bash
swarm worktree new --branch <branch> --agent <agent>
swarm worktree list
swarm worktree get <id>
swarm worktree merge <id>
swarm worktree clean <id>
swarm worktree clean-all   # clean all merged/abandoned worktrees
```

## Status & events

```bash
swarm status                           # agents / tasks / messages / runs at a glance
swarm prompt                           # full agent-priming context block
swarm events tail                      # stream live event log
swarm events tail --n 20               # last N events then exit
swarm events tail --filter task        # filter by event type prefix
```

## State layout

```
.swarm/
├── config.toml
├── agents/registry.json
├── messages/<agent>/inbox/<msg-id>.json
├── tasks/tasks.json
├── runs/runs.json
├── worktrees/worktrees.json
└── events/events.jsonl
```

`swarm msg`, `swarm task`, `swarm agent`, `swarm events`, and `swarm status` work without any multiplexer. Only `swarm pane` and `swarm run` require one.
