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

Pane/run commands require tmux, Zellij, or WezTerm running. The backend is auto-detected from `$TMUX`, `$ZELLIJ`, or `$WEZTERM_PANE`. Override with `$SWARM_BACKEND`.

## Agents

```bash
swarm agent register <name> --role <role>  # register yourself
swarm agent list                       # list all agents
swarm agent get <id>
swarm agent deregister <id>
```

## Tasks

```bash
swarm task list                        # see all tasks
swarm task add "description"           # create a task
swarm task assign <id> <agent>         # assign to an agent
swarm task claim <id> --as <agent>     # claim a task yourself
swarm task done <id>                   # mark complete
swarm task fail <id>                   # mark failed
swarm task block <id> --by <other-id>  # declare a dependency
swarm task check                       # check task store integrity
swarm task prompt                      # priming prompt for current task state
```

## Messaging

```bash
swarm msg send <agent> --subject "subj" --body "text"  # send a message
swarm msg inbox <agent>                # list messages
swarm msg read <agent> <msg-id>        # read a message (marks it read)
swarm msg reply <agent> <msg-id> --body "text"  # reply in thread
swarm msg clear <agent>                # clear read messages from inbox
```

## Panes & runs

```bash
swarm pane spawn <name>                # spawn a terminal pane
swarm pane list
swarm pane send <pane-id> "command"    # send keystrokes to a pane
swarm pane capture <pane-id>           # read pane output
swarm pane close <pane-id>

swarm run start [--name <name>] -- <cmd>  # run command in a managed pane
swarm run wait <run-id>                # block until run finishes
swarm run list
swarm run logs <run-id>
swarm run kill <run-id>
```

Signal run completion from inside the pane by printing `<promise>COMPLETE</promise>`.

## Worktrees

```bash
swarm worktree new --branch <branch> --agent <agent>
swarm worktree list
swarm worktree merge <id>
swarm worktree clean <id>
```

## Status & events

```bash
swarm status                           # agents / tasks / messages / runs at a glance
swarm events tail                      # stream the live event log
swarm events tail --n 20               # last N events then exit
swarm events tail --filter task        # filter by event type prefix
swarm prompt                           # generate an agent-priming context block
```

## State layout

```
.swarm/
├── config.toml
├── agents/registry.json
├── messages/<agent-id>/inbox/<msg-id>.json
├── tasks/tasks.json + .lock
├── runs/runs.json
├── worktrees/worktrees.json
└── events/events.jsonl
```
