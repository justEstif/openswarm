# Ghostty & Kitty Backend Automation Primitives

> **Research date:** 2026-03-30  
> **Researcher:** researcher agent (squad task: ghostty-kitty-backend)  
> **Sources:** Kitty remote-control docs (sw.kovidgoyal.net/kitty/remote-control/), workmux Kitty backend source (`/tmp/workmux-research/src/multiplexer/kitty.rs`), Ghostty docs (ghostty.org/docs), Ghostty GitHub issues  
> **Interface under analysis:** `Spawn(name,cmd,env)→PaneID | Send(id,text) | Capture(id)→string | Subscribe(ctx,id)→<-chan OutputEvent | List()→[]PaneInfo | Close(id) | Wait(id)→exitCode | Name()→string`

---

## Kitty

### 1. Remote Control Model

Kitty exposes a full remote-control protocol via the `kitten @` subcommand (previously `kitty @`). Three transport modes:

| Mode | How | When to use |
|------|-----|-------------|
| **Env-inherited socket** | `kitten @` with no `--to` flag; inherits `KITTY_LISTEN_ON` env var | When code runs _inside_ a kitty window |
| **Explicit Unix socket** | `kitten @ --to unix:/tmp/mykitty <cmd>` | From outside kitty (daemons, external tools) |
| **Stdin/stdout** | `kitten @ --to :pid-N` (pipe to parent kitty process) | Less common, works over SSH |

**Setup requirements:**
```
# kitty.conf
allow_remote_control yes
listen_on unix:/tmp/mykitty    # or tcp:localhost:PORT
```
Or at launch: `kitty -o allow_remote_control=yes --listen-on unix:/tmp/mykitty`

**Key environment variables** set automatically in every kitty window:
- `KITTY_WINDOW_ID` — integer ID of the containing window (pane)
- `KITTY_LISTEN_ON` — socket path to connect to (e.g., `unix:/tmp/mykitty`)

Fine-grained access control is available via `remote_control_password` in `kitty.conf`, allowing per-action permission grants. Authentication can also be done via `KITTY_RC_PASSWORD` env var.

A standalone `kitten` binary (statically compiled) is distributed via kitty releases and works on any UNIX system, so a Go backend can shell out to `kitten @` without requiring a full kitty installation.

---

### 2. Command-by-Command Reference for Our 8 Backend Methods

#### `Spawn(name, cmd, env) → PaneID`
```bash
kitten @ launch \
  --type=window \         # or tab, os-window, overlay
  --title=<name> \
  --env KEY1=VAL1 \
  --env KEY2=VAL2 \
  --cwd /path/to/dir \
  -- cmd arg1 arg2
```
- **Returns**: integer window ID printed to stdout (e.g., `"7\n"`)
- `--type=window` creates a split in the current tab; `--type=tab` creates a new tab
- `--env` can be repeated for multiple env vars; `--env KEY=` sets to empty; `--env KEY` removes it
- `--dont-take-focus` keeps focus on calling window
- The printed ID is a **window** ID (not tab ID)

**Confirmed by workmux** (`kitty.rs:create_window`, `split_pane_internal`):
```rust
let output = self.kitten_cmd()
    .args(&["launch", "--type=tab", "--tab-title", &full_name, "--cwd", ...])
    .run_and_capture_stdout()?;
let window_id = output.trim().to_string();
```

#### `Send(id, text)`
```bash
kitten @ send-text --match id:<id> 'text here'
# For multiline / structured input:
kitten @ send-text --match id:<id> --bracketed-paste 'multiline\ncontent'
# Explicit Enter key:
kitten @ send-text --match id:<id> '\r'
```
- Text follows Python escape rules: `\e`, `\u21fa`, `\r`, `\t`, etc.
- `--match` accepts `id:N`, `title:regex`, `pid:N`, `cmdline:regex`, `env:VAR=VAL`, etc.
- **Note**: "send-text always succeeds, even if no text was sent to any window" (no error on mismatch)
- Workmux sends command text then `\r` separately; uses `--bracketed-paste` for multiline

#### `Capture(id) → string`
```bash
kitten @ get-text --match id:<id>
# With ANSI color codes:
kitten @ get-text --match id:<id> --ansi
# Scrollback + screen:
kitten @ get-text --match id:<id> --extent all
# Only last command output (requires shell integration):
kitten @ get-text --match id:<id> --extent last_cmd_output
```
- `--extent` choices: `screen` (default), `all`, `selection`, `first_cmd_output_on_screen`, `last_cmd_output`, `last_visited_cmd_output`, `last_non_empty_output`
- Last 4 extent options require Kitty shell integration (OSC 133 marks)
- Returns raw text to stdout

**Workmux implementation** (`capture_pane`):
```rust
let output = self.kitten_cmd()
    .args(&["get-text", "--match", &format!("id:{}", pane_id), "--ansi"])
    .run_and_capture_stdout().ok()?;
// Manually slices to last N lines from output
```

#### `Subscribe(ctx, id) → <-chan OutputEvent`
❌ **No native push/subscribe API.** Kitty has no event stream or webhook mechanism.

**Workaround**: Poll `kitten @ get-text` at interval, diff output to detect changes. This is O(screen_size) per tick and has inherent latency. No way to get the PTY master FD externally.

#### `List() → []PaneInfo`
```bash
kitten @ ls
# Optional: filter to specific windows
kitten @ ls --match 'id:7'
# Output format: json (default) or session
kitten @ ls --format json
```
- Returns JSON tree: `[OS_window]` → `[tab]` → `[window]`
- Each OS window has: `id`, `is_focused`, `tabs[]`
- Each tab has: `id`, `title`, `is_active`, `is_focused`, `windows[]`
- Each window has: `id`, `title`, `cwd`, `pid`, `is_focused`, `is_active`, `cmdline`, `env`, `foreground_processes[]`
- `foreground_processes` contains `{pid, cwd, cmdline}` for all processes in the window

**Workmux** parses this into a flattened `FlatPane` struct with `os_window_id`, `tab_id`, `window_id`, etc. It uses the lowest-PID foreground process to avoid capturing transient `kitten` subprocesses.

#### `Close(id)`
```bash
kitten @ close-window --match id:<id>
# To close the entire tab (all windows in tab):
kitten @ close-tab --match id:<window_id>
```
- `--match id:N` resolves **window** IDs (not tab IDs) for both `close-window` and `close-tab`
- `close-tab` takes a window ID and closes the tab that contains that window

#### `Wait(id) → exitCode`
```bash
# Only available at SPAWN time, not for already-running windows:
kitten @ launch --wait-for-child-to-exit --response-timeout 86400 -- cmd
```
- `--wait-for-child-to-exit`: blocks until process exits, prints exit code (integer ≥ 0) or signal name (e.g., `SIGTERM`) to stdout
- `--response-timeout`: max seconds to wait, default 86400 (1 day)
- ❌ **No way to wait on an already-running window**. Once spawned without `--wait-for-child-to-exit`, the only option is to poll `kitten @ ls` and check if the window still exists, or monitor the PID externally via `waitpid()`.

**Workmux** workaround (`wait_until_windows_closed`):
```rust
loop {
    let current_windows = self.get_all_window_names()?;
    if !targets.iter().any(|t| current_windows.contains(t)) { return Ok(()); }
    thread::sleep(Duration::from_millis(500));
}
```

#### `Name() → string`
Returns `"kitty"` — trivial.

---

### 3. Pane ID Model

Kitty uses a **three-level integer hierarchy**:

```
OS Window (id: u64)
  └─ Tab (id: u64)       ← "window" in workmux terminology
       └─ Window (id: u64)  ← "pane" in workmux; the terminal split
```

- All IDs are unique integers assigned monotonically; they do **not** reset on close
- `id:-1` = most recently created window/tab
- `KITTY_WINDOW_ID` env var = the window (pane) ID of the current terminal
- `KITTY_LISTEN_ON` env var = socket path (`unix:/path` or `tcp:host:port`)
- `kitten @ launch` returns the **window ID** (pane ID), not the tab ID
- For `close-tab`, you still pass a **window ID** with `--match id:N`; kitty resolves which tab contains that window

**Important distinction from tmux**: In Kitty, `--match id:N` matches _window_ IDs across all `close-*`, `send-text`, `get-text`, `focus-window` commands. Tab IDs are only used in tab-specific commands via `--match-tab id:N`.

---

### 4. Gaps vs Our Interface

| Method | Support Level | Gap / Notes |
|--------|--------------|-------------|
| `Spawn` | ✅ Native | `kitten @ launch` + stdout window ID |
| `Send` | ✅ Native | `kitten @ send-text --match id:<id>` |
| `Capture` | ✅ Native | `kitten @ get-text --match id:<id>` |
| `Subscribe` | ❌ Not supported | Must poll `get-text`; no push stream |
| `List` | ✅ Native | `kitten @ ls` → JSON with full pane tree |
| `Close` | ✅ Native | `kitten @ close-window --match id:<id>` |
| `Wait` | ⚠️ Workaround | Only at spawn with `--wait-for-child-to-exit`; must poll `ls` for running windows |
| `Name` | ✅ Trivial | Hardcoded `"kitty"` |

**Additional gaps:**
- `allow_remote_control=yes` must be set in kitty.conf **before** launch — cannot be set post-hoc. This is an ops/setup constraint, not a Go API issue.
- Requires kitty to be started with `--listen-on` for external (non-child) process control
- No atomic spawn-and-wait; must combine `launch --wait-for-child-to-exit` or post-launch poll
- `send-text` silently succeeds even if the window doesn't exist (no error propagation)
- `get-text` only captures current screen buffer (default); scrollback needs `--extent all`

---

### 5. Verdict: MVP-ready or Post-MVP?

**Post-MVP — functional for most use cases but has real gaps.**

Kitty is the most scriptable terminal emulator and supports 6 of 8 interface methods natively. However:

1. **Subscribe gap** requires polling — acceptable for a first implementation
2. **Wait gap** requires polling `kitten @ ls` — also acceptable with caveats
3. **Setup friction**: kitty must be launched with `allow_remote_control=yes` and `--listen-on`; this cannot be patched at runtime

**Recommended approach**: Kitty backend is feasible as a secondary backend (after tmux/Zellij). A Go implementation would:
- Shell out to `kitten @` (or the standalone `kitten` binary)
- Use `KITTY_LISTEN_ON` when available (running inside kitty)
- Require `--to <socket>` for external control
- Implement `Subscribe` as a polling goroutine
- Implement `Wait` via `--wait-for-child-to-exit` at spawn time, or poll `ls` for already-running panes

**Workmux coverage** (confirmed): workmux uses `kitten @` for all 6 feasible methods and polls for `Wait`. It does not implement `Subscribe`-style streaming at all.

---

## Ghostty

### 1. Current Scripting/Automation Support

**Honest summary: Ghostty has no programmatic external control API as of March 2026.**

Ghostty does expose a set of ~62 actions (via `ghostty +list-actions`):

```
new_window, new_tab, new_split, goto_split, goto_window,
write_scrollback_file, write_screen_file, write_selection_file,
copy_to_clipboard, paste_from_clipboard, scroll_to_top, scroll_to_bottom,
jump_to_prompt, set_surface_title, set_tab_title, toggle_split_zoom,
resize_split, equalize_splits, reset_window_size, inspector, ...
```

But these actions are **only accessible via keybinds in `ghostty.conf`** — e.g.:
```
keybind = ctrl+f1=new_window
keybind = ctrl+s=write_scrollback_file:open
```

There is **no CLI equivalent of `kitten @`**. You cannot invoke Ghostty actions from an external process.

The `ghostty +new-window` CLI action opens a new OS window, but:
- Returns no window ID
- Creates no programmatic handle
- Has no equivalent for tabs or splits

The `write_screen_file` / `write_scrollback_file` actions write terminal content to a temp file — but are triggered only by the user pressing a keybind, not programmatically.

Shell integration via OSC 133 is supported, enabling prompt tracking and `jump_to_prompt` navigation, but this is terminal-side only.

---

### 2. Any IPC or Remote Control Mechanism

**None currently exists.**

Investigation covered:
- Ghostty config reference: no `listen_on`, no `allow_remote_control`, no socket configuration
- `ghostty --help`: no `--listen-on` or remote control flags
- DBus usage: Ghostty connects to DBus on Linux/Wayland, but **only for app identity** (`class` / `WM_CLASS` / Wayland app ID) and systemd service activation — not for scriptable control
- GitHub issues: Issue [#4625](https://github.com/ghostty-org/ghostty/issues/4625) (open as of research date) requests IPC/scripting; no milestone assigned
- GitHub searches for "scripting OR automation OR remote control": no relevant completed features found, only UI/rendering features

**What does exist** (indirect / workaround territory):
- `ghostty -e <command>` — launches ghostty with a specific command as the initial terminal's shell; does not return a pane ID, does not integrate with a running instance (creates a new process)
- The `write_screen_file:open` action writes screen content to a temp file and opens it — but only via keybind
- Shell integration (OSC 133) is available for tracking command boundaries within a session

---

### 3. Gaps vs Our Interface

| Method | Support | Gap |
|--------|---------|-----|
| `Spawn` | ❌ | No API; `ghostty -e cmd` launches a new process with no returned ID |
| `Send` | ❌ | No mechanism to send text to a specific pane externally |
| `Capture` | ❌ | No mechanism to capture pane output externally |
| `Subscribe` | ❌ | No event stream |
| `List` | ❌ | No mechanism to enumerate panes/windows programmatically |
| `Close` | ❌ | No mechanism to close a specific pane externally |
| `Wait` | ❌ | No mechanism to wait for a pane's child process |
| `Name` | ✅ | Trivial: hardcoded `"ghostty"` |

All 7 functional methods are unsupported. There is no way to build a Ghostty Backend with the current API surface.

---

### 4. Verdict: Post-MVP (Long-term / Aspirational)

**Not feasible for programmatic Backend implementation in 2026.**

Ghostty would need to implement a remote control protocol comparable to Kitty's `kitten @` system. Until that exists (tracked in open issue #4625 with no milestone), a Ghostty Backend cannot be built.

If Ghostty adds an IPC layer in a future version, the Backend interface design is well-suited to accommodate it — the same `Spawn/Send/Capture/List/Close/Wait` pattern would map cleanly.

**Recommended action**: Mark Ghostty as `future` in the Backend registry. Add a note in the design doc that the interface is Ghostty-compatible by design when their IPC ships.

---

## Cross-Backend Comparison Table

> Ratings: ✅ native support | ⚠️ workaround needed | ❌ not supported  
> Note: tmux and Zellij ratings are based on established knowledge of their CLIs; Kitty and Ghostty from this research.

| Method | tmux | Zellij v0.44 | Kitty | Ghostty |
|--------|------|--------------|-------|---------|
| **Spawn** `(name,cmd,env)→PaneID` | ✅ `tmux new-window`/`split-window -P -F "#{pane_id}"` | ✅ `zellij run -n name -- cmd` (tab) / `zellij action new-pane` | ✅ `kitten @ launch` → prints window ID | ❌ No pane API |
| **Send** `(id, text)` | ✅ `tmux send-keys -t %ID text Enter` | ⚠️ `zellij action write-chars "text"` (no stable pane target in CLI) | ✅ `kitten @ send-text --match id:<id>` | ❌ No pane API |
| **Capture** `(id)→string` | ✅ `tmux capture-pane -t %ID -p` (scrollback via `-S`) | ⚠️ `zellij action dump-screen` (active pane only, no scrollback) | ✅ `kitten @ get-text --match id:<id>` (`--extent all` for scrollback) | ❌ No pane API |
| **Subscribe** `(ctx,id)→<-chan` | ⚠️ No push; pipe via PTY master FD or poll `capture-pane` | ❌ No streaming/push mechanism | ❌ No push; must poll `get-text` | ❌ No pane API |
| **List** `()→[]PaneInfo` | ✅ `tmux list-panes -a -F` (rich format strings: id, pid, cmd, title) | ⚠️ `zellij action query-tab-names` (tab names only; no pane IDs in CLI) | ✅ `kitten @ ls` → full JSON tree with IDs, cwd, pid, cmdline | ❌ No pane API |
| **Close** `(id)` | ✅ `tmux kill-pane -t %ID` | ✅ `zellij action close-pane` (active pane only; no target ID) | ✅ `kitten @ close-window --match id:<id>` | ❌ No pane API |
| **Wait** `(id)→exitCode` | ⚠️ Poll `#{pane_dead}` / `#{pane_dead_status}`; no blocking wait-by-ID | ❌ No wait mechanism | ⚠️ `--wait-for-child-to-exit` at spawn only; poll `ls` for running | ❌ No pane API |
| **Name** `()→string` | ✅ `"tmux"` | ✅ `"zellij"` | ✅ `"kitty"` | ✅ `"ghostty"` |

### Summary by Backend

| Backend | MVP-ready? | Native methods | Workaround methods | Unsupported |
|---------|-----------|---------------|-------------------|-------------|
| **tmux** | ✅ Yes | 6/8 | 2/8 (Subscribe, Wait) | 0/8 |
| **Zellij** | ⚠️ Partial | 3/8 | 3/8 | 2/8 (Subscribe, Wait) |
| **Kitty** | ⚠️ Post-MVP | 6/8 | 2/8 (Subscribe, Wait) | 0/8 |
| **Ghostty** | ❌ Not feasible | 1/8 (Name only) | 0/8 | 7/8 |

### Key Design Implications

1. **tmux should be the primary Backend** — highest coverage, most stable CLI, works anywhere
2. **Kitty is viable as secondary Backend** — matches tmux coverage; requires pre-configured `allow_remote_control` + `listen_on`; two methods need polling workarounds
3. **Zellij needs attention** on pane targeting (`Send` to specific pane ID is unreliable in CLI) and `List` richness before it reaches parity
4. **Ghostty is aspirational only** — do not build a Backend until their IPC ships (open issue #4625)
5. **Subscribe** is universally a workaround across all backends — design it as an optional/polling interface from day one

---

## What Workmux Uses from Kitty (verified from source)

File: `/tmp/workmux-research/src/multiplexer/kitty.rs`

| workmux method | Kitty command used |
|---------------|-------------------|
| `create_window` (tab) | `kitten @ launch --type=tab --tab-title <name> --cwd <path> --dont-take-focus` |
| `split_pane` | `kitten @ launch --location vsplit|hsplit --match id:<id> --cwd <path>` |
| `kill_window` (tab) | `kitten @ close-tab --match id:<window_id>` |
| `kill_pane` | `kitten @ close-window --match id:<pane_id>` |
| `capture_pane` | `kitten @ get-text --match id:<id> --ansi` |
| `send_keys` | `kitten @ send-text --match id:<id> <text>` + `kitten @ send-text ... '\r'` |
| `paste_multiline` | `kitten @ send-text --match id:<id> --bracketed-paste <content>` |
| `select_pane` | `kitten @ focus-window --match id:<id>` |
| `select_window` | `kitten @ focus-tab --match id:<window_id>` |
| `set_status` | `kitten @ set-user-vars --match id:<id> workmux_status=<icon>` |
| `list_panes` | `kitten @ ls` → JSON parse into `Vec<FlatPane>` |
| `is_running` | `kitten @ ls` as a check (exit code) |
| `instance_id` | `KITTY_LISTEN_ON` env var |
| `current_pane_id` | `KITTY_WINDOW_ID` env var |

**Notable**: workmux does NOT use `kitten @ launch --wait-for-child-to-exit` — it polls `get_all_window_names()` in a loop for `wait_until_windows_closed`.
