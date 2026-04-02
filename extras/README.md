# extras

Supplementary integrations that make openswarm easier to use from a coding agent session.

## Universal skill

`skills/openswarm/SKILL.md` — a standard [agent skill](https://agentskills.io/what-are-skills) that teaches any compatible agent the full `swarm` CLI. Works with Claude Code, pi, and any other skill-compatible agent.

## Config reference

`config.toml` — a fully-commented example `.swarm/config.toml` covering every available option: backend selection, poll interval, pane placement, and agent profiles. Copy to `.swarm/config.toml` (project) or `~/.config/swarm/config.toml` (global).

## Coding agent integrations

| Integration | Path | What it adds |
|---|---|---|
| **Claude Code** | `claude-code/` | `SessionStart` hook (auto-init) + skill + `/assign-task` command |
| **opencode** | `opencode/` | Plugin: auto-init + compaction context injection + `/assign-task` command |
| **pi** | `pi/` | Extension: auto-init + `/swarm-status` + `/swarm-prompt` + `/assign-task` commands |

## Multiplexer note

`swarm pane` and `swarm run` commands require tmux, Zellij, or WezTerm. The CLI auto-detects the running backend from environment variables (`$TMUX`, `$ZELLIJ`, `$WEZTERM_PANE`). If none is detected, you'll get a clear error:

```
no supported multiplexer detected; set $SWARM_BACKEND or run inside tmux/zellij/wezterm
```

`swarm msg`, `swarm task`, `swarm agent`, `swarm events`, and `swarm status` all work without any multiplexer.
