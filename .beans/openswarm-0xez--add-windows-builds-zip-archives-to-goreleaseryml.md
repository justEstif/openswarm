---
# openswarm-0xez
title: Add Windows builds + zip archives to .goreleaser.yml
status: completed
type: task
priority: normal
created_at: 2026-03-30T17:37:44Z
updated_at: 2026-03-30T17:37:59Z
---

Update .goreleaser.yml to produce Windows binaries and zip archives for npm postinstall support.

## Summary of Changes\n\n- Added `windows` to the `goos` list under `builds` (now: linux, darwin, windows)\n- Added `format_overrides` to the `archives` block: Windows gets `.zip`, all others get `.tar.gz`\n- YAML validated with Python's yaml.safe_load\n- Did NOT commit (final-qa handles that)
