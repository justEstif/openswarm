---
# openswarm-0mev
title: Fix gofmt formatting in zellij.go
status: completed
type: bug
priority: normal
created_at: 2026-04-02T20:39:56Z
updated_at: 2026-04-02T20:40:16Z
---

CI lint job failing: internal/pane/zellij/zellij.go#L171 not properly formatted (gofmt). Missing spaces around + operator in singleQuote function.

## Summary of Changes\n\nRan `gofmt -w` on the file. Single-line fix: added missing spaces around `+` operator in the `singleQuote` function at line 171.
