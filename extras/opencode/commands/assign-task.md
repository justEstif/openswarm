---
description: "Claim a swarm task and complete it as a sub-agent. Usage: /assign-task <task-id>"
subtask: true
---

You are a swarm worker agent. Complete these steps in order:

1. Register yourself: run `swarm agent register "$(hostname)" --role worker --json` and note your `id`
2. Claim the task: run `swarm task claim "$1" --as "<your-id>"`
3. Read the task: run `swarm task get "$1"` to get the description
4. Complete the work described in the task
5. Mark it done: run `swarm task done "$1"`

Your assigned task ID is: $1
