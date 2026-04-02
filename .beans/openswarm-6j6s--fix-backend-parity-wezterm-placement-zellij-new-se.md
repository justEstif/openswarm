---
# openswarm-6j6s
title: 'Fix backend parity: WezTerm Placement, Zellij new_session, env quoting'
status: completed
type: bug
priority: high
created_at: 2026-04-02T20:21:14Z
updated_at: 2026-04-02T20:23:41Z
---

Three issues found in design review: (1) WezTerm ignores Placement+CloseOnExit, (2) Zellij new_session returns unusable PaneID via dead listPaneIDs(), (3) Zellij buildEnvCmd doesn't quote env values.

## Summary of Changes
- Zellij `buildEnvCmd`: added `singleQuote(v)` to all env values (bug: spaces/special chars were unquoted)
- Zellij `spawnNewTab`: merged two `list-tabs --json` calls into single `listTabsSnapshot()`
- Zellij `spawnNewSession`: removed bogus `listPaneIDs/findNewPaneID` (always looked at wrong session); returns synthetic ID directly with documented limitation
- Removed dead helpers: `listPaneIDs()`, `findNewPaneID()`
- WezTerm `Spawn()`: now maps `PlacementNewTab`/`PlacementNewSession` → `--new-window`; documents `CloseOnExit` as naturally handled
- Updated `lat.md/backends.md` Backend Coverage table with full Placement + CloseOnExit parity grid
- Added test cases for quoted env values (spaces, embedded single quotes)
