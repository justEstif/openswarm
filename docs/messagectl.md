# messages — Peer Messaging CLI

## What

A CLI tool (`messages`) that provides file-backed, inbox-per-agent messaging for coordinating communication between agents or processes. Any registered agent can send messages to any other agent by ID, read its own inbox, and reply to threads.

```
messages register --role researcher
→ researcher-a3f2

messages send researcher-a3f2 "your task is ready, see task-b7k1"
messages send researcher-a3f2 --file result.json
messages inbox
messages inbox --watch
messages reply msg-x4f2 "got it, starting now"
messages inbox --json
messages clear
```

It stores state as a directory tree of JSON message files. Delivery is via atomic file writes — no daemon required. Agents poll their inbox at their own pace.

**Primary use cases:**

1. Orchestrator agents notifying specialist agents of new task assignments
2. Agents reporting results back to an orchestrator without going through a shared task file
3. Agent-to-agent coordination — "I finished my part, you can start yours"
4. Human sending one-off instructions into a running agent's inbox

## Why

- **Agents need to talk to each other.** A shared task list handles work coordination, but sometimes an agent needs to pass freeform context, a file, or a status update directly to another agent.
- **No lightweight peer messaging exists for agent setups.** Claude Code's internal system uses `~/.claude/teams/{team}/messages/` but it's internal to TeammateTool and not accessible as a standalone primitive.
- **File-based is the right primitive.** Agents already read/write files constantly. A file-based inbox fits naturally into how agents operate — no daemon to manage, survives restarts, easy to debug with `cat` and `ls`.
- **Personal pain point.** Recreating Claude agent team patterns on top of muxctl requires a messaging layer that's multiplexer-agnostic and not tied to any specific coding agent.

## Competitors & Landscape

No existing tool fills this exact niche. The closest things are:

- **Claude Code's internal messaging** (`~/.claude/teams/{team}/messages/{session-id}/`) — internal to TeammateTool, not usable standalone
- **Named pipes / FIFOs** — blocking on write if reader isn't listening, fragile across restarts, no message history
- **Unix sockets** — require a running daemon, more moving parts than the problem warrants at MVP scale
- **Slack / Discord bots** — network dependency, auth overhead, overkill for local agent coordination

## MVP Scope

### Storage

```
.messages/
├── agents/
│   ├── researcher-a3f2/
│   │   └── inbox/
│   │       ├── msg-x4f2.json
│   │       └── msg-b1k9.json
│   └── orchestrator-7c3d/
│       └── inbox/
│           └── msg-p2q1.json
└── team.json     # registered agents + team metadata (written by `messages team init`)
```

Overridable via `$MESSAGES_DIR`. Default is `.messages/` in the current working directory. All writes are atomic (`mv` from temp file).

### Agent identity

Agents register with a role name. A short random suffix is appended to prevent collisions. The resulting ID is the agent's inbox address for its lifetime.

```
$ messages register --role researcher
→ researcher-a3f2
$ export AGENT_ID=researcher-a3f2
```

`$AGENT_ID` is used as the implicit sender for outgoing messages and the implicit recipient for `messages inbox`.

### Message schema

```json
{
  "id": "msg-x4f2",
  "from": "orchestrator-7c3d",
  "to": "researcher-a3f2",
  "body": "your task is ready, see task-b7k1",
  "payload_file": "",
  "reply_to": "",
  "sent_at": "2026-03-27T10:00:00Z",
  "read": false
}
```

### Commands

#### Registration

| Command | Description |
| --- | --- |
| `messages register --role <name>` | Register as a new agent, returns full agent ID (role + suffix) |
| `messages agents` | List all registered agents and their IDs |

#### Sending

| Command | Description |
| --- | --- |
| `messages send <agent-id> <body>` | Send a message to an agent |
| `messages send <agent-id> --file <path>` | Send a message with a file payload attached |
| `messages reply <msg-id> <body>` | Reply to a specific message (sets `reply_to`) |

#### Reading

| Command | Description |
| --- | --- |
| `messages inbox` | Read your inbox (uses `$AGENT_ID`), shows unread first |
| `messages inbox --all` | Show all messages including already-read |
| `messages inbox --watch` | Poll inbox, print new messages as they arrive |
| `messages inbox --json` | Machine-readable output |
| `messages read <msg-id>` | Mark a specific message as read |
| `messages clear` | Archive/delete all read messages from inbox |

#### Team context

| Command | Description |
| --- | --- |
| `messages team init --name <n>` | Initialize a named team context in `.messages/` |
| `messages team status` | Show registered agents and unread message counts |

### Output format

- Default: human-readable (sender, timestamp, body preview)
- `--json` flag for machine consumption (agent-friendly)
- `--watch` polls every 1s by default, configurable via `--interval <ms>`

### Example flow

```
# Orchestrator initializes team and registers
$ messages team init --name "api-build"
$ export AGENT_ID=$(messages register --role orchestrator)
→ orchestrator-7c3d

# Spawn researcher agent with its identity baked in
$ RESEARCHER_ID=$(messages register --role researcher)
$ muxctl pane new --cmd "AGENT_ID=$RESEARCHER_ID claude --prompt 'You are a researcher...'"

# Orchestrator sends task notification
$ messages send $RESEARCHER_ID "start on task-a3f2 — research auth patterns"

# Researcher agent reads inbox (in its pane)
$ messages inbox
  FROM                  SENT                 BODY
  orchestrator-7c3d    2026-03-27 10:00     start on task-a3f2 — research auth patterns

# Researcher replies when done
$ messages reply msg-x4f2 "done — output attached"
$ messages send orchestrator-7c3d --file research-output.json

# Orchestrator watches for replies
$ messages inbox --watch
→ [10:42] researcher-a3f2: done — output attached (payload: research-output.json)

# Check team health
$ messages team status
  AGENT               ROLE          UNREAD
  orchestrator-7c3d  orchestrator  1
  researcher-a3f2    researcher    0
```

## Non-goals for MVP

- Daemon or push delivery (polling is sufficient)
- Message encryption
- Broadcast / pub-sub messaging (direct peer-to-peer only)
- Message expiry or TTL
- Delivery receipts / acknowledgement protocol
- GUI / TUI inbox viewer

## Tech stack

- **Go** with **Cobra** for CLI framework
- Single binary, no runtime dependencies
- Atomic file writes via temp file + `os.Rename`
- `--watch` mode via simple polling loop (no inotify dependency)

## Success criteria

1. `messages send` + `messages inbox` round-trip works between two agents in separate panes
2. `messages inbox --watch` surfaces new messages within 1–2 seconds of delivery
3. `messages reply` correctly threads replies so an agent can reconstruct a conversation from `msg.reply_to` chains
4. File payloads are referenced by path, not embedded — large outputs don't bloat the message store
