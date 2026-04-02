# Modules

The `internal/` packages are the deep modules of openswarm. Command handlers are ≤15 lines; all business logic lives here. No raw `.swarm/` path strings, no JSON marshalling, no event emission in command handlers.

## swarmfs

The most fundamental module — everything else depends on it. It owns all `.swarm/` path construction and file I/O primitives.

See [[internal/swarmfs/swarmfs.go#Root]] for the central type. [[internal/swarmfs/swarmfs.go#FindRoot]] walks up from cwd to locate `.swarm/`. [[internal/swarmfs/swarmfs.go#InitRoot]] creates `.swarm/` idempotently on `swarm init`.

Key primitives:
- [[internal/swarmfs/swarmfs.go#AtomicWrite]] — temp file + `os.Rename`; readers never see partial writes
- [[internal/swarmfs/swarmfs.go#AppendLine]] — `O_APPEND` write for `events.jsonl`; safe for concurrent writers
- [[internal/swarmfs/swarmfs_unix.go#WithFileLock]] — `flock()` acquisition wrapping a callback; all mutations use this
- [[internal/swarmfs/swarmfs.go#NewID]] — `crypto/rand` ID generation, format `prefix-xxxxxx` (e.g. `task-a3f2k1`)

No other package constructs `.swarm/` path strings. If the directory layout changes, only `swarmfs` changes.

## config

Hides config file format (TOML), merge order (global → project → env vars), and defaults. [[internal/config/config.go#Load]] is the only entry point. [[internal/config/config.go#Config]] is the only interface — callers never know if values came from TOML, env, or default.

```go
type Config struct {
    TeamName      string
    DefaultAgent  string
    AgentProfiles []AgentProfile
    Backend       string  // "auto" | "tmux" | "zellij" | "wezterm"
    PollInterval  time.Duration
}
```

## events

Owns the `events.jsonl` format. All other modules call [[internal/events/events.go#Append]] — none know the JSON schema. [[internal/events/events.go#Tail]] streams events with optional type filter and configurable poll interval.

The event log accumulates from `swarm init` onward. Every mutating operation across all subsystems emits an event; the log is never written from command handlers.

Event type taxonomy (see constants in [[internal/events/events.go]]):
- `agent.registered`, `agent.deregistered`
- `task.created`, `task.assigned`, `task.claimed`, `task.done`, `task.failed`, `task.cancelled`, `task.blocked`, `task.unblocked`, `task.updated`
- `msg.sent`, `msg.read`
- `pane.created`, `pane.exited`, `pane.killed`
- `run.started`, `run.done`, `run.failed`
- `worktree.created`, `worktree.merged`, `worktree.cleaned`

## agent

Owns `agents/registry.json`. Canonical identity store shared by `msg` (routing) and `task` (assignment). See [[internal/agent/agent.go#Agent]] for the type.

Public API: [[internal/agent/agent.go#Register]], [[internal/agent/agent.go#List]], [[internal/agent/agent.go#Get]] (resolves both ID and name), [[internal/agent/agent.go#Deregister]].

## task

The most logic-heavy package. Owns `tasks.json` + `.lock`, ETag computation, blocked-status derivation, and event emission.

**Storage decision:** Single `tasks.json` with `flock()` rather than one-file-per-task. Task mutations are always read-then-write (find a task, update fields, recompute blocked status). Even with one-file-per-task a lock on cross-file `blocked_by` references is unavoidable — so the simplicity of one file wins.

See [[internal/task/task.go#Task]] for the schema. Key design points:

- [[internal/task/task.go#Claim]] is atomic: acquires lock, checks current owner, sets status+owner, releases — callers cannot race
- [[internal/task/task.go#List]] with `Ready: true` runs the 5-condition compound filter internally (not terminal, not draft, not in-progress, not deferred, not blocked)
- [[internal/task/task.go#Update]] with `ifMatch` validates ETag before mutating — optimistic locking for concurrent agents
- [[internal/task/task.go#Check]] validates `blocked_by` references; `--fix` auto-removes stale ones
- [[internal/task/task.go#Prompt]] returns agent-priming text with current task state (in-progress, ready, blocked sections)

Task status values: `draft | todo | in-progress | done | failed | cancelled`
Priority values: `critical | high | normal | low | deferred`

## msg

**Storage decision:** One file per message (lock-free sends) rather than `inbox.json`. Multiple agents can send to the same inbox simultaneously; lock-free atomic writes eliminate serialization and deadlock risk. Reads enumerate the inbox directory.

See [[internal/msg/msg.go#Message]] for the schema. Public API:
- [[internal/msg/msg.go#Send]] — atomic write of a new unique message file
- [[internal/msg/msg.go#Inbox]] — enumerate + optionally filter unread
- [[internal/msg/msg.go#Read]] — mark a message as read
- [[internal/msg/msg.go#Reply]] — create reply with `ReplyTo` set to original msg ID
- [[internal/msg/msg.go#Clear]] — remove all read messages from an inbox
- [[internal/msg/msg.go#UnreadCount]] — used by `swarm status` aggregator
- [[internal/msg/msg.go#Watch]] — polls inbox dir, emits new messages to channel

## pane

The multiplexer abstraction layer. Callers import only `internal/pane` — never a concrete driver package. The [[internal/pane/backend.go#Backend]] interface has 8 methods: `Spawn`, `Send`, `Capture`, `Subscribe`, `List`, `Close`, `Wait`, `Name`.

[[internal/pane/detect.go#DetectBackend]] implements the 5-level backend selection cascade:
`$SWARM_BACKEND` → `$TMUX` → `$WEZTERM_PANE` → `$ZELLIJ` → `$KITTY_WINDOW_ID` → fallback: tmux

Drivers register via `pane.Register()` in their `init()` — callers blank-import the driver packages they want active.

**Handshake pattern:** `Spawn()` includes a named-pipe handshake to eliminate the tmux spawn race (~50% corruption at 4+ simultaneous spawns). `$SWARM_READY_PIPE` is passed in env; shell startup writes to the pipe when ready. `Spawn()` blocks on pipe read before returning — callers always get a ready pane.

See [[backends]] for per-backend implementation details.

## run

A thin layer over `pane`. A background run is a pane (tracked by `pane`) plus a record in `runs.json` (tracked here). See [[internal/run/run.go#Run]] for the schema.

[[internal/run/run.go#Start]] calls `pane.Spawn()` and records the returned PaneID. [[internal/run/run.go#Wait]] polls pane output for the `<promise>COMPLETE</promise>` signal pattern as well as process exit — completion signal detection is a run-level concept, not a pane-level one.

Public API: [[internal/run/run.go#Start]], [[internal/run/run.go#Wait]], [[internal/run/run.go#List]], [[internal/run/run.go#Kill]], [[internal/run/run.go#Logs]].

## worktree

Wraps `git worktree` commands. Every serious multi-agent orchestration tool converges on git worktrees as the agent isolation primitive; this module makes it a first-class citizen.

See [[internal/worktree/worktree.go#Worktree]] for the schema. Public API:
- [[internal/worktree/worktree.go#New]] — create worktree + branch, record in `worktrees.json`
- [[internal/worktree/worktree.go#List]] / `Get`
- [[internal/worktree/worktree.go#Merge]] — rebase + optional PR
- [[internal/worktree/worktree.go#Clean]] — idempotent removal; no error if already gone
- [[internal/worktree/worktree.go#CleanAll]] — remove all merged/abandoned worktrees

## output

Every command output flows through this package — the single point where `--json` is applied. [[internal/output/output.go#Print]] renders structs as human-readable table/text or JSON. [[internal/output/output.go#PrintError]] renders errors as human text or `{"error": {"code": "...", "message": "..."}}`.

[[internal/output/output.go#SwarmError]] carries a machine-readable error code alongside the message. Constructors: `ErrNotFound`, `ErrConflict`, `ErrValidation`, `ErrIO`, `ErrLocked`.

Command handlers never call `json.Marshal` or `fmt.Printf` for data output — they call `output.Print(result, jsonFlag(cmd))`.
