---
# openswarm-le7g
title: 'Investigate: CI Lint failure - golangci-lint v1 vs v2 config schema mismatch'
status: completed
type: bug
priority: normal
created_at: 2026-03-31T11:46:03Z
updated_at: 2026-03-31T11:46:10Z
---

Investigating the root cause of repeated CI failures on justEstif/openswarm. All 5 most recent runs on main are failing.

## Investigation Findings

### CI Run History (last 5 runs — all failures)
| Run ID       | Branch | Created              | Conclusion |
|--------------|--------|----------------------|------------|
| 23759420108  | main   | 2026-03-30T17:52:52Z | failure    |
| 23759170924  | main   | 2026-03-30T17:47:19Z | failure    |
| 23758807770  | main   | 2026-03-30T17:39:04Z | failure    |
| 23758461442  | main   | 2026-03-30T17:31:19Z | failure    |
| 23758150234  | main   | 2026-03-30T17:24:27Z | failure    |

### Root Cause
golangci-lint v2 config format in .golangci.yml is incompatible with golangci-lint v1.64.8 resolved by `version: latest` in ci.yml.

### Structured Summary
See final output in the debugger agent report.
