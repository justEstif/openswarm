---
# openswarm-wn0o
title: 'cmd: swarm msg subcommands'
status: completed
type: task
priority: normal
created_at: 2026-03-30T14:36:03Z
updated_at: 2026-03-30T14:36:55Z
---

Implement cmd/swarm/commands/msg.go and register it in cmd/swarm/root.go

## Summary of Changes\n\n- Created  with MsgCmd and six subcommands: send, inbox, read, reply, clear, watch\n- Updated  init() to register MsgCmd\n- , , ?   	github.com/justEstif/openswarm/cmd/swarm	[no test files]
?   	github.com/justEstif/openswarm/cmd/swarm/commands	[no test files]
ok  	github.com/justEstif/openswarm/internal/agent	(cached)
ok  	github.com/justEstif/openswarm/internal/config	(cached)
ok  	github.com/justEstif/openswarm/internal/events	(cached)
ok  	github.com/justEstif/openswarm/internal/msg	(cached)
ok  	github.com/justEstif/openswarm/internal/output	(cached)
ok  	github.com/justEstif/openswarm/internal/swarmfs	(cached)
ok  	github.com/justEstif/openswarm/internal/task	(cached) all pass
