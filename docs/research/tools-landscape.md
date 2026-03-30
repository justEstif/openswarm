# Local Agent Orchestration Tools — Landscape Report

> **Generated:** 2026-03-29  
> **Purpose:** Inform design of `swarm` — a unified harness-agnostic CLI (`swarm agent`, `swarm msg`, `swarm task`, `swarm pane`, `swarm events`) with file-backed `.swarm/` storage and multi-multiplexer support.  
> **Method:** Each tool researched via live repo + defuddle-parsed README/docs, with parallel subagents.

---

## Table of Contents

1. [Conductor (Melty Labs)](#1-conductor-melty-labs)
2. [Vibe Kanban (Bloop AI)](#2-vibe-kanban-bloop-ai)
3. [Claude Squad (smtg-ai)](#3-claude-squad-smtg-ai)
4. [Gastown (steveyegge)](#4-gastown-steveyegge)
5. [workmux (raine)](#5-workmux-raine)
6. [NTM — Named Tmux Manager (Dicklesworthstone)](#6-ntm--named-tmux-manager)
7. [dmux (standardagents)](#7-dmux-standardagents)
8. [New Tools Discovered: 2025–2026](#8-new-tools-discovered-20252026)
9. [Cross-Tool Comparison Table](#9-cross-tool-comparison-table)
10. [Key Design Lessons for swarm](#10-key-design-lessons-for-swarm)

---

## 1. Conductor (Melty Labs)

**Repo / Source:** https://github.com/meltylabs/conductor-releases (releases only; source is closed)  
**Website:** https://www.conductor.build/  
**Docs:** https://docs.conductor.build/  
**Stack:** Closed-source native Mac app (Apple Silicon). Company: Melty Labs (YC S24).  
**Distribution:** Mac-only DMG download.

### Problem Solved

Developers running Claude Code or Codex hit a single-threaded bottleneck: one agent on one task at a time. Conductor provides a visual dashboard to run multiple agents in parallel, each isolated in its own git worktree, with a built-in diff viewer and merge workflow.

### Key Primitives

| Primitive | Description |
|-----------|-------------|
| **Workspace** | One git worktree + one agent session (⌘+N). |
| **conductor.json** | Repo-committed config: `{ scripts: { setup, run, archive }, runScriptMode, enterpriseDataPrivacy }` |
| **Scripts** | `setup` / `run` / `archive` hooks; personal or shared. |
| **Checkpoints** | Automatic git snapshots per workspace, enabling rollback. |
| **Diff Viewer** | Side-by-side diff before merge. |
| **Spotlight Testing** | Sync workspace changes to main checkout for integration testing without merging. |
| **Todos** | Agent-surfaced task list in the dashboard. |
| **MCP support** | Inbound: configure MCP servers for agents. |
| **Multi-model mode** | Run same prompt with Claude + Codex simultaneously for comparison. |
| **Parallel agents** | Multiple workspaces via ⌘+N. |

**Auth:** Re-uses existing Claude Code / Codex credentials. No separate billing layer.  
**GitHub integration:** Fine-grained GitHub App (migrated from broad OAuth after backlash); also supports `git`/`gh` CLI.  
**Privacy:** Chat stays local; PostHog analytics; `enterpriseDataPrivacy: true` disables all telemetry.

### Multiplexer Support

**None.** Conductor is a native Mac GUI. Isolation is via git worktrees; the GUI is its own multiplexing layer. No tmux, no zellij.

### Reusable / Lessons

1. **git worktrees are the canonical isolation primitive** — every serious tool in this space converges here.
2. **conductor.json manifest** — a repo-committed bootstrap config is an elegant pattern. `swarm` should have a similar `.swarm/config.yaml`.
3. **"Who needs attention?" model** — surface agents that are blocked or waiting, not just running.
4. **Checkpoint/snapshot model** — rollback points inside long-running sessions reduce the cost of agent mistakes.
5. **Multi-model comparison mode** — run competing models on the same task; let the human pick.
6. **Re-use existing auth** — never add a billing layer; piggyback on agent credentials.
7. **Security lesson** — broad OAuth caused immediate backlash. Fine-grained permissions from day one.

### Gap Left

- **Mac-only, closed-source** — Linux/Windows users and teams needing source transparency are excluded.
- **No terminal multiplexer bridge** — CLI-native developers can't use it without adopting the GUI.
- **No task/issue tracking** — no kanban, no structured work planning; tasks are ad-hoc prompts.
- **No cross-workspace coordination** — agents are fully independent; no supervisor, no DAG dependencies.
- **No CI/CD hooks** — no trigger from issues/PRs or push to CI.
- **Context amnesia** — no persistent memory across sessions.
- **GitHub-centric** — requires GitHub remote; no local-only repos.

---

## 2. Vibe Kanban (Bloop AI)

**Repo:** https://github.com/BloopAI/vibe-kanban (open-source, Bloop AI)  
**Website:** https://vibekanban.com/  
**Stack:** Rust backend + React frontend. `npx vibe-kanban` → localhost browser UI.  
**License:** MIT-adjacent.

### Problem Solved

*"In a world where engineers spend most of their time planning and reviewing coding agents, the most impactful way to ship more is to get faster at planning and review."*

Vibe Kanban adds a **full workflow layer** on top of CLI coding agents: kanban-based task planning, agent-agnostic execution, a diff/review panel with inline comment-back-to-agent, and a built-in browser preview — all in one browser interface.

### Key Primitives

| Primitive | Description |
|-----------|-------------|
| **Issue** | Kanban work unit: title, description, priority, status, tags, parent/sub-issues. |
| **Workspace** | 1 git worktree + 1 branch + 1 terminal + 1 dev server per task. |
| **Session** | A conversation thread with a specific agent inside a workspace. Multiple sessions per workspace. |
| **Repository** | A git repo registered in a project. |
| **Project** | Container grouping related repositories. |
| **Changes Panel** | Syntax-highlighted diff with inline comment → routed back to agent as follow-up. |
| **Browser Preview** | Built-in browser with devtools, inspect mode, device emulation. |
| **MCP Server (outgoing)** | Agents inside workspaces can connect to external MCP servers. |
| **MCP Server (incoming)** | `npx vibe-kanban --mcp` — exposes a local MCP server so external agents can manage issues/workspaces via MCP tools. |
| **PR creation** | Creates GitHub/Azure PRs with AI-generated descriptions. |

**Supported agents (10+):** Claude Code, OpenAI Codex, GitHub Copilot, Gemini CLI, Amp, Cursor Agent CLI, OpenCode, Factory Droid, CCR, Qwen Code.

**Backend (Rust crates):** `worktree-manager`, `workspace-manager`, `git`, `utils`.  
**Worktree storage:** macOS: persistent `~/.../vibe-kanban/worktrees`; Linux: tmpfs; Windows: temp dir.  
**Remote access:** Tunnel mode (`VK_TUNNEL` / Cloudflare Tunnel) — no tmux SSH fallback.

### Multiplexer Support

**None.** Entirely browser-based. Each agent session is a managed subprocess (pty) in the Rust backend. An integrated terminal is embedded in the browser UI; no external multiplexer required.

### Reusable / Lessons

1. **Kanban issues as the coordination primitive** — issues map 1:1 to workspaces; lifecycle (backlog → in-progress → done) mirrors workspace states. This is the right model for `swarm task`.
2. **MCP as two-way integration bus** — exposing a local MCP server allows meta-orchestration (agents managing agents via MCP). Design `swarm` to expose an MCP server surface.
3. **Agent-agnostic from day one** — abstracting the agent as a configurable subprocess means zero-fork support for the entire ecosystem.
4. **Inline diff comment → agent feedback loop** — comments on specific diff lines are routed back to the agent as follow-up instructions. The shortest possible review cycle.
5. **Multi-repo workspaces** — multiple repos in one workspace (each with its own worktree) handles full-stack monorepo-split projects.
6. **Rust backend for reliability** — worktree/process management in Rust; cross-platform (macOS + Linux + Windows).
7. **"Doomscrolling" mental model** — reviewing agent outputs should be fast and structured, like a feed.

### Gap Left

- **No terminal-native / TUI mode** — browser-only; developers who prefer staying in the terminal are not served.
- **No cross-agent coordination / supervisor model** — no "planner agent routes tasks to worker agents"; human plans manually.
- **Issues are manually created** — no auto-decomposition from high-level spec into sub-tasks.
- **No checkpoint/rollback inside sessions** — git history is the only rollback path.
- **No structured agent output validation** — no schema enforcement, test gates, or quality checks in the workspace lifecycle.
- **No DAG-style task dependencies** — if Task B depends on Task A, sequencing is manual.

---

## 3. Claude Squad (smtg-ai)

**Repo:** https://github.com/smtg-ai/claude-squad  
**Docs:** https://smtg-ai.github.io/claude-squad/  
**Stack:** Go single binary (`cs`). License: AGPL-3.0.

### Problem Solved

Let a human developer supervise multiple AI coding agents from a single terminal TUI. Each agent gets an isolated git worktree; the operator watches diffs live, attaches to reprompt, commits, or kills sessions without leaving the TUI. Core value: **human-in-the-loop multi-agent review dashboard**.

### Key Primitives

**CLI commands:**
```
cs                   # Launch TUI (default: claude)
cs -p "codex"        # Launch with alternate agent
cs -y                # Yolo mode (auto-accept all agent prompts)
cs reset             # Wipe all stored instances
cs version / debug
```

**TUI keybindings:**

| Key | Action |
|-----|--------|
| `n` | New session |
| `N` | New session with prompt |
| `D` | Kill session |
| `↵ / o` | Attach (reprompt) |
| `ctrl-q` | Detach |
| `s` | Commit + push branch |
| `c` | Commit + pause |
| `r` | Resume paused session |
| `tab` | Toggle preview/diff |
| `q` | Quit |

**Config:** `~/.claude-squad/config.json`
```json
{
  "default_program": "claude",
  "profiles": [
    { "name": "claude",  "program": "claude" },
    { "name": "codex",   "program": "codex" },
    { "name": "aider",   "program": "aider --model ollama_chat/gemma3:1b" }
  ]
}
```

**Spawn mechanics:** Creates tmux session + git worktree per agent instance; runs configured `program` inside the pane; reads git diff for live preview.

**Prerequisites:** `tmux`, `gh`.

### Multiplexer Support

**tmux only.** No zellij, no screen. A Claude Code upstream issue (#31901) explicitly requests zellij support, confirming the ecosystem is currently tmux-centric.

### Reusable / Lessons

1. **Profile system** in config.json — `name + program string` covers every agent type cleanly.
2. **Diff-first review UX** — agents work in background; human reviews before committing. Strong safety primitive.
3. **`-y` autoyes flag** — toggleable auto-accept mode at the session level. Useful for unattended runs.
4. **Minimal scope** — does exactly one thing (TUI agent dashboard) well; no coordinator overhead.
5. **~6 MB Go binary** — zero runtime deps beyond tmux/gh.

### Gap Left

- **No orchestration** — sessions are created manually; no automated dispatch, no agent-to-agent messaging.
- **No persistent state** beyond git — session metadata lost on exit; no audit trail, no task ledger.
- **No task queue or work assignment** — no work items, no backlog, no priority.
- **tmux hard dependency** — zellij, WezTerm, Ghostty unsupported.
- **No programmatic/headless API** — no way for another agent to spawn a `cs` session.
- **No watchdog** — stalled/crashed agents require human detection via TUI.
- **Flat agent structure** — all sessions are peers; no coordinator → worker hierarchy.

---

## 4. Gastown (steveyegge)

**Repo:** https://github.com/steveyegge/gastown  
**Beads (standalone):** https://github.com/steveyegge/beads  
**Website:** https://gastown.dev/  
**Stack:** Go. Binaries: `gt` (Gastown), `bd` (Beads). Author: Steve Yegge.

### Problem Solved

When you run 4–10+ AI agents simultaneously, they lose state on restart, have no shared work ledger, and coordination becomes manual chaos. Gastown provides: persistent agent identity, git-backed work state, structured task assignment (Beads + Convoys), automated watchdogs (Witness/Deacon/Dogs), and a Mayor coordinator agent — enabling reliable 20–30 agent workflows. Core value: **fully automated, self-healing multi-agent orchestration with persistent state**.

### Key Primitives

**`gt` CLI commands (orchestration layer):**
```bash
gt install ~/gt --git          # Initialize town workspace
gt rig add <name> <repo-url>   # Register a project
gt crew add <name> --rig <rig> # Create human workspace
gt mayor attach                # Start/attach to Mayor coordinator
gt sling <bead-id> <rig>       # Assign work bead to an agent rig
gt convoy create "Name" id1 id2 --notify
gt done                        # Agent signals completion
gt escalate                    # Agent signals blocker
gt seance --talk <id> -p "..."  # Query predecessor agent session
```

**`bd` CLI commands (Beads — standalone, composable):**
```bash
bd init                         # Initialize in a project
bd create "Title" -p 0          # Create P0 task
bd ready                        # List tasks with no open blockers (JSON)
bd update <id> --claim          # Atomically claim a task
bd dep add <child> <parent>     # Link tasks (blocks/related/parent-child)
bd show <id>                    # View task + audit trail
bd close <id> "reason"          # Mark complete
bd list / bd search --query ""
```

**Bead ID format:** `<prefix>-<5char-alphanumeric>` (e.g., `gt-abc12`). Prefix routes to the correct rig's Dolt database. Hierarchical: `bd-a3f8` (Epic) → `bd-a3f8.1` (Task) → `bd-a3f8.1.1` (Sub-task).

**Storage backend:** [Dolt](https://github.com/dolthub/dolt) — versioned SQL, Git-compatible. Embedded (`.beads/embeddeddolt/`) or server mode (port 3307).

**Workspace layout:**
```
~/gt/
├── .dolt-data/              # Dolt SQL database
├── settings/config.json     # Agent config (name → command)
├── <rig-name>/
│   ├── polecats/<name>/     # Worker agent git worktrees
│   ├── crew/<name>/         # Human workspace
│   └── hooks/               # Persistent git-worktree storage
└── daemon.log
```

**Agent hierarchy (all run as tmux sessions):**

| Role | Description |
|------|-------------|
| **Mayor** | Global coordinator. Human briefs it; it creates convoys and dispatches beads. |
| **Deacon** | Town-level daemon watchdog; runs patrol cycles across all rigs. |
| **Witness** | Per-rig lifecycle manager; detects stuck agents (GUPP violations). |
| **Refinery** | Per-rig merge queue; Bors-style bisecting merge with verification gates. |
| **Polecats** | Ephemeral worker sessions with persistent identity + history. |
| **Crew** | Human-managed workspaces (full clones, not worktrees). |

**Session continuity (Seance):** Agents log to `.events.jsonl`. `gt seance --talk <id>` lets a new agent query a predecessor session's history for decisions. Lightweight and crash-safe.

**Workflow templates (Molecules):** TOML formulas in `internal/formula/formulas/*.formula.toml`; instantiated as Molecules with tracked steps.

**Prerequisites:** Go 1.25+, Git 2.25+, Dolt 1.82.4+, beads (`bd`) 0.55.4+, sqlite3, tmux 3.0+, Claude Code CLI.

### Multiplexer Support

**tmux only.** `internal/tmux/tmux.go` wraps all lifecycle. No zellij support.

### Reusable / Lessons

1. **Beads (`bd`) is a standalone, composable tool** — installable independently. Brings Dolt-backed dependency-aware task graphs, atomic claim, audit trail, and JSON output to any project. **Directly reusable without Gastown.**
2. **Bead ID routing by prefix** — the ID encodes its destination rig. Clean multi-repo dispatch primitive.
3. **ZFC Principle (Zero Fragile Code)** — Go/infrastructure layer does data transport; LLM agents make decisions. No parsing stderr for branching logic. Core principle for `swarm`.
4. **Propulsion Principle** — agents must execute immediately on bead discovery; no human confirmation in the hot path. Key to scaling.
5. **Persistent agent identity** — polecats have names, history, current assignment in Dolt even across restarts. Solves the "blank slate" problem.
6. **Three-tier watchdog pattern** (Witness → Deacon → Dogs) — per-unit monitor → cross-unit supervisor → dispatched maintenance workers.
7. **Convoy pattern** — bundle related beads into a trackable unit (sprint/epic) for progress reporting.
8. **Seance (`.events.jsonl`)** — any agent can query predecessor context without shared memory.

### Gap Left

- **Heavy stack** — requires Dolt (full versioned SQL), Go 1.25+, tmux, beads; significant install burden.
- **tmux hard dependency** — same constraint as Claude Squad.
- **Mayor requires human briefing** — not fully autonomous at the top level.
- **Tightly coupled to Claude Code** — other runtimes are second-class.
- **Formulas are binary-embedded** — not user-extensible without recompiling.
- **Complexity cliff** — 0 → working Mayor requires ~8 setup steps across 4 tools.
- **No per-bead agent routing** — all polecats in a rig run the same agent command; no "some tasks go to claude, some go to codex" per bead.

---

## 5. workmux (raine)

**Repo:** https://github.com/raine/workmux  
**Docs:** https://workmux.raine.dev/guide/  
**Stack:** Rust. ~1.1k stars. Very active (weekly releases as of March 2026).  
**Multiplexer source:** `src/multiplexer/` (mod.rs, tmux.rs, wezterm.rs, kitty.rs, zellij.rs, …)

### Problem Solved

When running multiple AI agents on different tasks, each agent needs its own branch, directory, and terminal window. Doing this manually requires ~10 commands per feature: `git worktree add`, `tmux new-window`, split panes, copy `.env`, link `node_modules`, run setup hooks, etc. workmux reduces this to one command.

`workmux add <branch>` creates: git worktree + matching terminal window + configured pane layout + copied config files + post-create hooks.  
`workmux merge [branch]` tears it all down: merges branch, kills window, removes worktree.

### Key Primitives

**Commands:**

| Command | Description |
|---------|-------------|
| `workmux add <branch>` | Create worktree + window; optionally spawn agents with prompts |
| `workmux merge [branch]` | Merge branch into main, clean up everything |
| `workmux remove [name]` | Remove without merging |
| `workmux list` | List worktrees with agent/mux/merge status |
| `workmux open [name]` | Open/switch to window for existing worktree |
| `workmux dashboard` | Full-screen TUI: agents across sessions, diff view, patch mode |
| `workmux sidebar` | Toggle live agent status sidebar (tmux-only) |
| `workmux coordinator send/status/digest` | Multi-agent coordination |
| `workmux sandbox {pull,build,shell,agent,stop,prune}` | Sandbox management |

**Config (`.workmux.yaml`):**
```yaml
panes:
  - command: <agent>
    focus: true
  - split: horizontal
    size: 20
    command: nvim

agents:
  claude: "claude --dangerously-skip-permissions"

post_create:
  - npm install

files:
  copy: [.env]
  symlink: [node_modules]

mode: session   # or "window"
merge_strategy: rebase
```

**Prompt / Agent Features:**
- `workmux add <branch> -p "inline prompt"` — inject prompt at spawn
- `workmux add <branch> -P task.md` — read prompt from file with template vars
- `--foreach "platform:iOS,Android"` — matrix expansion; one worktree per combo
- `-a claude -a gemini` — agent-specific worktrees
- JSON lines: `gh repo list --json url,name | workmux add analyze ...`

### Backend Detection Mechanism

**Source:** `src/multiplexer/mod.rs` — `detect_backend()` function

**Priority cascade (exact, from source):**

| Priority | Env Var | Backend | Notes |
|----------|---------|---------|-------|
| 0 (override) | `$WORKMUX_BACKEND` | any | Accepts: `tmux`, `wezterm`, `kitty`, `zellij` |
| 1 | `$TMUX` | tmux | Set by tmux to socket path |
| 2 | `$WEZTERM_PANE` | WezTerm | Set by WezTerm to pane ID |
| 3 | `$ZELLIJ` | Zellij | Set by Zellij session UUID |
| 4 | `$KITTY_WINDOW_ID` | Kitty | Set by Kitty to window ID |
| 5 (fallback) | none | tmux | Backward-compatible default |

**Key design decisions:**
- Session-specific vars first (`$TMUX`, `$WEZTERM_PANE`); only set when *inside* that multiplexer, unlike `$KITTY_WINDOW_ID` which is inherited by child processes. Running tmux inside kitty → `$TMUX` is set → correctly picks tmux.
- `resolve_backend()` is a pure function, fully unit-tested.
- Tests cover: no env, single env, tmux-inside-kitty, tmux-inside-wezterm, tmux-inside-zellij, all-vars-set.

### Multiplexer Support

| Backend | Status | Detection | Key Limitations |
|---------|--------|-----------|-----------------|
| **tmux** | Primary / Full | `$TMUX` | None — all features supported |
| **WezTerm** | Experimental | `$WEZTERM_PANE` | No agent status in tabs; Windows unsupported |
| **Zellij** | Experimental | `$ZELLIJ` | Requires unreleased Zellij features; no session mode; 50/50 splits only; no dashboard |
| **kitty** | Experimental | `$KITTY_WINDOW_ID` | Requires `allow_remote_control` + `listen_on` |

**Multiplexer trait** (`src/multiplexer/mod.rs`): ~40+ methods including `create_window`, `split_pane`, `send_keys`, `capture_pane`, `set_status`, `create_handshake`, `setup_panes`.

### Reusable / Lessons

1. **Backend detection pattern** — the 5-level env-var cascade is the canonical multi-multiplexer detection approach. **`swarm` should adopt this exactly**, with `$SWARM_BACKEND` as the override.
2. **Trait-based multiplexer abstraction** — backend-specific details isolated; orchestration logic is shared.
3. **Handshake pattern** (`handshake.rs`) — named-pipe sync for shell startup before injecting commands; solves race conditions.
4. **`setup_panes()` default impl** — handles the full spawn flow; backends only implement primitives.
5. **State as filesystem JSON** — moved from tmux-specific state to filesystem JSON for multi-backend compatibility. `swarm` should do the same (`.swarm/` directory).
6. **Matrix/foreach spawning** — batch-spawning agents on parameter permutations is relevant for `swarm task` bulk operations.

### Gap Left

- **git-worktree-centric** — the abstraction leaks; not appropriate as a pure "swarm engine" without git repos.
- **Zellij requires unreleased features** — `--pane-id` targeting, `close-tab-by-id`, etc. not in any released Zellij as of March 2026. Effectively unusable today.
- **No agent-to-agent coordination** — manages windows; doesn't route messages between agents.
- **No programmatic API** — no HTTP, no IPC beyond tmux.
- **Status tracking requires hook installation** — can fail silently.
- **tmux-only for full features** — session mode, sidebar, status icons all tmux-only.

---

## 6. NTM — Named Tmux Manager

**Repo:** https://github.com/Dicklesworthstone/ntm  
**Stack:** Go (1.25+). License: MIT + additional rider. Single author; no external contributions accepted.  
**Install:** `brew install dicklesworthstone/tap/ntm` or curl installer.

### Problem Solved

Turns tmux into a **local control plane for multi-agent software development**. Adds durable coordination, work selection, safety policy, approvals, history, shared human/agent control, and machine-readable API surfaces on top of raw tmux sessions.

### Key Primitives

**Session lifecycle:**
```bash
ntm quick api --template=go         # Scaffold project + agents
ntm spawn api --cc=2 --cod=1 --gmi=1  # 2 Claude + 1 Codex + 1 Gemini
ntm add api --cc=1                  # Add agents to existing session
ntm status api / ntm view api / ntm attach api / ntm kill api
```

**Work dispatch:**
```bash
ntm send api --cc "Implement auth"      # Send to all Claude panes
ntm send api --all "Checkpoint"         # Broadcast to all agents
ntm interrupt api                       # Ctrl+C to all agents
ntm watch api --cc                      # Stream output
ntm diff api cc_1 cod_1                 # Compare two panes
ntm extract api --lang=go               # Extract code blocks
```

**Robot mode (machine-readable JSON surface):**
```bash
ntm --robot-snapshot          # Full state: sessions + beads + mail
ntm --robot-status            # Session list, agent states
ntm --robot-tail=api          # Recent output
ntm --robot-send=api --msg="..." --type=claude
ntm --robot-spawn=api --spawn-cc=2
```

**REST API (`ntm serve`):**
- REST under `/api/v1`
- Server-Sent Events at `/events`
- WebSocket at `/ws`
- OpenAPI spec at `docs/openapi.json`
- JWT auth via `~/.config/ntm/auth.token`

**Durable state:**
```bash
ntm checkpoint save api -m "pre-migration"
ntm checkpoint restore api
ntm timeline show <session-id>
ntm audit show api
ntm resume api
ntm history search "auth error"
```

**Safety system:**
```bash
ntm safety check -- git reset --hard   # Check if blocked
ntm policy show --all
ntm approve list / ntm approve <id> / ntm approve deny <id>
```

**Work graph (requires `br`/`bv`):**
```bash
ntm work triage                    # Prioritized task list
ntm work next                      # Single best next action
ntm assign api --auto --strategy=dependency
```

### Architecture

```
Human Operator
     |
     NTM (Go binary)
     ├── Session Orchestration    ← named tmux sessions, pane layout
     ├── Dashboard + Palette      ← charmbracelet/bubbletea TUI
     ├── Work Triage + Assignment ← br/bv integration
     ├── Safety + Policy          ← guards, approvals
     ├── Pipelines + Checkpoints  ← durable YAML pipeline state
     └── Robot/REST/WS surfaces   ← --robot-*, /api/v1, /ws, /events
          |
     ┌────┴──────────┐
     │ tmux sessions │   ← Claude / Codex / Gemini panes
     └───────────────┘
```

**Named pane convention:** `<project>__<agent>_<number>` (e.g., `myproject__cc_1`, `myproject__cod_2`).

**Spawn flow:** create session → split window into N panes → apply tiled layout → `tmux send-keys` per pane → label panes → store state in `.ntm/` as JSON.

**Context rotation:** monitors estimated token usage; at 80% warns, at 95% triggers compaction or fresh agent spawn with handoff summary.

### Multiplexer Support

**tmux ONLY.** Explicitly stated. No zellij, no wezterm, no kitty.

### Reusable / Lessons

1. **Robot mode as first-class API** — `--robot-*` flags as machine-readable surface (distinct from TUI) is excellent design. `swarm` needs this: agents controlling agents requires JSON-first APIs, not terminal scraping.
2. **REST + WebSocket** — `ntm serve` makes NTM usable as a local control plane from dashboards, scripts, and agents. This is the right model for `swarm events`.
3. **Named pane convention** — `<project>__<agent>_<number>` is clean and parseable. `swarm pane` should adopt a similar naming scheme.
4. **Context rotation as a first-class primitive** — automatically tracking token usage and rotating agents with handoff summaries prevents silent context-exhaustion failures.
5. **Safety system** — blocking destructive commands via policy rules + approval workflows is the right pattern for autonomous execution.
6. **Checkpoint/resume** — explicit checkpoints + timeline replay enables recovery after crashes.
7. **Work assignment strategies** — `balanced`, `speed`, `quality`, `dependency` — different strategies for different contexts. Worth having in `swarm task assign`.
8. **`ntm deps -v` dependency checker** — verifying all optional integrations are present is great UX for complex tools.

### Gap Left

- **tmux-only** — no zellij, no multi-backend support.
- **Heavy dependency chain** — Agent Mail, `br`, `bv`, `cass`, `dcg`, `pt` each require separate install. Most users get partial value.
- **No git worktree isolation** — agents share the working tree; file conflicts possible.
- **Single repo assumption** — no multi-feature parallel branch isolation.
- **No external contributions** — project can stall if author stops.
- **Context rotation is heuristic-based** — token counting estimated, not exact.

---

## 7. dmux (standardagents)

**Repo:** https://github.com/standardagents/dmux  
**Website:** https://dmux.ai/  
**Stack:** Node.js. `npm install -g dmux`. Authors: Justin Schroeder & Andrew Boyd (FormKit / StandardAgents).

### Problem Solved

Running multiple AI coding agents simultaneously in the same git repo causes file conflicts and context bleed. dmux gives every task its own tmux pane + isolated git worktree + branch, so agents work in parallel without interference. Human manages panes via an interactive TUI; merging back is one keystroke.

### Key Primitives

**Entry point:**
```bash
cd /path/to/project
dmux   # launches TUI
```

**TUI keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `n` | New pane (prompt → pick agent → worktree auto-created) |
| `t` | New plain terminal pane |
| `j` / Enter | Jump into pane |
| `m` | Open pane menu (merge, rename, etc.) |
| `f` | Browse files in pane's worktree |
| `x` | Close pane |
| `p` / `P` | Multi-project pane operations |
| `s` | Settings |
| `q` | Quit |

**Supported agents (multi-select per prompt):** Claude Code, Codex, OpenCode, Cline CLI, Gemini CLI, Qwen CLI, Amp CLI, pi CLI, Cursor CLI, Copilot CLI, Crush CLI.

**Optional:** OpenRouter API key for AI-generated branch names and commit messages.

### Git Worktree Integration

- New pane creation auto-runs `git worktree add` in a fresh directory with a new branch.
- Branch names can be AI-generated (via OpenRouter) or manual.
- Merge: pane menu → **Merge** → auto-commit → merge to main → cleanup worktree.
- **Lifecycle hooks:** `worktree-create`, `pre-merge`, `post-merge` — inject scripts (install deps, run tests) at defined points.

### Multiplexer Support

**tmux only.** tmux 3.0+ required. No zellij, no screen.

### Reusable / Lessons

1. **Worktree-per-task as the atomic primitive** — zero shared state between agents at the filesystem level.
2. **Multi-select agent launch** — user picks which agents run on a prompt simultaneously; useful for comparison/parallelism.
3. **Pane visibility controls** — hide/show/isolate; useful for focus.
4. **Multi-project in one session** — add multiple repos; handles cross-repo tasks.
5. **Lifecycle hooks** — lightweight escape hatch for custom scripts at defined lifecycle points.
6. **AI naming** — branch/commit message generation via OpenRouter is a small but meaningful UX improvement.

### Gap Left

- **No automated coordination** — completely human-driven; no supervisor agent, no task queue, no dependency graph.
- **No CI reaction** — PR CI failures not routed back to agents automatically.
- **No programmatic / API surface** — no REST, no socket, no way to drive dmux from another agent or script.
- **No inter-agent communication** — only the human can broker information between agents.
- **tmux lock-in** — no zellij, no Docker/K8s for remote/headless scenarios.
- **No status/health tracking** — no dashboard showing agent state, tokens used, whether an agent is stuck.

---

## 8. New Tools Discovered: 2025–2026

### CAO — CLI Agent Orchestrator (AWS Labs)

**Repo:** https://github.com/awslabs/cli-agent-orchestrator  
**Install:** `uv tool install git+https://github.com/awslabs/cli-agent-orchestrator.git@main`  
**Multiplexer:** tmux 3.3+ (required)

**What it is:** Lightweight Python orchestration system for hierarchical multi-agent sessions in tmux. A supervisor agent coordinates work and delegates to specialized worker agents via an MCP server for inter-agent messaging.

**Key innovations:**
- **MCP server as the inter-agent message bus** — agents communicate via MCP rather than filesystem conventions.
- **Three orchestration patterns:** Handoff (synchronous), Assign (async/parallel), Send Message (direct).
- **Agent profiles as markdown files** — define agent roles in `.md`; installable from local file or URL (`cao install ./my-agent.md`).
- **Flow scheduling** — cron-like scheduling for automated/unattended workflow execution.
- **Agent-agnostic:** Kiro, Claude Code, Codex, Gemini CLI, Kimi CLI, GitHub Copilot, Q CLI.

**CLI:** `cao install`, `cao launch`, `cao-server`, `cao run-flow`

---

### Composio Agent Orchestrator (`ao`)

**Repo:** https://github.com/ComposioHQ/agent-orchestrator  
**Install:** `npm install -g @composio/ao`  
**Multiplexer:** tmux (default); Docker, K8s, process runtimes swappable via plugin.

**What it is:** Fleet management for parallel AI coding agents. Each issue/task gets its own agent, git worktree, branch, and PR. CI failures and review comments are automatically routed back to the responsible agent.

**Key innovations:**
- **Reaction system** — `ci-failed → send-to-agent`, `changes-requested → send-to-agent`, `approved-and-green → auto-merge`. CI becomes a feedback loop, not just a gate.
- **8-slot plugin architecture** — Runtime, Agent, Workspace, Tracker, SCM, Notifier, Terminal, Lifecycle are all swappable TypeScript interfaces.
- **Agent-agnostic + tracker-agnostic** — Claude Code, Codex, Aider; GitHub, Linear.
- **YAML config:** `agent-orchestrator.yaml` auto-generated by `ao start`, editable.
- **`ao start <url>`** — single command: clone repo, configure, open dashboard.

---

### agentmux

**Website:** https://agentmux.app/  
**Install:** `curl -4fsSL https://agentmux.app/install.sh | bash`  
**Multiplexer:** tmux (required). macOS, Linux, WSL.

**What it is:** Commercial TUI orchestrator for AI coding agents. Works in any terminal emulator or IDE. Paid product (one-time license, $29 single / $60 bundle for 3 devices).

**Key innovation:** Terminal-native, no Electron, no GUI requirement. Polished purpose-built TUI. Unlimited devcontainers via bind-mount. Commercial model implies ongoing support.

**Gap:** Paid/closed-source; no API surface documented. Lite on public technical details.

---

### cmux

**Repo:** https://github.com/manaflow-ai/cmux  
**Website:** https://cmux.dev/  
**Install:** DMG or `brew install --cask cmux` (macOS only)  
**Multiplexer:** **None — replaces tmux entirely** with its own GPU-accelerated native terminal (Swift + AppKit + libghostty).

**What it is:** Native macOS terminal application for running many AI coding agent sessions in parallel. NOT a tmux wrapper.

**Key innovations:**
- **Notification rings** — visual ring on panes + tab lighting when an agent needs attention (picks up OSC 9/99/777 sequences and `cmux notify` CLI).
- **Sidebar with agent context** — git branch, linked PR status, working directory, listening ports, latest notification per workspace.
- **Built-in scriptable browser** — agents can snapshot accessibility tree, click, fill forms, evaluate JS.
- **CLI + socket API** — `cmux notify`, create workspaces/tabs, split panes, send keystrokes; scriptable from agent hooks.
- **"Primitive, not a solution" philosophy** — composable building blocks; doesn't impose workflow.

---

### multiclaude (dlorenc)

**Repo:** https://github.com/dlorenc/multiclaude  
**Install:** `go install github.com/dlorenc/multiclaude/cmd/multiclaude@latest`  
**Multiplexer:** tmux (required). Each agent gets its own tmux window in a named session (`mc-<repo>`).

**What it is:** Go binary that spawns multiple Claude Code instances in parallel, each with its own tmux window and git worktree, coordinated by built-in agents (supervisor, merge-queue, pr-shepherd, workspace, worker, reviewer). Uses CI as the progress ratchet.

**Key innovations:**
- **"Brownian Ratchet" philosophy** — random/redundant agent work is acceptable; CI is the one-way gate that only lets passing code through. Forward progress is permanent; wasted work is cheap.
- **Two modes** — Single Player (merge-queue auto-merges on green CI) vs Multiplayer (pr-shepherd respects team review process).
- **Built-in role agents defined in markdown** — supervisor, merge-queue, pr-shepherd, etc.; user-extensible via `~/.multiclaude/repos/<repo>/agents/*.md`.
- **Self-hosting** — multiclaude built itself.

**CLI:** `multiclaude start`, `multiclaude repo init <url>`, `multiclaude worker create "<task>"`.  
**Prerequisites:** tmux, git, gh (GitHub CLI authenticated).

---

### agent-flow (patoles)

**Repo:** https://github.com/patoles/agent-flow  
**Install:** `npx agent-flow-app` or VS Code extension  
**Multiplexer:** None (visualization layer only)

**What it is:** Real-time visualization for Claude Code agent orchestration. NOT an orchestrator — a read-only observer that renders agent execution as an interactive node graph.

**Key innovations:**
- **Live node graph** — tool calls, branching, subagent coordination as interactive graph with real-time streaming.
- **Zero-latency via Claude Code hooks** — lightweight HTTP hook server.
- **File attention heatmap** — shows which files agents are spending time on.
- **JSONL log replay** — replay or watch agent activity from any `.jsonl` log.
- **VS Code extension + standalone.**

**Role for swarm:** Complementary. `swarm events` could emit in agent-flow-compatible JSONL format for visualization.

---

## 9. Cross-Tool Comparison Table

| Tool | Multiplexer Support | Task Primitives | Messaging | File Format / State | Gap Filled by `swarm` |
|------|--------------------|-----------------|-----------|--------------------|----------------------|
| **Conductor** | None (native Mac GUI) | None (ad-hoc prompts) | None | `conductor.json` (scripts) | No terminal-native mode; no zellij; no task graph; Mac-only; closed-source |
| **Vibe Kanban** | None (browser-based) | Kanban issues, workspaces, sessions | MCP server (incoming + outgoing) | Local DB + optional cloud | No TUI/CLI mode; no cross-agent coordination; browser required |
| **Claude Squad** | tmux only | None (ad-hoc prompts per session) | None | `~/.claude-squad/config.json` | No zellij; no task queue; no inter-agent API; no automation |
| **Gastown** | tmux only | Beads (full dependency graph, atomic claim, priority, hierarchy) | Seance (`.events.jsonl`), Escalate | Dolt SQL (`.dolt-data/`), `.events.jsonl` | No zellij; heavy Dolt dependency; complexity cliff |
| **workmux** | tmux (full) + WezTerm/Kitty/Zellij (experimental, Zellij unusable) | None (worktrees + prompts) | None | `.workmux.yaml`, filesystem JSON | No agent messaging; Zellij broken; git-repo required; no API |
| **NTM** | tmux only | Work triage (`br`/`bv`), assignment strategies, pipelines | Agent Mail (external), broadcast send | `.ntm/` JSON, YAML pipelines, audit logs | No zellij; no git worktree isolation; heavy dep chain; no external contributions |
| **dmux** | tmux only | None (human-driven per-pane) | None | TUI-only settings | No automation; no API; no zellij; no coordination |
| **CAO (AWS)** | tmux 3.3+ only | Flow scheduling, MCP-dispatched work | MCP server (supervisor ↔ worker) | Agent profiles as `.md` files | No zellij; Python-only; no file-backed persistent state |
| **Composio AO** | tmux default (Docker/K8s swappable) | Issue/task per agent, reactions | CI reactions → agent routing | `agent-orchestrator.yaml` | Not file-backed `.swarm/`; no TUI; SaaS-oriented |
| **agentmux** | tmux only | None visible | None visible | Unknown (commercial) | Paid/closed; no API; no zellij |
| **cmux** | None (replaces tmux) | None | CLI/socket API (`cmux notify`) | None (primitive-first) | macOS-only; no file-backed state; not harness-agnostic |
| **multiclaude** | tmux only | Worker tasks (`multiclaude worker create`) | CI as feedback (merge-queue) | Markdown agent role files | No zellij; Claude-only; no generic task graph |
| **agent-flow** | None (observer) | None | None | JSONL logs | Visualization only; not an orchestrator |

---

## 10. Key Design Lessons for `swarm`

### 10.1 Backend Detection (adopt verbatim from workmux)

```
Priority order:
  0. $SWARM_BACKEND (explicit override)
  1. $TMUX          → tmux
  2. $WEZTERM_PANE  → wezterm
  3. $ZELLIJ        → zellij
  4. $KITTY_WINDOW_ID → kitty
  5. fallback        → tmux
```

The logic is pure, fully tested, and correctly handles multiplexers nested inside each other (tmux inside kitty → `$TMUX` wins).

### 10.2 Architecture Principles

**ZFC (Zero Fragile Code):** The Go/infra layer does data transport and state management; LLM agents make decisions. Never parse agent stderr for branching logic.

**Propulsion:** Agents must execute immediately on task pickup — no human confirmation in the hot path.

**File-backed state in `.swarm/`:** Every piece of runtime state lives in a flat, inspectable, grep-able directory. No database required at the core. Take inspiration from Gastown's design but remove the Dolt dependency for the base layer.

### 10.3 What Each `swarm` Subcommand Should Own

| `swarm` subcommand | Inspired by | Key design |
|--------------------|-------------|------------|
| `swarm agent` | Claude Squad profiles + Gastown polecats | Named, persistent agent sessions; profile config as YAML; supports any agent runtime |
| `swarm task` | Gastown beads (`bd`) + Vibe Kanban issues | Dependency graph, atomic claim, `bd ready` pattern; JSON output; hierarchical IDs |
| `swarm msg` | NTM robot-mode + Gastown seance | Structured message passing to/from agents; `.events.jsonl` per agent; queryable history |
| `swarm pane` | workmux + NTM named panes | Backend-agnostic pane lifecycle (create/split/send/kill); `<project>__<agent>_<N>` naming |
| `swarm events` | NTM REST/SSE + agent-flow JSONL | SSE stream of structured events; JSONL log; REST `/api/v1`; agent-flow-compatible format |

### 10.4 Structural Gaps `swarm` Fills

1. **Harness-agnostic, multiplexer-agnostic** — no tool supports both tmux and zellij with full feature parity. `swarm` designs the multiplexer as a pluggable backend from day one.

2. **File-backed storage, no external DB** — all tools either have no persistence (Claude Squad) or require Dolt (Gastown) or a database. `.swarm/` flat-file state is inspectable, scriptable, and backup-friendly.

3. **Unified CLI surface** — every tool exposes a different mental model (TUI, browser, tmux wrappers, Go CLI). `swarm` gives a single, composable CLI (`swarm agent`, `swarm task`, `swarm msg`, `swarm pane`, `swarm events`) that can be called from scripts, CI, and other agents.

4. **JSON-first API surface** — `--robot-mode` exists in NTM but is bolted on. `swarm` designs JSON output as the first-class output format for every command.

5. **Cross-agent coordination without a supervisor agent** — most tools require either a human (Claude Squad, dmux) or a dedicated LLM coordinator (Gastown Mayor) as the routing layer. `swarm task` makes the task graph itself the routing layer; agents pick up tasks via `swarm task claim`, removing the need for a hardcoded supervisor.

6. **Notification model** — dmux and cmux highlight that agents finishing silently is a real pain point. `swarm events` should emit structured events consumable by OS notification systems and external dashboards.

7. **No git-worktree hard dependency** — workmux leaks the git-worktree abstraction; `swarm pane` manages panes independently of git, letting users opt into worktree integration via `swarm task`.

---

*Research produced by 4 parallel pi subagents (alpha, beta, gamma, delta) on 2026-03-29. See `findings-alpha.md`, `findings-beta.md`, `findings-gamma.md`, `findings-delta.md` for full raw findings.*
