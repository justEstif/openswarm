---
# openswarm-sq52
title: internal/run — run tracking package
status: completed
type: task
priority: normal
created_at: 2026-03-30T15:18:17Z
updated_at: 2026-03-30T15:20:14Z
---

Implement internal/run/run.go + internal/run/run_test.go. Run tracking layer on top of pane.Backend. Storage in .swarm/runs/runs.json.

## Summary of Changes

Delivered:
- `internal/run/run.go` — full run tracking package
- `internal/run/run_test.go` — 22 tests, all passing

### What was implemented
- **Types**: `RunStatus` (running/done/failed/killed), `Run` struct with all required JSON tags
- **Start**: spawns pane via `b.Spawn` with `/bin/sh -c 'cmd'` wrapper, persists to runs.json under flock, emits `run.started`
- **Wait**: blocks on `b.Wait`, captures output, detects `<promise>COMPLETE</promise>`, updates runs.json, emits `run.done` or `run.failed`
- **List**: returns all runs sorted newest-first
- **Get**: returns single run by ID, returns `output.ErrNotFound` if missing
- **Kill**: closes pane via `b.Close`, marks run as killed, emits `run.failed`
- **Logs**: returns stored output for completed runs; calls `b.Capture()` live for running runs
- Storage: `swarmfs.WithFileLock` + `swarmfs.AtomicWrite` on all mutations
- Lock file at `filepath.Join(filepath.Dir(root.RunsPath()), ".lock")`

Build checks:
- `go build ./internal/run/...` ✅
- `go vet ./internal/run/...` ✅  
- `go test ./internal/run/...` ✅ (22/22 tests pass)
