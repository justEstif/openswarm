# Architecture

openswarm is a unified Go CLI (`swarm`) that recreates Claude Code's multi-agent team mode as open, composable primitives — decoupled from any specific AI provider or terminal multiplexer.

## Goal

Re-implement the coordination substrate that powers Claude Code's internal agent teams as a standalone, composable tool stack usable with any agent, any multiplexer.

Claude Code's internal system is file-backed with `flock()`, no daemon, flat JSON files — our design mirrors this validated architecture while making it available outside Claude Code.

## Subsystems

openswarm has three primary subsystems, unified under one binary and one state root (`.swarm/`):

- **`swarm msg`** — lock-free peer messaging between agents via per-message inbox files
- **`swarm task`** — shared task queue with flock-safe atomic mutations
- **`swarm pane` / `swarm run`** — spawn/control terminal panes across multiplexers (tmux, Zellij, WezTerm)
- **`swarm worktree`** — git worktree lifecycle tied to agent identity
- **`swarm events`** — tail the append-only cross-subsystem event log

## Module Structure

All business logic lives in `internal/`. Command handlers under `cmd/swarm/` are ≤15 lines — they parse flags, call `internal/`, and call `output.Print`.

```
cmd/swarm/           — binary entry point + thin cobra command handlers
internal/
  swarmfs/           — DEEP MODULE: all .swarm/ path construction + file I/O primitives
  config/            — DEEP MODULE: config loading (TOML + env merging)
  events/            — DEEP MODULE: append-only event log
  agent/             — DEEP MODULE: agent registry
  task/              — DEEP MODULE: task subsystem
  msg/               — DEEP MODULE: messaging subsystem
  pane/              — DEEP MODULE: multiplexer abstraction + drivers
  run/               — DEEP MODULE: background run tracking
  worktree/          — DEEP MODULE: git worktree management
  output/            — DEEP MODULE: output formatting (human + JSON)
```

## Design Principles

The design follows "A Philosophy of Software Design" — deep modules, information hiding, pull complexity downward.

### Deep Modules Over Shallow Ones

Each `internal/` package hides its full complexity behind a small verb-oriented interface. The deepest is `task` — it hides JSON schema, `flock()`, ETag computation, event emission, and blocked-status derivation behind ~15 exported functions.

### Information Hiding

Each package owns exactly the knowledge that belongs to it. Nothing leaks across package boundaries.

| Knowledge | Owned by | Not leaked to |
|---|---|---|
| `tasks.json` JSON schema | `task/task.go` | command handlers, status |
| `events.jsonl` format | `events/events.go` | task, msg, pane, run |
| Backend selection (5-level cascade) | `pane/detect.go` | commands, run |
| Config file format (TOML) | `config/config.go` | everything else |
| `.swarm/` path construction | `swarmfs/swarmfs.go` | everything else |

No caller ever constructs a `.swarm/` path string by hand. All paths flow through [[internal/swarmfs/swarmfs.go#Root]] methods.

### Pull Complexity Downward

Complex logic is pushed into the deepest module so callers remain simple. Examples:

- `task.Claim()` is atomic — callers don't manage locking
- `task.List(f{Ready: true})` runs the 5-condition compound filter — callers don't implement it
- `pane.Spawn()` includes the handshake — callers always receive a ready pane
- `events.Append()` constructs the full Event struct — callers pass 4 strings
- `output.Print()` handles text vs JSON — callers never call `json.Marshal`

### Command Handler Pattern

Every command handler follows this exact pattern with no exceptions:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    root, cfg := mustRoot(cmd)     // middleware: FindRoot + Load
    result, err := task.Add(root, args[0], task.AddOpts{...})
    if err != nil {
        output.PrintError(err, jsonFlag(cmd))
        return nil
    }
    return output.Print(result, jsonFlag(cmd))
}
```

### Define Errors Out of Existence

Idempotent operations never error on repeated calls:
- `pane.Close()` on already-closed pane → nil
- `worktree.Clean()` on non-existent worktree → nil
- `task.Done()` on already-done task → nil
- `swarmfs.FindRoot()` failure → clear actionable error: `"no .swarm/ found; run swarm init"`

## Data Flow

A command handler's lifecycle:

```
1. cobra parses flags
2. mustRoot(): swarmfs.FindRoot() + config.Load()   ← once, shared
3. call internal package function (e.g. task.Add())
   └── internal function:
       a. swarmfs.WithFileLock(...)
       b. read + unmarshal state
       c. mutate
       d. marshal + swarmfs.AtomicWrite(...)
       e. events.Append(...)          ← automatic, never forgotten
4. output.Print(result, jsonFlag)
```

Callers never touch `flock`, never touch `AtomicWrite`, never emit events directly.
