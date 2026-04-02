# openswarm Roadmap

> **Context:** This roadmap is derived from a structured gap analysis comparing Claude Code's internal coordinator architecture (`coordinatorMode.ts`) against openswarm's current primitives. Items marked 🎯 are the ones that would make openswarm a credible drop-in complement to — or replacement for — Claude Code's built-in multi-agent coordination.

---

## Theme 1: Worker Result Protocol

Structured output from workers back to coordinators. Currently openswarm signals completion with `<promise>COMPLETE</promise>` but returns no structured result envelope. Claude Code uses a `<task-notification>` XML schema carrying status, summary, result text, and usage — the coordinator's entire decision loop is driven by this signal.

---

### 1. 🎯 Structured task-completion envelope

**Priority:** critical

**Motivation:** When a worker finishes, the coordinator currently has no machine-readable way to receive (a) terminal status (`completed` / `failed` / `killed`), (b) a one-line summary, (c) the worker's final text output, or (d) resource usage. Claude Code's `<task-notification>` XML carries all four, enabling coordinators to synthesize results and route follow-up work without screen-scraping. Without a structured envelope, openswarm coordinators must poll pane output and parse it ad hoc — fragile and provider-specific.

**Scope:**
- Add a `Result` struct to the `run` package: `{ RunID, Status, Summary, Output, Tokens, ToolUses, DurationMs }`.
- Extend `run.Wait()` to populate `Result` from (a) the pane's exit code and (b) a delimited output block: `<swarm-result status="completed">…</swarm-result>` printed by the worker before exit.
- `swarm run wait <id>` already blocks and returns JSON — extend the JSON schema to include the full `Result`.
- Add `swarm run result <id>` as a non-blocking query for a completed run's stored result.
- Store results in `.swarm/runs/results/<run-id>.json` (one file per result, lock-free, immutable once written).
- Workers that are pi / Claude Code agents should emit the delimiter automatically via the extras skill; workers that are shell scripts print the delimiter manually.

**Dependency:** None. Buildable immediately.

---

### 2. 🎯 Result routing: inject worker completions into coordinator inbox

**Priority:** high

**Motivation:** Claude Code delivers `<task-notification>` as a **user-role message** injected directly into the coordinator's conversation — zero polling required. Openswarm coordinators today must watch their inbox with `swarm msg watch` or poll `swarm run list`. The gap is that there's no automatic "when run R completes, deliver a formatted message to coordinator C" binding — coordinators must wire it manually. Closing this gap means the coordinator's agentic loop needs no external polling code.

**Scope:**
- Add a `notify` field to `run.Start` options: `NotifyAgent string` — the agent to message when this run completes.
- When `run.Wait()` resolves (or the background completion poller detects exit), call `msg.Send` with a structured body derived from the `Result` envelope (item 1).
- Define a canonical message subject: `"run.complete:<run-id>"` so coordinators can filter their inbox by subject.
- The body should be a JSON block matching the `Result` struct — the coordinator can extract it with `--json` or parse it with `jq`.
- Extras skill and `/assign-task` commands should pass `--notify $SWARM_AGENT_ID` so workers auto-route results back to their spawner.

**Dependency:** Depends on item 1 (structured result envelope).

---

## Theme 2: Coordinator Mode / Prompting Layer

Claude Code's coordinator is a distinct agent role with a 600-line injected system prompt (`getCoordinatorSystemPrompt()`), dynamic capability context (`getCoordinatorUserContext()`), and a mode flag (`CLAUDE_CODE_COORDINATOR_MODE`). Openswarm has `swarm prompt` (task-state injection) but no coordinator persona, no tool-capability advertisement, and no mode concept.

---

### 3. 🎯 Coordinator system prompt: `swarm prompt --mode coordinator`

**Priority:** high

**Motivation:** The biggest single thing that makes Claude Code's coordinator effective is an injected system prompt that shapes its role: it knows it orchestrates workers, never thanks them, synthesizes results before delegating, and runs parallel fan-out. `swarm prompt` currently emits task-queue state. Adding a coordinator-mode variant would let any agent — Claude Code, opencode, pi, or a custom agent — adopt the coordinator role by calling `swarm prompt --mode coordinator`, receiving a prompt they can inject into their system context. This is the highest-leverage documentation artifact in the system.

**Scope:**
- Extend `task.Prompt()` (or add `prompt.Coordinator()`) to emit a structured coordinator system prompt covering: role definition, tool inventory (derived from registered agent profiles), worker spawn/continue/stop semantics, research-then-synthesize workflow, parallel fan-out pattern, and verification discipline.
- `swarm prompt --mode coordinator` prints the coordinator prompt (existing `swarm prompt` with no flag continues to emit task-state context for workers).
- Prompt should include a `## Active Team` section derived from `swarm agent list` — who is registered, what their roles are.
- Extras (Claude Code `settings.json`, opencode plugin, pi extension) should call `swarm prompt --mode coordinator` on `SessionStart` for sessions flagged as coordinator sessions.
- The prompt should be configurable via `.swarm/config.toml` keys (e.g. `[coordinator] style = "minimal|full"`).

**Dependency:** None for the basic implementation. Item 5 (session mode) enhances auto-selection.

---

### 4. Dynamic worker capability context

**Priority:** normal

**Motivation:** Claude Code's `getCoordinatorUserContext()` tells the coordinator exactly which tools each worker class has access to — bash, file edit, MCP servers, skills. Today openswarm's coordinator has no machine-readable way to know this: it can see agent profiles in `registry.json` but there's no enumeration of tool capabilities. This leads to coordinators writing prompts that ask workers to use tools they don't have.

**Scope:**
- Add an optional `tools` list to the `AgentProfile` in `config.toml`: a free-form string list of tool names the agent has (e.g. `["bash", "file_edit", "file_read", "mcp:github"]`).
- `swarm prompt --mode coordinator` should incorporate these capability lists in the worker-tools section of the coordinator prompt (item 3).
- `swarm agent list --json` should expose the `tools` field so coordinators can query it programmatically.
- The extras skill should document how to populate `tools` for Claude Code, pi, and opencode agents.

**Dependency:** Depends on item 3 (coordinator prompt).

---

### 5. Session mode tracking: `swarm session` subcommand

**Priority:** normal

**Motivation:** Claude Code's `matchSessionMode()` reads a stored `sessionMode: 'coordinator' | 'normal'` flag on resume and flips the live env var to match. Openswarm has no session record at all — runs persist but there's no way to know "this `.swarm/` was last used as a coordinator session." When an agent resumes a project and runs `swarm prompt`, it gets worker-context, not coordinator-context. Session mode tracking closes this gap cleanly.

**Scope:**
- Add `.swarm/session.json`: `{ mode: "coordinator" | "worker", last_agent: "<agent-id>", updated_at: "..." }`.
- `swarm session set --mode coordinator` writes the file.
- `swarm session get` prints current mode (JSON-compatible).
- `swarm prompt` (no flags) reads session mode and auto-selects the right prompt template, so agents calling `swarm prompt` at startup always get the correct context.
- Extras hooks (`SessionStart`, `session_start`) should call `swarm session get` and inject the correct prompt mode.

**Dependency:** Enhances item 3 (coordinator prompt).

---

## Theme 3: Scratchpad / Shared Knowledge Primitive

Claude Code injects a `scratchpadDir` path into the coordinator context: "Workers can read and write here without permission prompts. Use this for durable cross-worker knowledge — structure files however fits the work." Openswarm's `.swarm/` is fully structured (tasks, messages, events) but offers no free-form shared workspace. Agents today use git worktrees for isolation, not for sharing.

---

### 6. 🎯 `.swarm/scratch/` — free-form shared workspace

**Priority:** high

**Motivation:** Research workers accumulate context that coordinators need in order to write precise follow-up prompts. Without a shared drop zone, coordinators must parse that context out of message bodies — lossy and fragile. Claude Code's scratchpad gives workers a place to write structured notes (file trees, type signatures, error logs) and coordinators a place to read them back. This is one of the primitives that most directly enables the "synthesize before delegating" workflow.

**Scope:**
- `swarm init` creates `.swarm/scratch/` as an empty directory.
- `swarm scratch write <key> [--body|-]` — atomic write of `.swarm/scratch/<key>` (key is a path-safe name like `auth-research.md`); reads body from stdin or `--body`.
- `swarm scratch read <key>` — print contents.
- `swarm scratch list` — enumerate keys (filenames) with size and modified time.
- `swarm scratch rm <key>` — idempotent delete.
- No locking needed — files are named by key; concurrent writes to different keys are lock-free. Concurrent writes to the same key use `AtomicWrite` (temp + rename) for safety.
- `swarmfs` should expose `Root.ScratchPath(key string) string` and `Root.ScratchDir() string` — no caller constructs scratch paths by hand.
- The coordinator prompt (item 3) should mention the scratch directory and encourage workers to write research summaries there.

**Dependency:** None.

---

## Theme 4: Session Continuity

Claude Code tracks session mode and can reconcile a resumed session's state. Openswarm treats each CLI invocation as stateless — tasks persist but there's no session envelope that captures coordinator/worker role, active run IDs, or partially-resolved task results.

---

### 7. Run result persistence and re-query

**Priority:** normal

**Motivation:** When a coordinator session is interrupted mid-flight (agent killed, terminal closed), any run results that arrived as inbox messages survive in `.swarm/messages/`, but the connection between "run R produced result X for task T" is lost. A coordinator resuming work must re-read its inbox, correlate messages to tasks, and reconstruct its mental model manually. Persisting run results (item 1 already stores them in `.swarm/runs/results/`) and tying them to the task they served closes this gap.

**Scope:**
- Extend the `Task` schema with `result_run_id string` (optional) — set when a worker marks the task `done` or `failed` via `swarm task done <id> --run <run-id>`.
- `swarm task show <id>` should inline the run result when `result_run_id` is set (equivalent to `swarm run result <run-id>`).
- `swarm prompt --mode coordinator` should include a section listing tasks with pending/completed results so the coordinator can pick up where it left off.
- This gives `swarm prompt` the ability to act as a session-resume primitive: run it, get full current state including results already delivered.

**Dependency:** Depends on item 1 (structured result envelope).

---

### 8. Context compaction hook: preserve coordination state

**Priority:** normal

**Motivation:** The opencode plugin already hooks `experimental.session.compacting` to append `swarm prompt` output to the compaction context. This pattern should be formalized and extended to all extras: when a long-running coordinator session approaches context limits, the agent should be able to call `swarm prompt --mode coordinator --full` and receive a compact, high-signal summary of active tasks, pending results, and scratch notes — a single inject that restores situational awareness post-compact. Without this, coordinators "forget" what workers they launched after compaction.

**Scope:**
- `swarm prompt --full` (works with both modes) emits an extended summary that includes: all in-progress and todo tasks with assignees, all unread inbox messages (subject + first 200 chars), all scratch keys with first line, active runs with status.
- Normal `swarm prompt` continues to emit the concise task-state summary (for startup injection, token-efficient).
- The opencode plugin's compaction hook should be updated to use `--full`.
- The Claude Code `settings.json` hook (which only fires on `startup`) should document that users should add a `compact` hook manually calling `swarm prompt --full`.
- The pi extension's `/swarm-prompt` command should pass `--full` when the user is in an active coordination session.

**Dependency:** Depends on item 5 (session mode) for mode auto-selection; can ship without it.

---

## Theme 5: Additional Gaps

Gaps that don't fit neatly into themes 1–4 but materially affect coordinator effectiveness.

---

### 9. 🎯 Parallel run wait: `swarm run wait --all <id...>`

**Priority:** high

**Motivation:** Claude Code's most powerful coordination pattern is parallel fan-out: launch N workers simultaneously, synthesize results as they arrive. Openswarm can launch N runs with `swarm run start` but has no primitive for "wait until all of these finish and collect their results." Coordinators today must poll `swarm run list` in a loop or run N concurrent `swarm run wait` calls. A single `--all` flag on `wait` makes parallel coordination a first-class citizen.

**Scope:**
- `swarm run wait --all <id1> <id2> ...` blocks until all named runs complete and prints a JSON array of `Result` records (in completion order or sorted by ID).
- `swarm run wait --any <id1> <id2> ...` (bonus) blocks until the first run completes and returns that result.
- Implementation: goroutine per run, channel-merge, context cancellation.
- `--timeout <duration>` flag on `swarm run wait` for both single and `--all` variants.
- Coordinators can use `--json` output and pipe to `jq` to extract individual results.

**Dependency:** Depends on item 1 (structured result envelope).

---

### 10. Worker stop/continue semantics: `swarm run pause` and `swarm run resume`

**Priority:** low

**Motivation:** Claude Code has `TASK_STOP_TOOL_NAME` — stop a worker that went in the wrong direction, then continue it with corrected instructions via `SEND_MESSAGE_TOOL_NAME`. Openswarm's `swarm run kill` is destructive: the pane closes and context is gone. For AI agent workers this matters less (the agent can be re-spawned with a corrected prompt), but for long-running shell workers or agents with expensive context, stop/resume is valuable. At minimum, `swarm run kill` should be documented as non-resumable and a workflow for "correct and re-spawn" should be standardized in the extras skill.

**Scope:**
- Phase 1 (documentation): update the extras skill and coordinator prompt to document the "kill + re-spawn with corrected prompt" pattern as the standard stop/continue workflow. Document which run fields to copy when re-spawning (same task, same notify agent, corrected prompt).
- Phase 2 (optional, future): `swarm run pause <id>` sends SIGSTOP to the pane's process and records `status: paused` in `runs.json`. `swarm run resume <id>` sends SIGCONT. Useful for shell scripts; not meaningful for LLM agents.
- The task system already supports re-claiming a failed task — document `swarm task update <id> --status todo` as the "reset and retry" primitive.

**Dependency:** Phase 2 depends on `pane.Send` having SIGSTOP support (backend-specific).

---

## Drop-in Complement / Replacement Checklist

The items marked 🎯 are the ones that collectively make openswarm a credible complement to or replacement for Claude Code's internal multi-agent coordination:

| Item | What it closes |
|------|---------------|
| 1. Structured result envelope | `<task-notification>` parity — structured status/summary/result back to coordinator |
| 2. Result routing via inbox | Automatic delivery of worker completions without polling |
| 3. Coordinator system prompt | `getCoordinatorSystemPrompt()` parity — any agent can become a coordinator |
| 6. `.swarm/scratch/` | `scratchpadDir` parity — durable cross-worker knowledge store |
| 9. Parallel run wait | Fan-out/join parity — first-class parallel coordination |

With items 1, 2, 3, 6, and 9 implemented, an agent using openswarm can match Claude Code's coordination loop precisely: spawn parallel workers, receive structured results automatically, synthesize via a scratchpad, and re-delegate with precise prompts — all without being Claude Code.

---

## Implementation Order

```
Phase A (foundation):  1 → 2 → 6
Phase B (coordinator): 3 → 4 → 5
Phase C (continuity):  7 → 8
Phase D (concurrency): 9
Phase E (polish):      10
```

Phase A unblocks everything else. Start there.
