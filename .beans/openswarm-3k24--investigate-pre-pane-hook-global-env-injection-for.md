---
# openswarm-3k24
title: 'Investigate: pre-pane hook / global env injection for swarm run'
status: todo
type: feature
priority: high
created_at: 2026-04-02T19:35:53Z
updated_at: 2026-04-02T19:35:53Z
---

When `swarm run start` spawns a pane via Zellij/tmux/WezTerm, the pane inherits a minimal shell env (`sh -c`). Tools managed by mise/nvm/pyenv are not on PATH, forcing every run script to manually prepend shims. Investigate and implement a config-driven solution.

## Problem

Zellij (and other backends) spawn panes with `sh -c <cmd>`, which does not source `~/.bashrc` or activate mise/nvm/pyenv. Agents end up with missing tools unless they manually export PATH in every script.

Discovered during: spawning pi research agents — `pi`, `lat`, `qry` all missing until `/home/estifanos/.local/share/mise/shims` was prepended manually.

## Options to investigate

- [ ] **A. Global `[pane]` env table in config.toml** — extend `Config` struct with `PaneEnv map[string]string`; pass to every `Spawn()` call alongside any per-profile env
- [ ] **B. `pane_init_script` config key** — path to a shell script to source before every spawned command: rewrite cmd as `. $init_script && <original-cmd>`
- [ ] **C. Change backend shell from `sh -c` to `bash -l -c`** — login shell sources profile, activating mise/nvm automatically; risk: slower spawn, requires bash
- [ ] **D. `--env KEY=VAL` flag on `swarm run start` / `swarm pane spawn`** — per-invocation env injection without config changes
- [ ] **E. `SWARM_PANE_SHELL` env var** — let users override the shell used for pane spawning

## Recommendation to evaluate

Option A (global env in config.toml) + Option D (per-run --env flag) covers both static project-wide setup and dynamic per-spawn overrides, with no shell-compatibility risk. Option C is the simplest but hardest to override.
