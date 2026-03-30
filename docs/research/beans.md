# Beans CLI Research Report

**Source**: https://github.com/hmans/beans  
**Cloned**: `/tmp/beans-research`  
**Research date**: 2026-03-30  
**Purpose**: Find UX patterns and features worth borrowing into `swarm task`

---

## Overview

Beans is a Go CLI task tracker that stores issues as individual Markdown files with YAML front-matter inside a `.beans/` directory. Its tagline: _"A CLI-based, flat-file issue tracker for humans and robots."_ It is explicitly designed for both human and AI-agent use. The project has a web UI, TUI, and GraphQL API on top of the core CLI.

Key stats: actively developed (~186 open issues at time of research), structured as a monorepo with `beans` CLI binary, `beans-tui` TUI binary, and `beans-serve` web server binary. Written in Go with Cobra for CLI, Bleve for full-text search, gqlgen for GraphQL.

---

## 1. Command Surface

### Beans commands vs. `swarm task`

| Beans command | Alias(es) | swarm task equivalent |
|---|---|---|
| `init` | — | (implicit on first use) |
| `create <title>` | `c`, `new` | `swarm task add` |
| `list` | `ls` | `swarm task list` |
| `show <id> [id…]` | — | *(missing — we only have list)* |
| `update <id>` | `u` | `swarm task status/assign` (fragmented) |
| `archive` | — | *(missing)* |
| `delete <id> [id…]` | `rm` | `swarm task rm` |
| `check [--fix]` | — | *(missing)* |
| `roadmap` | — | *(out of scope)* |
| `prime` | — | *(no equivalent)* |
| `graphql` | `query` | *(out of scope for MVP)* |
| `version` | — | standard |
| `tui` | — | *(out of scope)* |
| `serve` | — | *(out of scope)* |

### Commands beans doesn't have but `swarm task` does
- `assign` — beans uses `update --assigned-agent` (or equivalent update flag)
- `done` / `fail` — beans uses `update --status completed/scrapped`
- `watch` — beans relies on TUI/web UI for real-time updates
- `block` / `unblock` — beans uses `update --blocking/--remove-blocking`
- `clear` — (no equivalent in beans; not really needed)

### Commands beans has that `swarm task` is missing

**Worth borrowing:**
- **`show <id> [id…]`** — detailed single-task view separate from list. Supports `--raw`, `--body-only`, `--etag-only`, `--json`. Much more useful than making `list` do everything. Agents call `list --json` to find IDs, then `show --json <id>` to read full details.
- **`check [--fix]`** — validates referential integrity (broken blocking links, cycles, self-references). Auto-fixes broken links with `--fix`. Very useful in multi-agent environments where agents create/delete tasks concurrently.
- **`prime`** — outputs agent-priming instructions in one shot. We should have `swarm task prompt` or similar.

**Not relevant:**
- `roadmap` — project overview generator from milestones/epics hierarchy. We don't have a type hierarchy.
- `graphql` — full GraphQL query engine. Overkill for our use case; `--json` flags on all commands are sufficient.
- `tui` / `serve` — interactive UI. Out of scope for `swarm task`.
- `init` — beans needs explicit init because it creates config files. Our `.swarm/tasks/tasks.json` can be created lazily.

---

## 2. Storage Format

### Beans storage
- **One Markdown file per task** in `.beans/` directory
- **Filename format**: `{prefix}{nanoid}--{slug}.md`  
  Example: `beans-abc1--implement-user-login.md`
- **ID generation**: NanoID using base-36 alphabet (`0-9a-z`), configurable length (default 4 chars), configurable prefix (e.g. `beans-`, `myproject-`)
- **Blocked-words filter**: generated IDs are checked against an offensive word list and regenerated if needed
- **Slug**: auto-generated from title via `Slugify()`, max 50 chars
- **Archive**: completed/scrapped items move to `.beans/archive/` subdirectory, still queryable

**Bean file format** (YAML front-matter + Markdown body):
```markdown
---
# beans-0ajg  ← comment with ID for humans
title: Implement user login
status: in-progress
type: feature
priority: high
tags: [auth, backend]
created_at: 2025-12-27T21:44:04Z
updated_at: 2026-03-07T23:10:48Z
order: VV           ← fractional index for manual sort ordering
parent: beans-mmyp  ← parent task ID
blocking: [beans-xyz1, beans-xyz2]
blocked_by: [beans-abc2]
---

Description here. Full Markdown body.

## Implementation Plan
- [ ] Set up auth middleware
- [x] Write login endpoint
```

### Comparison with `swarm task`
| Aspect | Beans (one file per task) | swarm task (tasks.json) |
|---|---|---|
| Git diff | Clean, per-task diffs | All changes in one file |
| Human readability | Open any `.md` in editor | Need to parse JSON |
| Concurrent writes | No locking needed (separate files) | flock() mutex required |
| Query performance | Load all files on startup | One JSON parse |
| Body/notes | First-class Markdown body | `output` field (string) |
| File browsing | `ls .beans/` shows task list | Not useful |

**Worth borrowing:** The slug-in-filename idea (`id--slug`) is clever even if we keep JSON storage. Not directly applicable, but the ID comment trick (embedding the ID as a YAML comment at top of front-matter) is a nice "file is self-describing" pattern.

**Not relevant:** One-file-per-task storage. We've decided to own the implementation with `tasks.json`. Our flock-based atomic write is the right call for multi-agent concurrent access.

---

## 3. UX Patterns

### 3a. `--quiet` / `-q` flag on list
```bash
beans list --quiet          # prints IDs only, one per line
beans list -q -s in-progress | xargs beans show --json
```
Dead simple, extremely useful for piping. Agents can chain commands without JSON parsing.

**Worth borrowing** into `swarm task list -q`.

### 3b. `--body-only` and `--etag-only` on show
```bash
beans show --body-only abc1    # just the markdown body
beans show --etag-only abc1    # just the ETag hash (for --if-match)
```
`--etag-only` is specifically designed for scripting optimistic lock workflows:
```bash
ETAG=$(beans show abc1 --etag-only)
beans update abc1 --if-match "$ETAG" --status completed
```

**Worth borrowing** as `swarm task show --output-only` (our `output` field). The `--etag-only` equivalent is less relevant since we use flock rather than content-hash optimistic locking.

### 3c. Stdin reading with `-`
```bash
cat notes.md | beans create "Big task" --body -
echo "## Done" | beans update abc1 --body-append -
```
`-` as a value for `--body`, `--body-file`, `--body-append` reads from stdin.

**Worth borrowing** for `swarm task add --output -`.

### 3d. `--full` flag on list (body excluded by default)
```bash
beans list --json           # body field is empty string (saves tokens!)
beans list --json --full    # body field included
```
By default, `list --json` strips the body to reduce token consumption when agents are scanning many tasks. Agents fetch body only when they need it via `show`.

**Worth borrowing** — we should exclude `output` field from `list --json` by default and add `--full` to include it. This is a significant token-saving measure for agents scanning many tasks.

### 3e. Sort options on list
```bash
beans list --sort created      # newest first
beans list --sort updated      # recently updated first
beans list --sort status       # grouped by status
beans list --sort priority     # highest priority first
beans list --sort id           # lexicographic by ID
# default: status order → priority → type → title
```

**Worth borrowing** — `swarm task list --sort priority|created|updated|status`. Particularly `--sort priority` is useful when an agent needs to pick what to work on next.

### 3f. `--ready` compound filter
```bash
beans list --ready    # not blocked + excludes in-progress/completed/scrapped/draft
                      # + excludes tasks whose parent is in a terminal state
```
Answers "what can I work on right now?" in a single command. Agents don't have to construct complex filter combinations.

**Worth borrowing** as `swarm task list --actionable` or `--ready`. Specifically: exclude `in-progress`, `done`, `failed`, exclude tasks that are blocked, exclude tasks whose `blocked_by` list has any non-done/non-failed tasks.

### 3g. Include/exclude variants for every filter dimension
```bash
beans list --status todo --status in-progress    # include (OR logic, repeatable)
beans list --no-status completed                 # exclude
beans list --tag auth --tag backend              # include
beans list --no-tag wip                          # exclude
beans list --priority critical --priority high   # include
beans list --no-priority deferred                # exclude
```
The `--no-*` exclusion pattern is clean and covers the "show me everything except…" use case without requiring the user to enumerate all the values they want.

**Worth borrowing** — add `--exclude-status`, `--exclude-agent` variants to `swarm task list`.

### 3h. `--type <type>` on list and create
Beans supports task types (`milestone`, `epic`, `bug`, `feature`, `task`) that can be filtered on. Not relevant for us (we don't have a type hierarchy), but the filtering mechanics are the model.

**Not relevant** for `swarm task` MVP.

### 3i. Body modification flags on update (surgical patches)
```bash
# Replace exact text (errors if not found or found >1 time)
beans update abc1 --body-replace-old "- [ ] Deploy" --body-replace-new "- [x] Deploy"

# Append to body
beans update abc1 --body-append "## Summary\n\nCompleted via PR #42"

# Combine with status change (atomic)
beans update abc1 \
  --body-replace-old "- [ ] Deploy" --body-replace-new "- [x] Deploy" \
  --status completed
```
Agents can update task progress notes in-place without reading and rewriting the full body. The `--body-replace-old` is required to match exactly once — this is a safety mechanism that prevents accidental multi-replace bugs.

**Worth borrowing** — `swarm task update <id> --output-append "text"` and optionally `--output-replace-old/new`. The "must match exactly once" constraint on replace is clever and safe.

### 3j. Multi-ID show
```bash
beans show abc1 abc2 abc3    # show multiple tasks at once
```
Reduces round-trips when an agent needs to inspect several tasks found via `list`.

**Worth borrowing** — `swarm task show <id> [id…]` accepting multiple IDs.

### 3k. Delete warns about incoming links
```bash
beans delete abc1
# → Warning: 2 bean(s) link to 'Implement login':
#     - beans-xyz1 (auth flow) via blocking
#   Delete anyway and remove references? [y/N]
```
Before deleting, beans checks which other tasks reference the target and warns the user. `--force` or `--json` skips the prompt.

**Worth borrowing** — `swarm task rm <id>` should warn if other tasks have this ID in their `blocked_by` list, and offer to clean those references.

### 3j. Tree view as default for list
```bash
beans list           # shows hierarchical tree (parent → child indented)
beans list --json    # flat array (no tree)
```
Human output is tree view; JSON output is flat for programmatic parsing. Smart separation of concerns.

**Not directly relevant** since we don't have a parent-child hierarchy, but the pattern of "human view ≠ JSON view" is good hygiene.

---

## 4. Status Model

### Beans statuses (ordered by priority in UI)
| Status | Color | Description | Archive? |
|---|---|---|---|
| `in-progress` | yellow | Currently being worked on | no |
| `todo` | green | Ready to be worked on | no |
| `draft` | blue | Needs refinement before it can be worked on | no |
| `completed` | gray | Finished successfully | **yes** |
| `scrapped` | gray | Will not be done | **yes** |

Key differences from our `todo/in-progress/done/failed`:

1. **`draft`** — a planning status meaning "not ready to work on yet". Maps to our use case when an agent creates a placeholder task but hasn't fleshed it out. We could add this.

2. **`scrapped`** vs **`failed`** — Beans distinguishes intentional abandonment (`scrapped`: "will not be done") from our `failed` which implies an error or unexpected problem. These are semantically different:
   - `failed` = "we tried and it broke"
   - `scrapped` = "we decided not to do this"

3. **Archive statuses** — tasks with `completed` or `scrapped` status can be automatically moved to `.beans/archive/`. This is a different mechanism than just a status field.

4. **Status ordering** has semantic meaning — `in-progress` sorts first (active work), then `todo`, then `draft`, then terminal statuses last.

**Worth borrowing:**
- Add **`cancelled`** (our equivalent of `scrapped`) alongside `failed`. They are meaningfully different for agent coordination: `cancelled` means "skip this" while `failed` means "something went wrong, may need retry or different approach."
- Consider **`draft`** status for tasks that need planning before they can be assigned.
- The status sort order concept: `in-progress` should sort before `todo` in `list` output.

---

## 5. Tagging / Filtering

Beans has the most comprehensive filtering I've seen in a task CLI:

### Tag support
```bash
beans create "Fix auth bug" --tag auth --tag backend --tag urgent
beans list --tag auth              # any task with auth tag (OR logic)
beans list --tag auth --tag urgent # any task with auth OR urgent
beans list --no-tag wip            # exclude tasks tagged wip
beans update abc1 --tag done-review --remove-tag needs-review
```

Tags are:
- Lowercase only, validated (`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
- Can be added/removed granularly on `update` (not just full replacement)
- OR logic for filtering (any matching tag = included)

### Full-text search
```bash
beans list --search "authentication"     # simple term
beans list --search "auth~"              # fuzzy (1 edit distance)
beans list --search "user AND login"     # boolean
beans list --search "title:login"        # field-specific
beans list -S "login~2"                  # 2-edit-distance fuzzy
```
Uses [Bleve](https://blevesearch.com/) for full-text search with BM25 scoring. Indexed fields: ID, slug, title, body.

### Blocking relationship filters
```bash
beans list --is-blocked               # tasks blocked by something active
beans list --ready                    # not blocked, not in terminal state
beans list --has-blocking             # tasks that are blocking others
beans list --no-blocking              # tasks that aren't blocking anything
```

### Hierarchy filters
```bash
beans list --has-parent               # tasks with a parent
beans list --no-parent                # top-level tasks only
beans list --parent abc1              # children of abc1
```

**Worth borrowing:**
- **Tags** — even a basic tag array on tasks gives agents a flexible labeling mechanism without requiring schema changes for every new categorization need. Add `tags: []string` to our task schema and `--tag`/`--no-tag` filters to `swarm task list`.
- **`--is-blocked` and `--ready` filters** — critical for agent workflows.
- **`--search`** — Full-text search is high-value for large task sets, but Bleve is heavyweight. A simple substring match on title+output is a good start.

**Not relevant:**
- Hierarchy filters — we don't have parent-child relationships (yet).

---

## 6. Priority Model

### Beans priorities (highest → lowest)
| Priority | Color | Description |
|---|---|---|
| `critical` | red | Urgent, blocking work. Address immediately. |
| `high` | yellow | Important, should be done before normal work. |
| `normal` | white | Standard priority (default, empty = normal). |
| `low` | gray | Less important, can be delayed. |
| `deferred` | gray | Explicitly pushed back, avoid unless necessary. |

Key design decisions:
- Priority is **optional** — omitting it implies `normal` for sorting and filtering
- `deferred` is explicitly distinct from `low` — it signals a conscious decision to not work on something now (useful for agents to know "skip this")
- Empty priority treated as `normal` in all sorting/filtering logic

**Worth borrowing:**
- **5-level priority** (`critical`, `high`, `normal`, `low`, `deferred`) is more expressive than a simple numeric scale. The `deferred` level is particularly useful for agent coordination: `swarm task list --ready` should exclude `deferred` tasks by default.
- **Optional priority** with implicit default — our current design with a `priority` field is good, but we should define the valid values explicitly.

---

## 7. Agent-Friendly Features

### 7a. `--json` on ALL commands with consistent envelope
```json
// mutation responses (create, update, delete, archive):
{ "success": true, "bean": {...}, "message": "Bean created" }

// error responses:
{ "success": false, "error": "bean not found: abc1", "code": "NOT_FOUND" }
```

**But** `show --json` and `list --json` emit bare objects (no wrapper):
```bash
beans show --json abc1 | jq '.title'      # works directly
beans list --json | jq '.[].id'           # array, no wrapper
```
This design decision is clever: mutations need an envelope (did it succeed? what's the message?), but read queries should emit the raw data for easy `jq` chaining.

Error codes: `NOT_FOUND`, `NO_BEANS_DIR`, `INVALID_STATUS`, `FILE_ERROR`, `VALIDATION_ERROR`, `CONFLICT`

**Worth borrowing:**
- **Differentiate JSON envelope for mutations vs. reads** — our `--json` should be consistent: mutations emit `{success, task, message}`, list emits a bare array, show emits a bare object.
- **Machine-readable error codes** in JSON error responses (`"code": "NOT_FOUND"`).

### 7b. `prime` command — agent onboarding
```bash
beans prime    # outputs a prompt with full instructions for agents
```
Output is injected via SessionStart hooks in `.claude/settings.json`:
```json
{
  "hooks": {
    "SessionStart": [{ "hooks": [{ "type": "command", "command": "beans prime" }] }]
  }
}
```
The prime output includes: all CLI commands with examples, valid values for all fields, GraphQL schema, agent-specific workflow instructions (always use beans instead of TodoWrite, etc.).

**Worth borrowing** as `swarm task prompt` — outputs a ready-to-inject agent prompt. This is the key integration point for getting agents to use `swarm task` correctly. Include: all commands, valid status values, JSON output examples, agent workflow guidance.

### 7c. ETag-based optimistic concurrency control
```bash
# Get etag
ETAG=$(beans show abc1 --etag-only)

# Update only if nobody else changed it
beans update abc1 --if-match "$ETAG" --status completed
# → CONFLICT error if another agent changed it first
```

ETag is FNV-1a 64-bit hash of the full rendered file content. Config option `require_if_match: true` makes the ETag mandatory for all updates.

**Partially worth borrowing** — for `swarm task`, our flock() mutex already handles concurrent writes safely. However, an ETag-equivalent (content hash in the JSON record) could let agents detect mid-flight changes: "I read the task, someone updated it while I was working, let me re-read before overwriting." The `--if-match` pattern is elegant for multi-agent scenarios.

### 7d. Implicit status inheritance
If a parent task is `completed` or `scrapped`, all children inherit that terminal status implicitly. The `--ready` filter and GraphQL subscriptions respect this automatically — agents won't start work on a sub-task whose parent was cancelled.

**Worth borrowing in spirit** — since we don't have parent hierarchies, the direct equivalent is: if a task is `cancelled` or `done`, any tasks blocked by it should be automatically unblocked (or the block resolved). Currently `swarm task` doesn't handle this; agents have to manually clean up.

### 7e. `BEANS_PATH` environment variable
```bash
BEANS_PATH=/tmp/test-project beans list  # override data directory
```
Also: `--beans-path` CLI flag and `--config` flag for config file override.

**Worth borrowing** — `SWARM_TASKS_PATH` env var to override the `.swarm/tasks/tasks.json` location. Useful for testing and for running multiple swarms in the same environment.

---

## 8. Uniquely Clever Things

### 8a. Slug in filename — human-readable filesystem navigation
```
beans-abc1--implement-user-login.md
beans-xyz2--fix-memory-leak-in-worker.md
```
The filename is self-describing. You can `ls .beans/` and immediately understand your task list. `grep` across filenames to find tasks. Git blame on a specific task file. No need to open anything to understand the structure.

**Worth borrowing concept** — even in our JSON storage, include the title as a human-readable comment or consider a naming convention for exported task snapshots. Not directly applicable to `tasks.json`, but the philosophy (make things human-readable in the filesystem) is sound.

### 8b. `beans list --ready` — compound "actionable" filter
As detailed above: one command answers "what should I work on right now?" Combines: not blocked, not in terminal state, not in-progress, not draft, not a child of a terminal parent. This is the single most useful agent command in beans.

**Worth borrowing** as `swarm task list --ready` or `--actionable`.

### 8c. Surgical body modification (`--body-replace-old/new`)
Agents can update a task's notes in-place without reading and rewriting. The "must match exactly once" constraint prevents bugs. Combined with `--body-append`, agents can maintain structured task bodies (e.g., checking off todo items, appending progress notes) incrementally.

**Worth borrowing** — `swarm task update <id> --output-append "text"` is a clear win. The `replace` pattern requires agents to have read the current output first (to know what to replace), which is a reasonable constraint.

### 8d. Blocked-word filter for ID generation
Generated IDs are checked against an explicit list of offensive words (ass, fuck, shit, cock, etc.) and regenerated if any word is found. Small detail but important for professional contexts.

**Worth borrowing** — add a blocked-words check to our NanoID generation.

### 8e. Fractional indexing for manual ordering
Each task has an `order` field using lexicographically-sortable base-62 strings. Inserting a task between two others only requires computing `OrderBetween(a, b)` — no renumbering, no conflicts. Implemented with a well-tested `OrderBetween(a, b string) string` function.

**Worth borrowing** if we ever add manual task ordering (drag-and-drop in TUI, or `swarm task reorder`). The fractional indexing algorithm is clean and worth copying.

### 8f. `check --fix` — referential integrity repair
```bash
beans check           # report broken links, self-references, cycles
beans check --fix     # auto-remove broken links and self-references
                      # (cycles require manual intervention)
```
Multi-agent environments inevitably create referential integrity issues (agent A deletes a task that agent B's task is blocked by). `check --fix` heals the data automatically.

**Worth borrowing** — `swarm task check [--fix]` that validates `blocked_by` references are all valid task IDs and removes stale references. Essential for long-running multi-agent workflows.

### 8g. Archive with auto-unarchive
Completed/scrapped tasks move to `.beans/archive/` on `beans archive`. Archived tasks are still fully queryable. If you run `beans update <archived-id> --status todo`, the task is automatically moved back to the main directory.

**Partially worth borrowing** — an `swarm task archive` command that moves completed/failed/cancelled tasks to a separate `.swarm/tasks/archive.json` would keep the main `tasks.json` clean in long-running projects. Auto-unarchive on status change is elegant.

---

## 9. Things NOT Worth Borrowing

| Feature | Reason |
|---|---|
| One-file-per-task storage | We've decided on `tasks.json`; flat JSON + flock is simpler and sufficient |
| GraphQL query interface | Overkill; `--json` + `jq` covers 95% of use cases |
| YAML front-matter format | Our JSON is simpler for programmatic access |
| TUI (`beans-tui`) | Out of scope for `swarm task` MVP |
| Web UI (`beans-serve`) | Out of scope |
| Git worktree integration | Out of scope; our model is file-backed shared state |
| Type hierarchy (milestone/epic/feature) | Overkill for agent task coordination |
| Roadmap generation | Out of scope |
| Bleve full-text search | Heavy dependency; simple string match is sufficient for MVP |
| Per-project config file (`.beans.yml`) | `.swarm/tasks/tasks.json` is self-contained |
| Fractional ordering (for now) | Not needed until we have a TUI or ordering requirement |
| OpenCode / Claude hooks integration | Agent-specific; address in agent integration layer, not in `swarm task` |

---

## Summary: Borrow List

Ranked by implementation effort vs. value for agent coordination:

### High value, low effort
1. **`--quiet` / `-q` flag** on `swarm task list` — prints IDs only, one per line. One-liner to add.
2. **`--ready` / `--actionable` filter** on list — compound filter: not blocked, not in terminal state, not already in-progress. Critical for agent "what should I work on?" workflows.
3. **Machine-readable error codes** in `--json` output — `"code": "NOT_FOUND"`, `"CONFLICT"`, etc.
4. **`cancelled` status** alongside `failed` — semantically distinct: abandoned vs. errored.
5. **5-level priority** (`critical/high/normal/low/deferred`) with `deferred` level. Agents can explicitly defer work.
6. **`--exclude-status` / `--no-status`** exclusion variants on list filters.
7. **`--full` flag** on `list --json` — exclude `output` field by default (save tokens!).
8. **`--sort priority|created|updated|status`** on list.

### High value, medium effort
9. **`swarm task show <id> [id…]`** — dedicated detailed view command with `--json` and `--output-only` flags. Separate from list.
10. **`swarm task update <id> --output-append "text"`** — append to output field without full rewrite.
11. **`swarm task prompt`** — outputs agent-priming instructions for injection into agent context. The single most important agent integration feature.
12. **Tags** (`tags: []string`) on task schema + `--tag`/`--no-tag` filter on list. Free-form labeling.
13. **`swarm task check [--fix]`** — validate blocked_by references, auto-remove stale ones.
14. **Bare objects for read JSON** vs. envelope for mutation JSON — `list --json` emits array, `show --json` emits object, `add --json` emits `{success, task, message}`.

### Medium value, medium effort
15. **`SWARM_TASKS_PATH` env var** — override tasks file location without flags.
16. **Delete warns about incoming references** — `rm <id>` warns if other tasks have this ID in `blocked_by`, offers to clean up.
17. **Blocked-word filter for ID generation** — check NanoIDs against offensive word list.
18. **ETag / `--if-match`** for optimistic concurrency — content hash per task, `update --if-match` fails if task changed since last read. Lower priority since flock() handles concurrent writes; this adds agent-level race detection.

### Lower priority
19. **Draft status** — "not ready to work on" planning status.
20. **Auto-cleanup when blocker resolves** — when task is marked done/cancelled, automatically clean up `blocked_by` references in tasks it was blocking.
21. **`swarm task archive`** — move done/failed/cancelled tasks to `archive.json`.
22. **`--output-replace-old/new`** surgical text replacement — useful for agents checking off inline todo items.
