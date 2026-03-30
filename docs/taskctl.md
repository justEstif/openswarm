# tasks — Shared Task List CLI

## What

A CLI tool (`tasks`) that provides a shared, file-backed task registry for coordinating work across multiple agents or processes. Any agent in a team can create, assign, update, and read tasks from a common list.

```
tasks add --title "research anthropic funding" --priority high
tasks list
tasks assign task-a3f2 researcher-9b1c
tasks status task-a3f2 in-progress
tasks done task-a3f2 --output "funding round was $X"
tasks get task-a3f2
tasks block task-a3f2 --on task-9b1c
tasks unblock task-a3f2
```

It stores state as a JSON file in the project directory (or a configurable path) and is designed to be read and written by multiple concurrent agents safely via atomic writes.

**Primary use cases:**

1. Agent teams coordinating parallel work — one task list, multiple agents claiming and completing tasks
2. Orchestrator agents assigning work to specialist agents and tracking completion
3. Human oversight — inspect what agents are working on and what's done

## Why

- **No shared state between agents.** Claude Code's native agent teams use a file-based task system, but it's tied to Claude Code's internal tooling. `tasks` makes this pattern available to any agent, script, or multiplexer setup.
- **Agents need coordination primitives.** Without a shared task list, agents either duplicate work, step on each other, or require a human to manually sequence everything.
- **Blocking/dependency tracking matters.** Real workflows have sequencing constraints. Task B can't start until Task A finishes. This needs to be first-class, not bolted on.
- **Personal pain point.** Building agent team workflows on top of muxctl requires a coordination layer that doesn't assume Claude Code internals.

## Competitors & Landscape

No existing tool fills this exact niche — a lightweight, file-backed, CLI-native shared task list designed for agent coordination. The closest things are:

- **Claude Code's internal task system** (`~/.claude/tasks/{team-name}/`) — not accessible outside Claude Code, tied to TeammateTool
- **Todo.txt / taskwarrior** — human-oriented, no agent-friendly JSON output, no blocking/dependency model
- **GitHub Issues / Linear** — requires network, auth, far too heavy for local agent coordination

## MVP Scope

### Storage

```
.tasks/
└── tasks.json     # single flat list of all tasks
```

Overridable via `$TASKS_DIR`. Default is `.tasks/` in the current working directory. All writes are atomic (`mv` from temp file) to prevent corruption from concurrent agents.

### Task schema

```json
{
  "id": "task-a3f2",
  "title": "research anthropic funding",
  "status": "todo",
  "priority": "high",
  "assigned_agent": "researcher-9b1c",
  "blocked_by": ["task-9b1c"],
  "output": "",
  "created_at": "2026-03-27T10:00:00Z",
  "updated_at": "2026-03-27T10:05:00Z"
}
```

### Commands

#### Task lifecycle

| Command | Description |
| --- | --- |
| `tasks add --title <t> [--priority <p>]` | Create a new task, returns task ID |
| `tasks list [--status <s>] [--assigned <agent-id>]` | List tasks, optionally filtered |
| `tasks get <id>` | Show full detail for a single task |
| `tasks assign <id> <agent-id>` | Assign a task to an agent |
| `tasks status <id> <status>` | Update task status (`todo`, `in-progress`, `done`, `failed`) |
| `tasks done <id> [--output <text>]` | Mark task done, optionally attach result |
| `tasks fail <id> [--output <text>]` | Mark task failed, optionally attach error |
| `tasks rm <id>` | Delete a task |
| `tasks clear` | Remove all completed/failed tasks |

#### Blocking / dependencies

| Command | Description |
| --- | --- |
| `tasks block <id> --on <id>` | Mark a task as blocked by another task |
| `tasks unblock <id>` | Clear all blockers from a task |
| `tasks blocked` | List all tasks currently blocked and what's blocking them |

#### Team context

| Command | Description |
| --- | --- |
| `tasks team init --name <name>` | Initialize a named team context in `.tasks/` |
| `tasks team status` | Summary view: task counts by status, agent assignments |

### Output format

- Default: human-readable table
- `--json` flag for machine consumption (agent-friendly)

### Example flow

```
# Orchestrator sets up tasks
$ tasks team init --name "api-build"
$ tasks add --title "design auth endpoints" --priority high
→ task-a3f2 created
$ tasks add --title "implement auth endpoints" --priority high
→ task-b7k1 created
$ tasks block task-b7k1 --on task-a3f2

# Orchestrator spawns agents, assigns work
$ tasks assign task-a3f2 designer-9b1c
$ tasks assign task-b7k1 coder-x4f2

# Designer agent picks up its task
$ tasks status task-a3f2 in-progress
$ tasks done task-a3f2 --output "spec written at docs/auth-spec.md"

# task-b7k1 is now unblocked — coder can proceed
$ tasks list --status todo
  ID        TITLE                       PRIORITY  ASSIGNED     BLOCKED
  task-b7k1 implement auth endpoints    high      coder-x4f2   —

# Coder finishes
$ tasks done task-b7k1 --output "implemented at src/auth/"

# Human or orchestrator checks in
$ tasks team status
  done: 2  in-progress: 0  todo: 0  failed: 0
```

## Non-goals for MVP

- GUI / TUI dashboard
- Task comments or history log
- Subtasks / nested task hierarchy
- Remote/networked task store
- Priorities beyond `low`, `medium`, `high`, `critical`
- Notifications or webhooks on status change

## Tech stack

- **Go** with **Cobra** for CLI framework
- Single binary, no runtime dependencies
- Atomic file writes via temp file + `os.Rename`
- File locking (`flock`) for concurrent write safety

## Success criteria

1. Two agents can concurrently update different tasks without corrupting `tasks.json`
2. `tasks list --json` returns consistent, parseable output an agent can act on
3. Blocking correctly prevents `tasks list --status todo` from surfacing blocked tasks until their blockers are done
