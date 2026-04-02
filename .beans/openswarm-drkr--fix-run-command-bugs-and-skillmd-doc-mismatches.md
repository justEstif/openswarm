---
# openswarm-drkr
title: Fix run command bugs and SKILL.md doc mismatches
status: completed
type: bug
priority: high
created_at: 2026-04-02T12:08:44Z
updated_at: 2026-04-02T12:10:48Z
---

Three issues to fix:
1. BUG: run.Start double-wraps commands in sh -c (in run.go AND backends)
2. BUG: 'swarm run start' subcommand does not exist; SKILL docs reference it but root RunCmd falls through
3. DOCS: 4 SKILL.md command mismatches (agent register, task assign, task block, pane spawn)

Fixes needed:
- internal/run/run.go: remove '/bin/sh -c' wrapping in Start(), pass cmd directly to b.Spawn
- cmd/swarm/commands/run.go: add 'start' as an alias/subcommand for swarm run
- extras/skills/openswarm/SKILL.md + dotfiles copy: fix 4 command syntax errors
- lat.md: update run module docs to reflect correct behavior

## Summary of Changes

- **internal/run/run.go**: Removed erroneous `/bin/sh -c '...'` wrapping in `Start()`. Cmd is now passed directly to `b.Spawn()`. Each backend already does its own `sh -c` wrapping — double-wrapping caused `/bin/sh: 1: <cmd>: not found`.
- **internal/run/run_test.go**: Updated `TestStart_CommandWrappedInShell` → `TestStart_CommandPassedDirectly` to assert the correct (no pre-wrap) behavior.
- **cmd/swarm/commands/run.go**: Added `runStartCmd` subcommand aliasing the root `runStartWait` handler. SKILL docs referenced `swarm run start` which previously fell through Cobra as a positional arg.
- **extras/skills/openswarm/SKILL.md** + **dotfiles copy**: Fixed 4 command syntax mismatches:
  - `agent register <name> <role>` → `agent register <name> --role <role>`
  - `task assign <id> --to <agent>` → `task assign <id> <agent>`
  - `task block <id> --on <id>` → `task block <id> --by <id>`
  - `pane spawn --name <name>` → `pane spawn <name>`
  - `run start --name <name> -- <cmd>` → `run start [--name <name>] -- <cmd>`
- **lat.md/modules.md**: Updated run module description to document the pass-through contract.
- **lat.md/backends.md**: Added shell-wrapping contract note to Backend Interface section.
