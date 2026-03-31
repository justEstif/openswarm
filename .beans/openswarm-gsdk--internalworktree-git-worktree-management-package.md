---
# openswarm-gsdk
title: internal/worktree — git worktree management package
status: completed
type: task
priority: normal
created_at: 2026-03-30T16:14:06Z
updated_at: 2026-03-30T16:15:38Z
---

Implement internal/worktree package with New, List, Get, Merge, Clean, CleanAll functions. Wraps git worktree CLI commands, persists records in worktrees.json, emits events.

## Summary of Changes\n\nImplemented internal/worktree package:\n- worktree.go: Types (Worktree, Status, MergeOpts), New, List, Get, Merge, Clean, CleanAll, runGit helper, sanitizeBranch, worktreePath\n- worktree_test.go: TestSanitizeBranch, TestList_empty, TestNew, TestClean, TestGet\n- All tests pass: go build, go vet, go test all clean
