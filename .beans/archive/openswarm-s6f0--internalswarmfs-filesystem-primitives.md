---
# openswarm-s6f0
title: internal/swarmfs — filesystem primitives
status: completed
type: task
priority: normal
created_at: 2026-03-30T12:52:08Z
updated_at: 2026-03-30T12:54:28Z
---

Implement internal/swarmfs package: Root type + path methods, FindRoot, InitRoot, AtomicWrite, AppendLine, WithFileLock, NewID. Also add cobra dependency.

## Todo
- [x] Add cobra dependency (go get github.com/spf13/cobra@latest && go mod tidy)
- [x] Create internal/swarmfs/swarmfs.go
- [x] Create internal/swarmfs/swarmfs_test.go
- [x] Verify all tests pass (16/16 PASS, -race clean)

## Summary of Changes

- Added `github.com/spf13/cobra v1.10.2` (+ pflag, mousetrap) to go.mod/go.sum via `go get github.com/spf13/cobra@latest && go mod tidy`
- Created `internal/swarmfs/swarmfs.go` with:
  - `Root` struct + 7 path methods (TasksPath, TasksLockPath, AgentsPath, InboxPath, RunsPath, WorktreesPath, EventsPath)
  - `FindRoot()` — walks up from cwd, returns actionable error if not found
  - `InitRoot(base)` — idempotent directory creation + config.toml touch
  - `AtomicWrite(path, data)` — temp-file + os.Rename, creates parent dirs
  - `AppendLine(path, data)` — O_APPEND write, creates file+dirs on first call
  - `WithFileLock(lockPath, fn)` — syscall.Flock(LOCK_EX), creates lock file
  - `NewID(prefix)` — crypto/rand, format `<prefix>-<6 alphanum chars>`
- Created `internal/swarmfs/swarmfs_test.go` with 16 tests covering all functions including concurrent lock test (20 goroutines, -race clean)
