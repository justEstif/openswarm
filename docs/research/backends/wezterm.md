# WezTerm CLI Automation Primitives — Research Report

**Date:** 2026-03-30  
**Researcher:** researcher agent  
**WezTerm version confirmed:** 20240203-110809-5046fc22 (installed locally)  
**Source verified:** Official docs (wezterm.org), workmux Rust source (`/tmp/workmux-research/src/multiplexer/wezterm.rs`), GitHub code search, live `--help` output  

---

## 1. Pane ID Model

### Identifier Format
WezTerm pane IDs are **plain unsigned integers** (u64). There is no sigil prefix (unlike tmux's `%N`).

```
WEZTERM_PANE=3   # integer, set in every child process spawned inside WezTerm
```

### Environment Variables
Two env vars are critical for external automation:

| Variable | Purpose | Example |
|---|---|---|
| `WEZTERM_PANE` | Current pane ID — inherited by all child processes | `3` |
| `WEZTERM_UNIX_SOCKET` | Path to the WezTerm mux server socket — identifies the *instance* | `/tmp/wezterm-mux-...` |

`WEZTERM_PANE` acts as the default `--pane-id` for all `wezterm cli` subcommands. If set, no explicit `--pane-id` flag is needed.

### Socket / Instance Model
WezTerm auto-detects the correct instance to talk to via this priority:
1. `--prefer-mux` flag → use first unix domain from `wezterm.lua`
2. `$WEZTERM_UNIX_SOCKET` → use that socket (auto-inherited by children)
3. Fallback to the running GUI instance

**Implication for Go backend**: If our Go process is a child of WezTerm (spawned inside a WezTerm pane), both env vars are already set and inherited. No socket config needed. If the Go binary is launched externally (e.g., as a daemon), it must pass `WEZTERM_UNIX_SOCKET` explicitly or use `--unix-socket PATH` on the top-level `wezterm` command.

### Hierarchy
```
Window (window_id: u64)
  └── Tab (tab_id: u64)  ← WezTerm's concept of a "window" for our purposes
        └── Pane (pane_id: u64)  ← primary addressing unit
              └── Workspace (string label, e.g., "default")
```

Workmux uses **tab_title** (set via `wezterm cli set-tab-title`) as the stable human-readable window name, since pane_id is the only stable programmatic identifier.

---

## 2. Command-by-Command Reference

### `Name() → string`
```
"wezterm"
```
Static. No CLI call needed.

---

### `Spawn(name, cmd, env) → PaneID`

**Primary command:** `wezterm cli spawn`

```bash
# Basic — spawns default shell, returns pane_id on stdout
wezterm cli spawn

# With cwd
wezterm cli spawn --cwd /path/to/dir

# With command (use -- to separate)
wezterm cli spawn --cwd /path -- bash -l

# In a new window (not a new tab in current window)
wezterm cli spawn --new-window --cwd /path -- bash -l

# In a named workspace
wezterm cli spawn --new-window --workspace myspace --cwd /path -- bash -l
```

**Returns:** pane_id (integer) printed to stdout.

**Full flag synopsis:**
```
wezterm cli spawn [OPTIONS] [PROG]...

Options:
  --pane-id <PANE_ID>       Reference pane for domain/window context (defaults to WEZTERM_PANE)
  --domain-name <NAME>      Multiplexer domain to spawn into
  --window-id <WINDOW_ID>   Spawn tab into specific window (mutually exclusive with --workspace/--new-window)
  --new-window              Spawn into a new window, not a new tab
  --cwd <CWD>               Working directory for spawned program
  --workspace <WORKSPACE>   Workspace name when using --new-window (default: "default")
```

**Gap — env vars:** `wezterm cli spawn` has **NO `--set-environment` flag**. The Lua `SpawnCommand` API supports `set_environment_variables`, but this is not exposed via CLI.

**Workaround for `env` parameter:**
```bash
# Inject env vars by wrapping command in `env`
wezterm cli spawn --cwd /path -- env KEY1=val1 KEY2=val2 /actual/command arg1
# or via sh -c
wezterm cli spawn --cwd /path -- sh -c 'export KEY1=val1 KEY2=val2; exec /actual/command arg1'
```
Workmux uses `sh -c` wrapping for all commands.

**Name assignment:** Set tab title separately after spawn:
```bash
PANE_ID=$(wezterm cli spawn --cwd /path -- cmd)
wezterm cli set-tab-title --pane-id "$PANE_ID" "my-pane-name"
```

---

### `Send(id, text)`

**Command:** `wezterm cli send-text`

```bash
# Send literal text (no enter) — bracketed paste mode
wezterm cli send-text --pane-id 3 "hello world"

# Send raw (bypass bracketed paste)
wezterm cli send-text --pane-id 3 --no-paste "hello world"

# Send a newline/Enter
wezterm cli send-text --pane-id 3 --no-paste $'\r'

# Pipe from stdin
echo "hello" | wezterm cli send-text --pane-id 3
```

**Full flag synopsis:**
```
wezterm cli send-text [OPTIONS] [TEXT]

Arguments:
  [TEXT]   Text to send. If omitted, reads from stdin

Options:
  --pane-id <PANE_ID>   Target pane (defaults to WEZTERM_PANE)
  --no-paste            Send directly, bypassing bracketed paste
```

**Notes:**
- Without `--no-paste`: sends as bracketed paste (apps that respect bracketed paste treat it as a paste event, not keyboard input — correct for multi-line text)
- With `--no-paste`: sends as raw keystrokes — correct for single-line commands, control characters, Enter key
- Workmux sends command text with `--no-paste`, then sends `\r` as a separate call for Enter
- For multi-line code blocks, workmux sends *without* `--no-paste` to use bracketed paste, then follows with `\r`

---

### `Capture(id) → string`

**Command:** `wezterm cli get-text`

```bash
# Capture visible screen (main terminal area, no scrollback)
wezterm cli get-text --pane-id 3

# Capture with ANSI color/style escapes
wezterm cli get-text --pane-id 3 --escapes

# Capture last N lines of scrollback (negative line numbers = scrollback)
# e.g. capture from 200 lines into scrollback to end of screen
wezterm cli get-text --pane-id 3 --start-line -200

# Capture specific range
wezterm cli get-text --pane-id 3 --start-line -100 --end-line -10
```

**Full flag synopsis:**
```
wezterm cli get-text [OPTIONS]

Options:
  --pane-id <PANE_ID>        Target pane (defaults to WEZTERM_PANE)
  --start-line <START_LINE>  Start line (0 = top of screen, negative = into scrollback)
  --end-line <END_LINE>      End line (default = bottom of screen)
  --escapes                  Include ANSI color/style sequences
```

**Notes:**
- Default captures ONLY the main screen area (non-scrollback). Use `--start-line -N` for scrollback.
- Workmux explicitly avoids `--escapes` to get clean text for dashboard display.
- Workmux takes the entire output, then slices to last N lines in application code.
- Since: version 20230320-124340-559cb7b0 (newer than kill-pane and list).

---

### `List() → []PaneInfo`

**Command:** `wezterm cli list`

```bash
# JSON output (machine-readable)
wezterm cli list --format json
```

**JSON schema** (confirmed from workmux source + official docs):
```json
[
  {
    "window_id": 0,
    "tab_id": 0,
    "pane_id": 0,
    "workspace": "default",
    "size": { "rows": 24, "cols": 80 },
    "title": "bash",
    "cwd": "file://hostname/home/user/project",
    "tty_name": "/dev/pts/3",
    "is_active": true,
    "is_zoomed": false,
    "cursor_x": 0,
    "cursor_y": 0,
    "tab_title": "my-custom-tab-title"
  }
]
```

**Field notes:**
- `cwd` format: `"file://hostname/path"` or `"file:///path"` (empty hostname = localhost)
- `tab_title`: reflects user-set title via `set-tab-title` — this is the stable "name" field
- `title`: dynamic terminal title set by running process via escape sequences
- `tty_name`: TTY device path (used by workmux to get PID via `ps -t`)
- Returns **all panes across all workspaces** — no workspace filtering at CLI level

**Table output (default):**
```
WINID TABID PANEID WORKSPACE SIZE  TITLE                         CWD
    0     0      0 default   80x24 bash                          file://host/home/user/
```

---

### `Close(id)`

**Command:** `wezterm cli kill-pane`

```bash
wezterm cli kill-pane --pane-id 3
```

**Full flag synopsis:**
```
wezterm cli kill-pane [OPTIONS]

Options:
  --pane-id <PANE_ID>   Target pane (defaults to WEZTERM_PANE)
```

**Notes:**
- Immediate and without prompting
- If the pane is the only one in its tab, the tab closes too
- Since: version 20230326-111934-3666303c
- Workmux kills all panes in a tab to close a "window", iterating in reverse order

---

### `Wait(id) → exitCode`

**Status: NOT NATIVELY SUPPORTED**

There is no `wezterm cli wait-pane` or blocking mechanism. Exit codes are not accessible via CLI.

**Workaround — polling loop (from workmux pattern):**
```go
for {
    panes := list() // wezterm cli list --format json
    if !paneExists(panes, id) {
        return 0, nil // pane gone — exit code unknown, assume 0 or -1
    }
    time.Sleep(500 * time.Millisecond)
}
```

**Exit code gap:** There is no CLI mechanism to retrieve the exit code of a process that ran inside a pane. Options:
1. Have the spawned process write its exit code to a temp file before the pane closes
2. Use a wrapper script: `cmd; echo $? > /tmp/exit-$PANE_ID`
3. Accept `exitCode = -1` (unknown) as a convention

Workmux does not implement `Wait` per se — it only polls for window closure (not individual pane exit). This is a known limitation of the WezTerm CLI model.

---

### `Subscribe(ctx, id) → <-chan OutputEvent`

**Status: NOT NATIVELY SUPPORTED**

There is no `wezterm cli watch` or streaming output command. The CLI is entirely synchronous/polling.

**Available Lua events** (in-process only, not usable from Go CLI):
- `user-var-changed` — triggered by OSC 1337 escape sequences
- `window-focus-changed`
- `bell`
- Various other UI events

**Workaround — polling goroutine:**
```go
func (b *WeztermBackend) Subscribe(ctx context.Context, id PaneID) <-chan OutputEvent {
    ch := make(chan OutputEvent, 16)
    go func() {
        defer close(ch)
        var prev string
        ticker := time.NewTicker(200 * time.Millisecond)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                current, err := capture(id) // wezterm cli get-text --pane-id N --start-line -200
                if err != nil {
                    continue
                }
                if current != prev {
                    ch <- OutputEvent{PaneID: id, Data: diff(prev, current)}
                    prev = current
                }
            }
        }
    }()
    return ch
}
```

Poll interval recommendation: 100–500ms. Lower intervals increase CPU load; higher intervals increase latency. Workmux uses 500ms for window-close polling.

---

## 3. What Workmux Uses from WezTerm

Source: `/tmp/workmux-research/src/multiplexer/wezterm.rs` (full file, ~960 lines)

### Commands Used

| Workmux operation | WezTerm CLI command |
|---|---|
| `create_window` | `wezterm cli spawn --cwd PATH` then `set-tab-title` |
| `respawn_pane` (tab only) | `wezterm cli spawn --cwd PATH -- sh -c CMD` |
| `split_pane` | `wezterm cli split-pane --pane-id N --cwd PATH [--horizontal\|--top-level] [--percent N] -- sh -c CMD` |
| `kill_pane` | `wezterm cli kill-pane --pane-id N` |
| `kill_window` | iterate panes by tab_title, kill each with `kill-pane` |
| `capture_pane` | `wezterm cli get-text --pane-id N` (slices last N lines in Rust) |
| `send_keys` | `wezterm cli send-text --pane-id N --no-paste TEXT` + separate `\r` call |
| `paste_multiline` | `wezterm cli send-text --pane-id N TEXT` (bracketed paste) + `\r` |
| `select_pane` | `wezterm cli activate-pane --pane-id N` |
| `select_window` | `wezterm cli activate-tab --tab-id N` |
| `list_panes` | `wezterm cli list --format json` |
| `is_running` | `wezterm cli list` (exit code check) |
| `set_tab_title` | `wezterm cli set-tab-title --pane-id N TITLE` |
| `schedule_window_close` | `nohup sh -c 'sleep N; wezterm cli kill-pane --pane-id ...'` |
| `wait_until_windows_closed` | polling loop on `list_panes`, 500ms interval |
| `get_live_pane_info` (PID) | `ps -t TTY -o pid=,stat=` to find foreground process |

### Key Architectural Decisions in Workmux

1. **Tab = "window"**: Workmux maps its `window` concept to WezTerm tabs. Tab identity is maintained via `tab_title` (set with `set-tab-title`), not `tab_id` (which is unstable across restarts).

2. **Pane ID is the primary key**: All operations use `pane_id` (integer). Workmux stores pane_id in its state files.

3. **No session support**: WezTerm workspaces are not equivalent to tmux sessions. Workmux explicitly returns errors for session operations and falls back to window mode.

4. **Instance ID = socket path**: `WEZTERM_UNIX_SOCKET` is used as the instance identifier so all workspaces on one WezTerm server share state — matching tmux's multi-session model.

5. **PID resolution via TTY**: Since `list --format json` doesn't expose PIDs, workmux uses `tty_name` from the list output and runs `ps -t TTY` to find the foreground process PID and command.

6. **Cross-workspace switching via Lua**: When switching to a pane in a different workspace, workmux sends an OSC 1337 `SetUserVar` escape sequence. This requires a matching `user-var-changed` Lua handler in `wezterm.lua`. The CLI alone cannot switch workspaces.

7. **Command wrapping**: All commands are wrapped in `sh -c` to handle quoting correctly.

8. **`--no-paste` for keys, plain for paste**: Single-line commands and control characters use `--no-paste`; multi-line code blocks use bracketed paste (without the flag).

---

## 4. Gaps vs Our Backend Interface

| Method | Support | Notes |
|---|---|---|
| `Name() → string` | ✅ Full | Returns `"wezterm"` |
| `Spawn(name, cmd, env) → PaneID` | ⚠️ Partial | Spawn + set-tab-title ✓; **env vars need `env VAR=val` wrapper** |
| `Send(id, text)` | ✅ Full | `send-text --pane-id N [--no-paste]` |
| `Capture(id) → string` | ✅ Full | `get-text --pane-id N [--start-line -N]` |
| `Subscribe(ctx, id) → <-chan OutputEvent` | ❌ Gap | **No streaming CLI** — requires polling goroutine |
| `List() → []PaneInfo` | ✅ Full | `list --format json` returns rich data |
| `Close(id)` | ✅ Full | `kill-pane --pane-id N` |
| `Wait(id) → exitCode` | ❌ Gap | **No blocking wait, no exit code** — polling until pane disappears, exitCode always -1/unknown |

### Gap Detail: `Spawn` env vars
- **Severity:** Low — workaround is clean (`env K=V cmd` prefix)
- **Pattern:** `wezterm cli spawn -- env KEY1=val1 KEY2=val2 actualcmd args`
- **Alternative:** `sh -c 'export K=V; exec cmd args'`

### Gap Detail: `Subscribe` (no streaming)
- **Severity:** Medium — polling at 200ms is functional but adds ~200ms latency and wastes CPU
- **Pattern:** Polling goroutine calling `get-text --start-line -200` and diffing output
- **Limitation:** Cannot detect scroll position changes that don't alter last-200-lines content
- **Note:** WezTerm Lua events exist but are in-process only; not accessible from Go CLI

### Gap Detail: `Wait` (no exit code)
- **Severity:** Medium — polling is fine but exit code is structurally inaccessible via CLI
- **Pattern:** Poll `list --format json` every 500ms until pane_id disappears
- **Workaround for exit code:** Spawn via wrapper: `sh -c 'cmd; echo $? > /tmp/wezterm-exit-$WEZTERM_PANE; exit 0'`
- **Alternative:** Use `tty_name` + `ps` to detect process termination (workmux approach)

### Missing Commands (not in CLI)
- No `wezterm cli wait-pane` — must poll
- No `wezterm cli get-exit-code` — exit codes lost when pane closes
- No `wezterm cli watch` — no event subscription
- No `wezterm cli spawn --set-environment` — env vars must be in the command

### Lua-Only Capabilities (not exposed via CLI)
- Cross-workspace pane switching (requires escape sequence + Lua handler)
- Event subscriptions (user-var-changed, pane-focus-changed, etc.)
- `SpawnCommand.set_environment_variables` — env support in spawn
- Per-window configuration, color schemes, etc.

---

## 5. Verdict: MVP-Ready or Post-MVP?

### Decision: **MVP-Ready with known workarounds**

**Rationale:**

The core 6 of 8 methods are fully supported by the WezTerm CLI with exact command mappings:
- `Name`, `Spawn` (with env wrapper), `Send`, `Capture`, `List`, `Close` → all implementable in <50 LOC each

The 2 gaps (`Subscribe`, `Wait`) require polling patterns that are already validated by workmux's production use. Neither gap is a blocker:
- `Subscribe` via 200ms polling is adequate for a pane output follower — this is the same approach tmux-based backends use
- `Wait` without exit code is acceptable if we document the `exitCode = -1` convention, or implement the exit-file wrapper

**Prerequisites for MVP:**
1. Must be running *inside* WezTerm (`WEZTERM_PANE` + `WEZTERM_UNIX_SOCKET` inherited) — OR the socket path must be passed explicitly
2. WezTerm version ≥ `20230326-111934-3666303c` for `kill-pane` and `get-text`
3. A unix domain socket configured in `wezterm.lua` for reliable `WEZTERM_UNIX_SOCKET` propagation (optional but recommended for production use)

**Risk factors:**
- `wezterm cli` is marked "experimental mux server" in help text — API surface may change
- `list --format json` doesn't include `tab_title` in official docs JSON example (but workmux uses it in production and it's confirmed in list field reference)
- No exit-code propagation is a real gap for any use case needing process result checking

**Comparison with tmux backend:**
WezTerm is slightly weaker than tmux (no native wait, no exit codes, no env in spawn) but all gaps have clean workarounds. Workmux ships a production WezTerm backend today, confirming MVP viability.

---

## Appendix: Full CLI Subcommand Reference

As of WezTerm 20240203-110809-5046fc22:

```
wezterm cli [OPTIONS] <COMMAND>

Commands:
  list                    list windows, tabs and panes
  list-clients            list clients
  proxy                   start rpc proxy pipe
  tlscreds                obtain tls credentials
  move-pane-to-new-tab    Move a pane into a new tab
  split-pane              split the current pane (returns pane_id)
  spawn                   Spawn a command into a new window or tab (returns pane_id)
  send-text               Send text to a pane (bracketed paste or raw)
  get-text                Retrieves the textual content of a pane to stdout
  activate-pane-direction Activate an adjacent pane (Up/Down/Left/Right/Next/Prev)
  get-pane-direction      Determine the adjacent pane in a direction
  kill-pane               Kill a pane immediately
  activate-pane           Activate (focus) a pane
  adjust-pane-size        Adjust the size of a pane directionally
  activate-tab            Activate a tab
  set-tab-title           Change the title of a tab
  set-window-title        Change the title of a window
  rename-workspace        Rename a workspace
  zoom-pane               Zoom, unzoom, or toggle zoom state

Global Options:
  --no-auto-start         Don't automatically start the server
  --prefer-mux            Prefer connecting to background mux server
  --class <CLASS>         Target a specific GUI instance by window class
```

## Appendix: Sources Checked

- ✅ `/tmp/workmux-research/src/multiplexer/wezterm.rs` — full Rust source (~960 lines)
- ✅ `https://wezterm.org/cli/cli/index.html` — CLI overview and targeting logic
- ✅ `https://wezterm.org/cli/cli/list.html` — list command with JSON schema
- ✅ `https://wezterm.org/cli/cli/spawn.html` — spawn flags and return value
- ✅ `https://wezterm.org/cli/cli/send-text.html` — send-text flags
- ✅ `https://wezterm.org/cli/cli/get-text.html` — get-text flags and scrollback
- ✅ `https://wezterm.org/cli/cli/split-pane.html` — split-pane flags
- ✅ `https://wezterm.org/cli/cli/list-clients.html` — list-clients JSON schema
- ✅ `https://wezterm.org/cli/cli/kill-pane.html` — kill-pane since version
- ✅ `https://wezterm.org/config/lua/window-events/` — event list
- ✅ `https://wezterm.org/config/lua/window-events/user-var-changed.html` — OSC 1337 events
- ✅ `https://wezterm.org/multiplexing.html` — unix domain socket model
- ✅ `wezterm cli --help` (live, v20240203) — confirmed available subcommands
- ✅ `gh search code 'wezterm cli spawn' --language shell --limit 5` — real-world examples
- ✅ `gh repo view wez/wezterm --json description`
- ❌ Did NOT check: WezTerm Lua API for pane:get_lines(), wezterm.mux.* API (not relevant for Go CLI backend)
- ❌ Did NOT check: Windows-specific behavior (unix sockets on WSL2)
