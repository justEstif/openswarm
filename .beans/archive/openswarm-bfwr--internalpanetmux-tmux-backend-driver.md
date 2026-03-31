---
# openswarm-bfwr
title: internal/pane/tmux — tmux backend driver
status: completed
type: task
priority: normal
created_at: 2026-03-30T15:17:57Z
updated_at: 2026-03-30T15:21:46Z
---

Implement the tmux backend driver for the pane multiplexer abstraction. Delivers internal/pane/tmux/tmux.go and internal/pane/tmux/tmux_test.go

## Todo
- [x] Create internal/pane/tmux/tmux.go
- [x] Create internal/pane/tmux/tmux_test.go
- [x] go build ./internal/pane/tmux/... — clean
- [x] go vet ./internal/pane/tmux/... — clean
- [x] go test ./internal/pane/tmux/... — 16 pass, 4 skip (no tmux)

## Summary of Changes
Implemented the tmux backend driver at internal/pane/tmux/tmux.go + tmux_test.go.

### tmux.go
- init() registers "tmux" driver in pane.Registry
- TmuxBackend struct implementing all 8 Backend interface methods
- buildEnvCmd(): builds 'env K=V... sh -c CMD' prefix with POSIX single-quoting
- singleQuote(): escapes single quotes using POSIX '\'' pattern
- parseListOutput(): parses tab-separated list-panes output into []PaneInfo
- Spawn: uses new-window (inside TMUX) or new-session (outside), sets remain-on-exit
- Send: send-keys -l for literal text
- Capture: capture-pane -p -S -500
- Subscribe: polling goroutine at 200ms, diff-based delta, pane_dead exit detection
- List: list-panes -a with pane_id/window_name/pane_dead/pane_current_command
- Close: kill-pane, idempotent (ignores 'no pane' errors)
- Wait: polls pane_dead at 200ms, reads pane_dead_status on exit

### tmux_test.go
- Package tmux (white-box) for access to unexported helpers
- TestName, TestRegistered (no tmux needed)
- TestBuildEnvCmd_* (5 cases), TestParseListOutput_* (6 cases), TestSingleQuote_* (3 cases)
- Integration tests (Spawn/Close, Send/Capture, List, Wait) skip when tmux not found
