---
# openswarm-apo0
title: Dynamic worker capability context in agent profiles
status: todo
type: feature
priority: normal
created_at: 2026-04-02T20:50:53Z
updated_at: 2026-04-02T20:50:53Z
parent: openswarm-wevr
blocked_by:
    - openswarm-7olx
---

Add optional tools list to AgentProfile in config.toml (e.g. ["bash", "file_edit", "mcp:github"]). swarm prompt --mode coordinator incorporates tool lists in worker-tools section. swarm agent list --json exposes the tools field. Extras skill documents how to populate tools for Claude Code, pi, and opencode agents. Closes the gap with Claude Code's getCoordinatorUserContext() tool enumeration.
