---
# openswarm-m1iu
title: 'Investigate: pre-pane hook / global env injection for swarm run'
status: todo
type: feature
priority: high
created_at: 2026-04-02T19:35:59Z
updated_at: 2026-04-02T19:41:22Z
---

When swarm run start spawns a pane via Zellij/tmux/WezTerm, the pane inherits a minimal shell env (sh -c). Tools managed by mise/nvm/pyenv are not on PATH, forcing every run script to manually prepend shims.

Discovered during: spawning pi research agents — pi, lat, qry all missing until mise shims were prepended manually in each script.

## Problem in detail

Zellij backend does: zellij action new-pane -- sh -c <cmd>
This sh -c does not source ~/.bashrc or activate mise/nvm/pyenv.
The only workaround today: prepend export PATH to every script, or use bash -l scriptfile.

## Resolved

- [x] PATH inheritance: swarm run start now forwards os.Getenv("PATH") to every Spawn() call — mise/nvm/pyenv tools available automatically (v0.1.2)
- [x] --no-wait default: flipped to fire-and-forget; use --wait to block

## Still to investigate (config-level env injection for swarm pane spawn)

swarm pane spawn does not yet forward PATH. Options:
- [ ] A. Global [pane] env table in config.toml — passed to every Spawn() call (pane + run)
- [ ] B. --env KEY=VAL flag on swarm pane spawn for per-invocation overrides
- [ ] C. SWARM_PANE_SHELL env var — let users override sh -c with bash -l -c

## Recommendation to evaluate

Option A (global PaneEnv in config.toml) + D (per-run --env flag) covers both static project-wide setup and dynamic per-spawn overrides with no shell-compat risk. Option C is simplest if bash is always available.
