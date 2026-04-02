# openswarm

Multi-agent task orchestration CLI.

## Install

```bash
npm install -g @justestif/openswarm
```

This downloads the pre-built binary for your platform from [GitHub Releases](https://github.com/justEstif/openswarm/releases).

**Requires:** Node.js ≥ 18, git (for `swarm worktree`)

## Usage

```bash
swarm init                    # initialise .swarm/ in your project
swarm task add "My task"      # create a task
swarm agent register alice    # register an agent
swarm msg send alice -s hi -b "hello"
swarm status                  # snapshot of current state
```

Full docs: https://github.com/justEstif/openswarm
