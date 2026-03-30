# openswarm — Architecture

> Design phase document. Not implementation.
> Applying "A Philosophy of Software Design" — deep modules, information hiding, pull complexity downward.

---

## Module Structure

```
cmd/swarm/              # binary entry point
  main.go
  root.go               # root cobra command, global flags, middleware

internal/
  swarmfs/              # DEEP MODULE: file system primitives
  config/               # DEEP MODULE: config loading
  events/               # DEEP MODULE: event log
  agent/                # DEEP MODULE: agent registry
  task/                 # DEEP MODULE: task subsystem
  msg/                  # DEEP MODULE: messaging subsystem
  pane/                 # DEEP MODULE: mux control
  run/                  # DEEP MODULE: background run tracking
  worktree/             # DEEP MODULE: git worktree management
  output/               # DEEP MODULE: output formatting

cmd/swarm/commands/     # THIN: cobra command definitions (parse flags, call internal, print)
  init.go
  agent.go
  task.go
  msg.go
  pane.go
  run.go
  worktree.go
  events.go
  status.go
  prompt.go
```

**Rule:** Command handlers parse flags and format output. Zero business logic. All logic lives in `internal/`.

---

## The Deep Modules

### `internal/swarmfs` — File system primitives

The most fundamental module. Everything else depends on it. It hides:

- Directory walk-up to find `.swarm/` root (like `git rev-parse --show-toplevel`)
- Atomic writes (temp file + `os.Rename`)
- Append-only writes (for event log)
- `flock()` acquisition and release
- NanoID generation (with offensive-word filter)

```go
// Root represents a resolved .swarm/ project root.
// Every command that touches project state needs one.
type Root struct {
    Dir        string // absolute path to .swarm/
    ConfigPath string
    // derived paths are methods, not fields — computed lazily
}

func FindRoot() (*Root, error)       // walks up from cwd, returns error if not found
func (r *Root) TasksPath() string
func (r *Root) InboxPath(agentID string) string
func (r *Root) EventsPath() string
// etc.

func AtomicWrite(path string, data []byte) error
func AppendLine(path string, data []byte) error  // for events.jsonl
func WithFileLock(path string, fn func() error) error
func NewID(prefix string) string     // "task-a3f2", "msg-x4f2", "run-b1k9"
```

**Information hiding:** No caller ever constructs a `.swarm/` path string by hand. All paths flow through `Root` methods. If the directory layout changes, only `swarmfs` changes.

---

### `internal/config` — Config loading

Hides config file format (TOML), merge order (global → project → env vars), and defaults.

```go
type Config struct {
    TeamName     string
    DefaultAgent string
    AgentProfiles []AgentProfile  // name + command + args
    Backend      string           // "auto" | "tmux" | "zellij" | "kitty"
}

type AgentProfile struct {
    Name    string
    Command string
    Args    []string
}

func Load(root *swarmfs.Root) (*Config, error)
```

**Information hiding:** No caller knows whether config came from TOML, env var, or default. The `Config` struct is the only interface.

---

### `internal/events` — Event log

Owns the `events.jsonl` format. All other modules call `events.Append()` — none of them know the JSON schema.

```go
type Event struct {
    ID     string          `json:"id"`
    Source string          `json:"source"`   // "task" | "msg" | "pane" | "run" | "worktree" | "agent"
    Type   string          `json:"type"`     // see taxonomy below
    Ref    string          `json:"ref"`      // ID of the affected resource
    Data   json.RawMessage `json:"data,omitempty"` // source-specific payload
    At     time.Time       `json:"at"`
}

// Event type constants — the full taxonomy
const (
    TypeAgentRegistered   = "agent.registered"
    TypeAgentDeregistered = "agent.deregistered"

    TypeTaskCreated   = "task.created"
    TypeTaskAssigned  = "task.assigned"
    TypeTaskClaimed   = "task.claimed"
    TypeTaskDone      = "task.done"
    TypeTaskFailed    = "task.failed"
    TypeTaskCancelled = "task.cancelled"
    TypeTaskBlocked   = "task.blocked"
    TypeTaskUnblocked = "task.unblocked"
    TypeTaskUpdated   = "task.updated"

    TypeMsgSent = "msg.sent"
    TypeMsgRead = "msg.read"

    TypePaneCreated = "pane.created"
    TypePaneExited  = "pane.exited"  // Data: {"exit_code": 0}
    TypePaneKilled  = "pane.killed"

    TypeRunStarted = "run.started"
    TypeRunDone    = "run.done"
    TypeRunFailed  = "run.failed"

    TypeWorktreeCreated = "worktree.created"
    TypeWorktreeMerged  = "worktree.merged"
    TypeWorktreeCleaned = "worktree.cleaned"
)

func Append(root *swarmfs.Root, eventType, source, ref string, data any) error
func Tail(root *swarmfs.Root, filter string) (<-chan Event, error)  // streams events.jsonl
```

**Information hiding:** The `events.jsonl` format is owned entirely here. If we add a field, only this package changes.

**Pull complexity downward:** Other packages call `events.Append()` once per mutation — they don't construct Event structs, they don't know about `AppendLine`, they don't manage the path.

---

### `internal/agent` — Agent registry

Owns `agents/registry.json`. Canonical identity store shared by msg and task.

```go
type Agent struct {
    ID        string    `json:"id"`         // "researcher-a3f2"
    Name      string    `json:"name"`       // human label
    Role      string    `json:"role"`       // "researcher" | "implementer" | ...
    ProfileRef string   `json:"profile"`    // references config.AgentProfiles[*].Name
    CreatedAt time.Time `json:"created_at"`
}

func Register(root *swarmfs.Root, name, role, profile string) (*Agent, error)
func List(root *swarmfs.Root) ([]*Agent, error)
func Get(root *swarmfs.Root, idOrName string) (*Agent, error)   // resolves either form
func Deregister(root *swarmfs.Root, idOrName string) error
```

---

### `internal/task` — Task subsystem

**Design choice (chosen after "design it twice"):** Single `tasks.json` with `flock()`, not one-file-per-task.

**Why:** Task mutations are always read-then-write (find a task, update its fields, recompute blocked status). Even with one-file-per-task, you need a lock to safely update `blocked_by` references across multiple files. The flock is unavoidable, so the simplicity of one file wins.

**What this module hides:**

- `tasks.json` schema (callers never see raw JSON)
- `flock()` acquisition
- `blocked` status computation (derived at read time from `blocked_by` + task statuses)
- ETag generation (SHA-256 of task content)
- Event emission on every mutation
- NanoID generation for new tasks

```go
type Task struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Status    Status    `json:"status"`
    Priority  Priority  `json:"priority"`
    Tags      []string  `json:"tags"`
    AssignedTo string   `json:"assigned_to,omitempty"`
    BlockedBy  []string `json:"blocked_by,omitempty"`
    Output    string    `json:"output,omitempty"`
    Notes     string    `json:"notes,omitempty"`
    ETag      string    `json:"etag"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Status  string  // "draft" | "todo" | "in-progress" | "done" | "failed" | "cancelled"
type Priority string // "critical" | "high" | "normal" | "low" | "deferred"

type ListFilter struct {
    Status      []Status
    ExcludeStatus []Status
    AssignedTo  string
    Tags        []string
    Ready       bool     // compound: not blocked + not terminal + not in-progress + not deferred
    SortBy      string   // "priority" | "created" | "updated" | "status"
}

func Add(root *swarmfs.Root, title string, opts AddOpts) (*Task, error)
func List(root *swarmfs.Root, f ListFilter) ([]*Task, error)
func Get(root *swarmfs.Root, id string) (*Task, error)
func Update(root *swarmfs.Root, id string, opts UpdateOpts, ifMatch string) (*Task, error)
func Assign(root *swarmfs.Root, id, agentID string) (*Task, error)
func Claim(root *swarmfs.Root, id, agentID string) (*Task, error)  // atomic: only succeeds if unclaimed
func Done(root *swarmfs.Root, id, output string) (*Task, error)
func Fail(root *swarmfs.Root, id, reason string) (*Task, error)
func Cancel(root *swarmfs.Root, id string) (*Task, error)
func Block(root *swarmfs.Root, id, blockedByID string) (*Task, error)
func Unblock(root *swarmfs.Root, id, blockerID string) (*Task, error)
func Remove(root *swarmfs.Root, id string) error    // warns if other tasks depend on it
func Check(root *swarmfs.Root, fix bool) ([]Problem, error)
func Prompt(root *swarmfs.Root) string              // returns agent-priming text
```

**Pull complexity downward:**

- `List()` with `Ready: true` runs the compound filter internally — callers don't implement it
- `Claim()` is atomic: acquires lock, checks status, sets status+owner, releases lock in one operation
- `Update()` with `ifMatch` validates ETag before mutating — callers don't manage optimistic locking
- Every mutating function calls `events.Append()` — callers never emit task events

---

### `internal/msg` — Messaging subsystem

**Design choice:** One file per message (lock-free sends), not inbox.json.

**Why:** Multiple agents can send to the same inbox simultaneously. If sends require a lock on inbox.json, we serialize all sends — a performance bottleneck and a potential deadlock risk. With one file per message, sends are lock-free (each send = atomic write of a new unique file). Reads enumerate the directory. This matches Claude Code's actual pattern.

```go
type Message struct {
    ID        string    `json:"id"`
    From      string    `json:"from"`      // agent ID
    To        string    `json:"to"`        // agent ID
    Body      string    `json:"body"`
    ReplyTo   string    `json:"reply_to,omitempty"`  // msg ID being replied to
    Read      bool      `json:"read"`
    CreatedAt time.Time `json:"created_at"`
}

func Send(root *swarmfs.Root, from, to, body string, opts SendOpts) (*Message, error)
func Inbox(root *swarmfs.Root, agentID string, unreadOnly bool) ([]*Message, error)
func Read(root *swarmfs.Root, agentID, msgID string) (*Message, error)   // marks as read
func Reply(root *swarmfs.Root, agentID, msgID, body string) (*Message, error)
func Clear(root *swarmfs.Root, agentID string) error
func UnreadCount(root *swarmfs.Root, agentID string) (int, error)         // used by swarm status
func Watch(root *swarmfs.Root, agentID string) (<-chan *Message, error)   // polls inbox dir
```

---

### `internal/pane` — Mux control subsystem

The most structurally complex module because it must hide the differences between tmux, Zellij, and future backends.

**Key design decision: Backend interface depth**

**Option A (workmux-style, 40 methods):** Thin interface, exposes each multiplexer primitive.
**Option B (swarm-style, 8 methods):** Deep interface, exposes only what swarm needs.

**Chosen: Option B.** A 40-method interface is shallow — it exposes the multiplexer's model rather than hiding it. The `Backend` interface should be defined by swarm's use cases, not by multiplexer capabilities. If we need a new multiplexer operation, we add it to the interface when we need it.

```go
// Backend is the multiplexer abstraction.
// Implementations: TmuxBackend, ZellijBackend (future: KittyBackend, GhosttyBackend)
type Backend interface {
    // Spawn creates a new pane running cmd, returns its ID.
    // Blocks until the pane shell is ready (handshake).
    Spawn(name string, cmd string, env map[string]string) (PaneID, error)

    // Send sends text to a pane's stdin.
    Send(id PaneID, text string) error

    // Capture returns the current viewport + scrollback of a pane.
    Capture(id PaneID) (string, error)

    // Subscribe streams output events from a pane until it exits or ctx is cancelled.
    Subscribe(ctx context.Context, id PaneID) (<-chan OutputEvent, error)

    // List returns all panes in the current session.
    List() ([]PaneInfo, error)

    // Close closes a pane. Idempotent — does not error if already gone.
    Close(id PaneID) error

    // Wait blocks until the pane exits, returns exit code.
    Wait(id PaneID) (int, error)

    // Name returns the backend name ("tmux" | "zellij" | ...).
    Name() string
}

type PaneID string

type PaneInfo struct {
    ID      PaneID
    Name    string
    Running bool
    Command string
}

type OutputEvent struct {
    PaneID  PaneID
    Text    string
    Exited  bool
    Code    int
}

// DetectBackend implements the 5-level cascade.
// Priority: $SWARM_BACKEND → $TMUX → $WEZTERM_PANE → $ZELLIJ → $KITTY_WINDOW_ID → fallback:tmux
func DetectBackend(cfg *config.Config) (Backend, error)

// Public API — callers use these, not Backend directly
func Spawn(root *swarmfs.Root, b Backend, name, cmd string, env map[string]string) (*PaneRecord, error)
func Send(root *swarmfs.Root, b Backend, id PaneID, text string) error
func Capture(root *swarmfs.Root, b Backend, id PaneID) (string, error)
func List(root *swarmfs.Root, b Backend) ([]*PaneRecord, error)
func Close(root *swarmfs.Root, b Backend, id PaneID) error
func Wait(root *swarmfs.Root, b Backend, id PaneID) (int, error)
```

**Handshake pattern:** `Spawn()` includes a named-pipe (FIFO) handshake — it passes `SWARM_READY_PIPE=<path>` in env, and the shell startup script writes to that pipe when ready. `Spawn()` blocks on the pipe read before returning. This eliminates the tmux spawn race (the ~50% corruption bug at 4+ simultaneous spawns).

- **tmux**: uses `tmux wait-for -L <channel>` (built-in lock/unlock mechanism)
- **Zellij / WezTerm**: uses a Unix FIFO at `/tmp/swarm_pipe_{pid}_{nanos}` (no built-in sync primitive)
- Both approaches validated in production by workmux

**Information hiding:**

- Callers never import `pane/tmux` or `pane/zellij` — only `pane`
- Backend selection logic is entirely in `DetectBackend()` — callers never check env vars
- The handshake mechanism is invisible to callers — `Spawn()` always returns a ready pane

---

### `internal/run` — Background run tracking

A background run is: a pane (tracked by `pane`) + a record in `runs.json` (tracked here).

The `run` package depends on `pane`. It calls `pane.Spawn()` and records the returned PaneID in its run record.

```go
type Run struct {
    ID      string    `json:"id"`
    Name    string    `json:"name"`
    Cmd     string    `json:"cmd"`
    PaneID  pane.PaneID `json:"pane_id"`
    Status  string    `json:"status"`   // "running" | "done" | "failed"
    ExitCode int      `json:"exit_code,omitempty"`
    StartedAt time.Time `json:"started_at"`
    EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func Start(root *swarmfs.Root, b pane.Backend, name, cmd string, env map[string]string) (*Run, error)
func Wait(root *swarmfs.Root, b pane.Backend, id string) (*Run, error)
func List(root *swarmfs.Root) ([]*Run, error)
func Kill(root *swarmfs.Root, b pane.Backend, id string) error
```

---

### `internal/worktree` — Git worktree management

```go
type Worktree struct {
    ID        string    `json:"id"`
    Branch    string    `json:"branch"`
    Path      string    `json:"path"`     // absolute path to worktree dir
    AgentID   string    `json:"agent_id,omitempty"`
    Status    string    `json:"status"`   // "active" | "merged" | "abandoned"
    CreatedAt time.Time `json:"created_at"`
}

func New(root *swarmfs.Root, branch, agentID string) (*Worktree, error)
func List(root *swarmfs.Root) ([]*Worktree, error)
func Merge(root *swarmfs.Root, id string, opts MergeOpts) error  // rebase + optional PR
func Clean(root *swarmfs.Root, id string) error   // idempotent
func CleanAll(root *swarmfs.Root) ([]*Worktree, error)  // cleans all merged/abandoned
```

---

### `internal/output` — Output formatting

Every command produces output. Rather than each command handler deciding how to format things, all output flows through this package. This is the point where `--json` is applied.

```go
// Print renders v as human-readable table/text or JSON depending on the command's --json flag.
// v must be a struct or slice of structs with json tags.
func Print(v any, asJSON bool) error

// PrintError renders an error as human-readable text or structured JSON.
// JSON form: {"error": {"code": "NOT_FOUND", "message": "..."}}
func PrintError(err error, asJSON bool)

// Error codes for structured errors
type SwarmError struct {
    Code    string `json:"code"`    // "NOT_FOUND" | "CONFLICT" | "VALIDATION_ERROR" | ...
    Message string `json:"message"`
}
func (e *SwarmError) Error() string
```

**Pull complexity downward:** Command handlers never call `json.Marshal` or `fmt.Printf` for data output. They call `output.Print(result, jsonFlag)` and return.

---

## Backend Comparison

*Full research in `research/backends/`. All findings from workmux Rust source + official docs.*

| Method | tmux | Zellij v0.44.0 | WezTerm | Kitty | Ghostty |
|---|---|---|---|---|---|
| **Spawn** | ✅ `new-window -P -F '#{pane_id}'` | ✅ `new-pane` returns `terminal_N` | ✅ `cli spawn` returns int pane_id | ✅ `kitten @ launch` returns window_id | ❌ no API |
| **Send** | ✅ `send-keys -t %N -l text` | ✅ `write-chars --pane-id N` | ✅ `cli send-text --pane-id N` | ✅ `kitten @ send-text --match id:N` | ❌ no API |
| **Capture** | ✅ `capture-pane -t %N -p -S -500` | ✅ `dump-screen --pane-id N --full` | ✅ `cli get-text --pane-id N --start-line -200` | ✅ `kitten @ get-text --match id:N --extent all` | ❌ no API |
| **Subscribe** | ⚠️ poll `capture-pane` ~200ms | ✅ **`zellij subscribe --pane-id N --format json`** (push, NDJSON) | ⚠️ poll `get-text` ~200ms | ⚠️ poll `get-text` ~200ms | ❌ no API |
| **List** | ✅ `list-panes -a -F #{pane_id}...` | ✅ `list-panes --json` (rich JSON) | ✅ `cli list --format json` | ✅ `kitten @ ls` (JSON tree) | ❌ no API |
| **Close** | ✅ `kill-pane -t %N` | ✅ `close-pane --pane-id N` | ✅ `cli kill-pane --pane-id N` | ✅ `kitten @ close-window --match id:N` | ❌ no API |
| **Wait** | ⚠️ poll `#{pane_dead_status}` (need `remain-on-exit on` at spawn) | ⚠️ poll `list-panes` for `exit_status` (two-step) | ⚠️ poll until pane gone; **exit code lost** | ⚠️ `--wait-for-child-to-exit` at spawn only | ❌ no API |
| **Env inject** | ⚠️ `env K=V cmd` prefix (no `-e` flag) | ⚠️ `env K=V cmd` prefix | ⚠️ `env K=V cmd` prefix | ✅ `--env KEY=VAL` flag | ❌ no API |
| **Handshake** | ✅ `wait-for -L/-U channel` (built-in) | ✅ Unix FIFO | ✅ Unix FIFO | ✅ Unix FIFO | ❌ |
| **Detection env var** | `$TMUX` | `$ZELLIJ` | `$WEZTERM_PANE` | `$KITTY_WINDOW_ID` | none |

### Backend readiness

| Backend | MVP? | Native methods | Notes |
|---|---|---|---|
| **tmux** | ✅ Yes | 6/8 (Subscribe + Wait polled) | Most battle-tested; workmux production-validated |
| **Zellij v0.44.0** | ✅ Yes | **7/8** (only Wait is two-step) | Unique: only backend with native push Subscribe. Exit code gap is minor. |
| **WezTerm** | ✅ Yes | 6/8 (Subscribe + Wait polled) | Same coverage as tmux; exit code structurally inaccessible |
| **Kitty** | ⚠️ Post-MVP | 6/8 (Subscribe + Wait polled) | Requires `allow_remote_control=yes` pre-configured; can't set at runtime |
| **Ghostty** | ❌ Aspirational | 1/8 (Name only) | Zero external API. Issue #4625 open, no milestone. |

### Key design implications for `internal/pane`

1. **Subscribe must be polling-by-default** — only Zellij has native push. The `Backend.Subscribe()` interface returns a `<-chan OutputEvent` regardless; implementation may poll or push. Document that Zellij provides lower latency.

2. **`Wait()` exit code** — tmux and Zellij can return actual exit codes (via `remain-on-exit` / `list-panes exit_status`). WezTerm cannot without a wrapper script. Backend.Wait() returns `(int, error)` where WezTerm returns `-1` (unknown) by convention.

3. **Env injection** — all backends use the `env K=V cmd` prefix pattern. Kitty is the only one with a native flag. Our `Spawn()` should always generate `env KEY=val ... actualcmd` regardless of backend.

4. **Zellij is now the strongest backend** — 7/8 native methods, native Subscribe, stable pane IDs, rich JSON from `list-panes`. Ship tmux and Zellij together as the v1 pair.

5. **Ghostty**: Register a stub `GhosttyBackend` that returns `ErrNotSupported` on all methods with a helpful message pointing to issue #4625. This lets users see a clear error rather than silent fallback.

---

## `.swarm/` Directory Layout (final)

```
.swarm/
├── config.toml                        # project config (team name, agent profiles, backend override)
├── agents/
│   └── registry.json                  # [{id, name, role, profile, created_at}]
├── messages/
│   └── <agent-id>/
│       └── inbox/
│           ├── msg-x4f2.json          # one file per message (lock-free sends)
│           └── msg-b1k9.json
├── tasks/
│   ├── tasks.json                     # [{id, title, status, priority, ...}] (flock protected)
│   └── .lock                          # flock target (separate from data)
├── runs/
│   └── runs.json                      # [{id, name, cmd, pane_id, status, ...}]
├── worktrees/
│   └── worktrees.json                 # [{id, branch, path, agent_id, status, ...}]
└── events/
    └── events.jsonl                   # append-only, one JSON object per line
```

**Key choice:** Messages use one-file-per-message (lock-free sends). Tasks use single-file-with-lock (consistent reads + atomic updates). This is the right tradeoff for each resource type.

---

## Data Flow

### A command handler's lifecycle

```
1. cobra parses flags
2. middleware: swarmfs.FindRoot() + config.Load()   ← once, shared
3. call internal package function (e.g. task.Add())
   └── internal function:
       a. swarmfs.WithFileLock(...)
       b. read + unmarshal state
       c. mutate
       d. marshal + swarmfs.AtomicWrite(...)
       e. events.Append(...)             ← automatic, never forgotten
4. output.Print(result, jsonFlag)
```

Callers never touch flock, never touch AtomicWrite, never emit events.

### Cross-subsystem: `swarm status`

```
status.Summary(root, backend) calls:
    agent.List(root)         → agent count
    task.List(root, {})      → task counts by status
    msg.UnreadCount(root, *) → total unread per agent
    pane.List(root, backend) → pane health
```

`status` is intentionally thin — it's an aggregator. Its only job is to call the deep modules and format their outputs side-by-side.

---

## Command Handler Pattern

All command handlers follow this exact pattern. No exceptions.

```go
// cmd/swarm/commands/task.go
var taskAddCmd = &cobra.Command{
    Use:   "add <title>",
    Short: "Add a new task",
    RunE: func(cmd *cobra.Command, args []string) error {
        root, cfg := mustRoot(cmd)   // middleware: FindRoot + Load
        _ = cfg                       // use if needed

        result, err := task.Add(root, args[0], task.AddOpts{
            Priority: mustFlag[task.Priority](cmd, "priority"),
            Tags:     mustFlag[[]string](cmd, "tag"),
        })
        if err != nil {
            output.PrintError(err, jsonFlag(cmd))
            return nil
        }
        return output.Print(result, jsonFlag(cmd))
    },
}
```

The handler is ~15 lines. All logic is in `task.Add()`.

---

## Design Principles Applied

### Deep modules over shallow ones

Each `internal/` package hides its full complexity behind a small, verb-oriented interface. The deepest is `task` — it hides JSON schema, flock(), ETag, event emission, and blocked-status computation from every caller.

### Information hiding

| Knowledge                           | Owned by           | Not leaked to            |
| ----------------------------------- | ------------------ | ------------------------ |
| `tasks.json` JSON schema            | `task/store.go`    | command handlers, status |
| `events.jsonl` format               | `events/events.go` | task, msg, pane, run     |
| Backend selection (5-level cascade) | `pane/detect.go`   | commands, run            |
| Config file format (TOML)           | `config/config.go` | everything else          |
| `.swarm/` path construction         | `swarmfs/root.go`  | everything else          |

### Pull complexity downward

- `task.Claim()` is atomic — callers don't manage locking
- `task.List(f{Ready: true})` runs the 5-condition compound filter — callers don't implement it
- `pane.Spawn()` runs the handshake — callers always get a ready pane
- `events.Append()` constructs the full Event — callers pass 4 strings
- `output.Print()` handles text vs JSON — callers don't call json.Marshal

### Define errors out of existence

- `pane.Close()` on an already-closed pane: returns nil (idempotent)
- `worktree.Clean()` on non-existent worktree: returns nil (idempotent)
- `task.Done()` on already-done task: returns nil (idempotent)
- `swarmfs.FindRoot()` failure produces a clear, actionable error: "no .swarm/ found; run `swarm init`"

### Consistency

Every mutating command:

1. Acquires lock (via `swarmfs.WithFileLock`)
2. Reads + deserializes
3. Validates (returns `SwarmError` with machine-readable code on failure)
4. Mutates
5. Serializes + atomically writes
6. Emits event
7. Returns the mutated object

This is the same sequence regardless of resource type. Developers learning one package immediately understand the others.

---

## Red Flags Watched For

| Risk                                           | Mitigation                                                                     |
| ---------------------------------------------- | ------------------------------------------------------------------------------ |
| `status` command knowing task/msg schemas      | `status` calls `task.List()`, `msg.UnreadCount()` — never reads files directly |
| Command handlers with business logic           | Hard rule: 15-line handlers, all logic in `internal/`                          |
| Config parameter explosion                     | `swarmfs.FindRoot()` returns `Root` — one object, not 4 path params            |
| `pane` package knowing about specific backends | Callers use `pane.Spawn()` only — backend type is an implementation detail     |
| Event emission forgotten in a new mutation     | Events emitted inside store functions, not in command handlers                 |
| `tasks.json` format leaking to `swarm status`  | Enforced by Go package visibility — `task/store.go` types are unexported       |

---

## Open Interface Questions (for next phase)

1. **`swarm worktree` surface**: standalone subgroup or flags on `swarm run`?
   - Lean toward: standalone subgroup. `swarm run --worktree` mixes concerns.

2. **`swarm prompt` vs per-subsystem `swarm task prompt`**:
   - `task.Prompt()` generates task-specific priming from command metadata
   - Top-level `swarm prompt` concatenates all subsystem prompts
   - Both useful; implement per-subsystem first

3. **Completion signal detection** (`<promise>COMPLETE</promise>`):
   - `pane.Subscribe()` output events can be scanned for this pattern
   - Add `pane.WaitForSignal(id, pattern string) error` to the Backend interface?
   - Or handle in `run.Wait()` which already watches pane output?
   - Lean toward: `run.Wait()` handles it — completion signals are a run-level concept, not a pane-level concept

4. **Build order**:
   - `swarm init` + `swarmfs` + `config` first — everything else depends on them
   - `swarm task` second — most self-contained, highest standalone value
   - `swarm msg` third
   - `swarm pane` / `swarm run` fourth (most complex, needs backend work)
   - `swarm worktree` fifth
   - `swarm events tail` last (log already accumulates; reading it is polish)
