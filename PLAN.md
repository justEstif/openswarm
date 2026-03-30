# openswarm — Development Plan

> High-level phase tracker. Updated as phases complete.
> Detailed design: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
> Decisions log: [`docs/NOTES.md`](docs/NOTES.md)

---

## Status legend

| Symbol | Meaning |
|---|---|
| ⬜ | Not started |
| 🔄 | In progress |
| ✅ | Done |

---

## Phase 0 — Foundation

*Everything else blocks on this. Zero dependencies between steps — but ship in order.*

| # | Deliverable | Status | Notes |
|---|---|---|---|
| 0.1 | `internal/swarmfs` — `FindRoot`, `AtomicWrite`, `AppendLine`, `WithFileLock`, `NewID` | ✅ | Owns all `.swarm/` path construction. Nothing else may construct paths by hand. |
| 0.2 | `internal/output` — `Print`, `PrintError`, `SwarmError` | ✅ | Zero deps. Establishes `--json` contract for every command from day one. |
| 0.3 | `internal/config` — `Load` | ✅ | Hides TOML format, env var merging, defaults. |
| 0.4 | `internal/events` — `Append`, `Tail` | ✅ | Append-only `events.jsonl`. Must exist before any store is written. |
| 0.5 | `cmd/swarm` root — root cobra command, `--json` global flag, `mustRoot()` middleware, `swarm init`, `swarm version` | ✅ | Skeleton all commands hang off of. |

**Gate:** `swarm init` creates `.swarm/`, `swarm version` prints version. Nothing crashes.

---

## Phase 1 — Task subsystem

*Highest standalone value. Works with zero multiplexer involvement.*

| # | Deliverable | Status | Notes |
|---|---|---|---|
| 1.1 | `internal/agent` — `Register`, `List`, `Get`, `Deregister` | ✅ | Small surface. Needed by task (assignment), msg (routing), worktree (binding). |
| 1.2 | `internal/task` — `Add`, `List`, `Get`, `Update`, `Assign`, `Claim`, `Done`, `Fail`, `Cancel`, `Block`, `Unblock`, `Check`, `Prompt` | ✅ | Most logic-heavy package. Single `tasks.json` + `flock()`. ETag optimistic locking. |
| 1.3 | `swarm agent` commands — `register`, `list`, `get`, `deregister` | ✅ | Thin cobra handlers (~15 lines each). |
| 1.4 | `swarm task` commands — `add`, `list`, `get`, `update`, `assign`, `claim`, `done`, `fail`, `cancel`, `block`, `check`, `prompt` | ✅ | Thin cobra handlers. |

**Gate:** `swarm init && swarm agent register alice researcher && swarm task add "build the thing" && swarm task claim <id> --as alice && swarm task done <id>` works end-to-end, human and `--json` output both correct.

---

## Phase 2 — Messaging

*Depends only on Phase 0 + agent. Lock-free one-file-per-message sends.*

| # | Deliverable | Status | Notes |
|---|---|---|---|
| 2.1 | `internal/msg` — `Send`, `Inbox`, `Read`, `Reply`, `Clear`, `UnreadCount`, `Watch` | ✅ | Sends are atomic file writes (no lock). Reads enumerate inbox dir. |
| 2.2 | `swarm msg` commands — `send`, `inbox`, `read`, `reply`, `clear` | ✅ | Thin cobra handlers. |

**Gate:** Two agents in the same project root can `swarm msg send` and `swarm msg inbox` with no multiplexer running.

---

## Phase 3 — Pane + Run

*Highest implementation risk. Delay until Phases 1–2 have validated the package structure.*

| # | Deliverable | Status | Notes |
|---|---|---|---|
| 3.1 | `Backend` interface + `DetectBackend` in `internal/pane/backend.go` | ⬜ | **Commit the interface before any implementation.** 8 methods: `Spawn`, `Send`, `Capture`, `Subscribe`, `List`, `Close`, `Wait`, `Name`. |
| 3.2 | tmux backend | ⬜ | Most battle-tested. Ship first to validate the interface concretely. |
| 3.3 | Zellij backend | ⬜ | Only backend with native push Subscribe (`zellij subscribe --format json`). |
| 3.4 | WezTerm backend | ⬜ | Same coverage as tmux (polling Subscribe). Exit code gap: returns `-1` by convention. |
| 3.5 | Ghostty stub | ⬜ | `ErrNotSupported` on all methods with pointer to issue #4625. |
| 3.6 | `internal/run` — `Start`, `Wait`, `List`, `Kill` | ⬜ | Thin layer on `pane`. `<promise>COMPLETE</promise>` signal detection lives here. |
| 3.7 | `swarm pane` commands — `spawn`, `send`, `capture`, `list`, `close` | ⬜ | Thin cobra handlers. |
| 3.8 | `swarm run` commands — `start`, `wait`, `list`, `kill`, `logs` | ⬜ | Thin cobra handlers. |

**Parallelism:** Once 3.1 (interface) is done, steps 3.2 / 3.3 / 3.4 are fully independent and can be assigned to separate agents simultaneously.

**Gate:** `swarm run --name my-build -- go build ./...` spawns a pane, waits for exit, records the run, emits `run.done` to the event log.

---

## Phase 4 — Worktrees

*Useful, not blocking. Pure `git worktree` wrapping.*

| # | Deliverable | Status | Notes |
|---|---|---|---|
| 4.1 | `internal/worktree` — `New`, `List`, `Merge`, `Clean`, `CleanAll` | ⬜ | Depends on `swarmfs`, `events`, `agent`. |
| 4.2 | `swarm worktree` commands — `new`, `list`, `merge`, `clean` | ⬜ | Thin cobra handlers. |

**Gate:** `swarm worktree new --branch feature/x --agent alice` creates a worktree at a canonical path and records it in `worktrees.json`.

---

## Phase 5 — Polish

*Event log has been accumulating since Phase 0. This phase is just readers and aggregators.*

| # | Deliverable | Status | Notes |
|---|---|---|---|
| 5.1 | `swarm events tail` | ⬜ | `events.Tail()` already works. This is the streaming CLI reader + `--filter` flag. |
| 5.2 | `swarm status` | ⬜ | Aggregator: calls `task.List`, `msg.UnreadCount`, `agent.List`, `pane.List`. Intentionally thin. |
| 5.3 | `swarm prompt` | ⬜ | Concatenates per-subsystem prompts for agent priming. Calls `task.Prompt()`. |

---

## Parallel opportunities

```
Phase 0  (sequential — each step unlocks the next)
    ↓
Phase 1 + Phase 2  (can run in parallel once Phase 0 is done)
    ↓
Phase 3.1  (interface definition — must be sequential, one owner)
    ↓
Phase 3.2 / 3.3 / 3.4  (backends — fully parallel after interface is locked)
    ↓
Phase 3.6 / Phase 4  (can overlap)
    ↓
Phase 5  (polish — fully parallel)
```

---

## Guiding principle

> Delay multiplexer complexity as long as possible.

Phases 1 and 2 deliver real, usable primitives with zero dependency on whether tmux, Zellij, or WezTerm is running. An agent can coordinate tasks and exchange messages from day one.

---

## Key constraints (do not violate)

- **Command handlers ≤ 15 lines.** All logic lives in `internal/`.
- **No raw `.swarm/` path strings outside `swarmfs`.** All paths through `Root` methods.
- **Every mutating function calls `events.Append()`.** Never from command handlers.
- **`--json` on every command.** Wired through `output.Print` / `output.PrintError`.
- **Backend callers never import `pane/tmux` or `pane/zellij`.** Only `pane`.
