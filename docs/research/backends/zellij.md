# Zellij v0.44.0 CLI Automation Primitives
## Research for the OpenSwarm Pane Backend Interface

**Date:** 2026-03-30  
**Sources:**
- `/tmp/workmux-research/src/multiplexer/zellij.rs` (1231 lines, read in full)
- `/tmp/workmux-research/src/multiplexer/handshake.rs`
- `/tmp/workmux-research/docs/guide/zellij.md`
- `gh release view v0.44.0 --repo zellij-org/zellij` (full body)
- https://zellij.dev/documentation/cli-actions.html
- https://zellij.dev/documentation/zellij-subscribe.html
- https://zellij.dev/documentation/zellij-run-and-edit.html
- https://zellij.dev/documentation/programmatic-control.html
- https://zellij.dev/documentation/controlling-zellij-through-cli.html

---

## 1. Pane ID Model

### Format

Zellij uses **prefixed integer IDs**. Two pane types share an integer namespace:

```
terminal_N   # terminal pane (N is a u32)
plugin_N     # plugin pane (N is a u32)
```

The integer N within each type is stable for the lifetime of the session. Since terminal and plugin panes have separate namespaces, `terminal_1` and `plugin_1` are different panes.

**Environment variables available inside a session:**
```bash
ZELLIJ=0                      # set if inside a Zellij session (value is always "0")
ZELLIJ_SESSION_NAME=my-sess   # current session name
ZELLIJ_PANE_ID=5              # bare integer (NOT the full "terminal_5" form)
```

Note: `ZELLIJ_PANE_ID` returns just the integer. To get the full pane ID string, you must prefix with `terminal_`:
```bash
FULL_PANE_ID="terminal_${ZELLIJ_PANE_ID}"
```

### ID Stability

Pane IDs are **stable integers** — they do not change during the session and do not reuse IDs when panes close. This is confirmed by:
- The `--pane-id` flag accepting bare integers `N` as equivalent to `terminal_N`
- workmux storing `terminal_N` format as a durable key in its state store
- The release notes: "CLI commands that create panes... now return the pane_id for easy storing and manipulation in following commands"

### Tab IDs

Tabs have their own stable integer IDs (`tab_id`), separate from pane IDs. Tabs also have a `position` (which can change when tabs reorder) vs `tab_id` (stable). The `position` is 0-indexed.

### workmux's Approach

workmux represents pane IDs internally as the string `"terminal_N"` and has a `parse_pane_id()` function that strips the `"terminal_"` prefix to extract the u32. The format confirmed in source (line ~67):

```rust
fn parse_pane_id(pane_id: &str) -> Option<u32> {
    pane_id
        .strip_prefix("terminal_")
        .and_then(|s| s.parse().ok())
}
```

---

## 2. Command-by-Command Reference (8 Backend Methods)

### 2.1 `Spawn(name, cmd, env) → PaneID`

**Best approach for a shell pane (interactive, persistent):**
```bash
# Create a new tab (returns tab_id on stdout)
TAB_ID=$(zellij action new-tab --name "my-pane" --cwd /path/to/cwd)

# Then query list-panes to find the terminal pane in that tab
PANE_ID=$(zellij action list-panes --json | jq -r ".[] | select(.tab_id == $TAB_ID and .is_plugin == false) | \"terminal_\" + (.id | tostring)")
```

**Best approach for a command pane (runs a specific process):**
```bash
# new-pane returns the pane ID directly on stdout
PANE_ID=$(zellij action new-pane --name "my-worker" --cwd /path/to/cwd -- /path/to/cmd arg1 arg2)
# or using zellij run:
PANE_ID=$(zellij run --name "my-worker" --cwd /path/to/cwd -- /path/to/cmd arg1 arg2)
```

**Key flags for `new-pane`:**
```
-n, --name <NAME>       name the pane
    --cwd <CWD>         working directory
-c, --close-on-exit    close immediately when command exits
-d, --direction <DIR>  right|down (for splitting)
-f, --floating         open as floating pane
```

**Returns:** Pane ID string like `terminal_5` on stdout.

**Environment variables:** Zellij does NOT support passing per-pane env vars via the CLI. Workaround: prepend `env VAR=value` to the command, or set env in the shell wrapper launched in the pane.

**Verdict for Backend.Spawn:** ✅ Clean mapping — `new-pane` returns pane ID directly. Tab-per-pane model (used by workmux) requires an extra `list-panes` call to find the pane ID after `new-tab`.

---

### 2.2 `Send(id, text)`

**Send a line of text (with newline):**
```bash
# Send the text
zellij action write-chars --pane-id terminal_5 "cargo build --release"
# Send Enter (ASCII 13)
zellij action write --pane-id terminal_5 13
```

**Send named keys:**
```bash
zellij action send-keys --pane-id terminal_5 "Enter"
zellij action send-keys --pane-id terminal_5 "Ctrl c"
zellij action send-keys --pane-id terminal_5 "Alt b" "Enter"
```

**Combined approach (workmux):**
```bash
# workmux uses write-chars + write "13" (two separate subprocess calls)
zellij action write-chars --pane-id "terminal_N" "text"
zellij action write --pane-id "terminal_N" "13"
```

**`--pane-id` targeting:** `--pane-id` accepts `terminal_N`, `plugin_N`, or bare `N` (= `terminal_N`). This works for **any pane regardless of which tab is currently focused** — this is a critical v0.44.0 feature (workmux docs note it landed via PR #4691, shipped in 0.44.0).

**Verdict for Backend.Send:** ✅ Clean mapping — pane-targeted write works without requiring the target tab to be active.

---

### 2.3 `Capture(id) → string`

**Dump viewport only:**
```bash
zellij action dump-screen --pane-id terminal_5
# Prints to stdout (no --path needed)
```

**Dump viewport + full scrollback:**
```bash
zellij action dump-screen --pane-id terminal_5 --full
```

**Dump with ANSI codes preserved:**
```bash
zellij action dump-screen --pane-id terminal_5 --full --ansi
```

**Dump to file:**
```bash
zellij action dump-screen --pane-id terminal_5 --path /tmp/capture.txt
```

**Scrollback support:** Yes, via `--full` flag. `--scrollback N` is available via `zellij subscribe`, not `dump-screen`.

**⚠️ Important workmux discrepancy:** workmux's `capture_pane()` implementation does NOT use `--pane-id`. Its code and comment say:

```rust
// Zellij limitation: dump-screen always captures the focused pane,
// not the pane specified by pane_id.
```

This was true BEFORE v0.44.0. The v0.44.0 docs confirm `--pane-id` is now supported for `dump-screen`. workmux's implementation is outdated. The correct Go implementation should use:

```bash
zellij action dump-screen --pane-id terminal_N --full
```

**Verdict for Backend.Capture:** ✅ Clean mapping — `dump-screen --pane-id` works in v0.44.0. workmux has a bug here (uses active-pane-only capture).

---

### 2.4 `Subscribe(ctx, id) → <-chan OutputEvent`

**Stream pane viewport in real time:**
```bash
zellij subscribe --pane-id terminal_5 --format json
```

**Stream with scrollback on initial delivery:**
```bash
zellij subscribe --pane-id terminal_5 --format json --scrollback 100
```

**Subscribe to multiple panes:**
```bash
zellij subscribe --pane-id terminal_5 --pane-id terminal_6 --format json
```

**Subscribe to a pane in a different session:**
```bash
zellij --session other-session subscribe --pane-id terminal_5 --format json
```

**Output format** (NDJSON, one JSON object per line):

```json
{"event":"pane_update","pane_id":"terminal_1","viewport":["line1","line2"],"scrollback":null,"is_initial":true}
{"event":"pane_update","pane_id":"terminal_1","viewport":["line1","line2 updated"],"scrollback":null,"is_initial":false}
{"event":"pane_closed","pane_id":"terminal_1"}
```

**Behavior:**
- Initial delivery: full viewport + scrollback (if requested) with `is_initial: true`
- Subsequent deliveries: only when viewport content changes
- Client exits automatically when all subscribed panes close or session ends

**Go implementation pattern:**
```go
cmd := exec.CommandContext(ctx, "zellij", "subscribe", "--pane-id", id, "--format", "json")
scanner := bufio.NewScanner(cmd.Stdout)
for scanner.Scan() {
    // parse NDJSON line -> OutputEvent
}
```

**Verdict for Backend.Subscribe:** ✅ Excellent fit — `zellij subscribe` is purpose-built for this use case. NDJSON format is easy to parse. Context cancellation kills the subprocess cleanly.

---

### 2.5 `List() → []PaneInfo`

**List all panes (full JSON):**
```bash
zellij action list-panes --json
```

**With tab and command info (workmux flags):**
```bash
zellij action list-panes --json --tab --command
```

**List all tabs:**
```bash
zellij action list-tabs --json
```

**Verdict for Backend.List:** ✅ Clean mapping — `list-panes --json` returns all needed data.

---

### 2.6 `Close(id)`

**Close a specific pane:**
```bash
zellij action close-pane --pane-id terminal_5
```

**Close a specific tab by its stable ID:**
```bash
zellij action close-tab-by-id 3
# or equivalently:
zellij action close-tab --tab-id 3
```

**⚠️ Note:** `close-pane --pane-id` is the correct primitive for closing a single pane. In workmux's window model (one tab per "window"), it uses `close-tab-by-id` to close the whole tab. The appropriate Backend.Close() implementation depends on whether panes map 1:1 to tabs or many-panes-per-tab.

**Verdict for Backend.Close:** ✅ Clean mapping — `close-pane --pane-id` works directly.

---

### 2.7 `Wait(id) → exitCode`

**Block until command exits (any status):**
```bash
zellij action new-pane --block-until-exit -- cargo test
```

**Block until exit with success (status 0):**
```bash
zellij action new-pane --block-until-exit-success -- cargo test
```

**Block until exit with failure (non-zero status):**
```bash
zellij action new-pane --block-until-exit-failure -- cargo test
```

**Block until pane is closed (by user or command):**
```bash
zellij action new-pane --blocking -- cargo test
# also: zellij run --blocking -- cargo test
```

**⚠️ Exit code retrieval:** The blocking flags do NOT return the exit code on stdout or as the subprocess's exit code. To get the exit code, poll `list-panes --json` and read the `exit_status` field:

```bash
zellij action list-panes --json | jq '.[] | select(.id == 5) | .exit_status'
# returns: 0, 1, null (null = still running)
```

**Available blocking variants:**
| Flag | Behavior |
|------|----------|
| `--blocking` | Block until pane closed (by user or command exit) |
| `--block-until-exit` | Block until command exits (any status) |
| `--block-until-exit-success` | Block until exit status 0 |
| `--block-until-exit-failure` | Block until exit status != 0 |

These flags are available on both `zellij run` and `zellij action new-pane`.

**Go implementation pattern for Wait:**
```go
// Option A: Use --block-until-exit at spawn time (must be set upfront)
cmd := exec.Command("zellij", "action", "new-pane", "--block-until-exit", "--", ...)
cmd.Wait()  // blocks until exit
// Then poll list-panes for exit_status

// Option B: Subscribe and watch for pane_closed event
// After pane_closed, query list-panes for exit_status
```

**Verdict for Backend.Wait:** ⚠️ Partial gap — blocking works but **exit code is not returned directly**. Must poll `list-panes --json` for `exit_status` after the blocking call returns. This is a two-step process. The blocking flag must be set at spawn time, not after-the-fact.

---

### 2.8 `Name() → string`

```go
func (b *ZellijBackend) Name() string {
    return "zellij"
}
```

**Verdict:** ✅ Trivial.

---

## 3. What workmux Actually Uses — Every Zellij Command

Extracted from `/tmp/workmux-research/src/multiplexer/zellij.rs` with exact flags:

### Query Commands
```bash
# Check if zellij is running (used as health check)
zellij action dump-screen /dev/null

# List all panes with tab + command metadata
zellij action list-panes --json --tab --command

# List all tabs
zellij action list-tabs --json

# Get focused tab info (text format, not JSON)
zellij action current-tab-info
# Parses output: "name: <tab_name>\nid: <id>\nposition: <pos>\n..."
```

### Tab Management
```bash
# Create new tab — returns tab_id (u32) on stdout
zellij action new-tab --name "<full_name>" --cwd "<cwd>"

# Switch to tab by stable ID (preferred)
zellij action go-to-tab-by-id <tab_id>

# Switch to tab by name (fallback)
zellij action go-to-tab-name "<full_name>"

# Close tab by stable ID (preferred, from PR #4695)
zellij action close-tab-by-id <tab_id>

# Close focused tab (fallback)
zellij action close-tab
```

### Pane Navigation (used in select_pane — workaround for missing focus-pane-by-id)
```bash
# Navigate to previous pane in tab
zellij action focus-previous-pane

# Navigate to next pane in tab
zellij action focus-next-pane
```

### Pane Creation
```bash
# Split pane — returns pane ID (e.g., "terminal_5") on stdout
zellij action new-pane --direction right  --cwd <cwd>           # horizontal split
zellij action new-pane --direction down   --cwd <cwd>           # vertical split
zellij action new-pane --direction <dir>  --cwd <cwd> -- sh -c '<script>'  # with command
```

### Pane Close
```bash
# Kill pane — workmux closes the entire TAB containing the pane, not just the pane
zellij action close-tab-by-id <tab_id>
```

### Text I/O (all use --pane-id for targeting)
```bash
# Send text characters to specific pane
zellij action write-chars --pane-id <pane_id> "<text>"

# Send raw bytes to specific pane (used for special keys)
zellij action write --pane-id <pane_id> 13    # Enter
zellij action write --pane-id <pane_id> 27    # Escape
zellij action write --pane-id <pane_id> 9     # Tab

# Clear pane buffer (with --pane-id, falls back without if error)
zellij action clear --pane-id <pane_id>
zellij action clear   # fallback for older versions
```

### Screen Capture (⚠️ BUG — does NOT use --pane-id)
```bash
# workmux captures the FOCUSED pane only (ignores pane_id argument)
zellij action dump-screen <temp_file_path>
# Note: v0.44.0 supports --pane-id here but workmux does not use it
```

### Shell Commands (generated strings, not directly invoked)
```bash
# Used in shell_select_window_cmd()
zellij action go-to-tab-by-id <tab_id> >/dev/null 2>&1

# Used in shell_kill_window_cmd()
zellij action close-tab-by-id <tab_id> >/dev/null 2>&1

# Used in schedule_window_close() (spawned in background)
sleep <delay_secs> && zellij action close-tab-by-id <tab_id>
sleep <delay_secs> && zellij action go-to-tab-name '<name>' && zellij action close-tab
```

### Session Management
```bash
# Background shell script execution
nohup sh -c '<script>' >/dev/null 2>&1 &
```

---

## 4. JSON Schemas

### 4.1 `list-panes --json` Schema

Command: `zellij action list-panes --json`  
Returns: JSON array of pane objects.

```json
[
  {
    "id": 1,                          // u32: numeric pane ID (prefix with "terminal_" for full ID)
    "is_plugin": false,               // bool: true if this is a plugin pane
    "is_focused": true,               // bool: is this the focused pane in its tab
    "is_fullscreen": false,           // bool
    "is_floating": false,             // bool
    "is_suppressed": false,           // bool
    "title": "/bin/bash",             // string: pane title bar text
    "exited": false,                  // bool: true if command has exited
    "exit_status": null,              // int|null: exit code (null if still running)
    "is_held": false,                 // bool: true if held (waiting for user Enter)
    "pane_x": 0,                      // int: pane left edge (column)
    "pane_content_x": 1,              // int: content area left (after border)
    "pane_y": 1,                      // int: pane top edge (row)
    "pane_content_y": 2,              // int: content area top (after border)
    "pane_rows": 24,                  // int: total rows including border
    "pane_content_rows": 22,          // int: usable rows
    "pane_columns": 80,               // int: total columns
    "pane_content_columns": 78,       // int: usable columns
    "cursor_coordinates_in_pane": [0, 5], // [col, row] | null
    "terminal_command": null,         // string|null: original command from pane config
    "plugin_url": null,               // string|null: plugin URL (plugin panes only)
    "is_selectable": true,            // bool: can receive keyboard input
    "tab_id": 0,                      // int: stable tab ID (available by default in v0.44.0)
    "tab_position": 0,                // int: tab's current position in bar
    "tab_name": "Tab #1",             // string: tab display name
    "pane_command": "bash",           // string|null: current running command (available with --command)
    "pane_cwd": "/home/user/project"  // string|null: process working directory (available with --command)
  }
]
```

**Notes:**
- `tab_id`, `tab_position`, `tab_name` are included by default in v0.44.0 (confirmed by workmux deserializing them without needing --tab flag — though workmux passes `--tab` for safety)
- `pane_command` and `pane_cwd` require the `--command` flag
- Plugin panes have `is_plugin: true`, `plugin_url` set, `pane_command: null`
- workmux's `PaneInfo` struct matches this schema exactly

### 4.2 `list-tabs --json` Schema

Command: `zellij action list-tabs --json`  
Returns: JSON array of tab objects.

```json
[
  {
    "position": 0,                          // int: current position in tab bar (can change)
    "name": "Tab #1",                       // string: tab display name
    "active": true,                         // bool: is this the active/focused tab
    "panes_to_hide": 0,                     // int
    "is_fullscreen_active": false,          // bool
    "is_sync_panes_active": false,          // bool
    "are_floating_panes_visible": false,    // bool
    "other_focused_clients": [],            // array: other client connections focused here
    "active_swap_layout_name": "default",   // string|null: current swap layout
    "is_swap_layout_dirty": false,          // bool
    "viewport_rows": 24,                    // int: usable rows in tab
    "viewport_columns": 80,                 // int: usable columns in tab
    "display_area_rows": 26,                // int: total display area rows
    "display_area_columns": 80,             // int: total display area columns
    "selectable_tiled_panes_count": 2,      // int
    "selectable_floating_panes_count": 0,   // int
    "tab_id": 0,                            // int: STABLE tab ID (does not change on reorder)
    "has_bell_notification": false,         // bool
    "is_flashing_bell": false               // bool
  }
]
```

**Key:** `tab_id` is stable; `position` changes when tabs reorder. Always use `tab_id` for addressing.

### 4.3 `current-tab-info` Text Format

Command: `zellij action current-tab-info`  
Returns: Plain text (default) or JSON with `--json`.

**Text format:**
```
name: Tab #1
id: 0
position: 0
```

**JSON format** (`--json`): Same schema as a single element from `list-tabs --json`.

### 4.4 `subscribe --format json` NDJSON Schema

Each line is one of:

**Pane update event:**
```json
{
  "event": "pane_update",
  "pane_id": "terminal_1",
  "viewport": ["line1", "line2", "..."],   // array of strings (rendered lines)
  "scrollback": null,                       // array|null (only on initial if --scrollback used)
  "is_initial": true                        // bool: true on first delivery
}
```

**Pane closed event:**
```json
{
  "event": "pane_closed",
  "pane_id": "terminal_1"
}
```

---

## 5. Known Gaps and Workarounds

### Gap 1: No Direct `focus-pane-by-id` Action

**Problem:** There is no `zellij action focus-pane-by-id N` command. To move keyboard focus to a specific pane, you must navigate via `focus-next-pane` / `focus-previous-pane`.

**workmux workaround:** Counts steps from current focused pane to target pane in the pane list, then sends N `focus-next-pane` or `focus-previous-pane` commands. Fragile if pane order changes between queries.

**Backend implication:** `select_pane()` in our Backend would need this same workaround. Not required for `Send()`, `Capture()`, or `Subscribe()` since all support `--pane-id` in v0.44.0.

### Gap 2: `Wait(id)` Does Not Return Exit Code

**Problem:** The blocking flags (`--block-until-exit`, `--blocking`) do not emit the exit code on stdout or as the subprocess's own exit code.

**Workaround:** After the blocking call returns (or after subscribing and receiving `pane_closed`):
```bash
zellij action list-panes --json | jq '.[] | select(.id == N) | .exit_status'
```

**Backend implication:** `Wait(id)` must be implemented as:
1. Either pass `--block-until-exit` at spawn time and then poll `list-panes` for the exit code
2. Or subscribe to the pane, wait for `pane_closed` event, then query `list-panes` (but the pane may already be gone — check `exited: true` before close)

**Recommendation:** At `Spawn()` time, if the caller will need to `Wait()`, use `new-pane --close-on-exit` to auto-close the pane when done. Then the `subscribe` `pane_closed` event signals completion. Store the exit status before the pane closes using `list-panes`.

### Gap 3: No Per-Pane Environment Variable Injection

**Problem:** `new-pane` and `zellij run` have no `--env VAR=val` flag.

**Workarounds:**
1. Prepend env vars to the command: `new-pane -- env MY_VAR=foo cargo build`
2. Write a shell wrapper script to a temp file, set the env there, exec the real command
3. Use `write-chars` to send `export VAR=val` to a shell pane before sending the actual command

### Gap 4: `dump-screen` Bug in workmux

**Problem:** workmux's `capture_pane()` does NOT use `--pane-id`. It always dumps the focused pane, creating a recursive capture loop when the workmux dashboard itself is focused. The code comment says this is a "Zellij limitation" but this was fixed in v0.44.0.

**Correct command for our Backend:**
```bash
zellij action dump-screen --pane-id terminal_N --full
```

### Gap 5: `new-tab` Returns Tab ID, Not Pane ID

**Problem:** If using the tab-per-"window" model (like workmux), `new-tab` returns a tab ID but you need a pane ID to target the initial shell pane.

**Workaround (used by workmux):**
```rust
let tab_id: u32 = /* from new-tab stdout */;
let panes = list_panes();
let pane = panes.iter().find(|p| !p.is_plugin && p.tab_id == Some(tab_id));
// pane.id gives the numeric pane ID -> format as "terminal_{pane.id}"
```

This requires one extra `list-panes` call per spawn. Race condition risk is low since `new-tab` synchronously returns after creating the tab.

### Gap 6: No Tab Insertion Ordering

**Problem:** New tabs always append at the end. No `--after-tab` or insertion position control.

**Backend implication:** For our `Spawn()`, document that panes appear at the end of the tab bar. Not a functional blocker.

### Gap 7: Pane Splits Always 50/50

**Problem:** `new-pane --direction right` always creates equal-size splits. No `--percentage` or exact size control via CLI.

**Backend implication:** Acceptable for most use cases. Document the limitation.

### Gap 8: Session Mode Not Supported

**Problem:** workmux's "session mode" (isolating panes per `--session` prefix in tmux) has no equivalent in Zellij. Zellij has sessions but they're whole terminal sessions, not sub-groupings of panes.

**Backend implication:** Only window/tab mode is supported. Session isolation must be achieved by naming conventions on tabs.

### Gap 9: No PID Exposure

**Problem:** `list-panes --json` does not include the PID of the foreground process.

**Backend implication:** Cannot implement process-level operations (kill by PID, resource monitoring). This is consistent with workmux's `LivePaneInfo.pid = None` for Zellij.

---

## 6. Verdict: How Cleanly Does Zellij v0.44.0 Map to the Backend Interface?

### Summary Table

| Backend Method | CLI Primitive | Quality | Notes |
|----------------|---------------|---------|-------|
| `Spawn(name, cmd, env)` | `new-pane --name --cwd` | ✅ Clean | No native env injection; workaround exists |
| `Send(id, text)` | `write-chars --pane-id` + `write --pane-id 13` | ✅ Clean | Pane-targeted without focus requirement |
| `Capture(id)` | `dump-screen --pane-id --full` | ✅ Clean | v0.44.0 added `--pane-id` support |
| `Subscribe(ctx, id)` | `zellij subscribe --pane-id --format json` | ✅ Excellent | Purpose-built, NDJSON, clean exit |
| `List()` | `list-panes --json` | ✅ Clean | Rich JSON schema, covers all needed fields |
| `Close(id)` | `close-pane --pane-id` | ✅ Clean | Direct pane targeting |
| `Wait(id)` | `--block-until-exit` + poll `list-panes` | ⚠️ Gap | Exit code not returned directly; two-step |
| `Name()` | hardcoded `"zellij"` | ✅ Trivial | |

### Overall Assessment: **Good fit with one notable gap**

**Zellij v0.44.0 is substantially more capable than v0.43.x** for programmatic control. The critical features that land in v0.44.0 for our Backend:

1. **`dump-screen --pane-id`** — enables Capture() without focus dependency
2. **`write-chars --pane-id`** and **`write --pane-id`** — enables Send() without focus dependency  
3. **`close-pane --pane-id`** — enables Close() without focus dependency
4. **`zellij subscribe --format json`** — enables Subscribe() as a clean streaming primitive
5. **`new-pane` returns pane ID** — enables Spawn() to capture the pane ID

The only meaningful gap is **`Wait(id) → exitCode`**: exit codes are not directly returned by the blocking primitives. The exit code is available in `list-panes --json` as `exit_status` (int|null), but requires an extra query call. A practical implementation:

```go
func (b *ZellijBackend) Wait(ctx context.Context, id PaneID) (int, error) {
    // Subscribe to the pane, wait for pane_closed event
    // Then immediately query list-panes before the pane is garbage collected
    // Return exit_status field
}
```

Note: There is a race between `pane_closed` event delivery and `list-panes` still having the pane. Using `--close-on-exit` at spawn time means the pane closes immediately on exit — capturing the exit status requires the subscribe stream, which includes `exited` state in `pane_update` events. Alternative: use `--block-until-exit` flag at spawn time and **not** use `--close-on-exit`, so the pane lingers after exit and `list-panes` can still read its `exit_status`.

### Recommended Spawn Strategy for Backends Needing Wait()

```bash
# At Spawn time (no --close-on-exit):
PANE_ID=$(zellij action new-pane --name "my-task" -- cargo test)

# At Wait time: block until pane exits, then read exit code
# The pane stays visible (showing exit status) until explicitly closed
zellij action list-panes --json | jq --argjson id "${PANE_ID#terminal_}" \
  '.[] | select(.id == $id) | .exit_status'
# Then close:
zellij action close-pane --pane-id "$PANE_ID"
```

### Tab vs Pane Model Decision

workmux uses **one tab per "window"** (which maps to one agent pane per tab). For our Backend:

- **Tab-per-pane model** (workmux approach): Provides tab naming, visual isolation, easy tab-close to kill the pane. Overhead: extra `list-panes` call per `Spawn()`.
- **Single-tab multi-pane model**: Simpler pane management, but no named tabs per pane. Requires pane-splitting for layout.

**Recommendation:** Use the tab-per-pane model for agent panes to leverage tab naming (visible in the Zellij tab bar). Use multi-pane-per-tab for auxiliary split panes within a single agent's workspace.

### Handshake: Why UnixPipeHandshake?

Zellij uses `UnixPipeHandshake` (the same as WezTerm) because:

1. **No `tmux wait-for` equivalent** — Zellij has no built-in synchronization primitive for "wait until a process in a pane is ready"
2. **Shell startup is asynchronous** — After `new-tab` returns, the shell inside may not have started yet. Sending commands immediately can race.

**Protocol:**
1. Create a named FIFO at `/tmp/workmux_pipe_{pid}_{nanos}` with `0o600` permissions
2. Set the pane's initial command to: `echo ready > /path/to/fifo; exec '<shell>' -l`
3. Wait (with `poll()`) for data to appear on the FIFO (50ms poll interval, 5s timeout)
4. Once `ready` is received, the shell is guaranteed to be running and accepting input

This is the correct approach for Zellij. Our Backend implementation should do the same.

---

## Appendix: Version Context

| Feature | Version |
|---------|---------|
| `--pane-id` targeting for `write-chars`, `write`, `send-keys`, `close-pane`, `clear` | v0.44.0 (PR #4691) |
| `dump-screen --pane-id` | v0.44.0 |
| `close-tab-by-id`, `go-to-tab-by-id`, stable tab IDs | v0.44.0 (PR #4695) |
| `new-pane` / `new-tab` return created ID on stdout | v0.44.0 |
| `zellij subscribe` with `--format json` and `--pane-id` | v0.44.0 |
| `--block-until-exit` / `--block-until-exit-success` / `--block-until-exit-failure` | v0.44.0 |
| `list-panes --json` with `exit_status` field | v0.44.0 |
| Native Windows support | v0.44.0 |

**Pre-v0.44.0 workmux compatibility note:** workmux's docs say "requires Zellij built from source (uses unreleased features)". All of those features are now in the official v0.44.0 release. A Backend targeting v0.44.0+ can use all these primitives without a custom Zellij build.
