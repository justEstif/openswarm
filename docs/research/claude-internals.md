# Claude Code Agent Teams — Internal Architecture Research Report

**Date:** 2026-03-29  
**Scope:** Claude Code v2.1.32–v2.1.83 (experimental Agent Teams feature)  
**Sources:** Official documentation, GitHub issue analysis, published reverse-engineering studies, binary string analysis, on-disk artifact inspection  
**Confidence notation:** Findings marked `[CONFIRMED]` = multiple independent sources agree. `[SPECULATIVE]` = inferred, single source, or extrapolated.

---

## Table of Contents

1. [CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 Flag](#1-the-flag)
2. [Internal File Structure and Schemas](#2-file-structure)
3. [TeammateTool — Spawning, IPC, and Peer Messaging](#3-teammatetool)
4. [Ctrl+T Task List Overlay](#4-ctrlt-task-list)
5. [Dependency Auto-Unblock](#5-dependency-auto-unblock)
6. [File Locking](#6-file-locking)
7. [Agent Identity and Registration](#7-agent-identity)
8. [Known Bugs and Architectural Limitations](#8-known-bugs)
9. [CustomPaneBackend Protocol Proposal (Issue #26572)](#9-custompanebackend-proposal)
10. [Key Findings Summary](#10-key-findings)
11. [References](#11-references)

---

## 1. The Flag

### What It Enables

`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` unlocks a set of coordination tools that are hidden from the model by default. Once set, the following tools become available in the system prompt:

| Tool | Role |
|------|------|
| `TeamCreate` | Creates team config directory and task directory |
| `TeamDelete` | Removes team files (requires all teammates shut down first) |
| `TaskCreate` | Creates a task JSON file in the task directory |
| `TaskList` | Returns all task files with current status |
| `TaskUpdate` | Modifies a task (status, owner, fields) |
| `SendMessage` | Writes a message to a recipient's inbox file |

The `Task` tool (the subagent spawner) gains two additional parameters: `team_name` and `name`, which enroll a spawned agent as a named team member.

**Minimum version:** Claude Code v2.1.32. Check with `claude --version`.

### How to Activate

**Method 1 — Settings file (permanent, project-scoped):**
```json
// .claude/settings.json  (project) or ~/.claude/settings.json (global)
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

**Method 2 — Shell environment (session-scoped):**
```bash
export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1
claude
```

**Method 3 — Inline for one run:**
```bash
CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 claude
```

### Known Flag Behavior Note [CONFIRMED]

Issue #23816 (Claude Code v2.1.32–2.1.34) reported that after enabling the flag, `TeamCreate` succeeded but `TaskCreate`, `TaskList`, and `TaskUpdate` were not available at runtime — not discoverable via `ToolSearch` and not listed in the agent's tool list. The `TeamCreate` system prompt explicitly referenced these tools, but they were gated behind a separate rollout. This was resolved in v2.1.47+, where all task management tools appear as expected.

**Tool availability by context [CONFIRMED from issue #32723]:**
- `TeamCreate` and `TeamDelete` are available to standalone subagents outside a team, but NOT to agents running inside a team (teammates). This prevents nested team creation.
- Teammates have: `TaskCreate`, `TaskList`, `TaskUpdate`, `SendMessage`.
- The lead has all tools.

---

## 2. File Structure

### Directory Layout [CONFIRMED]

All team state is stored under `~/.claude/`:

```
~/.claude/
├── teams/
│   └── {team-name}/
│       ├── config.json                  # team membership registry
│       └── inboxes/
│           ├── team-lead.json           # lead's mailbox (JSON array)
│           ├── researcher.json          # teammate mailbox
│           └── implementer.json         # teammate mailbox (created on demand)
└── tasks/
    └── {team-name}/
        ├── .lock                        # 0-byte file; flock() mutex
        ├── .highwatermark               # integer string: next task ID
        ├── 1.json                       # task file
        ├── 2.json
        └── ...
```

**Observation from runtime monitoring [CONFIRMED, dev.to study]:**  
Running `watch -n 0.5 'tree ~/.claude/teams/ ~/.claude/tasks/'` while a team is active shows files being created/updated in real time. Of 42 task directories observed in one study, only 5 contained task JSON files — the rest had only `.lock` and `.highwatermark`, consistent with cleanup-after-completion or sessions that used the internal task list without creating subtask files.

---

### 2.1 Team Config Schema

**File:** `~/.claude/teams/{team-name}/config.json`

```json
{
  "members": [
    {
      "name": "team-lead",
      "agentId": "abc-123",
      "agentType": "leader"
    },
    {
      "name": "researcher",
      "agentId": "def-456",
      "agentType": "general-purpose"
    },
    {
      "name": "implementer",
      "agentId": "ghi-789",
      "agentType": "general-purpose"
    }
  ]
}
```

**Field semantics [CONFIRMED]:**
- `name`: human-readable label; **this is the primary addressing key** for messaging and task assignment. All `SendMessage` `to:` fields and `TaskUpdate` `owner:` fields use the name, not the ID.
- `agentId`: UUID generated at spawn time. Exists for reference only; not used for routing.
- `agentType`: role descriptor. Known values: `"leader"`, `"general-purpose"`. Custom subagent types inherit their `.claude/agents/` definition name.

---

### 2.2 Task File Schema

**File:** `~/.claude/tasks/{team-name}/{id}.json`

Real example from a session (dev.to study, Claude Code v2.1.47):

```json
{
  "id": "1",
  "subject": "Hunt for bugs across the codebase",
  "description": "Examine all source files in src/ for potential bugs, race conditions, null pointer dereferences, and incorrect error handling. Produce a prioritized list.",
  "activeForm": "Hunting for bugs",
  "owner": "bug-hunter",
  "status": "completed",
  "blocks": [],
  "blockedBy": []
}
```

**Full schema [CONFIRMED]:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Numeric string. Auto-incremented via `.highwatermark`. |
| `subject` | `string` | Imperative-form title (e.g., "Run tests"). |
| `description` | `string` | Full task requirements and acceptance criteria. This is the primary context a teammate reads to understand the task. |
| `activeForm` | `string` | Present-continuous form for TUI spinner display ("Running tests"). |
| `status` | `string` | State machine: `"pending"` → `"in_progress"` → `"completed"`. Also `"deleted"` (soft delete). |
| `owner` | `string` | Name of the agent that has claimed or been assigned this task. Empty string when unassigned. |
| `blocks` | `string[]` | Task IDs that this task blocks (i.e., tasks that cannot start until this one is complete). |
| `blockedBy` | `string[]` | Task IDs that must be in `"completed"` status before this task can be claimed. |

**Real tool call that produces a task file [CONFIRMED, alexop.dev]:**
```json
TaskCreate({
  "subject": "QA: Core pages respond with 200 and valid HTML",
  "description": "Fetch all major pages at localhost:4321 and verify they return HTTP 200. Test: /, /posts, /tags, /notes, /tils, /search, /projects, /404 (should be 404), /rss.xml.",
  "activeForm": "Testing core page responses"
})
```

---

### 2.3 Inbox File Schema

**File:** `~/.claude/teams/{team-name}/inboxes/{agent-name}.json`

Real example (Quriosity reverse-engineering study, 2026-02-28):

```json
[
  {
    "from": "observer",
    "text": "Hello lead, I'm observer, I'm up and running!",
    "summary": "Observer reporting in",
    "timestamp": "2026-02-12T09:21:46.491Z",
    "color": "blue",
    "read": true
  }
]
```

**Envelope schema [CONFIRMED]:**

| Field | Type | Description |
|-------|------|-------------|
| `from` | `string` | Sender's agent name. |
| `text` | `string` | Message body. May be plain text OR a JSON string (double-encoded) for protocol messages. |
| `summary` | `string` | Brief summary for lead visibility of DM traffic. |
| `timestamp` | `ISO-8601` | Send timestamp. |
| `color` | `string` | Visual color label for the sender (set at spawn time). |
| `read` | `boolean` | Delivery acknowledgment flag. `false` = unread. Set to `true` after recipient processes. |

**Inbox files are created on demand [CONFIRMED]:** If no messages have been sent to an agent, their inbox file does not exist. The sender creates it on first write.

---

### 2.4 Protocol Message Format (JSON-in-JSON)

System-level messages encode a JSON object as a string in the `text` field:

```json
{
  "from": "observer",
  "text": "{\"type\":\"idle_notification\",\"from\":\"observer\",\"idleReason\":\"available\"}",
  "read": true
}
```

**Known `type` values [CONFIRMED from binary analysis + docs]:**

| Type | Direction | Purpose |
|------|-----------|---------|
| `task_assignment` | lead → teammate | Full task context assignment |
| `message` | any → any | Direct message |
| `broadcast` | lead → all | Same message to all inboxes |
| `shutdown_request` | lead → teammate | Graceful shutdown request |
| `shutdown_response` | teammate → lead | Approve or reject shutdown |
| `plan_approval_request` | teammate → lead | Submit plan for review (when `planModeRequired=true`) |
| `plan_approval_response` | lead → teammate | Approve or reject with feedback |
| `idle_notification` | teammate → lead | Auto-sent when teammate's turn ends |

**Real task_assignment payload [CONFIRMED, dev.to study]:**
```json
{
  "type": "task_assignment",
  "taskId": "1",
  "subject": "Phase 2: Control-plane - remove participants/presence",
  "description": "Remove multiplayer code from the control-plane package...",
  "assignedBy": "team-lead",
  "timestamp": "2026-02-18T02:37:16.890Z"
}
```

---

### 2.5 Highwatermark File

**File:** `~/.claude/tasks/{team-name}/.highwatermark`

Contains a single integer string, e.g. `"3"` or `"13"`. Represents the next available task ID. Incremented atomically (under `.lock`) each time a new task is created.

---

## 3. TeammateTool

### 3.1 Spawning Mechanism

Each teammate is a **separate invocation of the `claude` CLI** or an **in-process isolated context** depending on the `teammateMode` setting.

**The extended `Task` tool signature for teammates [CONFIRMED]:**
```json
Task({
  "description": "...",
  "subagent_type": "general-purpose",
  "name": "qa-pages",
  "team_name": "blog-qa",
  "model": "claude-sonnet-4-5",
  "prompt": "You are a QA agent. Your assigned task is Task #1: ..."
})
```

New parameters vs standard `Task`:
- `name`: the teammate's identity within the team (used for inbox routing and task ownership)
- `team_name`: the namespace linking this agent to the team's config and task files

**Context initialization [CONFIRMED]:** Each teammate loads fresh project context — `CLAUDE.md`, MCP servers, skills — but does **not** inherit the lead's conversation history. Only the `prompt` field from the `Task` call bootstraps the teammate's context.

**Important behavior [CONFIRMED from issue #1124]:** When a team is active, spawning an `Agent`/`Task` without `team_name` still auto-enrolls it as a teammate. There is no way to spawn a non-teammate agent while a team is active.

---

### 3.2 Display Modes and IPC

**In-process mode (default) [CONFIRMED]:**
- All teammates run inside the same Node.js process
- Context isolation via `AsyncLocalStorage`
- Terminated via `AbortController.abort()`
- No external process spawning; no tmux required
- Works in any terminal

**tmux split-pane mode [CONFIRMED]:**  
Claude Code calls approximately 20 distinct tmux subcommands to manage teammate panes. The KILD shim (issue #26572) catalogued the full list:

| Operation | tmux command |
|-----------|--------------|
| Spawn teammate | `tmux split-window -h <cmd>` or `tmux split-window -v <cmd>` |
| Send input | `tmux send-keys "<text>" Enter` |
| Kill pane | `tmux kill-pane -t <id>` |
| Read output | `tmux capture-pane -t <id>` |
| Get own ID | `tmux display-message -p "#{pane_id}"` |
| List panes | `tmux list-panes` |
| Session detection | `$TMUX` environment variable |
| Rename | `tmux rename-window` |
| Styling | `tmux set-option pane-border-format` etc. |

**Known race condition [CONFIRMED, issue #23615]:** With 4+ agents, ~50% corruption rate. `tmux send-keys` is a stateless subprocess; issuing it for multiple panes in rapid succession produces garbled input (`mmcd` instead of `cd`). A 200ms sleep workaround was attempted but is not reliable.

**iTerm2 split-pane mode [CONFIRMED]:** Uses the `it2` CLI. Detection: iTerm2-specific environment variables. Requires Python API enabled in iTerm2 preferences.

**Auto detection [CONFIRMED]:** Default `teammateMode: "auto"` priority: iTerm2 > tmux > in-process.

**Override:**
```bash
claude --teammate-mode in-process
```
or in `~/.claude.json`:
```json
{ "teammateMode": "in-process" }
```

---

### 3.3 IPC Mechanism

**Complete IPC stack [CONFIRMED]:**

There is **no background daemon, no socket server, no message broker**. The entire IPC system is:

1. **Write path:** Sender appends a message entry to `~/.claude/teams/{team-name}/inboxes/{recipient}.json`
2. **Read path:** Recipient polls their own inbox file between LLM turns
3. **Delivery trigger:** Internal function `injectUserMessageToTeammate` (confirmed from binary string analysis) — incoming inbox messages are injected as synthetic user-turn messages into the recipient's conversation context

**Timing constraint [CONFIRMED]:** One Claude API call = one turn. Messages received while an agent is mid-turn are queued and only processed when the turn completes. This means: if an agent is writing a large file (long tool call), messages sent during that time are buffered in the inbox and only injected at the next turn boundary.

This caused bug #24108: in tmux mode, a newly spawned teammate whose first message arrived before it had completed its first turn (startup/welcome screen) would deadlock — never polling, never processing.

---

### 3.4 Peer Messaging

Any agent can message any other agent directly via `SendMessage`:

```json
SendMessage({
  "type": "message",
  "recipient": "researcher",
  "content": "I found a relevant file at src/auth/jwt.ts — check it",
  "summary": "Pointer to JWT file"
})
```

**Lead visibility of peer DMs [CONFIRMED from system prompt]:** When a teammate DMs another teammate, the lead receives a brief summary in the sender's `idle_notification`. Full content is not relayed to the lead — only the summary. The lead does not need to poll for this; it arrives automatically.

**Broadcast [CONFIRMED]:** `SendMessage` with `type: "broadcast"` literally writes the same entry to every teammate's inbox file. Token cost scales linearly with team size (each teammate processes it in a separate context window turn).

---

### 3.5 Internal AsyncLocalStorage Context Fields [CONFIRMED from binary analysis, v2.1.47]

All fields managed via Node.js `AsyncLocalStorage`:

| Field | Type | Description |
|-------|------|-------------|
| `agentId` | UUID string | Unique agent identifier |
| `agentName` | string | Human-readable name |
| `teamName` | string | Team namespace |
| `parentSessionId` | string | Session ID of the spawning lead |
| `color` | string | Visual color label for TUI display |
| `planModeRequired` | boolean | Whether plan approval gate is active |

**Internal functions confirmed from binary string analysis (v2.1.47):**
- `isTeammate()` — role check: is this context a teammate?
- `isTeamLead()` — role check: is this context the lead?
- `waitForTeammatesToBecomeIdle()` — synchronization primitive for lead
- `getTeammateContext()` — read context from AsyncLocalStorage
- `setDynamicTeamContext()` — mutate context at runtime
- `createTeammateContext()` — initialize context on spawn
- `injectUserMessageToTeammate` — inject inbox message as user turn
- `getTeamName`, `getAgentName`, `getAgentId` — accessors

---

## 4. Ctrl+T Task List Overlay

### What It Is [CONFIRMED]

In **in-process mode**, pressing `Ctrl+T` toggles a TUI overlay showing the current state of all tasks in `~/.claude/tasks/{team-name}/`. The overlay reads directly from the task JSON files on disk.

In **split-pane mode** (tmux/iTerm2), each teammate pane is interactive — click into a pane to interact directly. `Ctrl+T` is not applicable.

### Keyboard Navigation (in-process mode) [CONFIRMED from docs]

| Key | Action |
|-----|--------|
| `Ctrl+T` | Toggle task list overlay |
| `Shift+Down` | Cycle to next teammate |
| `Shift+Up` | Cycle to previous teammate |
| `Enter` | View selected teammate's session |
| `Escape` | Interrupt teammate's current turn |
| `Ctrl+J` | Toggle agent (expand/collapse) |

### What Data the Overlay Reads [CONFIRMED]

The overlay reads from `~/.claude/tasks/{team-name}/` and renders:
- Task `subject` as the list item label
- Task `status` as visual indicator (pending/in_progress/completed)
- Task `owner` displayed next to each task
- Task `activeForm` used for spinner text when status is `"in_progress"` (e.g., "Running tests" rather than "Run tests")
- `blockedBy` relationships shown to indicate dependency state [SPECULATIVE: visual representation not documented, but field exists]

### Task Status Flow [CONFIRMED]

```
TaskCreate  →  "pending"
               │
               ▼  (TaskUpdate: status=in_progress, owner=name)
            "in_progress"
               │
               ├──▶  "completed"   (TaskUpdate: status=completed)
               │
               └──▶  "deleted"     (TaskUpdate: status=deleted)
```

The overlay updates on each file read cycle. **No filesystem watch is used** [CONFIRMED from architecture: polling-based]; the display refreshes between turns.

---

## 5. Dependency Auto-Unblock

### Mechanism [CONFIRMED]

The `blocks` / `blockedBy` fields implement a dependency graph entirely in the task JSON files. There is **no event system** — auto-unblocking is computed at read time.

**Blocking example:**
```json
// Task 1 (must complete before Task 3 can start)
{ "id": "1", "subject": "Run DB migrations", "status": "completed", "blocks": ["3"] }

// Task 2 (also must complete before Task 3)
{ "id": "2", "subject": "Build API schema", "status": "completed", "blocks": ["3"] }

// Task 3 (blocked until both 1 and 2 are completed)
{
  "id": "3",
  "subject": "Run integration tests",
  "status": "pending",
  "owner": "",
  "blockedBy": ["1", "2"],
  "blocks": []
}
```

**Auto-unblock algorithm [CONFIRMED from docs + inferred from design]:**

When a teammate calls `TaskList`:
1. All task files in the directory are read
2. For each `pending` task, check every ID in `blockedBy`
3. If all blocking tasks have `status: "completed"`, the task is considered **claimable**
4. If any blocking task is still `pending` or `in_progress`, the task remains unclaimed

When teammate A writes `"status": "completed"` to task 1.json and teammate B subsequently polls `TaskList`, B's next read will compute that task 3's `blockedBy` constraints are now satisfied and B can claim task 3.

**Official documentation statement [CONFIRMED]:**  
> "The system manages task dependencies automatically. When a teammate completes a task that other tasks depend on, blocked tasks unblock without manual intervention."

**No notification push:** There is no push notification when a task unblocks. Teammates discover unblocked work on their next `TaskList` poll (which happens after each completed task or at end of turn). This introduces latency equal to one full LLM turn between a task completing and its dependents being discovered.

---

## 6. File Locking

### Task Claiming Lock [CONFIRMED]

**File:** `~/.claude/tasks/{team-name}/.lock`  
**Mechanism:** POSIX `flock()` on the `.lock` file  
**Scope:** Protects the claim operation (read-task-status → write-owner) as an atomic unit

The typical claim sequence under lock:
```
flock(.lock, LOCK_EX)
  read N.json                       // verify status still "pending"
  write N.json (status=in_progress, owner=self)
flock(.lock, LOCK_UN)
```

Without this, two teammates could both read `status: "pending"` and both write `owner: self`, producing a double-claim. `flock()` serializes this.

**Confirmed by:**
- Official docs: "Task claiming uses file locking to prevent race conditions when multiple teammates try to claim the same task simultaneously."
- KILD shim author (issue #26572): "our pane registry uses file-based locking"
- dev.to study: ".lock file present in all 42 task directories observed"

### Highwatermark Lock [CONFIRMED]

The `.lock` file also covers the `.highwatermark` counter increment during `TaskCreate`. This prevents two concurrent `TaskCreate` calls from producing the same task ID.

### Inbox Write Concurrency [CONFIRMED — known weakness]

Inbox files (the JSON arrays in `inboxes/`) do **not** have dedicated locking. Concurrent appends from multiple senders can produce corrupted JSON. This is acknowledged in the reverse-engineering literature:

> "Concurrency safety by gentleman's agreement — .lock files exist but aren't strict mutexes. No atomicity (concurrent writes can conflict)." — Quriosity study

This is accepted as a design tradeoff: typical teams are 2–4 agents with low inbox write contention. At larger team sizes, broadcast storms (lead sending to all teammates simultaneously) can trigger concurrent writes to multiple inbox files.

---

## 7. Agent Identity

### Registration Flow [CONFIRMED]

1. Lead calls `TeamCreate({ team_name, description })`:
   - Creates `~/.claude/teams/{team-name}/config.json` with empty `members: []`
   - Creates `~/.claude/tasks/{team-name}/` directory with `.lock` and `.highwatermark = "0"`

2. Lead calls `Task({ ..., team_name, name })`:
   - A new UUID is generated for `agentId`
   - The spawned claude process or in-process context receives environment variables:
     - `CLAUDE_CODE_TEAM_NAME = {team-name}`
     - `CLAUDE_CODE_PLAN_MODE_REQUIRED = "true"` (if plan approval required)
   - On startup, the teammate registers itself in `config.json` by appending to the `members` array

3. Teammate is now discoverable: other agents can read `config.json` to find its `name`, `agentId`, and `agentType`.

### Identity Storage [CONFIRMED]

**In-process mode:** `AsyncLocalStorage` holds the full context: `agentId`, `agentName`, `teamName`, `parentSessionId`, `color`, `planModeRequired`.

**tmux mode:** The process receives identity via environment variables and CLI flags (e.g., `claude --agent-id researcher@my-team --parent-session-id abc123`). [SPECULATIVE: exact CLI flags not publicly documented; inferred from KILD shim's CustomPaneBackend `spawn_agent` example in issue #26572]

### Identity Rules [CONFIRMED from system prompt]

- **Names are routing keys**, not UUIDs. All `SendMessage` and `TaskUpdate` operations reference agents by `name`.
- **UUIDs exist but are not used** for communication — they are for internal tracking only.
- **Peer discovery:** Teammates call `Read("~/.claude/teams/{team-name}/config.json")` to enumerate team members.

### Permission Inheritance [CONFIRMED]

All teammates inherit the **lead's permission mode at spawn time**. If the lead runs `--dangerously-skip-permissions`, all teammates run the same. Post-spawn, individual teammate modes can be changed, but per-teammate modes cannot be set at spawn time.

---

## 8. Known Bugs and Architectural Limitations

### Active Issues [CONFIRMED from GitHub]

| Issue | Problem | Status |
|-------|---------|--------|
| **#23620** | Context compaction erases team awareness — lead forgets the team exists after compaction | Open |
| **#25131** | Catastrophic agent lifecycle failures — agents repeatedly re-spawn | Open |
| **#24130** | Auto-memory doesn't support concurrency — multiple teammates overwrite MEMORY.md | Open |
| **#24977** | TaskUpdate notifications flood context, accelerating compaction | Open |
| **#23629** | Task state desynchronization — team-level vs session-level mismatch | Open |
| **#23615** | tmux send-keys race condition with 4+ agents (~50% corruption) | Open |
| **#23572** | Silent tmux fallback to in-process when detection fails, no error surfaced | Open |
| **#24108** | In tmux mode, newly spawned teammate deadlocks if first message arrives before first turn | Fixed in v2.1.47+ |
| **#23816** | TaskCreate/TaskList/TaskUpdate missing at runtime despite TeamCreate docs referencing them (v2.1.32-2.1.34) | Fixed in v2.1.47+ |
| **#1124** | Agent teams incompatible with SDK/headless mode — lead exits via end_turn before teammates complete | Open |

### Design Limitations [CONFIRMED]

- **No session resumption with in-process teammates**: `/resume` and `/rewind` do not restore in-process teammates
- **One team per session**: a lead can manage only one team at a time
- **No nested teams**: teammates cannot spawn sub-teams or their own teammates
- **Lead is fixed**: the session that creates the team is the lead for its lifetime; leadership cannot be transferred
- **Split panes not available in**: VS Code integrated terminal, Windows Terminal, Ghostty
- **Broadcast cost**: scales linearly with team size (each message consumes a full LLM turn per recipient)

---

## 9. CustomPaneBackend Protocol Proposal (Issue #26572)

This GitHub issue, authored by the creator of KILD (a purpose-built agent runner), documents a reverse-engineering of exactly which tmux commands Claude Code issues and proposes a clean abstraction.

### KILD tmux Shim Findings

The KILD project ships `kild-tmux-shim`, a drop-in `tmux` replacement that intercepts all Claude Code commands. Its existence confirms the full list of tmux subcommands Claude Code uses:

| tmux command | Purpose | Real requirement |
|--------------|---------|-----------------|
| `split-window` | Create pane | Spawn agent process |
| `send-keys` | Send input | Write to stdin |
| `kill-pane` | Terminate | Kill process |
| `capture-pane` | Read output | Read scrollback |
| `display-message` | Query metadata | Get own pane ID |
| `list-panes` | Enumerate | List live contexts |
| `has-session` | Detection | Connectivity check |
| `new-session/window` | Setup | Session management |
| `select-layout` | Geometry | No-op (cosmetic) |
| `resize-pane` | Geometry | No-op (cosmetic) |
| `break-pane/join-pane` | Rearrange | No-op (cosmetic) |
| `set-option pane-border-format` | Styling | No-op (cosmetic) |
| `rename-window` | Label | Metadata only |

**Key finding:** ~20 tmux subcommands called, but only 6 do real work. The rest are layout/cosmetic operations.

### Proposed CustomPaneBackend Protocol

The proposal defines a minimal JSON-RPC 2.0 protocol over NDJSON for a clean pane management abstraction:

**Activation via environment variable:**
```bash
CLAUDE_PANE_BACKEND=/path/to/backend-binary        # spawn on demand
CLAUDE_PANE_BACKEND_SOCKET=/path/to/server.sock    # pre-running server
```

**Handshake:**
```json
// Claude Code → Backend
{"id":"1","method":"initialize","params":{"protocol_version":"1","capabilities":["events"]}}

// Backend → Claude Code
{"id":"1","result":{
  "protocol_version": "1",
  "capabilities": ["events", "capture"],
  "self_context_id": "ctx_0"
}}
```

**Core operations (7 total):**
```json
// Spawn agent (argv[], never shell string)
{"id":"2","method":"spawn_agent","params":{
  "command": ["claude","--agent-id","researcher@my-team","--parent-session-id","abc123"],
  "cwd": "/project",
  "env": {"CLAUDECODE":"1"},
  "metadata": {"name":"researcher","color":"blue","role":"teammate"}
}}
→ {"id":"2","result":{"context_id":"ctx_1"}}

// Write to stdin
{"id":"3","method":"write","params":{"context_id":"ctx_1","data":"<base64>"}}

// Read scrollback
{"id":"4","method":"capture","params":{"context_id":"ctx_1","lines":200}}

// Terminate
{"id":"5","method":"kill","params":{"context_id":"ctx_1"}}

// List live contexts
{"id":"6","method":"list","params":{}}
```

**Push events (unsolicited):**
```json
// Essential: context exit notification
{"method":"context_exited","params":{"context_id":"ctx_1","exit_code":0}}

// Optional: stream output
{"method":"context_output","params":{"context_id":"ctx_1","data":"<base64>"}}
```

**Status as of 2026-03-29:** This is a community proposal filed as a feature request. No official response from Anthropic confirming or denying implementation plans. The proposal is open. Zellij (#24122), WezTerm (#23574), and Ghostty (#24189) are all blocked on this or similar abstractions.

---

## 10. Key Findings Summary

### Architecture in One Sentence
Claude Code Agent Teams is a **file-based multi-process coordination system** where a shared `~/.claude/` directory acts as the entire coordination substrate: task board, message queue, and agent registry — with no background daemon, no socket server, and no in-memory shared state.

### Key Data Points

| Dimension | Detail |
|-----------|--------|
| **Coordination substrate** | Flat JSON files in `~/.claude/tasks/` and `~/.claude/teams/` |
| **IPC mechanism** | File append (write) + file poll (read); no sockets, no daemons |
| **Message delivery** | `injectUserMessageToTeammate` — synthetic user-turn injection |
| **Task concurrency** | `flock()` on `.lock` file; inbox writes unprotected |
| **Identity** | `AsyncLocalStorage` (in-process) or env vars (tmux) |
| **Spawn mechanism** | `tmux split-window` + `send-keys` (tmux) or AsyncLocalStorage fork (in-process) |
| **Dependency resolution** | Computed at `TaskList` read time; no push notification |
| **Message delivery timing** | Between LLM turns only; mid-turn messages are buffered |
| **Token overhead** | ~4× for 3-person team; ~7× for 3-person team in plan mode |
| **Known tmux race** | ~50% corruption with 4+ simultaneous spawns |
| **Minimum version** | Claude Code v2.1.32 |
| **Full feature availability** | Claude Code v2.1.47+ |

---

## 11. References

### Official Sources
- [Claude Code Agent Teams Documentation](https://code.claude.com/docs/en/agent-teams) — `code.claude.com/docs/en/agent-teams`
- [Claude Code Interactive Mode — Task List](https://code.claude.com/docs/en/interactive-mode#task-list)
- [Claude Code Hooks](https://code.claude.com/docs/en/hooks)
- [Claude Code Costs — Agent Team Token Costs](https://code.claude.com/docs/en/costs#agent-team-token-costs)

### GitHub Issues (anthropics/claude-code)
- [#24122 — Zellij split-pane support (reveals tmux detection logic)](https://github.com/anthropics/claude-code/issues/24122)
- [#26572 — CustomPaneBackend protocol proposal (full tmux command inventory + clean abstraction)](https://github.com/anthropics/claude-code/issues/26572)
- [#23615 — tmux race condition with 4+ agents](https://github.com/anthropics/claude-code/issues/23615)
- [#23572 — Silent tmux fallback bug](https://github.com/anthropics/claude-code/issues/23572)
- [#23816 — TaskCreate/TaskList missing at runtime despite docs](https://github.com/anthropics/claude-code/issues/23816)
- [#32723 — TeamCreate available to subagents but not teammates](https://github.com/anthropics/claude-code/issues/32723)
- [#1124 (claude-code-action) — Agent teams incompatible with SDK/headless mode](https://github.com/anthropics/claude-code-action/issues/1124)

### Reverse Engineering Studies
- [Reverse-Engineering Claude Code Agent Teams: Architecture and Protocol — nwyin (dev.to, 2026)](https://dev.to/nwyin/reverse-engineering-claude-code-agent-teams-architecture-and-protocol-o49) — primary source; binary analysis of v2.1.47, on-disk artifacts, AsyncLocalStorage field enumeration
- [Claude Code Agent Teams: The File System Is the Message Queue — Quriosity (GitHub, 2026-02-28)](https://github.com/Quriosity-agent/articles/blob/main/2026-02-28/claude-code-agent-teams-reverse-engineering-en.md) — binary `injectUserMessageToTeammate` confirmation, full message timeline, inbox protocol analysis
- [Claude Code Architecture (Reverse Engineered) — Substack](https://vrungta.substack.com/p/claude-code-architecture-reverse) — architectural overview, AsyncLocalStorage context fields, TAOR loop
- [Inside Claude Code: Deep-Dive Reverse Engineering Report — ShareAI Lab (BrightCoding, 2025-06-29)](https://www.blog.brightcoding.dev/2025/07/17/inside-claude-code-a-deep-dive-reverse-engineering-report/) — internal bus (`h2A`), multi-agent runtime (`I2A`), context compressor (`wU2`); note: studied v1.0.33, predates Agent Teams feature

### System Prompt Extractions
- [TeammateTool System Prompt — Piebald-AI](https://github.com/Piebald-AI/claude-code-system-prompts/blob/main/system-prompts/tool-description-teammatetool.md) — full system prompt injected when EXPERIMENTAL_AGENT_TEAMS is enabled; includes task workflow instructions, idle state semantics, config.json structure

### Community Analysis
- [From Tasks to Swarms: Agent Teams in Claude Code — alexop.dev (2026-02-08)](https://alexop.dev/posts/from-tasks-to-swarms-agent-teams-in-claude-code/) — full tool call sequences from real sessions, TaskCreate/TaskUpdate/SendMessage schemas, token cost analysis
- [Claude Code Ultimate Guide — FlorianBruniaux](https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/guide/workflows/agent-teams.md) — activation examples, keyboard shortcuts

---

*Report compiled 2026-03-29. All findings are from public sources. No Anthropic proprietary code was accessed.*
