---
# openswarm-gpys
title: internal/pane — Backend interface, registry, DetectBackend
status: completed
type: task
priority: normal
created_at: 2026-03-30T15:16:28Z
updated_at: 2026-03-30T15:17:27Z
---

Define the pane abstraction layer. Backend interface + core types, registry with Register/New, DetectBackend cascade, and error types. NO backend implementations.

## Summary of Changes

-  — `Backend` interface (8 methods: Spawn, Send, Capture, Subscribe, List, Close, Wait, Name), plus `PaneID`, `OutputEvent`, `PaneInfo` types
- `internal/pane/registry.go` — `Register()` + `New()` with `sync.RWMutex`-protected driver map
- `internal/pane/detect.go` — `DetectBackend()` with 5-level cascade: $SWARM_BACKEND → cfg.Backend → $TMUX → $WEZTERM_PANE → $ZELLIJ → error
- `internal/pane/errors.go` — `ErrNotSupported`, `ErrPaneNotFound()`, `ErrNoBackend()`

`go build ./internal/pane/...` and `go vet ./internal/pane/...` both pass.
