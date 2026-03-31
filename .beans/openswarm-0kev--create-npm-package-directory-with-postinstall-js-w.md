---
# openswarm-0kev
title: Create npm/ package directory with postinstall + JS wrapper
status: completed
type: task
priority: normal
created_at: 2026-03-30T17:37:45Z
updated_at: 2026-03-30T17:38:15Z
---

Create the npm/ directory with package.json, postinstall.js, bin/swarm.js, and README.md for publishing openswarm to npm.

## Summary of Changes\n\nCreated npm/ directory with all required files:\n- npm/package.json — package manifest for openswarm npm package\n- npm/bin/swarm.js — thin Node.js wrapper that exec's the Go binary\n- npm/postinstall.js — downloads correct platform binary from GitHub Releases (zero external deps)\n- npm/README.md — short install/usage docs\n\nBoth JS files pass node --check syntax validation.
