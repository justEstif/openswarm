---
# openswarm-filn
title: Set up homebrew-tap repo and GoReleaser integration
status: completed
type: task
priority: normal
created_at: 2026-03-31T12:03:20Z
updated_at: 2026-03-31T12:04:18Z
---

Create justEstif/homebrew-tap on GitHub, add brews section to .goreleaser.yml, set HOMEBREW_TAP_GITHUB_TOKEN secret, update release workflow.

## Summary of Changes\n\n- Created https://github.com/justEstif/homebrew-tap (public repo)\n- Added Formula/ directory with .gitkeep placeholder\n- Added README with install instructions\n- Added HOMEBREW_TAP_GITHUB_TOKEN secret to openswarm repo\n- Added brews block to .goreleaser.yml\n- Passed HOMEBREW_TAP_GITHUB_TOKEN through release.yml to GoReleaser
