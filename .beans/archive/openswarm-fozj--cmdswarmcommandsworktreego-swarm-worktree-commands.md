---
# openswarm-fozj
title: cmd/swarm/commands/worktree.go — swarm worktree commands
status: completed
type: task
priority: normal
created_at: 2026-03-30T16:16:47Z
updated_at: 2026-03-30T16:18:09Z
---

Implement swarm worktree cobra command group. Thin handlers only. Commands: new, list, get, merge, clean, clean-all.

## Summary of Changes\n\nDelivered cmd/swarm/commands/worktree.go with all 6 subcommands:\n- worktree new (--branch required, --agent optional)\n- worktree list (table: ID | BRANCH | STATUS | AGENT | CREATED_AT)\n- worktree get <id>\n- worktree merge <id> (--squash, --delete-branch)\n- worktree clean <id>\n- worktree clean-all\n\nAll handlers ≤ 15 lines. go build + go vet both pass.
