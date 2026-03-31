---
# openswarm-cpg1
title: internal/agent — agent registry
status: completed
type: task
priority: normal
created_at: 2026-03-30T14:01:22Z
updated_at: 2026-03-30T14:02:42Z
---

Implement internal/agent package with Register, List, Get, Deregister functions. Storage: registry.json with flock. Events emitted on mutations.

## Summary of Changes

Delivered  and .

### What was implemented
-  struct with ID, Name, Role, ProfileRef, CreatedAt
-  — flock → readAll → duplicate check → append → writeAll → events.Append; returns CONFLICT on duplicate name
-  — returns []*Agent sorted by CreatedAt ascending (secondary sort by ID for determinism)
-  — resolves by ID or name; returns NOT_FOUND if missing
-  — flock → find → remove → writeAll → events.Append; returns NOT_FOUND if missing
-  /  internal helpers; empty registry serialises as  not 
- Lock file at ; events use  / 

### Tests (12/12 pass)
Register happy path, duplicate CONFLICT, with profile; List empty + sorted; Get by ID, by name, missing NOT_FOUND; Deregister happy path, by name, leaves others intact, missing NOT_FOUND.
