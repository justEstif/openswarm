# Alpha Findings: Conductor + Vibe Kanban

_Researched: 2026-03-29 | Agent: alpha_

---

## Tool 1: Conductor (Melty Labs)

### Repo / Source

- **Website**: https://www.conductor.build/
- **Docs**: https://docs.conductor.build/
- **GitHub (releases only)**: https://github.com/meltylabs/conductor-releases — source is **closed**
- **Company**: Melty Labs (YC S24), previously built the open-source AI editor "Melty" / "Chorus"
- **Distribution**: Mac-only native app download (Apple Silicon required; Intel in progress; no Windows/Linux)
- **Latest noted version**: 0.44.0

---

### Problem Solved

Developers running Claude Code or Codex hit a **single-threaded bottleneck**: one agent on one task at a time. The naive fix — manually cloning the repo into 3 directories and running Claude in each — was described by the founders as _"driving a Subaru with a jet engine strapped on."_

Conductor solves:
1. **Workspace isolation** — each agent runs in its own git worktree (a separate working dir on a new branch, sharing object storage with the main repo).
2. **Visual orchestration** — a unified dashboard showing which agent is working on what, what needs attention, and surfacing diffs for review.
3. **Merge workflow** — review diffs and merge worktree branches back to main without leaving the app.
4. **Multi-model comparison** — run the same prompt simultaneously with Claude and Codex in separate tabs to compare approaches.

---

### Key Primitives

| Primitive | Description |
|-----------|-------------|
| **Workspace** | One git worktree + one agent session. Created with ⌘+N. |
| **conductor.json** | Repo-committed config file for sharing setup/run scripts with teammates. Schema: `{ scripts: { setup, run, archive }, runScriptMode, enterpriseDataPrivacy }` |
| **Scripts** | `setup` (e.g., `npm install`), `run` (e.g., `npm run dev`), `archive` (cleanup). Can be personal or shared via conductor.json. |
| **Checkpoints** | Automatic git snapshots inside a workspace, enabling rollback mid-session. |
| **Diff Viewer** | Side-by-side diff for all changes made in a workspace; inline review before merge. |
| **Spotlight Testing** | Sync changes from a workspace back to your main repo checkout for integration testing without merging. |
| **Todos** | Agent-surfaced task list visible in the dashboard. |
| **MCP support** | Configure MCP servers available to agents inside workspaces. |
| **Slash commands** | In-agent slash commands (specifics not fully documented publicly). |
| **Parallel agents** | Multiple workspaces run concurrently; ⌘+N spins up each one. |
| **Multi-model mode** | Same prompt → Claude + Codex simultaneously for comparison. |

**Authentication**: Re-uses user's existing Claude Code login (API key, Claude Pro, or Claude Max). No separate billing.

**GitHub integration**: Originally used GitHub OAuth (controversial — required full org read/write). Now migrated to GitHub App for fine-grained permissions. Supports `git`/`gh` CLI auth as alternative.

**Privacy**: Chat history stays local. Analytics via PostHog (workspace creation, model selection, errors). `enterpriseDataPrivacy: true` in conductor.json disables all telemetry.

---

### Multiplexer Support

**None.** Conductor is a **native Mac GUI application**. It does not use tmux, zellij, or any terminal multiplexer. Isolation is achieved entirely via git worktrees; the UI is its own multiplexing layer. Agents run as headless processes managed by the Conductor app process directly.

This is a deliberate design choice toward a polished GUI experience rather than a CLI/TUI approach.

---

### Reusable / Lessons

1. **git worktrees are the right isolation primitive** — every serious tool in this space (Conductor, Vibe Kanban, Claude Squad, Crystal) converges on worktrees. They share object storage, avoid re-cloning, and keep branch history. This is the de-facto standard.
2. **conductor.json pattern** — a repo-committed config that captures setup/run scripts is elegantly simple. Any orchestrator should have this: a lightweight manifest that bootstraps a workspace reproducibly.
3. **"Who needs attention?" dashboard** — surfacing agents that are blocked or waiting, not just running, is the key UX insight. Don't just show status; highlight what needs human input.
4. **Checkpoint/snapshot model** — automatic rollback points inside a long-running agent session reduce the cost of agent mistakes. Worth implementing.
5. **Multi-model comparison mode** — running competing models on the same task and letting the human pick the best output is a high-value pattern with no extra complexity.
6. **Re-use existing auth** — don't add a billing layer; piggyback on the agent's own credentials (Claude Code, Codex). This removes a major friction point.
7. **The security lesson** — broad OAuth permissions caused immediate backlash. Any tool that touches GitHub needs fine-grained GitHub App permissions from day one, or explicit git/gh CLI auth as a local-only alternative.

---

### Gap Left

1. **Mac-only, closed-source** — Linux/Windows users and teams requiring full source transparency are excluded. Crystal and Claude Squad fill part of this gap, but Conductor's polish level doesn't exist in the open-source space.
2. **No terminal multiplexer integration** — developers who live in the terminal can't use Conductor without adopting its GUI. No zellij/tmux bridge.
3. **No task/issue tracking** — Conductor has no concept of issues, kanban, or structured work planning. You manually describe tasks to each workspace. Vibe Kanban directly addresses this gap.
4. **No cross-workspace coordination** — agents are completely independent. There's no way for Workspace A to depend on Workspace B's output, share context, or have a supervisor agent orchestrate sub-agents.
5. **Context amnesia** — no persistent memory between sessions; developers must maintain CLAUDE.md files or re-explain conventions each time.
6. **No CI/CD hooks** — no built-in way to trigger workspace creation from issues/PRs or push results to CI.
7. **GitHub-centric** — requires GitHub; no support for GitLab, Bitbucket, or purely local repos at launch (requires cloning from remote rather than working with existing local checkout).

---

## Tool 2: Vibe Kanban

### Repo / Source

- **GitHub**: https://github.com/BloopAI/vibe-kanban — **open source** (MIT-adjacent, by Bloop AI)
- **Website / Docs**: https://vibekanban.com/docs
- **npm package**: `vibe-kanban` — launch with `npx vibe-kanban`
- **Stack**: Rust backend + Node/pnpm frontend (React). Multi-crate Cargo workspace.
- **Cloud tier**: Optional hosted cloud (organizations, shared boards, relay tunnels). Self-hostable via Docker Compose.
- **Discord**: Active community at discord.gg/AC4nwVtJM3

---

### Problem Solved

The core insight (stated in the README): _"In a world where software engineers spend most of their time planning and reviewing coding agents, the most impactful way to ship more is to get faster at planning and review."_

Vibe Kanban is a **full workflow layer** on top of CLI coding agents. It solves:

1. **Task planning chaos** — developers have no structured way to plan, prioritize, and track what each agent should be doing. Vibe Kanban adds a kanban board (issues, priorities, statuses) as the coordination primitive.
2. **Agent fragmentation** — users had to pick one agent and stay loyal to it. Vibe Kanban is **agent-agnostic**, supporting 10+ agents switchably within the same workflow.
3. **Review friction** — reviewing agent output means context-switching to GitHub, running diffs in terminals, etc. Vibe Kanban brings the diff viewer, inline comments, PR creation, and a **built-in browser preview** into a single UI.
4. **Workspace bootstrapping** — spinning up isolated environments for each task manually (cloning, branching, installing deps) is error-prone. Vibe Kanban automates worktree creation, branching, and lifecycle management.

---

### Key Primitives

| Primitive | Description |
|-----------|-------------|
| **Issue** | A unit of planned work on the kanban board (title, description, priority, status, tags, parent/sub-issues). Lives in local DB or cloud project. |
| **Workspace** | An isolated execution environment: 1 git worktree + 1 branch + 1 terminal + 1 dev server (per task). Multiple sessions possible per workspace. |
| **Session** | A conversation thread with a specific coding agent inside a workspace. Multiple sessions per workspace supported (e.g., backend session + frontend session). |
| **Repository** | A git repo registered in a project. Workspaces can span multiple repos (each gets its own worktree sub-dir). |
| **Project** | A container grouping related repositories; configured once in Settings. |
| **Changes Panel** | Syntax-highlighted diff viewer with inline comment capability; feedback sent directly back to agent. |
| **Browser Preview** | Built-in browser with devtools, inspect mode, and device emulation for testing the running dev server. |
| **MCP Server (outgoing)** | Agents inside workspaces can connect to external MCP servers (configured in Settings). |
| **MCP Server (incoming)** | Vibe Kanban itself exposes a local MCP server: `npx vibe-kanban --mcp`. Clients (Claude Desktop, Raycast, other agents) can manage issues, workspaces, and repos via MCP tools. |
| **Slash Commands** | In-workspace slash commands for common actions. |
| **PR creation** | Creates GitHub/Azure PRs with AI-generated descriptions directly from a workspace. |
| **`DISABLE_WORKTREE_CLEANUP`** | Env var to preserve worktrees for debugging. |
| **`VK_TUNNEL`** | Relay tunnel mode for remote/cloud access without direct network exposure. |

**Supported agents (10+)**: Claude Code, OpenAI Codex, GitHub Copilot, Gemini CLI, Amp, Cursor Agent CLI, OpenCode, Factory Droid, CCR (Claude Code Router), Qwen Code.

**Backend architecture** (Rust): Separate Cargo crates — `worktree-manager`, `workspace-manager`, `git`, `utils`. Worktrees stored in OS-appropriate dirs (macOS: persistent `~/.../vibe-kanban/worktrees`, Linux: tmpfs-backed, Windows: temp dir).

---

### Multiplexer Support

**None — Vibe Kanban does not use tmux or zellij.** Isolation is git-worktree-based, UI is a local web server accessed via browser (`npx vibe-kanban` → http://localhost:<port>). Each agent session runs as a managed subprocess (pty/terminal) inside the Rust backend.

There is an **integrated terminal** in the Workspaces UI (built-in terminal for running commands without leaving the browser interface). No external terminal multiplexer is required or used.

Remote access is handled via **tunnel mode** (Cloudflare Tunnel / ngrok / `VK_TUNNEL`), not by SSH into a tmux session.

---

### Reusable / Lessons

1. **Kanban board as the coordination layer** — using issues as the unit of work that maps 1:1 to workspaces is a clean abstraction. Issues have a lifecycle (backlog → in-progress → done) that naturally mirrors workspace states.
2. **MCP as a two-way integration bus** — exposing a local MCP server is a brilliant pattern. It means the orchestrator itself becomes a tool that other agents can call, enabling meta-orchestration (an agent managing other agents via MCP).
3. **Agent-agnostic design from day one** — abstracting the agent as a configurable subprocess rather than hardcoding Claude Code means the same tool works across the entire ecosystem without forking.
4. **Inline review → agent feedback loop** — the ability to comment on specific diff lines and have those comments routed back to the agent as follow-up instructions closes the human-in-the-loop cycle elegantly.
5. **Multi-repo workspaces** — supporting multiple repos in one workspace (each with its own worktree) is essential for full-stack monorepo-split projects. Few tools handle this.
6. **Open source + self-hostable** — the Docker Compose self-hosting path means teams with security/compliance requirements can run it entirely on their own infrastructure.
7. **Rust backend** — using Rust for the worktree/process management layer is a good call for reliability and cross-platform support (macOS + Linux + Windows all supported, unlike Conductor).
8. **The "doomscrolling" mental model** — the explicit framing that reviewing agent outputs should feel like reviewing a feed (fast, structured, low-friction) shapes the entire UX. The right metaphor drives the right design.

---

### Gap Left

1. **No terminal-native / TUI mode** — Vibe Kanban is entirely browser-based. Developers who prefer staying in the terminal (zellij, tmux workflows) have no CLI/TUI equivalent. `npx vibe-kanban` opens a browser, not a terminal interface.
2. **No cross-agent coordination / supervisor model** — like Conductor, agents in different workspaces are independent. There's no built-in mechanism for a "planner" agent to break down a spec into issues and assign them to worker agents automatically.
3. **MCP meta-orchestration is immature** — while the MCP server exists, using it for agents-orchestrating-agents (e.g., a Claude instance that creates and monitors other workspaces via MCP) requires external tooling; not built-in.
4. **Issues are manually created** — the kanban board requires human-authored issues. There's no auto-decomposition from a high-level spec into sub-tasks.
5. **No checkpoint/rollback inside sessions** — unlike Conductor's checkpoint feature, Vibe Kanban has no built-in mid-session snapshot mechanism (git history is the only rollback path).
6. **Cloud tier adds complexity** — the local vs. cloud distinction (local projects vs. remote projects) creates configuration friction and a mental model split.
7. **No structured agent output validation** — agents can return anything; there's no schema enforcement, test-gate, or quality check baked into the workspace lifecycle.
8. **Dependency between workspaces is manual** — if Task B depends on Task A's output, the user must sequence them manually. No DAG-style task dependency support.

---

## Cross-Cutting Observations

| Dimension | Conductor | Vibe Kanban |
|-----------|-----------|-------------|
| **Isolation primitive** | git worktree | git worktree |
| **UI paradigm** | Native Mac GUI | Browser-based web app |
| **Multiplexer** | None | None |
| **Agent support** | Claude Code + Codex | 10+ (agent-agnostic) |
| **Task planning** | None (ad hoc prompts) | Kanban issues |
| **Cross-platform** | Mac only (Apple Silicon) | macOS + Linux + Windows |
| **Source** | Closed | Open source (Bloop AI) |
| **Self-hostable** | No | Yes (Docker Compose) |
| **MCP** | Inbound (agents use MCP servers) | Both: outbound + exposes own MCP server |
| **PR workflow** | Yes | Yes (with AI-generated descriptions) |
| **Diff/review UI** | Yes (diff viewer) | Yes (changes panel + inline comments) |
| **Rollback** | Yes (checkpoints) | No (git history only) |
| **Multi-model compare** | Yes | Not built-in |
| **Security posture** | Initially weak (fixed), now GitHub App | Local by default, fine-grained |

**Shared gap**: Neither tool supports **programmatic/automatic cross-workspace coordination** — the "supervisor agent routes tasks to worker agents" model. Both require a human to manually plan, assign, and sequence work. This is the most significant gap for a project like OpenSwarm.
