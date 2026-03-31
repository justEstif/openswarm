---
# openswarm-zpec
title: WezTerm backend and Ghostty stub
status: completed
type: task
priority: normal
created_at: 2026-03-30T15:18:05Z
updated_at: 2026-03-30T15:21:10Z
---

Implement internal/pane/wezterm/wezterm.go + wezterm_test.go and internal/pane/ghostty/ghostty.go + ghostty_test.go

## Summary of Changes

Delivered 4 files:

### internal/pane/wezterm/wezterm.go
- WeztermBackend implementing all 8 pane.Backend methods
- Spawn: wraps cmd in `sh -c 'export K=V; exec cmd'` for env injection
- Send: uses `wezterm cli send-text --pane-id N --no-paste`
- Capture: uses `wezterm cli get-text --pane-id N --start-line -200`
- Subscribe: 200ms polling loop, exit detection via list JSON
- List: parses `wezterm cli list --format json` (tab_title → title fallback)
- Close: idempotent kill-pane (non-zero exit ignored)
- Wait: polls list every 200ms until pane gone, returns Code=-1 (documented limitation)
- init() registers under 'wezterm' key

### internal/pane/wezterm/export_test.go
- Exports parseWeztermList for external test package

### internal/pane/wezterm/wezterm_test.go
- Unit tests for JSON parsing (valid fixture, empty, invalid)
- Integration tests skip if wezterm not in PATH or WEZTERM_PANE unset
- Tests: registration, Name(), Spawn+Close, List, Subscribe cancel, error cases

### internal/pane/ghostty/ghostty.go
- GhosttyBackend stub: all 8 methods return ErrNotSupported
- References upstream issue #4625
- init() registers under 'ghostty' key

### internal/pane/ghostty/ghostty_test.go  
- Tests all 8 methods wrap pane.ErrNotSupported (via errors.Is)
- Tests Name() returns 'ghostty'
- Tests registration via pane.New()
