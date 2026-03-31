---
# openswarm-filn
title: Set up homebrew-tap repo and GoReleaser integration
status: completed
type: task
priority: normal
created_at: 2026-03-31T12:03:20Z
updated_at: 2026-03-31T12:12:12Z
---

Create justEstif/homebrew-tap on GitHub, add brews section to .goreleaser.yml, set HOMEBREW_TAP_GITHUB_TOKEN secret, update release workflow.

## Summary of Changes\n\n- Created https://github.com/justEstif/homebrew-tap (public repo)\n- Added Formula/ directory with .gitkeep placeholder\n- Added README with install instructions\n- Added HOMEBREW_TAP_GITHUB_TOKEN secret to openswarm repo\n- Added brews block to .goreleaser.yml\n- Passed HOMEBREW_TAP_GITHUB_TOKEN through release.yml to GoReleaser

## Final Notes\n\n- Formula committed to homebrew-tap root (not Formula/) by GoReleaser — that's fine, brew works with root-level  files\n- GoReleaser warns  is deprecated in favor of  — non-blocking for now, can migrate later\n- First release v0.1.0 shipped with 6 platform binaries
