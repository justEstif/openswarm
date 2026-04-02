---
# openswarm-0mev
title: Fix gofmt formatting in zellij.go
status: in-progress
type: bug
created_at: 2026-04-02T20:39:56Z
updated_at: 2026-04-02T20:39:56Z
---

CI lint job failing: internal/pane/zellij/zellij.go#L171 not properly formatted (gofmt). Missing spaces around + operator in singleQuote function.
