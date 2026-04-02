# State

All openswarm state lives under `.swarm/` in the project root (or `$SWARM_DIR`). No daemons, no sockets — just files.

## Directory Layout

```
.swarm/
├── config.toml                         # project config (team name, agent profiles, backend)
├── agents/
│   └── registry.json                   # [{id, name, role, profile, created_at}]
├── messages/
│   └── <agent-id>/
│       └── inbox/
│           ├── msg-x4f2.json           # one file per message (lock-free sends)
│           └── msg-b1k9.json
├── tasks/
│   ├── tasks.json                      # [{id, title, status, priority, ...}]
│   └── .lock                           # flock target (separate from data file)
├── runs/
│   └── runs.json                       # [{id, name, cmd, pane_id, status, ...}]
├── worktrees/
│   └── worktrees.json                  # [{id, branch, path, agent_id, status}]
└── events/
    └── events.jsonl                    # append-only, one JSON object per line
```

`swarm init` creates all directories idempotently via [[internal/swarmfs/swarmfs.go#InitRoot]].

## Storage Patterns

Two different patterns are used depending on concurrency requirements.

### Single-file with flock

Used for `tasks.json`, `agents/registry.json`, `runs.json`, `worktrees.json`.

Mutations acquire an exclusive flock via [[internal/swarmfs/swarmfs_unix.go#WithFileLock]], read the full file, mutate in memory, and write back atomically via [[internal/swarmfs/swarmfs.go#AtomicWrite]] (temp + rename). Readers never see a partial write.

The flock file for tasks is `.swarm/tasks/.lock` — separate from the data file so the lock target is stable even as data is replaced by rename.

### One-file-per-record (lock-free)

Used for messages: `.swarm/messages/<agent-id>/inbox/<msg-id>.json`.

Multiple agents can send to the same inbox concurrently — each send is an atomic write of a uniquely-named file with no lock required. Reads enumerate the directory. This matches the pattern used by Claude Code's internal messaging.

### Append-only log

`events.jsonl` is written with `O_APPEND` via [[internal/swarmfs/swarmfs.go#AppendLine]]. Safe for concurrent writers without a lock (append to the same file is atomic for small writes on POSIX). Consumers use `tail -f` or [[internal/events/events.go#Tail]] which polls from a byte offset.

## Config

Config merges in order: global (`~/.config/swarm/config.toml`) → project (`.swarm/config.toml`) → environment variables. See [[internal/config/config.go#Load]].

Environment variables:
- `$SWARM_DIR` — override project root (the directory containing `.swarm/`)
- `$SWARM_BACKEND` — force a specific multiplexer backend
- `$SWARM_AGENT_ID` — implicit sender/recipient for `swarm msg` commands

## ETag / Optimistic Locking

Task records use content-hash ETags for optimistic concurrency control.

Every `Task` carries an `etag` (SHA-256 of mutable fields). `swarm task update --if-match <etag>` returns `ErrConflict` if the task changed since the agent last read it. Prevents lost-update races without long-held locks.
