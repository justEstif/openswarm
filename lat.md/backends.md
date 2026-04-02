# Backends

The multiplexer backend abstraction lets `swarm pane` and `swarm run` work identically across tmux, Zellij, and WezTerm. All logic lives behind the [[internal/pane/backend.go#Backend]] interface.

## Backend Interface

The [[internal/pane/backend.go#Backend]] interface has exactly 8 methods — defined by swarm's use cases, not by multiplexer capabilities. A 40-method shallow interface would expose the multiplexer's model; 8 methods hide it.

```go
Spawn(name, cmd string, env map[string]string) (PaneID, error)
Send(id PaneID, text string) error
Capture(id PaneID) (string, error)
Subscribe(ctx context.Context, id PaneID) (<-chan OutputEvent, error)
List() ([]PaneInfo, error)
Close(id PaneID) error
Wait(id PaneID) (int, error)
Name() string
```

`Close()` is idempotent — no error if the pane is already gone. `Wait()` returns -1 for exit code when the backend cannot provide it (WezTerm). **Shell-wrapping contract**: each backend wraps `cmd` in its own `sh -c` internally; callers must pass the raw command string and must NOT pre-wrap it. Double-wrapping causes `sh: command not found` errors.

## Backend Registration

Drivers register via `pane.Register()` in their `init()` functions. Callers blank-import driver packages to activate them. Callers never import `pane/tmux` or `pane/zellij` directly — only `internal/pane`.

## Detection

[[internal/pane/detect.go#DetectBackend]] implements the 5-level cascade (highest priority first):

1. `$SWARM_BACKEND` config value — explicit override
2. `$TMUX` set → tmux
3. `$WEZTERM_PANE` set → wezterm
4. `$ZELLIJ` set → zellij
5. `$KITTY_WINDOW_ID` set → kitty (post-MVP)
6. Fallback → tmux

## Handshake Pattern

All backends use a named-pipe handshake to eliminate the spawn race. Without it, ~50% of tasks are corrupted at 4+ simultaneous spawns (a known Claude Code bug).

`Spawn()` passes `SWARM_READY_PIPE=<path>` in the env. The shell startup script writes to the pipe when ready. `Spawn()` blocks on the pipe read before returning — callers always get a ready pane.

- **tmux**: uses `tmux wait-for -L <channel>` (built-in lock/unlock mechanism)
- **Zellij / WezTerm**: uses a Unix FIFO at `/tmp/swarm_pipe_{pid}_{nanos}`

## Backend Coverage

All three MVP backends implement the full interface. Zellij is uniquely the only backend with native push `Subscribe`.

| Method | tmux | Zellij ≥0.44 | WezTerm |
|---|---|---|---|
| Spawn | ✅ | ✅ | ✅ |
| Send | ✅ | ✅ | ✅ |
| Capture | ✅ | ✅ | ✅ |
| Subscribe | ⚠️ poll | ✅ **native push** | ⚠️ poll |
| List | ✅ | ✅ | ✅ |
| Close (idempotent) | ✅ | ✅ | ✅ |
| Wait (exit code) | ✅ | ✅ (two-step) | ⚠️ returns -1 |
| Name | ✅ | ✅ | ✅ |

Zellij provides lower latency via `zellij subscribe --format json` (native push). Validated since v0.44.0 (2026). WezTerm cannot provide exit codes structurally; `Wait()` returns -1 by convention.

## Ghostty Stub

Ghostty has no external API (issue #4625, no milestone). A `GhosttyBackend` stub returns `ErrNotSupported` on all methods with a message pointing to #4625, so users see a clear error rather than a silent fallback.

## Completion Signal

`swarm run wait` detects the `<promise>COMPLETE</promise>` pattern in pane output, in addition to process exit. This is a run-level concept (in `internal/run`) — the `Backend` interface does not expose it.
