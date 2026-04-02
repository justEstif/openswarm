---
# openswarm-bykv
title: 'Pane placement: config + CLI for current_tab / new_tab / new_session'
status: in-progress
type: feature
priority: high
created_at: 2026-04-02T20:06:28Z
updated_at: 2026-04-02T20:06:28Z
---

Add pane_placement config key and --placement CLI flag. Backends spawn in current tab (split), new tab, or new session. Each placement mode injects a cleanup trailer so the container (tab/session) self-closes when the command exits. CloseOnExit=!wait so --wait mode still captures output.
