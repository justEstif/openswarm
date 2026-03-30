# openswarm — Project Notes

## Goal

Recreate Claude Code's swarm/team mode as open, composable CLI primitives — decoupled from any specific agent or multiplexer.

## The Unified `swarm` Binary

The three subsystems (messaging, tasks, mux control) are strong designs as isolated specs, but they share enough concepts that the _interface_ should be conceived as one unified CLI from day one — not three separate binaries retrofitted under a common name later.

### What breaks if they stay separate

| Problem                             | Symptom                                                                                                                                  |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| **Agent identity has no home**      | `messages` defines `register`, but `tasks` also uses agent IDs. No canonical registry — tools have to agree out-of-band via `$AGENT_ID`. |
| **Init is duplicated**              | Both `messages` and `tasks` have their own `team init`. Same concept, twice.                                                             |
| **State is scattered**              | `.messages/`, `.tasks/`, `.events/` — three dotdirs, same project context.                                                               |
| **Config is scattered**             | `$AGENT_ID`, `$MESSAGES_DIR`, `$TASKS_DIR`, `$EVENTS_DIR` — four env vars to manage.                                                     |
| **Cross-tool views are impossible** | "Show me everything about agent `researcher-a3f2`" requires querying three separate tools.                                               |
| **Event log has no clear owner**    | `.events/` is cross-cutting. Bolting it onto any one tool is awkward.                                                                    |

### What snaps into place with `swarm`

- **`swarm init`** — one command, one project context
- **`swarm agent`** — canonical identity subcommand group, shared by all subsystems
- **`.swarm/`** — single project root for all state
- **`swarm status`** — cross-tool dashboard (agents + tasks + inbox + pane health)
- **`swarm events`** — natural home, no awkward ownership question
- **One config** — `~/.config/swarm/config.toml` + `.swarm/config.toml` per project
- **One binary, one install**

### Proposed command structure

```
swarm init [--name <team>]
swarm status                              # cross-tool: agents, task counts, unread msgs, pane health

swarm agent register --role <role>        # → researcher-a3f2
swarm agent list

swarm msg send <agent-id> <body>
swarm msg send <agent-id> --file <path>
swarm msg reply <msg-id> <body>
swarm msg inbox [--watch] [--all] [--json]
swarm msg clear

swarm task add --title <t> [--priority <p>]
swarm task list [--status <s>] [--assigned <id>]
swarm task assign <id> <agent-id>
swarm task status <id> <status>
swarm task done <id> [--output <text>]
swarm task block <id> --on <id>
swarm task watch

swarm pane list
swarm pane new [--cmd <cmd>]
swarm pane send <id> <text>
swarm pane read <id>
swarm pane status <id>
swarm tab list / new / focus / close
swarm session list / new / focus / close
swarm run --bg [--name <n>] <cmd>         # spawn + track
swarm run wait <id>                       # block until done
swarm run list                            # tracked background tasks

swarm events tail [--type <filter>]
swarm events on <type> --exec <cmd>
```

### Unified state layout

```
.swarm/
├── config.toml
├── agents/
│   └── registry.json          # canonical agent registry (shared by msg + task)
├── messages/
│   └── <agent-id>/inbox/
├── tasks/
│   └── tasks.json
├── runs/
│   └── runs.json              # muxctl background task tracking
└── events/
    └── events.jsonl           # append-only, written by all subsystems
```

### The three MVPs are still valid

The specs in `messagectl.md`, `taskctl.md`, `muxctl.md` describe what each subsystem does — the data model, storage, commands, success criteria. That thinking is all still correct. What changes:

- Commands are namespaced under `swarm <group>` instead of standalone binaries
- `team init` collapses into `swarm init`
- Agent registration moves to `swarm agent` (shared)
- State directories unify under `.swarm/`
- Env vars collapse: `$SWARM_DIR` (project root), `$SWARM_AGENT_ID`

---

## The Three Subsystems

Claude's internal swarm has three pieces. We build each as a subsystem of `swarm`:

---

## The Three Pieces

### 1. `swarm msg` — Messaging subsystem

**Spec:** `messagectl.md`

Peer messaging between agents. File-backed inbox per agent. No daemon.

- Agents registered via `swarm agent register` get a unique ID (`researcher-a3f2`)
- Send messages, attach files, reply to threads
- `swarm msg inbox --watch` for live polling
- Storage: `.swarm/messages/<agent-id>/inbox/*.json`

**Gap it fills:** No standalone peer messaging primitive exists for local agent coordination.

---

### 2. `swarm task` — Task registry subsystem

**Spec:** `taskctl.md`

Shared task registry across agents. Atomic file writes for concurrent safety.

- Create, assign, update, complete tasks
- Blocking/dependency model: `swarm task block task-b --on task-a`
- Storage: `.swarm/tasks/tasks.json`

**Open question:** Are there existing lightweight CLI task tools worth wrapping or replacing? (mentioned: "beans?") — needs investigation.

**Gap it fills:** Claude Code's internal task system is not accessible outside its tooling.

#### Features to borrow from beans (`research/beans.md`)

Schema additions:
- `status`: add `cancelled` (intentional abandon) and `draft` (needs refinement, excluded from ready) alongside todo/in-progress/done/failed
- `priority`: expand to 5 levels — `critical / high / normal / low / deferred` (`deferred` explicitly excluded from `--ready`)
- `tags: []string` — free-form labeling, no schema changes needed
- `etag` per task — content hash for optimistic locking; `swarm task update --if-match <etag>` fails if task changed since read

Command additions:
- `swarm task show <id> [id...]` — detailed single-task view (separate from list); `--output-only` flag
- `swarm task update <id> --output-append <text>` — append to output field incrementally (agents log progress without full rewrite)
- `swarm task check [--fix]` — validate `blocked_by` references, auto-remove stale ones
- `swarm task prompt` — emit agent-priming instructions for all task commands, injectable into agent context

Filter/output additions:
- `--ready` filter on `swarm task list` — compound: not blocked + not terminal + not in-progress + not deferred. The primary "what should I work on?" query for agents.
- `--sort priority|created|updated|status` on list
- `-q / --quiet` on list — IDs only, one per line; pipe-friendly
- `--exclude-status <s>` — inverse filter
- `--full` on `--json` — output field excluded by default to save agent tokens; `--full` includes it
- Machine-readable error codes in `--json` responses: `{"error": {"code": "NOT_FOUND", "message": "..."}}`
- `$SWARM_TASKS_PATH` env var override

---

### 3. `swarm pane` / `swarm run` — Mux control subsystem

**Spec:** `muxctl.md`

Single consistent interface over tmux, Zellij, Kitty, Ghostty. Backend driver model.

- `pane`, `tab`, `session` resource hierarchy with shared verbs
- `swarm run --bg` + `swarm run wait` for async agent delegation
- Auto-detects active backend
- MVP backends: **Zellij** (v0.1) + **tmux** (v0.1)
- Config: `~/.config/swarm/config.toml`

**Closest prior art:**

- `workmux` — multi-backend detection but git worktree manager, not general pane control
- CustomPaneBackend proposal (claude-code #26572, 50+ upvotes) — spec only, not shipped

**Gap it fills:** Every agent orchestration tool is tmux-locked. Zellij/Kitty/Ghostty users have no equivalent.

---

## Notifications

### Two distinct layers

1. **Detection** — knowing something changed. Each tool already owns this: `tasks` knows when a task status changes, `messages` knows when a message lands, `muxctl` knows when a pane exits.
2. **Delivery** — routing that fact to the right consumer. This is harness-dependent and currently scattered (each tool has its own `--watch`/`--on-done`/polling mechanism).

### The cross-cutting problem

An orchestrator agent doesn't want to run three separate watch processes. It wants one stream: _anything happened in this swarm._ Right now that requires multiplexing `messages inbox --watch`, `tasks watch` (not yet built), and `muxctl task wait` — three separate polling loops.

### The harness problem

Different execution environments have fundamentally different models for receiving signals:

| Harness              | Native signal model                                     |
| -------------------- | ------------------------------------------------------- |
| Shell scripts        | Blocking `wait`, `while` poll loop, `tail -f`           |
| pi / Zellij          | Pane output scanning, `zellij_orchestrate wait/collect` |
| Claude Code          | Internal file watching via TeammateTool                 |
| MCP clients (future) | Push notification or resource subscription              |
| Human                | `notify-send` / `osascript` / terminal bell (`\a`)      |

A separate notification CLI would need to know about all of these. That knowledge doesn't belong in any one of the three core tools either — it's cross-cutting.

### The right primitive: a shared event log

Each tool appends structured events to a shared append-only log when state changes. Any consumer tails that file using whatever mechanism their harness supports.

```
.events/
└── events.jsonl    # append-only, one JSON object per line
```

Event schema:

```json
{"id": "evt-x4f2", "source": "tasks",    "type": "task.done",         "ref": "task-a3f2",   "at": "2026-03-29T10:00:00Z"}
{"id": "evt-b1k9", "source": "messages", "type": "message.received",   "ref": "msg-x4f2",   "at": "2026-03-29T10:01:00Z"}
{"id": "evt-p2q1", "source": "muxctl",   "type": "pane.exited",        "ref": "task:research", "at": "2026-03-29T10:02:00Z"}
```

Key properties:

- **No daemon required** — just `O_APPEND` writes. Safe for concurrent writers.
- **Stays file-based** — consistent with the rest of the system, survives restarts, debuggable with `cat`/`jq`.
- **Single tail point** — one stream for all three tools. Harness adapters are thin.
- **Decoupling** — detection lives in each tool, delivery lives in the consumer or adapter layer.

### How each harness consumes events

```bash
# Shell scripts — tail + jq filter
tail -f .events/events.jsonl | jq 'select(.type=="task.done")'

# pi / Zellij — orchestration layer tails the log in a dedicated pane,
#               or each subagent checks the log as a tool call

# Claude Code — TeammateTool extension could watch .events/events.jsonl
#               instead of its internal task file

# Human — a delivery adapter routes matching events to OS notifications
swarm events --on task.done --notify        # notify-send / osascript
swarm events --on message.received --bell   # terminal bell
```

### Do we need a 4th CLI?

**Not for MVP.** The event log is the primitive. Reading it is just `tail -f` + `jq`.

Post-MVP, a thin `swarm events` command (or top-level `swarm` binary) adds:

- `swarm events tail [--type <filter>]` — ergonomic stream view
- `swarm events on <type> --exec <cmd>` — declarative delivery hooks
- Harness-specific delivery backends (notify-send, bell, webhook, MCP notification)

This is also the natural home for a future `swarm` unified binary that wraps all three tools + the event layer under one namespace.

### What each tool needs to add

- **All three tools** — append to `.events/events.jsonl` on every state mutation (same atomic-write pattern, same `$EVENTS_DIR` override)
- **`tasks`** — add a `tasks watch` command (currently missing)
- **`muxctl`** — `pane.exited` event should include exit code in payload

---

## How They Compose

```
orchestrator agent
    │
    ├── muxctl run --bg "claude researcher"   # spawn agent in pane
    ├── tasks add --title "..." --assign ...  # delegate work
    ├── messages send researcher-a3f2 "..."   # send context/instructions
    │
    └── [later]
        ├── muxctl task wait <id>             # await pane exit
        ├── tasks get task-a3f2               # check result
        └── messages inbox --watch            # receive replies
```

Each tool is independently useful. Together they replicate Claude's swarm primitives on any agent + any multiplexer.

---

## Tech Stack (all three tools)

- **Go** + **Cobra** for CLI
- Single binary, no runtime deps
- Atomic writes via temp file + `os.Rename`
- `--json` flag everywhere for agent-friendly output

---

## Research Synthesis

_From three parallel subagent investigations — 2026-03-29. Full reports in `research/`._

### How Claude Code Agent Teams actually works

**Architecture in one sentence:** A file-based multi-process coordination system where `~/.claude/` is the entire coordination substrate — task board, message queue, and agent registry — with no daemon, no socket server, no shared memory.

| Dimension              | Detail                                                                              |
| ---------------------- | ----------------------------------------------------------------------------------- |
| Coordination substrate | Flat JSON files in `~/.claude/tasks/{team}/` and `~/.claude/teams/{team}/`          |
| IPC mechanism          | File write (sender) + file poll (reader). No sockets.                               |
| Message delivery       | `injectUserMessageToTeammate` — synthetic user-turn injection between LLM turns     |
| Task concurrency       | `flock()` on a `.lock` file in the task dir                                         |
| Dependency resolution  | Computed at `TaskList` read time (pull, not push — no watcher process)              |
| Agent identity         | `AsyncLocalStorage` in-process, or env vars via tmux                                |
| Spawn mechanism        | `tmux split-window` + `send-keys` (external) or AsyncLocalStorage fork (in-process) |
| Token overhead         | ~4× for a 3-person team, ~7× in plan-approval mode                                  |
| Known bug              | ~50% task file corruption with 4+ simultaneous tmux spawns                          |

**File layout:**

```
~/.claude/
├── teams/{team-name}/
│   ├── config.json          # agent registry: [{name, agentId, agentType}]
│   └── inboxes/
│       ├── team-lead.json   # JSON array of messages
│       └── researcher.json
└── tasks/{team-name}/
    ├── .lock                # 0-byte flock() mutex
    ├── .highwatermark       # integer string: next task ID
    ├── 1.json
    └── 2.json
```

**Task schema:**

```json
{
  "id": "1",
  "subject": "...",
  "status": "pending | in_progress | completed | blocked",
  "owner": "researcher",
  "blockedBy": ["2", "3"],
  "fileLock": ["src/auth.ts"]
}
```

**Tools exposed when flag is set:** `TeamCreate`, `TeamDelete`, `TaskCreate`, `TaskList`, `TaskUpdate`, `SendMessage`. Teammates get task tools + SendMessage. Lead gets all tools. Teammates cannot create nested teams.

**Implications for swarm:**

- Our design is validated — the real system is file-based with flock(), same approach
- `swarm task claim` needs a flock()-protected atomic claim (matches our design)
- Dependency resolution at read time (not push) — `swarm task list` computes unblocked tasks on the fly
- Message delivery between agent turns only — `swarm msg` polling model is correct
- The tmux spawn race is a known bug we can fix by adding a handshake step (see workmux findings)

---

### The Ralph Loop pattern

**Core insight:** Context reset = process exit. Spawn a fresh agent process per task. Each iteration stays in the "smart zone" of the context window (first 40–60%).

**The loop:**

```bash
for story in prd.json[passes==false]; do
  invoke_agent(story)  # fresh process
  if quality_gates_pass: set passes=true, git commit, append progress.txt
  # if gates fail: no state changes, retry next iteration
done
```

**`prd.json` schema (snarktank/ralph — 9,800 stars):**

```json
{
  "project": "MyApp",
  "branchName": "ralph/feature",
  "userStories": [
    {
      "id": "US-001",
      "title": "...",
      "description": "As a X, I need Y",
      "acceptanceCriteria": ["..."],
      "priority": 1,
      "passes": false,
      "notes": ""
    }
  ]
}
```

- `passes` is **binary** — no in-progress state. Failed quality gates = no state change, retry
- `priority` integer = sort order. Agent picks lowest where `passes: false`
- Completion signal: agent prints `<promise>COMPLETE</promise>` to stdout; loop detects via `grep -q`

**Four memory channels** (persist across context resets):

1. `git history` — code changes (auto-read)
2. `progress.txt` — append-only learnings log
3. `prd.json` — task state
4. `AGENTS.md` — module-specific knowledge (auto-read)

**Implications for swarm:**

- `swarm task`'s binary done/not-done is correct — don't over-engineer status states
- `swarm run --bg` naturally implements the Ralph Loop: each `swarm run` = fresh process
- Completion detection via stdout pattern (`<promise>COMPLETE</promise>`) is a clean primitive to support in `swarm pane status`
- `progress.txt` maps directly to our `.swarm/events/events.jsonl` append log

---

### Tools landscape — what converged

**14 tools investigated.** Full report in `research/tools-landscape.md`.

**What every serious tool converges on:**

- **Git worktrees** as the agent isolation primitive — settled design space, every tool does this
- **tmux** as the only production multiplexer — Zellij support universally absent or broken

**The gaps swarm uniquely fills:**

1. No tool has a composable subcommand CLI with JSON-first output (`swarm agent`, `swarm task`, etc.)
2. No tool has file-backed state that is grep-able, scriptable, zero-dependency (tools are either stateless or require Dolt SQL engine)
3. Zellij/Kitty/Ghostty users have no equivalent to any of these tools

**Direct design inputs:**

| Finding                           | What to adopt                                                                                 |
| --------------------------------- | --------------------------------------------------------------------------------------------- |
| workmux 5-level backend detection | `$SWARM_BACKEND` → `$TMUX` → `$WEZTERM_PANE` → `$ZELLIJ` → `$KITTY_WINDOW_ID` → fallback:tmux |
| workmux handshake pattern         | Named-pipe sync before injecting commands — fixes the tmux spawn race                         |
| Gastown beads (`bd` binary)       | `swarm task claim` atomic claim + dependency graph + `swarm task ready`                       |
| NTM `--robot-*` + REST/SSE        | `--json` on every subcommand + `swarm events` SSE stream                                      |
| Vibe Kanban MCP server            | Post-MVP: expose `swarm task` and `swarm events` via MCP                                      |
| Agent-agnostic subprocess model   | `.swarm/config.yaml` agent profiles: `name + command + args`                                  |

**Zellij v0.44.0 (2026) ships all primitives needed for a full backend** — the workmux warning about unreleased features is now obsolete:

| Primitive needed         | v0.44.0 command                                        |
| ------------------------ | ------------------------------------------------------ |
| List panes with IDs      | `zellij action list-panes --json`                      |
| Create pane, get ID back | `zellij run` → returns `pane_id`                       |
| Send to specific pane    | `zellij action send-keys --pane-id <id>`               |
| Capture pane output      | `zellij action dump-screen --pane-id <id>`             |
| Stream pane updates live | `zellij subscribe --json`                              |
| Block until pane exits   | `zellij run --blocking` / `--block-until-exit-success` |

Zellij is now a first-class backend alongside tmux. Both can be built in parallel from the start.

---

### What to add to scope: git worktrees

Every serious tool treats git worktrees as the isolation primitive. This isn't in our current MVPs. `swarm` should add:

```
swarm worktree new <branch>    # create worktree + branch for an agent
swarm worktree list
swarm worktree merge <branch>  # rebase + open PR
swarm worktree clean           # remove finished worktrees
```

Or integrate it into `swarm agent spin-up` as a flag: `swarm run --bg --worktree <branch> <cmd>`.

**Decision: add `swarm worktree` as a first-class subcommand group.**

---

## Decisions Log

| Decision | Choice | Rationale |
|---|---|---|
| Repo structure | **Monorepo, one binary** | All subsystems ship as `swarm <group>`, single install, shared state model |
| Multiplexer backends | **tmux + Zellij in parallel** | Zellij v0.44.0 provides all needed primitives |
| `swarm worktree` | **First-class subcommand group** | Every serious tool converges on git worktrees as isolation primitive |
| Completion signaling | **Support `<promise>COMPLETE</promise>` in `swarm pane status`** | Matches ralph pattern; clean stdout-based detection |
| `swarm task` ownership | **Own it, don't wrap beans** | Full control over schema; research beans for feature inspiration only |
| Current phase | **Planning → Architecture → Interface design** | Not building yet |

## Open Questions

- [x] beans research: done — borrow list folded into `swarm task` design above
- [x] Event type taxonomy: defined in `ARCHITECTURE.md` (`events` package constants)
- [x] Module boundaries: defined in `ARCHITECTURE.md`
- [x] Backend interface: settled on 8-method deep interface (not 40-method shallow)
- [x] Task storage: single `tasks.json` with flock() (not one-file-per-task)
- [x] Message storage: one-file-per-message in `inbox/` (lock-free sends)
- [ ] `swarm worktree` surface: standalone subgroup vs. flags on `swarm run`? (lean: standalone)
- [ ] `swarm prompt` vs per-subsystem prompts? (lean: per-subsystem first, top-level later)
- [ ] Completion signal (`<promise>COMPLETE</promise>`): `run.Wait()` or `pane.WaitForSignal()`? (lean: `run.Wait()`)

**Build order** (decided):
1. `swarm init` + `internal/swarmfs` + `internal/config`
2. `swarm task` (most self-contained, highest standalone value)
3. `swarm msg`
4. `swarm pane` / `swarm run` (most complex)
5. `swarm worktree`
6. `swarm events tail` (log accumulates from step 2 onward; reading it is polish)

See `ARCHITECTURE.md` for full module structure, interface designs, and design principle rationale.

## Post-MVP

- `swarm events` SSE stream + REST API (NTM pattern)
- MCP server exposing `swarm task` + `swarm events`
- Kitty + Ghostty drivers
- Harness delivery adapters (notify-send, osascript, webhook)
- `swarm events on <type> --exec <cmd>` declarative hooks

---

## Reference

- Claude Code internal: `~/.claude/teams/{team}/messages/` (messaging), `~/.claude/tasks/{team}/` (tasks)
- Claude Code issues: #24122 (Zellij support), #24189 (Ghostty), #26572 (CustomPaneBackend proposal)
