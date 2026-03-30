# Delta Findings: dmux + New Tools 2025-2026

> Research date: 2026-03-29

---

## Tool 1: dmux

### Repo / Source
- GitHub: https://github.com/standardagents/dmux
- Website: https://dmux.ai/
- Authors: Justin Schroeder & Andrew Boyd (FormKit / StandardAgents)
- Install: `npm install -g dmux`

### Problem Solved
Running multiple AI coding agents simultaneously in the same git repo causes file conflicts and context bleed. Plain tmux splits share a single working directory — agents stomp on each other. dmux solves this by giving every task its own tmux pane + isolated git worktree + branch, so agents work in parallel without interference. Humans manage panes via an interactive TUI; merging back is one keystroke.

### Key Primitives

**CLI entry point:**
```
cd /path/to/project
dmux          # launches TUI
```

**TUI keyboard shortcuts:**
| Key | Action |
|-----|--------|
| `n` | New pane (prompt → pick agent → worktree created automatically) |
| `t` | New plain terminal pane |
| `j` / Enter | Jump into pane |
| `m` | Open pane menu (merge, rename, etc.) |
| `f` | Browse files in that pane's worktree |
| `x` | Close pane |
| `h` / `H` | Hide/show pane(s) |
| `p` / `P` | Multi-project pane operations |
| `s` | Settings |
| `q` | Quit |

**Config:** Settings reachable via `s` key in TUI (no standalone config file documented in README).

**Supported agents (multi-select per prompt):** Claude Code, Codex, OpenCode, Cline CLI, Gemini CLI, Qwen CLI, Amp CLI, pi CLI, Cursor CLI, Copilot CLI, Crush CLI.

**Optional:** OpenRouter API key for AI-generated branch names and commit messages.

### Git Worktree Integration
- On new pane creation, dmux automatically runs `git worktree add` to create an isolated checkout in a fresh directory with a new branch.
- Branch names can be AI-generated (via OpenRouter) or manual.
- When task is done: pane menu → **Merge** → auto-commits changes, merges back to main branch, cleans up worktree.
- **Lifecycle hooks** available: `worktree-create`, `pre-merge`, `post-merge`, and more — lets you inject scripts (e.g., install deps, run tests) at defined lifecycle points.

### Multiplexer Support
- **tmux only** (tmux 3.0+ required).
- No zellij, screen, or other multiplexer support.
- Each pane is a native tmux pane; dmux wraps tmux session management.
- macOS native notifications for background panes that need attention.

### Reusable / Lessons
1. **Worktree-per-task as the atomic primitive** — cleanest isolation model; zero shared state between agents at the filesystem level.
2. **Multi-select agent launch** — user picks which agents run on a prompt simultaneously; good for comparison/parallelism.
3. **TUI-first, human-driven** — no background automation; human stays in control of every pane lifecycle decision.
4. **Built-in file browser** — inspect any worktree without leaving the tool; reduces context-switching.
5. **Pane visibility controls** — hide/show/isolate; useful for focus.
6. **Multi-project in one session** — add multiple repos; useful for cross-repo tasks.
7. **Lifecycle hooks** — lightweight escape hatch for custom scripts at defined points.
8. **AI naming** — branch and commit message generation is a small but meaningful UX improvement.

### Gap Left
- **No automated coordination** — completely human-driven. No supervisor agent, no task queuing, no dependency graph between panes.
- **No CI reaction** — if a PR's CI fails, dmux has no mechanism to route that back to an agent automatically.
- **No programmatic / API surface** — no REST, no socket, no way to drive dmux from another agent or script.
- **No inter-agent communication** — agents cannot see each other's progress or coordinate; only the human can broker information.
- **tmux lock-in** — no zellij support, no Docker/K8s runtime option for remote/headless scenarios.
- **No status/health tracking** — no dashboard showing agent state, tokens used, whether an agent is stuck.
- **macOS-only notifications** — Linux users get no attention alerts.

---

## New Tools Discovered (2025-2026)

### 1. CLI Agent Orchestrator (CAO)
- **Repo:** https://github.com/awslabs/cli-agent-orchestrator
- **Author:** AWS Labs
- **Install:** `uv tool install git+https://github.com/awslabs/cli-agent-orchestrator.git@main`
- **Description:** Lightweight Python orchestration system for managing hierarchical multi-agent sessions in tmux. A **supervisor agent** coordinates work and delegates to specialized **worker agents** via MCP server for inter-agent messaging.
- **Multiplexer:** tmux 3.3+ (required). Each agent runs in an isolated tmux session.
- **Key Innovation:**
  - **MCP server as the inter-agent message bus** — agents communicate through Model Context Protocol rather than filesystem conventions.
  - **Three orchestration patterns:** Handoff (synchronous), Assign (async/parallel), Send Message (direct).
  - **Agent profiles as markdown files** — define agent roles in `.md`, installable from local file or URL (`cao install ./my-agent.md`).
  - **Flow scheduling** — cron-like scheduling for automated/unattended workflow execution.
  - **Agent-agnostic:** Kiro, Claude Code, Codex, Gemini CLI, Kimi CLI, GitHub Copilot, Q CLI.
  - **Direct worker steering** — humans can intervene and guide individual worker agents in real-time, unlike pure sub-agent systems.
- **CLI commands:** `cao install`, `cao launch`, `cao-server`, `cao run-flow`

---

### 2. Composio Agent Orchestrator (ao)
- **Repo:** https://github.com/ComposioHQ/agent-orchestrator
- **Install:** `npm install -g @composio/ao`
- **Description:** Fleet management for parallel AI coding agents. Each issue/task gets its own agent, git worktree, branch, and PR. CI failures and review comments are automatically routed back to the responsible agent. A web dashboard at `localhost:3000` shows fleet status.
- **Multiplexer:** tmux (default runtime); Docker, K8s, process runtimes are swappable via plugin.
- **Key Innovation:**
  - **Reaction system** — `ci-failed → send-to-agent`, `changes-requested → send-to-agent`, `approved-and-green → notify/auto-merge`. CI becomes a feedback loop, not just a gate.
  - **8-slot plugin architecture** — Runtime, Agent, Workspace, Tracker, SCM, Notifier, Terminal, Lifecycle are all swappable TypeScript interfaces.
  - **Agent-agnostic + tracker-agnostic** — Claude Code, Codex, Aider; GitHub, Linear.
  - **YAML config:** `agent-orchestrator.yaml` auto-generated by `ao start`, editable for fine-tuning reactions.
  - **`ao start <url>`** — single command to clone repo, configure, and open dashboard.
  - 3,288 test cases in the codebase.

---

### 3. agentmux
- **Website:** https://agentmux.app/
- **Install:** `curl -4fsSL https://agentmux.app/install.sh | bash` (requires tmux)
- **Description:** Commercial TUI orchestrator for AI coding agents. Positioned as a "blazing fast orchestrator" that works in any terminal emulator or IDE. Paid product (one-time license, $29 single / $60 bundle for 3 devices).
- **Multiplexer:** tmux (required). Supports macOS, Linux, WSL.
- **Supported agents:** Claude Code, Codex CLI, Gemini CLI, Aider, OpenCode, and more.
- **Key Innovation:**
  - **Terminal-native, works inside any terminal emulator or IDE** — no Electron, no GUI requirement.
  - **Purpose-built TUI** with screenshot evidence of polished interface.
  - **Unlimited devcontainers** — devcontainer bind-mount pattern for license sharing across containers.
  - **Commercial model** distinguishes it — implies ongoing support and polish commitment.
- **Gap:** Paid/closed-source; no API surface documented. Lite on public technical details.

---

### 4. cmux
- **Repo:** https://github.com/manaflow-ai/cmux
- **Website:** https://cmux.dev/
- **Install:** DMG download or `brew install --cask cmux` (macOS only)
- **Description:** Native macOS terminal application (Swift + AppKit + libghostty) designed for running many AI coding agent sessions in parallel. NOT a tmux wrapper — it is a standalone GPU-accelerated terminal with a custom sidebar, notification system, and built-in browser.
- **Multiplexer:** None (replaces tmux entirely with its own native terminal multiplexing). Reads existing `~/.config/ghostty/config`.
- **Key Innovation:**
  - **Notification rings** — visual ring on panes + tab lighting when an agent needs attention (picks up OSC 9/99/777 sequences and `cmux notify` CLI).
  - **Sidebar with agent context** — shows git branch, linked PR status/number, working directory, listening ports, latest notification text per workspace.
  - **Built-in scriptable browser** — ported from vercel-labs/agent-browser; agents can snapshot accessibility tree, click, fill forms, evaluate JS — split browser pane next to terminal.
  - **CLI + socket API** — `cmux notify`, create workspaces/tabs, split panes, send keystrokes, open URLs; scriptable from agent hooks.
  - **"Primitive, not a solution" philosophy** — composable building blocks; doesn't impose workflow.
  - **macOS-only** — not cross-platform; built with Swift/AppKit for native performance.

---

### 5. multiclaude
- **Repo:** https://github.com/dlorenc/multiclaude
- **Install:** `go install github.com/dlorenc/multiclaude/cmd/multiclaude@latest`
- **Description:** Go binary that spawns multiple Claude Code instances in parallel, each with its own tmux window and git worktree, coordinated by built-in agents (supervisor, merge-queue, pr-shepherd, workspace, worker, reviewer). Uses CI as the progress ratchet.
- **Multiplexer:** tmux (required). Each agent gets its own tmux window in a named session (`mc-<repo>`).
- **Key Innovation:**
  - **"Brownian Ratchet" philosophy** — random/redundant agent work is acceptable; CI is the one-way gate that only lets passing code through. Forward progress is permanent; wasted work is cheap.
  - **Two modes** — Single Player (merge-queue auto-merges on green CI) vs Multiplayer (pr-shepherd coordinates human reviewers, respects team process). Fork detection is automatic.
  - **Built-in role agents defined in markdown** — supervisor, merge-queue, pr-shepherd, workspace, worker, reviewer. User-extensible via `~/.multiclaude/repos/<repo>/agents/*.md`.
  - **Self-hosting** — multiclaude built itself; agents wrote the code.
  - **CLI:** `multiclaude start`, `multiclaude repo init <url>`, `multiclaude worker create "<task>"`.
  - Prerequisites: tmux, git, gh (GitHub CLI authenticated).

---

### 6. NTM (Named Tmux Manager)
- **Repo:** https://github.com/Dicklesworthstone/ntm
- **Install:** `curl -fsSL ".../install.sh" | bash -s -- --easy-mode`
- **Description:** Comprehensive Go binary that turns tmux into a local control plane for multi-agent software development. Goes far beyond session launching — adds work intelligence, safety policy, Agent Mail coordination, durable state, and machine-readable API surfaces.
- **Multiplexer:** tmux (required and central). Pure tmux wrapper; no standalone terminal.
- **Key Innovation:**
  - **Work intelligence layer** — `ntm work triage` (graph-aware), `ntm work next` (dependency-based next-step selection), `ntm assign` (task assignment to specific panes).
  - **Agent Mail** — structured inter-agent and human-to-agent messaging system; inbox views.
  - **Safety & policy** — destructive command protection, approval workflows (`ntm approve`), guards, policy editing.
  - **Durable state** — checkpoints, timelines, audit logs, saved sessions, pipeline state. Recoverable after crashes.
  - **REST / SSE / WebSocket API + OpenAPI** — `ntm serve --port 7337`; `ntm --robot-snapshot` for machine-readable JSON of full system state. Enables external dashboards and agent self-inspection.
  - **File reservations** — lock files to prevent multi-agent write conflicts.
  - **Shell integration** — `eval "$(ntm shell zsh)"`.
  - **Heavy integrations** — `br`, `bv`, `cass`, `dcg`, `pt` for advanced work graph analysis.
  - **Multi-label swarms** — `ntm spawn payments --label backend --cc=2 --cod=1` for coordinated sub-swarms on same project.

---

### 7. agent-flow (patoles)
- **Repo:** https://github.com/patoles/agent-flow
- **Install:** `npx agent-flow-app` or VS Code extension
- **Description:** Real-time visualization tool for Claude Code agent orchestration. NOT an orchestrator itself — a read-only observer that makes agent execution visible as an interactive node graph.
- **Multiplexer:** None (visualization layer only; works alongside any orchestrator).
- **Key Innovation:**
  - **Live node graph** — tool calls, branching, subagent coordination rendered as interactive graph with real-time streaming.
  - **Zero-latency via Claude Code hooks** — lightweight HTTP hook server; events come directly from Claude Code.
  - **File attention heatmap** — shows which files agents are spending time on.
  - **JSONL log replay** — point at any `.jsonl` log file to replay or watch agent activity.
  - **VS Code extension + standalone** — works both ways; auto-detects Claude Code sessions.
  - **Key gap it fills:** transparency/debugging for complex multi-agent runs. Complementary to any of the above tools.

---

## Cross-Cutting Observations

### Convergent patterns in 2025-2026 tooling:
1. **git worktrees as the isolation primitive** — virtually every tool uses them. Not optional; considered mandatory.
2. **tmux as the default runtime** — dominant, but pressure growing for zellij, Docker, K8s alternatives (Claude Code GitHub issues requesting zellij support).
3. **Agent-agnostic design** — Claude Code, Codex, Gemini CLI treated as interchangeable; no tool locks to one model.
4. **Markdown as agent configuration** — agent roles defined in `.md` files (CAO, multiclaude, NTM all do this).
5. **CI as coordination signal** — multiple tools (Composio AO, multiclaude) use CI pass/fail to close feedback loops automatically.
6. **Notification problem is real** — both dmux and cmux prominently feature notification systems; agents finishing silently is a UX pain point.
7. **Spectrum from primitive to platform** — cmux/NTM give raw primitives; Composio AO gives a full platform with web UI.

### What none of these tools do well:
- **Cross-machine / remote agent distribution** — all assume agents run on the local machine.
- **Agent output aggregation / diffing** — no tool provides a structured view of what each agent produced and how they differ.
- **Cost tracking** — no token/cost accounting across the swarm.
- **Zellij native support** — gap acknowledged in Claude Code issues but unimplemented.
