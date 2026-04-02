---
# openswarm-iz0o
title: swarm run pause/resume and stop/continue documentation
status: todo
type: feature
priority: low
created_at: 2026-04-02T20:51:25Z
updated_at: 2026-04-02T20:51:25Z
parent: openswarm-ab7c
---

Phase 1: update extras skill and coordinator prompt to document kill+re-spawn with corrected prompt as the standard stop/continue workflow. Document swarm task update <id> --status todo as the reset/retry primitive. Phase 2 (optional): swarm run pause <id> sends SIGSTOP; swarm run resume <id> sends SIGCONT; status recorded in runs.json. Phase 2 only meaningful for shell workers (not LLM agents).
