---
# openswarm-ycje
title: .swarm/scratch/ free-form shared workspace
status: todo
type: feature
priority: high
created_at: 2026-04-02T20:51:00Z
updated_at: 2026-04-02T20:51:00Z
parent: openswarm-skzm
---

swarm init creates .swarm/scratch/. Commands: swarm scratch write <key> [--body|-], swarm scratch read <key>, swarm scratch list, swarm scratch rm <key>. AtomicWrite for same-key concurrent safety; lock-free across different keys. swarmfs exposes Root.ScratchPath(key) and Root.ScratchDir(). Coordinator prompt (item 3) mentions scratch dir and encourages workers to write research summaries there. Closes the gap with Claude Code's scratchpadDir injection.
