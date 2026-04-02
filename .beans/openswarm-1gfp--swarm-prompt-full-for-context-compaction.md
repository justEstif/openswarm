---
# openswarm-1gfp
title: swarm prompt --full for context compaction
status: todo
type: feature
priority: normal
created_at: 2026-04-02T20:51:13Z
updated_at: 2026-04-02T20:51:13Z
parent: openswarm-jf3x
---

swarm prompt --full (both modes) emits extended summary: all in-progress/todo tasks with assignees, all unread inbox messages (subject + first 200 chars), all scratch keys with first line, active runs with status. Normal swarm prompt remains concise (token-efficient for startup). Opencode plugin compaction hook updated to use --full. Pi extension /swarm-prompt passes --full in active coordination sessions.
