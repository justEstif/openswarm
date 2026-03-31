---
# openswarm-t7h8
title: 'Fix failing CI: golangci-lint v2 config + errcheck in main.go'
status: completed
type: bug
priority: normal
created_at: 2026-03-30T12:52:30Z
updated_at: 2026-03-30T12:53:31Z
---

CI lint job fails because .golangci.yml declares version: "2" but uses v1 keys (linters-settings, issues.exclude-rules). Also fmt.Fprintln unchecked error in main.go triggers errcheck.

- [x] Fix .golangci.yml to use v2 format (move gofmt/goimports to formatters, rename linters-settings, migrate issues.exclude-rules)
- [x] Fix unchecked fmt.Fprintln error in cmd/swarm/main.go
