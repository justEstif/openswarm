---
# openswarm-df9s
title: internal/pane/zellij — Zellij backend driver
status: completed
type: task
priority: normal
created_at: 2026-03-30T15:18:09Z
updated_at: 2026-03-30T15:19:59Z
---

Implement the Zellij backend driver for the pane multiplexer abstraction. Delivers: internal/pane/zellij/zellij.go + internal/pane/zellij/zellij_test.go

## Summary of Changes

Implemented `internal/pane/zellij` — full Zellij v0.44.0+ backend driver.

### Files delivered
- `internal/pane/zellij/zellij.go` — ZellijBackend implementing all 8 pane.Backend methods
- `internal/pane/zellij/zellij_test.go` — 12 tests (11 pass, 1 skipped as integration)

### Key design decisions
- `paneInt(id)` uses `strings.CutPrefix` to strip `terminal_` prefix for `--pane-id` flags
- `Spawn` captures stdout for pane ID; falls back to `list-panes --json` name-match for pre-v0.44.0
- `buildEnvCmd` constructs `env K=V K=V <cmd>` prefix for environment injection
- `Subscribe` uses native `zellij subscribe --format json` NDJSON stream; falls back to 200ms polling
- `parsePaneList` extracted as standalone function for unit testing without Zellij
- `Wait` polls `list-panes --json` every 200ms; returns -1 if pane disappears before exit_status appears
- `Close` ignores 'not found' / 'no pane' errors for idempotency

### Build verification
```
go build ./internal/pane/zellij/...  ✅
go vet ./internal/pane/zellij/...    ✅
go test ./internal/pane/zellij/...   ✅ PASS (12 tests, 0.002s)
```
