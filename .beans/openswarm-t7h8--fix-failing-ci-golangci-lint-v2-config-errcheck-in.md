---
# openswarm-t7h8
title: 'Fix failing CI: golangci-lint v2 config + errcheck in main.go'
status: in-progress
type: bug
created_at: 2026-03-30T12:52:30Z
updated_at: 2026-03-30T12:52:30Z
---

CI lint job fails because .golangci.yml declares version: "2" but uses v1 keys (linters-settings, issues.exclude-rules). Also fmt.Fprintln unchecked error in main.go triggers errcheck.

- [ ] Fix .golangci.yml to use v2 format (move gofmt/goimports to formatters, rename linters-settings, migrate issues.exclude-rules)
- [ ] Fix unchecked fmt.Fprintln error in cmd/swarm/main.go
