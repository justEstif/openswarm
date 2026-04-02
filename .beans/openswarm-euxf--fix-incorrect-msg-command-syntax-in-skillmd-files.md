---
# openswarm-euxf
title: Fix incorrect msg command syntax in SKILL.md files
status: completed
type: bug
priority: normal
created_at: 2026-04-02T13:23:02Z
updated_at: 2026-04-02T13:23:42Z
---

msg send/read/reply all have wrong argument syntax in extras/skills/openswarm/SKILL.md and the installed skill. Found during local e2e testing.

## Summary of Changes

Fixed in both `extras/skills/openswarm/SKILL.md` and the installed `~/.pi/agent/skills/openswarm/SKILL.md`:

- `msg send`: was `send <agent-id> "text"` → now `send <agent> --subject "..." --body "..."`
- `msg read`: was `read <msg-id>` → now `read <agent> <msg-id>`
- `msg reply`: was `reply <msg-id> "text"` → now `reply <agent> <msg-id> --body "..."`
- `task check`: corrected description from 'check for actionable tasks' to 'check task store integrity'

Also added a note to lat.md/extras.md documenting the msg command argument requirements.
