---
# openswarm-gpc4
title: 'Update SKILL.md: document --no-wait, PATH setup, and script-file pattern for swarm run'
status: completed
type: task
priority: normal
created_at: 2026-04-02T19:36:09Z
updated_at: 2026-04-02T19:41:22Z
---

Update extras/skills/openswarm/SKILL.md with lessons learned from running pi agents via swarm run.

Three gaps discovered in practice (2026-04-02):

## 1. --no-wait is required for background agents

swarm run start blocks by default (calls run.Wait() until pane exits).
Long-running agents (pi, claude, opencode) will hang the caller forever.
Document: always pass --no-wait when spawning an agent; use swarm run wait <id> to join later.

## 2. PATH is not inherited in panes

Zellij/tmux panes spawn with sh -c — no mise/nvm/pyenv shims.
Document the two workarounds until the feature bean (openswarm-m1iu) is resolved:
  a. Prepend shims: export PATH="/path/to/mise/shims:$PATH" at top of script
  b. Use login shell: bash -l myscript.sh (sources ~/.bash_profile → ~/.bashrc)
If openswarm-m1iu lands a config-level solution, replace workaround docs with the proper approach.

## 3. Script-file pattern for complex commands

Inline prompts with special characters (parens, quotes, &&) cause shell parsing errors when passed as CLI args to swarm run start -- <cmd>.
Document: write the command to a .sh file; invoke as swarm run start --no-wait -- bash myscript.sh

## Files to update

- [ ] extras/skills/openswarm/SKILL.md — add a 'Running agents in panes' section
- [ ] lat.md/extras.md — note the --no-wait / PATH guidance
- [ ] Run lat check

## Summary of Changes

- Updated extras/skills/openswarm/SKILL.md: removed --no-wait workaround, replaced with correct default; removed manual PATH export workarounds (PATH now inherited automatically); kept script-file quoting tip
- Updated lat.md/extras.md with the same
- Installed to ~/.pi/agent/skills/openswarm/SKILL.md

Both changes landed in the same commit as the fix itself (swarm run PATH inheritance + --wait flag).
