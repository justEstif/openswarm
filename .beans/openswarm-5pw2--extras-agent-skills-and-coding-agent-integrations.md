---
# openswarm-5pw2
title: 'Extras: agent skills and coding agent integrations'
status: completed
type: epic
priority: normal
created_at: 2026-04-02T11:41:22Z
updated_at: 2026-04-02T11:43:41Z
---

Supplementary tools to make openswarm easier to adopt.

- [x] Universal SKILL.md (`extras/skills/openswarm/SKILL.md`)
- [x] Claude Code hooks (`extras/claude-code/`)
- [x] opencode plugin (`extras/opencode/`)
- [x] pi extension (`extras/pi/`)
- [x] `extras/README.md`
- [x] Update lat.md

## Summary of Changes

Created `extras/` with four integrations:
- `skills/openswarm/SKILL.md` — universal agent skill
- `claude-code/` — SessionStart hook + README
- `opencode/index.ts` — plugin: auto-init + compaction context
- `pi/extension.ts` — auto-init + /swarm-status + /swarm-prompt commands

Added `lat.md/extras.md` and linked it from `lat.md/lat.md`. All lat checks pass.
