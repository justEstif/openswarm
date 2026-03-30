---
# openswarm-x8t1
title: Implement internal/output package
status: completed
type: task
priority: normal
created_at: 2026-03-30T12:52:13Z
updated_at: 2026-03-30T12:53:55Z
---

Implement internal/output with SwarmError, Print, PrintError. Zero external deps (stdlib only). Establishes --json contract for all commands.

## Todo

- [x] Create internal/output/output.go
- [x] Create internal/output/output_test.go
- [x] Verify tests pass with go test

## Summary of Changes

Created `internal/output/output.go` and `internal/output/output_test.go`.

**output.go** (stdlib only — encoding/json, text/tabwriter, reflect, fmt, os, io, strings):
- `SwarmError` struct with `Code`/`Message` fields + `Error() string`
- Five constructors: `ErrNotFound`, `ErrConflict`, `ErrValidation`, `ErrIO`, `ErrLocked`
- `Print(v any, asJSON bool) error` — JSON to stdout via `json.MarshalIndent`; human output uses `text/tabwriter` for slices (header row from json tags, uppercased) and key:value field dump for single structs; nil is a no-op
- `PrintError(err error, asJSON bool)` — JSON mode emits `{"error":{"code":...,"message":...}}` to stdout; human mode emits to stderr; non-SwarmError inputs are wrapped as IO_ERROR

**output_test.go**: 20 tests covering all code paths — JSON/human for structs, slices, pointers, empty slices, nil inputs, hidden json:"-" fields, stdout/stderr routing, non-SwarmError wrapping.
