# Beta Findings: Claude Squad + Gastown

---

## Tool 1: Claude Squad

### Repo / Source
- **Primary repo:** https://github.com/smtg-ai/claude-squad
- **Docs site:** https://smtg-ai.github.io/claude-squad/
- **Binary name:** `cs`  
- **License:** AGPL-3.0  
- **Language:** Go

### Problem Solved
Claude Squad lets a human developer supervise multiple AI coding agents (Claude Code, Codex, Aider, Gemini) simultaneously from a single terminal TUI. Each agent gets an isolated git worktree so branches never conflict. The operator can watch diffs live, attach to reprompt, commit, push, or kill sessions without leaving the TUI. Core value: **human-in-the-loop multi-agent review dashboard**, not automated orchestration.

### Key Primitives

**CLI commands:**
```
cs                   # Launch TUI (default program: claude)
cs -p "codex"        # Launch with alternate agent
cs -y                # Auto-yes / yolo mode (auto-accept all prompts)
cs reset             # Wipe all stored instances
cs version
cs debug             # Print config file path
```

**TUI keybindings (bottom-of-screen menu):**
| Key | Action |
|-----|--------|
| `n` | New session |
| `N` | New session with prompt |
| `D` | Kill/delete session |
| `↵ / o` | Attach to session (reprompt) |
| `ctrl-q` | Detach from session |
| `s` | Commit + push branch to GitHub |
| `c` | Checkout (commit + pause) |
| `r` | Resume paused session |
| `tab` | Toggle preview / diff tab |
| `q` | Quit |

**Config file:** `~/.claude-squad/config.json`
```json
{
  "default_program": "claude",
  "profiles": [
    { "name": "claude", "program": "claude" },
    { "name": "codex",  "program": "codex" },
    { "name": "aider",  "program": "aider --model ollama_chat/gemma3:1b" }
  ]
}
```

**How it launches agents:**  
1. Creates a **tmux session** for each agent instance.  
2. Creates a **git worktree** on a new branch so each agent has an isolated copy of the codebase.  
3. Runs the configured program (e.g. `claude`) inside that tmux pane.  
4. The TUI reads the git diff of the worktree branch for live preview.

**Prerequisites:** `tmux`, `gh` (GitHub CLI)

### Multiplexer Support
**tmux only.** The README and prerequisites explicitly require tmux. There is no zellij support. An open upstream issue in Claude Code's repo (Issue #31901) requests native zellij support as an alternative, confirming that the whole ecosystem is currently tmux-centric. Claude Squad itself has no plans for zellij support visible in docs.

### Reusable / Lessons
1. **Git worktree isolation** per agent is the cleanest primitive here — no branch conflicts, each agent has its own filesystem snapshot. Directly reusable.
2. **Profile system** in config.json is a clean pattern for multi-runtime support: name + program string covers every agent type.
3. **Diff-first review UX**: agents work in background, human reviews diff before committing. Strong safety pattern.
4. **Minimal scope**: the tool does exactly one thing (TUI agent dashboard) and does it well. No coordinator overhead.
5. **`-y` / autoyes flag pattern**: toggleable auto-accept mode at the session level is a useful primitive for unattended runs.
6. The binary is ~6 MB Go single binary — zero runtime dependencies beyond tmux/gh.

### Gap Left
- **No orchestration**: sessions are created manually by the human. No automated task dispatch, no agent-to-agent messaging.
- **No persistent state beyond git**: if Claude Squad exits, session metadata is gone. No audit trail, no task ledger.
- **No task queue or work assignment**: there is no concept of a work item, backlog, or priority. Everything is ad-hoc.
- **tmux hard dependency**: zellij, screen, WezTerm, Ghostty — none supported.
- **Headless/programmatic use undefined**: no API, no SDK, no way for another agent to spawn a cs session.
- **No watchdog**: if an agent stalls or crashes, the human notices via the TUI; there's no automated recovery.
- **Flat agent structure**: all sessions are peers; no hierarchy (coordinator → worker).

---

## Tool 2: Gastown

### Repo / Source
- **Primary repo:** https://github.com/steveyegge/gastown
- **Docs site:** https://gastown.dev/
- **Beads (separate tool):** https://github.com/steveyegge/beads
- **DeepWiki reference:** https://deepwiki.com/steveyegge/gastown/2.2-beads-issue-tracking
- **Binary names:** `gt` (Gastown), `bd` (Beads)  
- **Language:** Go  
- **Author:** Steve Yegge

### Problem Solved
Gastown solves **multi-agent context loss and chaos at scale**. When you run 4-10+ AI agents simultaneously, they lose state on restart, have no shared work ledger, and coordination becomes manual chaos. Gastown provides: persistent agent identity, git-backed work state (Hooks), structured task assignment (Beads + Convoys), automated watchdogs (Witness/Deacon/Dogs), and a Mayor coordinator agent — enabling reliable 20-30 agent workflows. Core value: **fully automated, self-healing multi-agent orchestration with persistent state**.

### Key Primitives

**`gt` CLI commands (orchestration layer):**
```bash
# Workspace setup
gt install ~/gt --git          # Initialize town workspace
gt rig add <name> <repo-url>   # Register a project
gt crew add <name> --rig <rig> # Create human workspace

# Agent lifecycle
gt mayor attach                # Start/attach to Mayor coordinator
gt agents                      # List all running agents
gt daemon status               # Check background daemon health
gt daemon start / stop

# Work assignment
gt sling <bead-id> <rig>       # Assign a work bead to an agent
gt convoy create "Name" id1 id2 --notify  # Bundle beads into a convoy
gt convoy list                 # Track convoy progress

# Agent operations
gt done                        # Agent signals completion (triggers Refinery)
gt escalate                    # Agent signals blocker (routes via severity)
gt seance                      # Discover predecessor agent sessions
gt seance --talk <id> -p "..."  # One-shot question to predecessor session
```

**`bd` CLI commands (Beads issue tracker — standalone):**
```bash
bd init                        # Initialize in a project
bd create "Title" -p 0         # Create P0 task
bd ready                       # List tasks with no open blockers (JSON)
bd update <id> --claim         # Atomically claim a task
bd dep add <child> <parent>    # Link tasks (blocks/related/parent-child)
bd show <id>                   # View task + audit trail
bd close <id> "reason"         # Mark complete
bd list / bd search --query "" # Enumerate/search beads
```

**Bead ID format:** `<prefix>-<5char-alphanumeric>` e.g. `gt-abc12`, `hq-x7k2m`  
- Prefix routes bead to the correct rig's Dolt database  
- `gt-` prefix = town-level (headquarters) beads  
- Hierarchical: `bd-a3f8` (Epic) → `bd-a3f8.1` (Task) → `bd-a3f8.1.1` (Sub-task)

**Storage backends:**
- **Dolt** (versioned SQL, Git-compatible) — primary persistence layer  
- Embedded mode: `bd init` → data in `.beads/embeddeddolt/`  
- Server mode: `bd init --server` → connects to `dolt sql-server` on port 3307  
- Git worktrees under `<rig>/polecats/<name>/` for per-agent filesystem isolation

**Config / workspace layout:**
```
~/gt/                          # Town root
├── .git/
├── .dolt-data/                # Dolt SQL database
├── settings/config.json       # Agent config (name → command)
├── beads_hq/                  # Mayor's bead workspace
├── beads_deacon/              # Deacon's bead workspace
├── daemon.log
└── <rig-name>/
    ├── polecats/<name>/       # Worker agent git worktrees
    ├── crew/<name>/           # Human workspace (full clone)
    └── hooks/                 # Persistent git-worktree storage
```

**Agent hierarchy (all run as tmux sessions):**
1. **Mayor** — global coordinator, briefed by human, creates convoys + dispatches beads
2. **Deacon** — town-level daemon watchdog, runs patrol cycles across all rigs
3. **Witness** — per-rig lifecycle manager, detects stuck agents (GUPP violations)
4. **Refinery** — per-rig merge queue, Bors-style bisecting merge with verification gates
5. **Polecats** — ephemeral worker sessions with persistent identity + history
6. **Crew** — human-managed workspaces (full clones, not worktrees)

**How agents are launched:**
- `gt sling <bead-id> <rig>` causes the Witness to spawn a polecat tmux session
- Polecat reads its mail (bead assignment) via `bd ready`
- Runs `claude` (or configured agent) in that tmux pane with git worktree isolation
- On completion, calls `gt done` → Refinery picks up for merge
- Agent identity (name, history, current assignment) persists in Dolt even after session ends

**Workflow templates (Molecules):**  
TOML formulas in `internal/formula/formulas/*.formula.toml` → instantiated as Molecules with tracked steps. Example: `release.formula.toml` with `bump-version`, `run-tests`, `tag-release` steps.

**Session continuity (Seance):**  
Agents log to `.events.jsonl`. `gt seance --talk <id>` lets a new agent query a predecessor's session history for context and decisions.

**Prerequisites:** Go 1.25+, Git 2.25+, Dolt 1.82.4+, beads (`bd`) 0.55.4+, sqlite3, tmux 3.0+, Claude Code CLI

### Multiplexer Support
**tmux only.** The codebase explicitly wraps tmux (`internal/tmux/tmux.go`). Session names, pane IDs, health detection (zombie sessions where shell is alive but agent exited), and all agent lifecycle management are tmux-native. Zellij is not supported. The Gastown DeepWiki docs confirm: "All agents run as tmux sessions."

### Reusable / Lessons
1. **Beads/`bd`** is a **standalone, composable tool** installable independently (`npm install -g @beads/bd` or `brew install beads`). It brings Dolt-backed dependency-aware task graphs, atomic claim, audit trail, and JSON output to any project. Directly reusable without Gastown.
2. **Bead ID routing by prefix** is a clean multi-repo/multi-rig dispatch primitive: the ID encodes its destination.
3. **The ZFC Principle (Zero Fragile Code):** Go/infrastructure layer does data transport; LLM agents make decisions. No parsing of stderr for branching logic. Strong design principle for any agent harness.
4. **The Propulsion Principle:** agents must execute immediately on bead discovery — no human confirmation loop in the hot path. This is the key to scaling.
5. **Persistent agent identity (polecats):** agents have names, history, and current assignment in Dolt even across restarts. Solves the "blank slate" problem.
6. **Three-tier watchdog pattern** (Witness→Deacon→Dogs) is reusable: per-unit monitor → cross-unit supervisor → dispatched maintenance workers.
7. **Convoy pattern** for batching related beads into a trackable unit (like a sprint or epic) is a clean abstraction for progress reporting.
8. **Seance (`.events.jsonl`)** for session continuity: any agent can query predecessor context without shared memory. Lightweight and crash-safe.
9. **Wasteland federated network via DoltHub**: reputation + work-claiming across Gas Towns. Ambitious but shows git/Dolt as coordination backbone.

### Gap Left
- **Heavy stack**: requires Dolt (a full versioned SQL engine), Go 1.25+, tmux, beads — this is a significant install burden. Not lightweight.
- **tmux hard dependency**: same constraint as Claude Squad — no zellij, no other multiplexer.
- **Mayor requires human briefing**: the Mayor is a Claude Code instance the human must instruct. Not fully autonomous at the top level.
- **No lightweight headless mode**: the minimal mode (no tmux) still requires Dolt + daemon for state persistence.
- **Tightly coupled to Claude Code as runtime**: configurable via `gt config agent`, but docs, tooling, and examples all assume `claude`. Other runtimes are second-class.
- **Formulas are embedded in binary**: TOML formulas live in `internal/formula/formulas/` — not user-extensible without recompiling.
- **Complexity cliff**: going from 0 → working Mayor requires ~8 setup steps across 4 tools. High barrier to entry.
- **No cross-agent type orchestration within a rig**: all polecats in a rig run the same agent command. No "some tasks go to claude, some go to codex" routing per-bead without workarounds.
- **The Wasteland (federated mode) is experimental/ambitious**: not production-ready, requires DoltHub account.

---

## Cross-Tool Comparison

| Dimension | Claude Squad | Gastown |
|-----------|-------------|---------|
| **Target scale** | 2–10 agents, human-supervised | 10–30 agents, mostly automated |
| **State persistence** | None (git only) | Full (Dolt SQL + git worktrees) |
| **Agent coordination** | None — human manages | Mayor → Convoy → Beads → Polecats |
| **Task system** | None | Beads (`bd`) — full dependency graph |
| **Watchdog** | None | Witness + Deacon + Dogs |
| **Multiplexer** | tmux only | tmux only |
| **Setup complexity** | Low (1 binary + tmux) | High (4 binaries + Dolt) |
| **Programmatic API** | None | `gt` CLI + Dolt SQL queries |
| **Agent identity** | Ephemeral | Persistent (polecat names + history) |
| **License** | AGPL-3.0 | Not visible in searched docs |

## Key Insight for OpenSwarm
Both tools share the same blind spot: **tmux hard dependency**. Neither supports zellij. OpenSwarm, built on pi + zellij, occupies a structural gap neither tool fills. The beads pattern (git/Dolt-backed persistent work items with atomic claim) is the strongest reusable concept — worth adopting as the coordination primitive without pulling in the full Gastown stack.
