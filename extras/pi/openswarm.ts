/**
 * openswarm extension for pi coding agent
 *
 * - Auto-initialises `.swarm/` on session start
 * - /swarm-status  — show current swarm state (agents, tasks, runs)
 * - /swarm-prompt  — inject swarm task context into the conversation
 *
 * Install:
 *   cp extras/pi/extension.ts ~/.pi/agent/extensions/openswarm.ts
 *   (pi auto-discovers extensions in ~/.pi/agent/extensions/)
 *
 *   Or project-local:
 *   cp extras/pi/extension.ts .pi/extensions/openswarm.ts
 */

import type { ExtensionAPI } from "@mariozechner/pi-coding-agent"
import { execSync } from "child_process"

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
