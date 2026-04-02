---
# openswarm-abwp
title: Setup .swarm/config.toml and end-to-end local test
status: completed
type: task
priority: normal
created_at: 2026-04-02T12:59:57Z
updated_at: 2026-04-02T13:20:32Z
---

Create a proper .swarm/config.toml for the openswarm project with team name, backend, agent profiles, then run an end-to-end test of the CLI.

## Summary of Changes

- Created `.swarm/config.toml` with team_name, backend (zellij), poll_interval, and two agent profiles (coder/pi, reviewer/claude)
- Diagnosed and fixed stale binary: installed binary was from March 30 and lacked `events.Last` — rebuilt from source and installed to mise path
- End-to-end test passed: agent register/list, task create/assign/done, msg send/read, events tail all work correctly
