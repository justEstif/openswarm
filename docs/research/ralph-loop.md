# The Ralph Loop: Research Report

> **Status:** Research complete as of 2026-03-29  
> **Sources:** snarktank/ralph README + raw source, snarktank/antfarm README + docs, deepwiki.com/snarktank/ralph, ghuntley.com/ralph, docs.openclaw.ai, syuya2036/ralph-loop, ralph-cli.dev, claytonfarr.github.io/ralph-playbook, rywalker.com/research/antfarm

---

## Table of Contents

1. [The Ralph Loop: Core Concept](#1-the-ralph-loop-core-concept)
2. [snarktank/ralph: The Reference Implementation](#2-snarkrankralph-the-reference-implementation)
3. [The `prd.json` Schema](#3-the-prdjson-schema)
4. [How the Loop Executes (with Source)](#4-how-the-loop-executes-with-source)
5. [How "Reset Context" Is Implemented](#5-how-reset-context-is-implemented)
6. [The Four Memory Channels](#6-the-four-memory-channels)
7. [Antfarm: Multi-Agent Orchestration on the Ralph Pattern](#7-antfarm-multi-agent-orchestration-on-the-ralph-pattern)
8. [OpenClaw: The Runtime Antfarm Runs On](#8-openclaw-the-runtime-antfarm-runs-on)
9. [Other Implementations of the Pattern](#9-other-implementations-of-the-pattern)
10. [Comparison Table](#10-comparison-table)
11. [Open Questions and Gaps](#11-open-questions-and-gaps)
12. [Key Findings Summary](#12-key-findings-summary)

---

## 1. The Ralph Loop: Core Concept

**Origin:** Geoffrey Huntley ([@GeoffreyHuntley](https://ghuntley.com/ralph/)) coined the pattern. In its purest stated form:

```bash
while :; do cat PROMPT.md | claude-code; done
```

The core thesis: **LLM context windows degrade in quality as they fill up.** The "smart zone" is the first 40–60% of the available window. The Ralph Loop deliberately keeps each iteration within that zone by spawning a fresh process for every unit of work.

### What Ralph Is Not

- It is **not** multi-agent (in its original form) — it is a single agent looping monolithically
- It is **not** about prompt engineering perfection — the loop compensates for LLM nondeterminism
- It is **not** stateful within a session — all memory is externalized to the filesystem and git

### The Design Philosophy

> "Ralph is monolithic. Ralph works autonomously in a single repository as a single process that performs one task per loop." — Geoffrey Huntley

Huntley explicitly rejects the contemporary obsession with multi-agent systems for most use cases, comparing it to microservices vs. a monolith: non-deterministic agents communicating with non-deterministic agents produces compounding chaos. A single, tightly-scoped loop with file-based state is more reliable.

### Refinement over time

Huntley describes Ralph as a system you **tune**:

> "Each time Ralph does something bad, Ralph gets tuned — like a guitar."

Backpressure (typechecks, tests, builds) is critical. Without automated quality gates, Ralph generates plausible-looking broken code that compounds across iterations.

---

## 2. snarktank/ralph: The Reference Implementation

**Repo:** https://github.com/snarktank/ralph  
**Author:** Ryan Carson (also created `ai-dev-tasks`, 7.5K stars)  
**Stars:** 9,800+ (as of 2026-03)  
**Based on:** Geoffrey Huntley's Ralph pattern

Ryan Carson built the concrete, opinionated version of Ralph that:
- Uses `prd.json` as the task state file (instead of freeform markdown)
- Supports two agent backends: **Amp** (default) and **Claude Code**
- Prescribes four specific memory channels
- Adds branch tracking and archiving

### Key Files in the Repo

| File | Purpose |
|------|---------|
| `ralph.sh` | The bash loop orchestrator |
| `prompt.md` | Agent instructions for Amp |
| `CLAUDE.md` | Agent instructions for Claude Code |
| `prd.json` | Task state file (lives in your project) |
| `prd.json.example` | Schema reference |
| `progress.txt` | Append-only learnings log (lives in your project) |
| `skills/prd/` | Amp/Claude skill to generate PRDs |
| `skills/ralph/` | Amp/Claude skill to convert PRD → `prd.json` |
| `.claude-plugin/` | Plugin manifest for Claude Code Marketplace |

### Workflow

```
1. Create a PRD (via /prd skill or manually)
2. Convert PRD → prd.json (via /ralph skill or manually)
3. Run: ./scripts/ralph/ralph.sh [--tool amp|claude] [max_iterations]
4. Ralph loops until all stories have passes: true or max iterations hit
```

---

## 3. The `prd.json` Schema

> **Directly observed** from `prd.json.example` in the repo.

```json
{
  "project": "MyApp",
  "branchName": "ralph/task-priority",
  "description": "Task Priority System - Add priority levels to tasks",
  "userStories": [
    {
      "id": "US-001",
      "title": "Add priority field to database",
      "description": "As a developer, I need to store task priority so it persists across sessions.",
      "acceptanceCriteria": [
        "Add priority column to tasks table: 'high' | 'medium' | 'low' (default 'medium')",
        "Generate and run migration successfully",
        "Typecheck passes"
      ],
      "priority": 1,
      "passes": false,
      "notes": ""
    }
  ]
}
```

### Field Reference

| Field | Type | Level | Required | Notes |
|-------|------|-------|----------|-------|
| `project` | `string` | Root | Yes | Human-readable project name |
| `branchName` | `string` | Root | Yes | Git branch. Convention: `ralph/<feature-name>`. Used by loop for branch checkout and archiving detection |
| `description` | `string` | Root | Yes | One-line feature summary |
| `userStories` | `array` | Root | Yes | Ordered list of stories |
| `id` | `string` | Story | Yes | Unique story identifier. Convention: `US-NNN` |
| `title` | `string` | Story | Yes | Short display title |
| `description` | `string` | Story | Yes | User story in "As a X, I need Y so that Z" format |
| `acceptanceCriteria` | `string[]` | Story | Yes | Checklist of conditions that must be true for the story to pass. Typically ends with "Typecheck passes" |
| `priority` | `integer` | Story | Yes | Sort order. Lower number = higher priority. Agent picks the lowest priority number where `passes: false` |
| `passes` | `boolean` | Story | Yes | **The key state field.** `false` = not done, `true` = implemented + quality checks passed + committed. Agent only sets this to `true` after all quality gates pass |
| `notes` | `string` | Story | No | Free-text notes. Used for implementation hints, discovered gotchas |

### Status Model

The `passes` field is binary — there is no "in-progress" or "failed" status:

| `passes` value | Meaning |
|----------------|---------|
| `false` | Story is pending (or failed quality gates — same state, will be retried) |
| `true` | Story completed: implemented, quality checks passed, committed |

**Key consequence:** If an iteration fails quality gates, it makes **no state changes** — `passes` stays `false`, no git commit, no `progress.txt` append. The next iteration picks up the same story and retries from scratch.

### Dependency Model

> **Inferred** from the example and prompt instructions.

There is **no explicit dependency field** in the schema. Dependencies are expressed implicitly through:
1. **`priority` ordering** — lower priority number = earlier execution. Stories must be ordered by dependency in the JSON (e.g., DB schema before UI, backend before frontend)
2. **Acceptance criteria** — criteria can reference outputs of previous stories
3. **`notes` field** — can describe dependencies informally

The agent does not check or enforce dependencies; it assumes `priority` ordering is correct.

### Archiving

When `branchName` changes between runs, `ralph.sh` archives `prd.json` and `progress.txt` to:
```
archive/YYYY-MM-DD-<feature-name>/
```
(strips the `ralph/` prefix from the branch name for the folder)

---

## 4. How the Loop Executes (with Source)

> **Directly observed** from `ralph.sh` (raw source fetched from GitHub).

```bash
#!/bin/bash
# Parse --tool and max_iterations arguments
TOOL="amp"       # Default
MAX_ITERATIONS=10

# Paths (relative to script directory)
PRD_FILE="$SCRIPT_DIR/prd.json"
PROGRESS_FILE="$SCRIPT_DIR/progress.txt"
ARCHIVE_DIR="$SCRIPT_DIR/archive"
LAST_BRANCH_FILE="$SCRIPT_DIR/.last-branch"

# Archive previous run if branchName changed
# ... [archiving logic] ...

# Initialize progress.txt if missing

echo "Starting Ralph - Tool: $TOOL - Max iterations: $MAX_ITERATIONS"

for i in $(seq 1 $MAX_ITERATIONS); do
  echo "Ralph Iteration $i of $MAX_ITERATIONS ($TOOL)"

  # Spawn fresh agent process
  if [[ "$TOOL" == "amp" ]]; then
    OUTPUT=$(cat "$SCRIPT_DIR/prompt.md" | amp --dangerously-allow-all 2>&1 | tee /dev/stderr) || true
  else
    OUTPUT=$(claude --dangerously-skip-permissions --print < "$SCRIPT_DIR/CLAUDE.md" 2>&1 | tee /dev/stderr) || true
  fi

  # Check for completion signal
  if echo "$OUTPUT" | grep -q "<promise>COMPLETE</promise>"; then
    echo "Ralph completed all tasks!"
    echo "Completed at iteration $i of $MAX_ITERATIONS"
    exit 0
  fi

  echo "Iteration $i complete. Continuing..."
  sleep 2
done

echo "Ralph reached max iterations ($MAX_ITERATIONS) without completing all tasks."
exit 1
```

### Per-Iteration Sequence (from `CLAUDE.md` / `prompt.md`)

Each spawned agent process follows these steps (directly observed from agent instruction files):

```
1. Read prd.json                        → get story list + branchName
2. Read progress.txt                    → get Codebase Patterns section first
3. Check git branch matches branchName  → checkout/create if needed
4. Pick story: highest priority where passes: false
5. Implement that single story
6. Run quality checks (typecheck, lint, test — project-specific)
7. Update AGENTS.md files with any reusable patterns discovered
8. If checks pass:
   a. git commit ALL changes: "feat: [Story ID] - [Story Title]"
   b. Set passes: true in prd.json for this story
   c. Append to progress.txt (with thread URL for Amp)
9. Check if ALL stories have passes: true
   → If yes: output <promise>COMPLETE</promise>
   → If no: exit normally (next iteration picks up)
```

### Completion Signal

The completion check is a string search in `ralph.sh`:
```bash
if echo "$OUTPUT" | grep -q "<promise>COMPLETE</promise>"; then
```

The agent outputs this XML-like tag after verifying all stories are `passes: true`. The shell script never parses `prd.json` directly — it relies entirely on the agent's output signal.

### Quality Gate Atomicity

The agent instruction is explicit: **if quality checks fail, do nothing**. No commit, no `prd.json` update, no `progress.txt` append. This is an "all or nothing" contract enforced by the prompt, not by the shell script.

---

## 5. How "Reset Context" Is Implemented

> **Directly observed** — this is the central mechanism.

**It is simply process termination.** The agent CLI process (`amp` or `claude`) runs, completes (or errors), and exits. The bash script's `for` loop then starts the next iteration, spawning a completely new process.

```bash
# This is it. No daemon. No flags. No IPC.
for i in $(seq 1 $MAX_ITERATIONS); do
  OUTPUT=$(claude --dangerously-skip-permissions --print < CLAUDE.md 2>&1) || true
  # ... check completion ...
  sleep 2   # Brief pause between iterations
done
```

### Key Details

| Aspect | Implementation |
|--------|---------------|
| **Process model** | Each iteration = one `amp` or `claude` CLI subprocess |
| **Context reset** | Process exits → all in-memory state gone → new process has empty context |
| **State persistence** | File system only: `prd.json`, `progress.txt`, `AGENTS.md`, git history |
| **Error handling** | `|| true` — agent failures are swallowed; the loop continues |
| **Iteration delay** | `sleep 2` — 2-second pause between iterations |
| **Max iterations** | Configurable CLI arg, default 10. Hard stop at `MAX_ITERATIONS` |
| **Early exit** | `<promise>COMPLETE</promise>` in agent stdout → `exit 0` |
| **Failure exit** | All iterations exhausted without completion → `exit 1` |

**For Amp:**
```bash
cat prompt.md | amp --dangerously-allow-all
```
The prompt is piped to Amp via stdin.

**For Claude Code:**
```bash
claude --dangerously-skip-permissions --print < CLAUDE.md
```
The instruction file is redirected to Claude's stdin. `--print` is non-interactive mode.

### The `--dangerously-*` Flags

Both tools require disabling human-in-the-loop confirmations for autonomous operation:
- Amp: `--dangerously-allow-all` — allows all tool use without confirmation
- Claude Code: `--dangerously-skip-permissions` — skips permission prompts

---

## 6. The Four Memory Channels

> **Directly observed** from `CLAUDE.md`, `prompt.md`, README, and deepwiki.

The Ralph Loop externalizes all state to four persistent channels. Each fresh agent process reads all four at the start of its session.

### Channel 1: Git History

**What it is:** The git commit log of the repository being developed.

**How it's written:** The agent runs `git commit` after each successful story implementation. Commit message format: `feat: [Story ID] - [Story Title]`

**How it's read:** Both Amp and Claude Code automatically read recent git history as part of their tool use behavior (they run `git log`, `git diff`, etc. natively). No explicit instruction is required.

**What it provides:**
- Complete record of all code changes
- Commit messages explain implementation decisions
- Diffs show patterns of how similar changes were made
- The agent can check "was this already implemented?" via git log

**What it does NOT provide:** The reasoning behind decisions, discovered patterns, or project-specific conventions. That's what the other channels handle.

---

### Channel 2: `progress.txt`

**What it is:** An append-only chronological learnings log. Persists across all iterations.

**Format (directly observed from `CLAUDE.md`):**

```markdown
# Ralph Progress Log
Started: [date]
---

## Codebase Patterns
- Use `sql<number>` template for aggregations
- Always use `IF NOT EXISTS` for migrations
- Export types from actions.ts for UI components

## [Date/Time] - US-001
Thread: https://ampcode.com/threads/$AMP_CURRENT_THREAD_ID
- What was implemented
- Files changed
- **Learnings for future iterations:**
  - Patterns discovered (e.g., "this codebase uses X for Y")
  - Gotchas encountered (e.g., "don't forget to update Z when changing W")
  - Useful context (e.g., "the evaluation panel is in component X")
---

## [Date/Time] - US-002
...
```

**Key structural detail:** There is a `## Codebase Patterns` section at the **top** of the file. The agent instruction says: *"Read the Codebase Patterns section first."* This section is a curated, de-duplicated summary of the most generally applicable learnings — general reusable patterns, not story-specific details.

**Initialization by `ralph.sh`:**
```bash
echo "# Ralph Progress Log" > "$PROGRESS_FILE"
echo "Started: $(date)" >> "$PROGRESS_FILE"
echo "---" >> "$PROGRESS_FILE"
```

**Thread URLs (Amp-specific):** The Amp version of the instruction includes the thread URL in each entry, allowing future iterations to use Amp's `read_thread` tool to access the full conversation of a previous iteration.

---

### Channel 3: `prd.json` (Task State File)

**What it is:** The executable task list, doubling as state tracker.

**How it's written:** The agent updates `passes: true` in-place after a successful story. It uses whatever JSON editing tool is available (typically the agent's native write/edit tools).

**How it's read:** The agent reads the file at the start of each iteration to:
1. Know which branch to be on (`branchName`)
2. Find the next story to work on (lowest `priority` where `passes: false`)
3. Know when everything is done (all `passes: true` → output `<promise>COMPLETE</promise>`)

**Also read by `ralph.sh`:** The shell script reads `branchName` via `jq` for archiving purposes:
```bash
CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
```

---

### Channel 4: `AGENTS.md` Files

**What they are:** Distributed knowledge files placed in directories throughout the codebase. Each file contains conventions, patterns, and gotchas specific to that module/directory.

**Auto-loading behavior:** Both Amp and Claude Code **automatically** discover and read `AGENTS.md` files from the working directory and its parents. This is a native feature of both CLIs — no explicit instruction is needed for reading.

**How they're written:** The agent instruction says: before committing, check if edited files have learnings worth preserving in nearby `AGENTS.md` files, then add them.

**What goes in `AGENTS.md` (from instruction file):**
- API patterns or conventions specific to that module
- Gotchas or non-obvious requirements
- Dependencies between files
- Testing approaches for that area
- Configuration or environment requirements

**What does NOT go in `AGENTS.md`:**
- Story-specific implementation details
- Temporary debugging notes
- Information already in `progress.txt`

**Example entries from instructions:**
```markdown
- "When modifying X, also update Y to keep them in sync"
- "This module uses pattern Z for all API calls"
- "Tests require the dev server running on PORT 3000"
- "Field names must match the template exactly"
```

**Note on naming:** The repo's `CLAUDE.md` (the Claude Code version of the prompt) uses `AGENTS.md`. The Amp version (`prompt.md`) was updated to also use `AGENTS.md`. Both CLIs auto-read files named `AGENTS.md`. This is distinct from the agent instruction file `CLAUDE.md` which is fed to the agent as stdin.

---

### Memory Channel Comparison

| Channel | Written by | Read by | Read how | Scope |
|---------|-----------|---------|---------|-------|
| Git history | Agent (git commit) | Agent | Auto (native CLI feature) | All code changes |
| `progress.txt` | Agent (append) | Agent | Explicit read instruction | Chronological learnings + patterns |
| `prd.json` | Agent (JSON edit) + `ralph.sh` (branch tracking) | Both | Agent reads; script reads via `jq` | Task completion state |
| `AGENTS.md` | Agent (edit-in-place) | Agent | Auto (native CLI feature) | Module-specific conventions |

---

## 7. Antfarm: Multi-Agent Orchestration on the Ralph Pattern

**Repo:** https://github.com/snarktank/antfarm  
**Author:** Ryan Carson (same as Ralph)  
**Tagline:** "Build your agent team in OpenClaw with one command"  
**Description:** Antfarm scales the Ralph Loop to multiple specialized agents executing in a deterministic pipeline.

### How Antfarm Extends Ralph

The core insight: Ralph's "fresh context per iteration" principle is agent-agnostic. Antfarm generalizes it to **multiple specialized agents** where each agent is a role in a pipeline (planner → developer → verifier → tester → reviewer), each running in their own fresh OpenClaw session.

From the Antfarm README:
> "Each agent runs in a fresh session with clean context. Memory persists through git history and progress files — the same autonomous loop pattern from Ralph, scaled to multi-agent workflows."

### Architecture

```
YAML workflow definition
        ↓
TypeScript CLI (antfarm)
        ↓
SQLite (state tracking)    +    cron (polling)
        ↓
OpenClaw sessions (one per agent step, fresh context)
        ↓
Git + progress files (shared memory across agents)
```

**Stack:** TypeScript CLI + SQLite + cron. No Docker, no Redis, no queues.

### Workflow YAML Schema

> **Directly observed** from `antfarm/docs/creating-workflows.md` and `workflows/feature-dev/workflow.yml`.

```yaml
id: my-workflow             # Unique identifier (lowercase, hyphens)
name: My Workflow           # Human-readable
version: 1                  # Integer version
description: |              # Multi-line description
  What the workflow does.

polling:
  model: default
  timeoutSeconds: 120       # How long between cron polls

agents:
  - id: researcher          # Unique within workflow
    name: Researcher        # Display name
    role: analysis          # Controls tool access (see roles below)
    timeoutSeconds: 900     # Optional override for session timeout
    description: What it does.
    workspace:
      baseDir: agents/researcher
      files:
        AGENTS.md: agents/researcher/AGENTS.md
        SOUL.md: agents/researcher/SOUL.md
        IDENTITY.md: agents/researcher/IDENTITY.md
      skills:               # Optional: install into workspace
        - agent-browser

steps:
  - id: research            # Unique step identifier
    agent: researcher       # Which agent handles this step
    max_retries: 2          # Optional retry count
    input: |                # Prompt template (handlebars-like)
      Research {{task}} and report findings.
      Reply with:
      STATUS: done
      FINDINGS: what you found
    expects: "STATUS: done" # String output must contain for success
```

### Agent Roles

| Role | Access | Typical agents |
|------|--------|---------------|
| `analysis` | Read-only code exploration | planner, reviewer, investigator |
| `coding` | Full read/write/exec | developer, fixer, setup |
| `verification` | Read + exec, NO write | verifier |
| `testing` | Read + exec + browser/web, NO write | tester |
| `pr` | Read + exec only (runs `gh pr create`) | pr |
| `scanning` | Read + exec + web search, NO write | scanner |

### Context Passing Between Agents

Steps pass output via `KEY: value` pairs in agent output, which become `{{variables}}` in subsequent step templates:

```yaml
# Step 1 output:
# STATUS: done
# FINDINGS: the API uses REST with JWT

# Step 2 input template:
input: |
  Given these findings: {{findings}}
  Original task: {{task}}
```

### Bundled Workflows

| Workflow | Agents | Pipeline |
|----------|--------|---------|
| `feature-dev` | 7 | plan → setup → implement → verify → test → PR → review |
| `security-audit` | 7 | scan → prioritize → setup → fix → verify → test → PR |
| `bug-fix` | 6 | triage → investigate → setup → fix → verify → PR |

### The `feature-dev` Loop Detail

The `implement` step is where the Ralph Loop is most visible — the developer agent runs in a fresh OpenClaw session for each story, maintaining the same stateless-but-iterative contract. The `verifier` agent then checks each story in another fresh session before the pipeline continues.

### State Tracking

Antfarm uses SQLite (not the filesystem) for run state. The `progress.txt` pattern is retained within individual agent workspaces, but cross-agent coordination is managed via the database.

### Commands

```bash
antfarm workflow run feature-dev "Add user authentication with OAuth"
antfarm workflow status "OAuth"
antfarm workflow resume <run-id>   # Resume failed run
antfarm dashboard                   # Web UI on port 3333
antfarm logs [<lines>]             # Recent log entries
```

### Requirements

- Node.js ≥ 22
- OpenClaw v2026.2.9+
- `gh` CLI (for PR creation steps)
- MIT license

---

## 8. OpenClaw: The Runtime Antfarm Runs On

**Website:** https://docs.openclaw.ai  
**Description:** A self-hosted gateway that connects messaging apps (WhatsApp, Telegram, Discord, iMessage) to AI coding agents.

### What OpenClaw Is

OpenClaw is a **self-hosted AI agent runtime** with a gateway layer. Its core value proposition is running AI agents accessible from messaging apps, not from a terminal.

From docs:
> "OpenClaw is a self-hosted gateway that connects your favorite chat apps — WhatsApp, Telegram, Discord, iMessage — to AI coding agents like Pi."

### Architecture Relevant to Antfarm

The embedded agent runtime is built on the **Pi agent core** (models, tools, prompt pipeline). OpenClaw adds:
- Session management
- Multi-channel routing
- Tool wiring
- Workspace isolation

**Key workspace files OpenClaw uses:**
- `AGENTS.md` — operating instructions + "memory"
- `SOUL.md` — persona, boundaries, tone
- `IDENTITY.md` — agent name/vibe/emoji
- `TOOLS.md` — user-maintained tool notes
- `BOOTSTRAP.md` — one-time first-run ritual
- `USER.md` — user profile

These are injected into agent context on the first turn of each new session. This is the mechanism Antfarm uses to provision agent personas — it writes these files to each agent's workspace directory before the session starts.

### Why Antfarm Uses OpenClaw

Antfarm needs to spawn **isolated agent sessions** with fresh context for each step. OpenClaw provides:
1. Session isolation (each step = new session = fresh context)
2. Workspace isolation per agent (separate `AGENTS.md`, `SOUL.md`, etc.)
3. Cron-based polling for workflow coordination
4. Tool access control per role

Antfarm uses OpenClaw's cron tool for workflow orchestration and falls back to the `openclaw` CLI if the cron tool is unavailable (for older versions).

### Relationship Summary

```
Geoffrey Huntley → Ralph (the pattern/technique)
Ryan Carson → snarktank/ralph (Ralph Loop implementation)
Ryan Carson → snarktank/antfarm (Ralph Loop → multi-agent via OpenClaw)
Pi agent core → OpenClaw runtime (powers Antfarm's agent sessions)
```

---

## 9. Other Implementations of the Pattern

### 9.1 syuya2036/ralph-loop (Agent-Agnostic)

**Repo:** https://github.com/syuya2036/ralph-loop  
**Key difference:** The agent CLI is passed as a **runtime argument** rather than hardcoded.

```bash
# Usage
./ralph-loop/ralph.sh "<YOUR_AGENT_COMMAND>" [MAX_ITERATIONS]

# Examples
./ralph-loop/ralph.sh "claude --dangerously-skip-permissions" 20
./ralph-loop/ralph.sh "codex exec --full-auto" 20
./ralph-loop/ralph.sh "gemini --yolo" 20
./ralph-loop/ralph.sh "qwen" 20
```

This repo uses the same `prd.json` + `progress.txt` + `prompt.md` structure as snarktank/ralph but supports **any agent CLI** that accepts stdin. The prompt template is the same stateless pattern.

**Memory channels used:** Git history, `progress.txt`, `prd.json` (no `AGENTS.md` mention — may be agent-dependent).

### 9.2 ralph-cli.dev (Different CLI Tool, Different Schema)

**Website:** https://ralph-cli.dev  
**Key difference:** This is a **separate CLI tool** (not the bash script). Uses a **Markdown checkbox format** instead of JSON for task state.

Task states use plaintext indicators:
```
[✓] 1. Set up Express server with TypeScript
[→] 4. Implement user signup endpoint   ← in progress
[ ] 5. Implement user login endpoint    ← pending
```

Commands:
```bash
ralph task list
ralph task current
ralph task done 4
ralph task add --title "Add rate limiting"
```

> **Note:** ralph-cli.dev appears to be a separate project, not directly by Ryan Carson. The task format is distinct from snarktank/ralph's `prd.json`.

### 9.3 Claude Code Marketplace Plugin

**URL:** https://claude.com/plugins/ralph-loop  

A Claude Code plugin version that:
- Intercepts session exits via a **stop hook**
- Automatically re-feeds the prompt while preserving file modifications and git history
- This is the "plugin" approach vs. the external bash loop approach

> **Observation:** Ryan Carson explicitly posted a video saying the Claude Code plugin approach is not the optimal way to implement Ralph (he calls it out as "nah"). The bash loop is preferred for reliability and portability.

### 9.4 Geoffrey Huntley's CURSED Project

**What it is:** Huntley's original real-world application of Ralph — building a novel programming language ("CURSED") entirely via the Ralph Loop. Used as a live proving ground for the pattern.

Notable characteristics from Huntley's blog:
- Uses **subagents** within each iteration (primary context is a scheduler, subagents do the work)
- Specs stored in `specs/` directory (one file per "topic of concern")
- `fix_plan.md` serves as the task/prioritization file (less rigid than `prd.json`)
- Parallelism control: many subagents for file I/O, only 1 subagent for build/test
- Backpressure via Rust's type system

### 9.5 ai-dev-tasks (Human-in-the-Loop Predecessor)

**Repo:** https://github.com/snarktank/ai-dev-tasks (7,500+ stars)  
**Author:** Ryan Carson  
**Key difference:** **Not autonomous** — human approves each task before the AI moves to the next.

Uses markdown prompt files (`create-prd.md`, `generate-tasks.md`) that guide AI to create task lists with human review gates. Ralph is the fully autonomous evolution of this approach.

### 9.6 Clayton Farr's Ralph Playbook

**URL:** https://claytonfarr.github.io/ralph-playbook/  
**What it is:** A detailed synthesis of Huntley's approach into a three-phase framework.

#### Three Phases

| Phase | Name | Purpose | Iterations |
|-------|------|---------|------------|
| 1 | Define Requirements | Conversation → specs → one spec file per "topic of concern" | Pre-loop |
| 2 | Planning Loop | Gap analysis (specs vs. code) → `IMPLEMENTATION_PLAN.md` | 1–2 iterations |
| 3 | Building Loop | Implement from plan → test → commit | N iterations |

#### Key Insight from Farr

Uses `IMPLEMENTATION_PLAN.md` (Markdown) rather than `prd.json` (JSON):
> "Prefer Markdown over JSON — for better token efficiency."

Within each building iteration:
```
Orient → Read plan → Select task → Investigate → Implement 
→ Validate (1 subagent only for builds/tests) → Update plan → Update AGENTS.md → Commit
```

The "1 subagent for validation" rule directly reflects Huntley's guidance about preventing bad form back pressure.

---

## 10. Comparison Table

| Implementation | Task Format | Agent Support | Context Reset | Multi-Agent | State Backend | Autonomy |
|---------------|-------------|---------------|---------------|-------------|---------------|---------|
| **snarktank/ralph** | `prd.json` (JSON) | Amp, Claude Code | Process exit (bash loop) | No | Filesystem | Full |
| **syuya2036/ralph-loop** | `prd.json` (JSON) | Any CLI (stdin) | Process exit (bash loop) | No | Filesystem | Full |
| **Antfarm** | YAML workflow | OpenClaw (any model) | Session end (OpenClaw) | Yes (5–7 agents) | SQLite + filesystem | Full |
| **ralph-cli.dev** | Markdown checkboxes | Unknown | Unknown | No | Filesystem | Full |
| **Ralph Loop Plugin** | Prompt file | Claude Code only | Stop hook re-feed | No | Filesystem | Full |
| **ai-dev-tasks** | Markdown PRD | Any AI IDE | Manual (human gate) | No | Filesystem | Human-in-loop |
| **Huntley/CURSED** | `fix_plan.md` (Markdown) | Claude Code | Process exit (bash loop) | Via subagents | Filesystem | Full |

---

## 11. Open Questions and Gaps

1. **`notes` field usage:** The `notes` field in `prd.json` stories is present but the reference implementation doesn't explicitly instruct the agent to populate it. It may be populated by the `/ralph` skill during PRD-to-JSON conversion. *Not directly observed in practice.*

2. **Branch checkout logic:** The instruction says "Check you're on the correct branch from PRD `branchName`. If not, check it out or create from main." The exact git commands are left to the agent. *Inferred from instructions, not directly observed in execution.*

3. **`ralph.sh` doesn't parse `prd.json` for completion:** The shell script relies entirely on `<promise>COMPLETE</promise>` in stdout. If the agent crashes or produces no output, the loop simply continues to the next iteration. There is no `jq` check of `passes` fields in `ralph.sh`. *Directly observed.*

4. **`progress.txt` growth over time:** The file is append-only and could grow very large over many iterations. There is no compaction or summarization mechanism in the reference implementation. *Observed as a gap, not addressed in documentation.*

5. **Antfarm's SQLite schema:** The internal database schema for Antfarm is not publicly documented in detail. The relationship between SQLite state and `progress.txt`/`prd.json` within each agent workspace is unclear. *Inferred from high-level description.*

6. **OpenClaw versioning sensitivity:** Antfarm requires OpenClaw v2026.2.9+ for the cron tool. Older versions require the `openclaw` CLI fallback. This suggests tight version coupling. *Directly observed in README.*

7. **ralph-cli.dev project origins:** This appears to be a separate project from snarktank/ralph with a different task format. Its relationship to the original Ralph pattern is unclear — it may be inspired by the pattern but not a direct implementation.

---

## 12. Key Findings Summary

1. **The core pattern** is a bash `for` loop that spawns a fresh CLI agent process on every iteration. Context reset = process exit. No daemon, no IPC, no flags.

2. **`prd.json` is the canonical task format** in snarktank/ralph. The schema has 7 story-level fields. The only status field is binary `passes: boolean`. There is no "in-progress" state.

3. **Selection algorithm is simple:** pick the lowest `priority` integer where `passes: false`. No dependency graph, no topological sort — order is expressed via the `priority` field values.

4. **The four memory channels** are: git history (code changes), `progress.txt` (append-only learnings log), `prd.json` (task state), and `AGENTS.md` files (module-specific knowledge). Git history and `AGENTS.md` are auto-read by both Amp and Claude Code without explicit instructions.

5. **Antfarm scales Ralph to multi-agent** using OpenClaw as the session runtime, SQLite for coordination state, and YAML for workflow definitions. The Ralph Loop pattern is preserved at the step level — each agent step runs in a fresh session.

6. **OpenClaw** is a self-hosted AI gateway built on the Pi agent core. It provides the session isolation, workspace management, and tool wiring that Antfarm needs for multi-agent orchestration.

7. **Quality gates are the backpressure mechanism.** If checks fail, the agent makes zero state changes — no commit, no `passes: true`, no `progress.txt` update. The same story is retried in the next iteration.

8. **Completion signaling** uses the string `<promise>COMPLETE</promise>` in the agent's stdout, detected by `grep -q` in the shell script. The shell never parses `prd.json` to check completion.

9. **The pattern generalizes** to other agents (syuya2036/ralph-loop supports Claude, Codex, Gemini, Qwen). The core loop mechanism is the same; only the agent invocation command changes.

10. **Geoffrey Huntley's original insight** remains the key principle: keep the main context as a scheduler, use subagents for expensive work, run build/tests with only 1 subagent to prevent back pressure, and tune via prompts/specs rather than code.

---

*Research conducted: 2026-03-29. Sources: directly observed (raw source files fetched from GitHub), deepwiki analysis, ghuntley.com blog, rywalker.com research, docs.openclaw.ai.*
