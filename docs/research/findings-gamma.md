# Gamma Findings: workmux + NTM

---

## Tool 1: workmux

### Repo / Source

- **GitHub**: https://github.com/raine/workmux
- **Docs site**: https://workmux.raine.dev/guide/
- **Language**: Rust
- **License**: (not stated prominently)
- **Stars**: ~1.1k
- **Activity**: Very active (weekly releases as of March 2026)
- **Multiplexer source module**: `src/multiplexer/` (mod.rs, tmux.rs, wezterm.rs, kitty.rs, zellij.rs, types.rs, agent.rs, handle.rs, handshake.rs, util.rs)

---

### Problem Solved

workmux is a **git worktree + terminal multiplexer window manager** designed for parallel AI-agent development workflows.

**Core pain**: When running multiple AI agents on different tasks, you need each agent isolated in its own branch, directory, and terminal window. Doing this manually requires ~10 commands per feature: `git worktree add`, `tmux new-window`, split panes, copy `.env`, link `node_modules`, run setup hooks, etc. Cleanup is equally tedious.

**Solution**: One command (`workmux add <branch>`) creates:
- A git worktree in a parallel directory
- A matching tmux window (or kitty/WezTerm/Zellij tab)
- Configured pane layout (editor, agent, shell, dev server, etc.)
- Copied/symlinked config files (`.env`, `node_modules`)
- Post-create hooks (dependency install, DB setup)

One command (`workmux merge`) tears it all down: merges branch, kills window, removes worktree, deletes local branch.

---

### Key Primitives

#### Commands
| Command | Description |
|---|---|
| `workmux add <branch>` | Create worktree + window, optionally spawn agents with prompts |
| `workmux merge [branch]` | Merge branch into main, clean up everything |
| `workmux remove [name]` | Remove without merging (alias: `rm`) |
| `workmux list` | List worktrees with agent/mux/merge status |
| `workmux open [name]` | Open/switch to window for existing worktree |
| `workmux close [name]` | Close window, keep worktree |
| `workmux dashboard` | Full-screen TUI: agents across sessions, diff view, patch mode |
| `workmux sidebar` | Toggle live agent status sidebar (tmux-only) |
| `workmux resurrect` | Restore worktrees after crash/restart |
| `workmux setup` | Auto-detect agents, install status hooks and skills |
| `workmux config edit/path/reference` | Config management |
| `workmux init` | Scaffold `.workmux.yaml` |
| `workmux sandbox {pull,build,shell,agent,stop,prune}` | Sandbox management |
| `workmux coordinator send/status/digest` | Multi-agent coordination |

#### Config (`.workmux.yaml` or `~/.config/workmux/config.yaml`)
```yaml
# Pane layout
panes:
  - command: <agent>     # resolved to configured agent
    focus: true
  - split: horizontal
    size: 20
    command: nvim

# Agent mapping
agents:
  claude: "claude --dangerously-skip-permissions"

# Post-create hooks
post_create:
  - npm install

# File operations
files:
  copy:
    - .env
  symlink:
    - node_modules

# Status icon customization
status_icons:
  working: '🤖'
  waiting: '💬'
  done: '✅'

# Session mode (each worktree gets its own tmux session)
mode: session  # or "window" (default)

# Merge strategy
merge_strategy: rebase  # or "merge" (default)

# Sandbox
sandbox:
  enabled: true
  backend: container  # or "lima"
```

#### Prompt / Agent Features
- `workmux add <branch> -p "inline prompt"` — inject prompt into agent at spawn
- `workmux add <branch> -P task.md` — read prompt from file with template vars
- `--foreach "platform:iOS,Android"` — matrix expansion, creates one worktree per combo
- `--auto-name` / `-A` — LLM-generated branch name from prompt
- `-n <N>` — N identical worktrees (numbered)
- `-a claude -a gemini` — agent-specific worktrees per agent type
- Stdin piping: `echo -e "api\nauth" | workmux add refactor -P task.md`
- JSON lines: `gh repo list --json url,name | workmux add analyze ...`

#### Status Tracking
- Agent hooks integrate with Claude Code, OpenCode, Codex, Copilot CLI, Pi
- Hooks fire `workmux set-status` which updates a tmux window variable (`@workmux_status`)
- `workmux last-done` — jump to most recently finished/waiting agent
- `workmux last-agent` — toggle between two most recent agents

---

### Backend Detection Mechanism

**Source**: `src/multiplexer/mod.rs` — `detect_backend()` function

#### Algorithm (exact priority order, from source):

```rust
pub fn detect_backend() -> BackendType {
    // 1. Check explicit override
    if let Ok(val) = std::env::var("WORKMUX_BACKEND") {
        match val.parse() {
            Ok(bt) => return bt,
            Err(_) => eprintln!("workmux: invalid WORKMUX_BACKEND={val:?}, expected tmux|wezterm|kitty|zellij"),
        }
    }
    // 2. Auto-detect from env vars
    resolve_backend(
        std::env::var("TMUX").is_ok(),
        std::env::var("WEZTERM_PANE").is_ok(),
        std::env::var("ZELLIJ").is_ok(),
        std::env::var("KITTY_WINDOW_ID").is_ok(),
    )
}

fn resolve_backend(tmux: bool, wezterm: bool, zellij: bool, kitty: bool) -> BackendType {
    if tmux    { return BackendType::Tmux; }
    if wezterm { return BackendType::WezTerm; }
    if zellij  { return BackendType::Zellij; }
    if kitty   { return BackendType::Kitty; }
    BackendType::Tmux  // default for backward compatibility
}
```

#### Environment Variables Checked (in priority order):

| Priority | Env Var | Backend | Notes |
|---|---|---|---|
| 0 (override) | `$WORKMUX_BACKEND` | any | Explicit, accepts: `tmux`, `wezterm`, `kitty`, `zellij` |
| 1 | `$TMUX` | tmux | Set by tmux to socket path |
| 2 | `$WEZTERM_PANE` | WezTerm | Set by WezTerm to pane ID |
| 3 | `$ZELLIJ` | Zellij | Set by Zellij session UUID |
| 4 | `$KITTY_WINDOW_ID` | Kitty | Set by Kitty to window ID |
| 5 (fallback) | none | tmux | Backward-compatible default |

#### Key Design Decisions:
- **Session-specific vars first**: `$TMUX` and `$WEZTERM_PANE` are only set when *inside* that multiplexer, unlike `$KITTY_WINDOW_ID` which is inherited by child processes. This means running tmux inside kitty → `$TMUX` is set → correctly picks tmux.
- **Separated for testability**: `resolve_backend()` is pure function, all tests use it directly
- **Tests cover**: no env, single env, tmux-inside-kitty, tmux-inside-wezterm, tmux-inside-zellij, wezterm-inside-kitty, zellij-inside-kitty, all-vars-set

---

### Multiplexer Support

| Backend | Status | Detection | Key Limitations |
|---|---|---|---|
| **tmux** | Primary / Full | `$TMUX` | None — all features supported |
| **WezTerm** | Experimental | `$WEZTERM_PANE` | No agent status in tabs; tab ordering appends to end; Windows not supported; needs `wezterm.lua` config |
| **Zellij** | Experimental | `$ZELLIJ` | Requires Zellij built from source (unreleased features); no session mode; 50/50 splits only; no dashboard preview; no agent status in tabs |
| **kitty** | Experimental | `$KITTY_WINDOW_ID` | Requires `allow_remote_control` + `listen_on` in kitty config |

**Multiplexer trait** (`src/multiplexer/mod.rs`): Rich interface with ~40+ methods:
- `create_window`, `create_session`, `kill_window`, `kill_session`
- `split_pane`, `send_keys`, `capture_pane`, `respawn_pane`
- `set_status`, `clear_status`, `ensure_status_format`
- `create_handshake` — named-pipe synchronization for shell startup
- `setup_panes` — default impl handles full orchestration; backends only need primitives
- `instance_id` — for multi-instance state isolation

---

### Reusable / Lessons

1. **Backend detection pattern**: The 5-level env-var priority cascade (`$WORKMUX_BACKEND` → `$TMUX` → `$WEZTERM_PANE` → `$ZELLIJ` → `$KITTY_WINDOW_ID` → default) is the canonical approach for multi-multiplexer detection. **Directly adoptable for openswarm.**

2. **Override env var**: `$WORKMUX_BACKEND` is a clean escape hatch for CI, scripts, and testing. openswarm should have `$OPENSWARM_BACKEND`.

3. **Trait-based multiplexer abstraction**: The `Multiplexer` trait pattern (mod.rs) lets backend-specific details stay isolated while the orchestration logic is shared. Very clean architecture.

4. **Handshake pattern**: Named-pipe handshake (`handshake.rs`) for synchronizing shell startup before injecting commands — solves race conditions when spawning panes.

5. **Pane setup orchestration**: `setup_panes()` default impl handles the full flow: resolve agent, create handshake, respawn/split, wait for shell, inject command, set status. Only requires primitives from backends.

6. **Status via tmux window variables**: Using `@workmux_status` tmux variable for agent status display — can be queried by `workmux dashboard` or the tmux status bar.

7. **State as filesystem JSON**: Moved from tmux-specific state storage to filesystem JSON for multi-backend compatibility. openswarm should do the same.

8. **Matrix/foreach spawning**: The `--foreach`/stdin piping approach for batch-spawning agents on permutations is elegant — relevant for openswarm's multi-agent workflows.

---

### Gap Left

1. **Git-worktree centric**: The abstraction leaks — workmux is fundamentally about git worktrees, not pure agent tiling. Not appropriate as a "swarm engine" without git repos.

2. **Zellij requires unreleased features**: The Zellij backend depends on `--pane-id` targeting, `close-tab-by-id`, `go-to-tab-by-id`, and tab ID APIs that are not yet in any released version as of March 2026. Effectively unusable today.

3. **Zellij limitations are deep**: Even with future Zellij, no session mode, 50/50 splits only, no agent status in tabs, no dashboard preview — second-class citizen.

4. **No agent-to-agent coordination**: workmux is about managing agent windows, not routing messages between agents.

5. **No REST/programmatic API**: No way to control workmux programmatically from other processes (no HTTP, no IPC beyond tmux).

6. **Status tracking requires hook installation**: Agent status depends on each agent (Claude, Codex, etc.) having its hooks configured. Can fail silently.

7. **tmux-only for full features**: Session mode, sidebar, status icons in tab names — all tmux-only. Multi-backend is very uneven.

---

## Tool 2: NTM

### Repo / Source

- **GitHub**: https://github.com/Dicklesworthstone/ntm
- **Full name**: Named Tmux Manager
- **Language**: Go (1.25+)
- **License**: MIT + additional rider
- **Install**: `brew install dicklesworthstone/tap/ntm` or `go install` or curl install script
- **Note**: Single-author; no external contributions accepted (policy stated in README)

---

### Problem Solved

NTM turns tmux into a **local control plane for multi-agent software development**. It addresses the "easy to start, annoying to sustain" problem with parallel agents.

**Core pain**: Plain tmux gives you panes but no durable coordination, work selection, safety policy, approvals, history, or shared control model for humans and agents.

**Solution**: NTM wraps tmux sessions with:
- Named multi-agent sessions (spawn N Claude + M Codex + K Gemini in one command)
- Broadcast/target prompts across agents without manual copy-paste
- Work graph triage (via `br`/`bv` integration)
- Safety system blocking destructive commands
- Durable state: checkpoints, timelines, audit logs, pipeline state
- Machine-readable robot surfaces (`--robot-*`)
- Local REST/SSE/WebSocket API (`ntm serve`)

---

### Key Primitives

#### Session Lifecycle
```bash
ntm quick api --template=go         # Scaffold project + agents
ntm spawn api --cc=2 --cod=1 --gmi=1  # 2 Claude + 1 Codex + 1 Gemini
ntm add api --cc=1                  # Add more agents to existing session
ntm list                            # List all tmux sessions
ntm status api                      # Pane details + agent counts
ntm view api                        # Unzoom, tile, attach
ntm zoom api 3                      # Zoom to pane 3
ntm attach api                      # Attach to session
ntm kill api                        # Kill session
```

#### Work Dispatch
```bash
ntm send api --cc "Implement auth"         # To all Claude panes
ntm send api --all "Checkpoint progress"   # To all agents
ntm interrupt api                          # Ctrl+C to all agents
ntm watch api --cc                         # Stream Claude output
ntm grep "timeout" api -C 3               # Search pane history
ntm diff api cc_1 cod_1                    # Compare two panes
ntm extract api --lang=go                  # Extract code blocks
```

#### TUI Surfaces
```bash
ntm dashboard api    # Visual pane grid with color-coded cards
ntm palette api      # Fuzzy-searchable command palette (F6)
ntm dashboard api    # Columns: pane grid, live counts, token velocity, context usage
```

#### Robot Mode (machine-readable JSON)
```bash
ntm --robot-snapshot                   # Full state: sessions + beads + mail
ntm --robot-status                     # Session list, agent states
ntm --robot-tail=api --lines=50        # Recent output
ntm --robot-send=api --msg="..." --type=claude
ntm --robot-spawn=api --spawn-cc=2     # Spawn from script
ntm --robot-plan                       # bv execution plan
```

#### REST API (`ntm serve`)
- REST under `/api/v1`
- Server-Sent Events at `/events`
- WebSocket at `/ws`
- OpenAPI spec at `docs/openapi.json`
- JWT auth via `~/.config/ntm/auth.token`

#### Durable State
```bash
ntm checkpoint save api -m "pre-migration"
ntm checkpoint restore api
ntm timeline show <session-id>
ntm audit show api
ntm resume api
ntm history search "auth error"
```

#### Work Graph (requires `br`/`bv`)
```bash
ntm work triage           # Prioritized task list with recommendations
ntm work next             # Single best next action
ntm work impact auth.go   # Impact analysis
ntm assign api --auto --strategy=dependency
```

#### Safety System
```bash
ntm safety status
ntm safety check -- git reset --hard   # Check if blocked
ntm policy show --all
ntm approve list
ntm approve <id>
ntm approve deny <id> --reason "wrong branch"
```

---

### Architecture

```
Human Operator / Agent CLI
         |
        NTM (Go binary)
         |
    ┌────┴──────────────────────┐
    │  Session Orchestration    │  ← named tmux sessions, pane layout
    │  Dashboard + Palette      │  ← TUI with charmbracelet/bubbletea
    │  Work Triage + Assignment │  ← br/bv integration
    │  Safety + Policy          │  ← guards, approvals, SLB
    │  Pipelines + Checkpoints  │  ← durable YAML pipeline state
    │  Robot/REST/WS surfaces   │  ← --robot-*, /api/v1, /ws
    └────┬──────────────────────┘
         |
    ┌────┴────────┐    ┌─────────────────────────────┐
    │  State +    │    │  Optional integrations:     │
    │  Event Bus  │    │  br, bv, Agent Mail, cass,  │
    └────┬────────┘    │  dcg, pt, worktrees         │
         |             └─────────────────────────────┘
    ┌────┴────────────────┐
    │  tmux sessions+panes│
    │  Claude/Codex/Gemini│
    └─────────────────────┘
```

#### Pane Spawning Mechanics

NTM manages tmux sessions with a **named pane pattern**: `<project>__<agent>_<number>`

- `myproject__cc_1` — First Claude pane
- `myproject__cod_2` — Second Codex pane
- `myproject__gmi_1` — Gemini pane
- `myproject__user_1` — Human operator pane

**Spawn flow** (`ntm spawn api --cc=2 --cod=1`):
1. Create tmux session named `api` (with NTM prefix)
2. Split window into N+M+K+1 panes using `tmux split-window -v/-h`
3. Apply tiled layout via `tmux select-layout tiled`
4. Send `claude` / `codex` / `gemini` start command to respective panes via `tmux send-keys`
5. Label panes with `tmux select-pane -T`
6. Store session state in `.ntm/` directory as JSON

**Context Rotation**:
- Monitors estimated token usage per agent
- At 80% → warning; at 95% → triggers compaction or fresh agent spawn with handoff summary
- Recovery prompt can include `br` bead context for continuity

**Output Capture** (`ntm copy`):
- Uses `tmux capture-pane -p` internally
- Can filter by agent type, regex, code blocks
- Diff uses captured pane content comparison

---

### Multiplexer Support

**tmux ONLY** — explicitly stated in limitations:

> "NTM is intentionally tmux-centric."
> "Linux and macOS are the primary environments."

No zellij, no wezterm, no kitty. The entire architecture assumes tmux session/pane primitives.

---

### Reusable / Lessons

1. **Named pane convention**: `<project>__<agent>_<number>` is clean and parseable. Openswarm could adopt a similar naming pattern for pane tracking.

2. **Robot mode as first-class API**: Treating `--robot-*` flags as a machine-readable surface layer (distinct from human TUI) is excellent design. Openswarm needs this — agents controlling agents needs JSON surfaces, not terminal scraping.

3. **REST + WebSocket for long-lived integrations**: `ntm serve` with SSE/WebSocket makes NTM usable as a local control plane from dashboards, scripts, and other agents. This is the right model for openswarm.

4. **Agent capability matrix**: Claude → analysis/architecture; Codex → implementation/bug fixes; Gemini → docs/review. This kind of routing logic is worth encoding in openswarm's work assignment.

5. **Context rotation as a first-class primitive**: Automatically tracking token usage and rotating agents with handoff summaries prevents the silent failure mode of context exhaustion. Critical for long-running swarms.

6. **Safety system design**: Blocking destructive commands (`git reset --hard`, `rm -rf /`) via policy rules + approval workflows is the right pattern for autonomous agent execution. Two-person approval for high-risk operations.

7. **Checkpoint/resume for recovery**: Explicit checkpoints + timeline replay lets the operator recover from crashes without losing work. openswarm needs similar durable state.

8. **`ntm deps -v`**: A dependency checker that verifies all optional integrations are present is a great UX pattern for complex tools.

9. **Recipes and pipelines**: `ntm recipes` (session presets) and `ntm pipeline` (multi-step YAML workflows with resume) are clean patterns for reusable swarm configurations.

10. **Work assignment strategies**: `balanced`, `speed`, `quality`, `dependency` — different assignment strategies for different contexts. Worth having in openswarm's scheduler.

---

### Gap Left

1. **tmux-only — no multi-backend support**: Zero portability to zellij, kitty, or wezterm. If openswarm needs to run in any terminal, NTM's architecture cannot help directly.

2. **Heavy dependency chain**: Agent Mail, `br`, `bv`, `cass`, `dcg`, `pt` — each is a separate tool. Full value requires installing and configuring many external systems. Most users will only get partial value.

3. **No git worktree isolation**: NTM operates in a single project directory. Running multiple agents means they share the working tree and can conflict on file writes. No isolation between agents.

4. **No external contributions accepted**: Single author, intentionally closed. If the author stops maintaining it, the project stalls.

5. **Single repo assumption**: NTM doesn't handle the case where agents work on different features in different git branches simultaneously.

6. **Fragile session naming**: The `<project>__<agent>_<number>` pattern is useful but requires careful bookkeeping. If a pane crashes and is re-numbered, state can diverge.

7. **No cross-agent message passing**: Agents can only receive prompts from the human operator. No direct agent-to-agent communication beyond Agent Mail (external tool).

8. **Context rotation is heuristic-based**: Token counting is estimated, not exact (no API-level token counting). Can misfire.

---

## Cross-Tool Comparison

| Dimension | workmux | NTM |
|---|---|---|
| Language | Rust | Go |
| Primary backend | tmux (+ experimental others) | tmux only |
| Backend detection | Env-var cascade (`$TMUX`, `$WEZTERM_PANE`, `$ZELLIJ`, `$KITTY_WINDOW_ID`) | N/A |
| Git integration | Deep (worktrees as first-class) | None |
| Programmatic API | None | REST/SSE/WebSocket + robot mode |
| Agent coordination | Window-per-agent, status tracking | Broadcast, assignment, mail, triage |
| Safety system | None | Full (guards, policy, approvals) |
| Durable state | Filesystem JSON per worktree | Checkpoints, timelines, audit logs |
| Context rotation | None | Yes (heuristic) |
| Dashboard | Yes (tmux TUI, diff view, patch mode) | Yes (agent grid, palette) |
| Config format | YAML | TOML |
| Multi-agent types | Via `-a claude -a gemini` flags | `--cc`, `--cod`, `--gmi` flags |

---

## Key Findings for OpenSwarm

1. **Backend detection**: Use workmux's exact env-var cascade. It's correct, tested, and has a clean override (`$WORKMUX_BACKEND` → `$OPENSWARM_BACKEND`).

2. **Zellij is not ready for production**: workmux Zellij support requires unreleased Zellij features. Don't promise Zellij support at launch.

3. **tmux is the only battle-tested multiplexer**: Both tools converge on this. WezTerm/Kitty are secondary.

4. **Robot mode is essential**: NTM's `--robot-*` surface is the right design for agent-controlling-agents. Openswarm needs JSON-first APIs, not TUI scraping.

5. **REST API is worth the complexity**: `ntm serve` enables external dashboards, CI integration, and agent self-orchestration. Design this in early.

6. **Git worktree isolation solves file conflicts**: workmux's approach of one worktree per task/agent prevents the file conflict problem NTM has. Consider making this a primitive in openswarm.

7. **Multiplexer trait pattern**: workmux's Rust trait abstraction is cleaner than NTM's tmux-hardcoded approach. For openswarm, model the multiplexer as an interface from day one.
