---
# openswarm-6s42
title: 'cmd: swarm task subcommands'
status: completed
type: task
priority: normal
created_at: 2026-03-30T14:10:52Z
updated_at: 2026-03-30T14:15:12Z
---

Implement internal/task package and cmd/swarm/commands/task.go with all task subcommands, register in root.go

## Summary of Changes

### internal/task/task.go
- Task, Status, Priority, AddOpts, UpdateOpts, ListFilter, Problem types
- Add, List, Get, Update, Assign, Claim, Done, Fail, Cancel, Block, Unblock, Remove, Check, Prompt functions
- ETag-based optimistic locking, flock on .swarm/tasks/.lock, events on every mutation

### internal/task/task_test.go
- 7 tests covering Add, List, Get, Claim, Done, Block, Update (ETag conflict), Check (fix)

### cmd/swarm/commands/task.go
- TaskCmd group with 12 subcommands: add, list, get, update, assign, claim, done, fail, cancel, block, check, prompt
- Human and JSON output modes throughout

### cmd/swarm/root.go
- Added rootCmd.AddCommand(commands.TaskCmd)
