---
# openswarm-oon3
title: cmd/swarm agent subcommands
status: completed
type: task
priority: normal
created_at: 2026-03-30T14:03:23Z
updated_at: 2026-03-30T14:04:18Z
---

Implement cmd/swarm/commands/agent.go with register/list/get/deregister subcommands and register AgentCmd in root.go

## Summary of Changes

- Created `cmd/swarm/commands/agent.go` with AgentCmd group and 4 subcommands:
  - `register <name> [--role] [--profile]` → calls agent.Register, human/JSON output
  - `list` → calls agent.List, tabwriter table or JSON
  - `get <id-or-name>` → calls agent.Get, key:value dump or JSON
  - `deregister <id-or-name>` → calls agent.Deregister, human/JSON output
- Edited `cmd/swarm/root.go` init() to add `rootCmd.AddCommand(commands.AgentCmd)`
- All tests pass, smoke-tested all subcommands including error paths
