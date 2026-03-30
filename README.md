# openswarm

> Open, composable, file-backed primitives for multi-agent terminal orchestration.

`swarm` is a unified CLI that recreates Claude Code's agent-team mode as open primitives — decoupled from any specific AI provider or terminal multiplexer.

## Subsystems

| Command group | What it does |
|---|---|
| `swarm msg` | Agent-to-agent messaging via lock-free inbox files |
| `swarm task` | Shared task queue with flock-safe mutations |
| `swarm pane` | Spawn/control/capture terminal panes across multiplexers |
| `swarm run` | Run a command in a managed pane, wait for exit |
| `swarm worktree` | git worktree lifecycle tied to agent identity |
| `swarm events` | Tail the shared append-only event log |

## Multiplexer support

| Backend | Status |
|---|---|
| tmux | ✅ MVP |
| Zellij ≥ 0.44.0 | ✅ MVP |
| WezTerm | ✅ MVP |
| Kitty | 🚧 Post-MVP |
| Ghostty | 🔬 Aspirational |

## State

All state lives under `.swarm/` in the project root (or `$SWARM_DIR`).

```
.swarm/
├── config.toml
├── agents/registry.json
├── messages/<agent-id>/inbox/<msg-id>.json
├── tasks/tasks.json + .lock
├── runs/runs.json
├── worktrees/worktrees.json
└── events/events.jsonl
```

## Design principles

- **File-backed everything** — no daemons, atomic writes, `flock()` for mutations
- **Agent-friendly** — `--json` on every command, machine-readable exit codes
- **Multiplexer-agnostic** — 8-method Backend interface; swap backends without changing callers
- **Pull complexity downward** — cobra handlers are ~15 lines; `internal/` does the work

## Docs

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — full module structure, interface designs, data flows
- [`docs/NOTES.md`](docs/NOTES.md) — decisions log and research synthesis
- [`docs/research/`](docs/research/) — raw research outputs

## Installation

### npm (recommended)

```bash
npm install -g openswarm
```

### Homebrew (coming soon)

```bash
brew install justEstif/tap/openswarm
```

### Direct download

Download the binary for your platform from [GitHub Releases](https://github.com/justEstif/openswarm/releases) and add it to your `PATH`.

### Build from source

```bash
git clone https://github.com/justEstif/openswarm
cd openswarm
go install ./cmd/swarm
```

## Development

```bash
# Enter dev shell
nix develop

# Build
go build ./cmd/swarm

# Test
go test ./...

# Lint
golangci-lint run
```
