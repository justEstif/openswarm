---
# openswarm-7gea
title: cmd/swarm — cobra root, swarm init, swarm version
status: completed
type: task
priority: normal
created_at: 2026-03-30T13:01:46Z
updated_at: 2026-03-30T13:03:52Z
---

Wire up the cmd/swarm cobra command tree. Deliver: cmd/swarm/main.go, cmd/swarm/root.go, cmd/swarm/commands/init.go, cmd/swarm/commands/version.go

## Summary of Changes

Implemented the full cobra command tree for cmd/swarm:

### Files delivered
- **cmd/swarm/main.go** — 10 lines, calls Execute(), handles exit code
- **cmd/swarm/root.go** — rootCmd with --json persistent flag, Execute(), mustRoot(), jsonFlag() documented helpers, init() registers subcommands  
- **cmd/swarm/commands/helpers.go** — package-private jsonFlag() and mustRoot() for use by all command handlers (Go prohibits importing package main)
- **cmd/swarm/commands/init.go** — swarm init: idempotent InitRoot, human/JSON output, Already initialized detection
- **cmd/swarm/commands/version.go** — swarm version: ldflags-injectable version/commit/date vars, human/JSON output

### Also updated
- **.goreleaser.yml** — fixed ldflags to target full package path github.com/justEstif/openswarm/cmd/swarm/commands.*
- **go.mod/go.sum** — added github.com/spf13/cobra v1.10.2

### Phase 0 gate verified ✅
- go build ./... ✅
- go vet ./... ✅  
- go test ./... ✅ (all 43 tests pass)
- swarm init → Initialized .swarm/ in /tmp/swarm-test ✅
- swarm init (again) → Already initialized ✅
- swarm init --json → {"path": "/tmp/swarm-test/.swarm"} ✅
- swarm version → swarm dev (none, unknown) ✅
- swarm version --json → {"version":"dev","commit":"none","date":"unknown"} ✅
