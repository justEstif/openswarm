---
# openswarm-h6kt
title: 'Research: OSS AI orchestration frameworks (LangGraph, CrewAI, Smolagents, AG2, etc.)'
status: completed
type: task
priority: normal
created_at: 2026-04-02T19:25:20Z
updated_at: 2026-04-02T19:48:49Z
---

Research the open-source AI orchestration landscape. Cover LangGraph, CrewAI, Smolagents, AG2, Bee Agent Framework, Pydantic AI, and other notable OSS frameworks. Write report to .swarm/research/oss-frameworks.md

## Progress\n- [x] LangGraph research\n- [x] CrewAI research\n- [x] Smolagents research\n- [x] AG2 research\n- [x] Bee Agent Framework research\n- [x] OpenSwarm research\n- [x] Write report to .swarm/research/oss-frameworks.md

## Summary of Changes

Wrote comprehensive OSS AI orchestration framework research report to .swarm/research/oss-frameworks.md (~32KB).

Coverage:
- **LangGraph**: graph state machine, checkpoint persistence, interrupt() HITL, LangGraph Platform GA (2025), 28K stars
- **CrewAI**: role/backstory agents, sequential/hierarchical/consensus processes, Flows + triggers, 47.9K stars (most adopted)
- **Smolagents**: code-as-action (not JSON), ~1K LOC core, HF Hub integration, multi-modal, 26K stars
- **AG2**: community fork of AutoGen 0.2 after Microsoft merged AutoGen → Agent Framework + Semantic Kernel; full backward compat, 4.3K stars on fork
- **BeeAI**: IBM Research, RequirementAgent for behavioral constraints, ACP/A2A (Linux Foundation Aug 2025), dual Python+TS, 3.2K stars
- **OpenSwarm**: file-backed coordination substrate, no-daemon design, Go CLI, flock+inbox patterns from Claude Code's internals

Key ecosystem trends noted: MCP ubiquity, A2A as next frontier, code-agent evidence accumulating, production-readiness as competitive moat.
