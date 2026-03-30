---
# openswarm-x8t1
title: Implement internal/output package
status: in-progress
type: task
priority: normal
created_at: 2026-03-30T12:52:13Z
updated_at: 2026-03-30T12:52:18Z
---

Implement internal/output with SwarmError, Print, PrintError. Zero external deps (stdlib only). Establishes --json contract for all commands.

## Todo

- [ ] Create internal/output/output.go
- [ ] Create internal/output/output_test.go
- [ ] Verify tests pass with go test
