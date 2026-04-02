/**
 * openswarm extension for pi coding agent
 *
 * - Auto-initialises `.swarm/` on session start
 * - /swarm-status   — show current swarm state (agents, tasks, runs)
 * - /swarm-prompt   — inject swarm task context into the conversation
 * - /assign-task    — claim a task and spawn a headless sub-agent to complete it
 *
 * Install:
 *   cp extras/pi/openswarm.ts ~/.pi/agent/extensions/openswarm.ts
 *   (pi auto-discovers extensions in ~/.pi/agent/extensions/)
 *
 *   Or project-local:
 *   cp extras/pi/openswarm.ts .pi/extensions/openswarm.ts
 */

import type { ExtensionAPI } from "@mariozechner/pi-coding-agent"
import { execSync, spawn } from "child_process"
import { hostname } from "os"

export default function (pi: ExtensionAPI) {
  // Auto-init swarm on session start (idempotent)
  pi.on("session_start", async (_event, ctx) => {
    try {
      execSync("swarm init", { stdio: "ignore" })
    } catch {
      // swarm not installed or not in a project root — skip silently
    }
  })

  // /swarm-status — quick dashboard
  pi.registerCommand("swarm-status", {
    description: "Show current swarm state: agents, tasks, unread messages, active runs",
    handler: async (_args, ctx) => {
      try {
        const raw = execSync("swarm status --json", { encoding: "utf8" })
        const st = JSON.parse(raw) as {
          agents: number
          tasks: number
          tasks_done: number
          unread: number
          runs_active: number
          panes: number
        }
        ctx.ui.notify(
          `Agents: ${st.agents}  Tasks: ${st.tasks} (${st.tasks_done} done)  ` +
            `Unread: ${st.unread}  Active runs: ${st.runs_active}  Panes: ${st.panes}`,
          "info",
        )
      } catch (e) {
        ctx.ui.notify(`swarm status failed: ${String(e)}`, "error")
      }
    },
  })

  // /assign-task — claim a task and spawn a headless pi sub-agent to complete it
  pi.registerCommand("assign-task", {
    description:
      "Claim a swarm task and spawn a headless sub-agent to complete it. " +
      "Usage: /assign-task <task-id> [--provider <provider> --model <model>]",
    handler: async (args, ctx) => {
      const parts = args.trim().split(/\s+/).filter(Boolean)
      const taskId = parts[0]
      if (!taskId) {
        ctx.ui.notify("Usage: /assign-task <task-id> [--provider <p> --model <m>]", "warning")
        return
      }
      const extraArgs = parts.slice(1)

      try {
        // Register this agent instance in the swarm
        const agentJson = execSync(
          `swarm agent register "${hostname()}" --role worker --json`,
          { encoding: "utf8", shell: true },
        )
        const agent = JSON.parse(agentJson) as { id: string }

        // Claim the task so no other agent picks it up
        execSync(`swarm task claim "${taskId}" --as "${agent.id}"`, { stdio: "ignore" })

        // Spawn a headless pi sub-agent — fire and forget so this agent stays responsive
        const prompt =
          `You have been assigned swarm task ${taskId}. ` +
          `Run \`swarm task get ${taskId}\` to read its description, ` +
          `complete the work, then call \`swarm task done ${taskId}\`.`

        const child = spawn("pi", ["--print", ...extraArgs, prompt], {
          stdio: "ignore",
          detached: true,
        })
        child.unref() // don't block the parent process

        child.on("exit", (code) => {
          ctx.ui.notify(
            code === 0
              ? `Sub-agent finished task ${taskId}`
              : `Sub-agent exited with code ${code} on task ${taskId}`,
            code === 0 ? "info" : "warning",
          )
        })

        ctx.ui.notify(`Sub-agent spawned for task ${taskId} (pid ${child.pid})`, "info")
      } catch (e) {
        ctx.ui.notify(`assign-task failed: ${String(e)}`, "error")
      }
    },
  })

  // /swarm-prompt — inject coordination context into the conversation
  pi.registerCommand("swarm-prompt", {
    description: "Inject current openswarm task and agent context into the conversation",
    handler: async (_args, ctx) => {
      try {
        const prompt = execSync("swarm prompt", { encoding: "utf8" }).trim()
        if (!prompt) {
          ctx.ui.notify("swarm prompt returned empty — is .swarm/ initialised?", "warning")
          return
        }
        if (!ctx.isIdle()) {
          ctx.ui.notify("Agent is busy. Try again when idle.", "warning")
          return
        }
        pi.sendUserMessage(prompt)
      } catch (e) {
        ctx.ui.notify(`swarm prompt failed: ${String(e)}`, "error")
      }
    },
  })
}
