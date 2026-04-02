/**
 * openswarm plugin for opencode
 *
 * - Auto-initialises `.swarm/` on startup (idempotent)
 * - Injects current swarm task state into compaction context so agents
 *   retain coordination context across context-window resets
 *
 * Install:
 *   Copy this file to ~/.config/opencode/plugins/openswarm.ts
 *   or reference it in opencode.json:  "plugin": ["./openswarm.ts"]
 */

import type { Plugin } from "@opencode-ai/plugin"

export const OpenswarmPlugin: Plugin = async ({ directory, $ }) => {
  // Auto-init on every session start (idempotent — safe to run always)
  try {
    await $`swarm init`.cwd(directory).quiet()
  } catch {
    // swarm not installed or directory is not a project root — skip silently
  }

  return {
    /**
     * Inject swarm task/agent state into the compaction context so the
     * continuation summary includes coordination state.
     */
    "experimental.session.compacting": async (_input, output) => {
      try {
        const result = await $`swarm prompt`.cwd(directory).quiet()
        const text = result.stdout.toString().trim()
        if (text) {
          output.context.push(`\n## openswarm coordination state\n\n${text}\n`)
        }
      } catch {
        // swarm unavailable or no .swarm/ — skip silently
      }
    },
  }
}

export default OpenswarmPlugin
