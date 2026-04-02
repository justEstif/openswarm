---
# openswarm-3wuh
title: swarm session subcommand and mode tracking
status: todo
type: feature
priority: normal
created_at: 2026-04-02T20:50:53Z
updated_at: 2026-04-02T20:50:53Z
parent: openswarm-wevr
---

Add .swarm/session.json: { mode: coordinator|worker, last_agent, updated_at }. swarm session set --mode coordinator writes it. swarm session get prints current mode. swarm prompt (no flags) reads session mode and auto-selects the prompt template. Extras hooks call swarm session get at startup and inject the correct mode. Closes the gap with Claude Code's matchSessionMode() and stored sessionMode field.
