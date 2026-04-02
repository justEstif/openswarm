---
# openswarm-ds20
title: 'Pane placement: config + CLI for current_tab / new_tab / new_session'
status: completed
type: feature
priority: high
created_at: 2026-04-02T20:06:32Z
updated_at: 2026-04-02T20:10:24Z
---

Add pane_placement config key and --placement CLI flag.

## Summary of Changes

- Added `Placement` type + constants to `internal/pane/backend.go` (`SpawnOptions`)
- Added `[pane] placement` to config + `SWARM_PANE_PLACEMENT` env override
- Zellij: `current_tab` (new-pane -c), `new_tab` (new-tab + id discovery + close-tab-by-id trailer), `new_session` (zellij run + kill-session trailer)
- tmux: `current_tab` (split-window), `new_tab` (new-window), `new_session` (new-session), cleanup via kill-pane/kill-window/kill-session trailers
- `run.Start()` now accepts `pane.SpawnOptions` directly; `CloseOnExit = !wait`
- CLI: `--placement` flag on `swarm run [start]` and `swarm pane spawn`; config default respected
