---
# openswarm-0dm9
title: internal/config — config loading
status: completed
type: task
priority: normal
created_at: 2026-03-30T12:54:55Z
updated_at: 2026-03-30T12:56:33Z
---

Implement internal/config package with TOML loading, env var overrides, and defaults.

## Summary of Changes

### Files delivered

**** (86 lines):
- `Config` struct with TOML tags: `TeamName`, `DefaultAgent`, `Backend`, `AgentProfiles`
- `AgentProfile` struct with `Name`, `Command`, `Args`, `Env`
- `Defaults()` — returns Config with Backend="auto", empty TeamName/DefaultAgent
- `Load(root *swarmfs.Root)` — reads `root.ConfigPath`, handles missing file (returns defaults, no error), empty file (skips TOML parse), TOML decode, env var overrides via `applyEnv`
- `applyEnv` — overlays SWARM_BACKEND and SWARM_DEFAULT_AGENT if non-empty

**`internal/config/config_test.go`** (11 tests, all pass with -race):
- `TestDefaults` — verifies all default field values
- `TestLoad_MissingFile_ReturnsDefaults` — absent config.toml returns defaults, no error
- `TestLoad_EmptyFile_ReturnsDefaults` — empty file returns defaults
- `TestLoad_TOMLParsing_AllFields` — full TOML with team, agent, backend, env vars
- `TestLoad_TOMLParsing_MultipleAgents` — multiple `[[agent]]` sections
- `TestLoad_TOMLParsing_InvalidTOML_ReturnsError` — malformed TOML returns error
- `TestLoad_EnvOverride_Backend` — SWARM_BACKEND beats file value
- `TestLoad_EnvOverride_DefaultAgent` — SWARM_DEFAULT_AGENT beats file value
- `TestLoad_EnvOverride_OnMissingFile` — env overrides work even without a file
- `TestLoad_EnvOverride_EmptyEnvVarIgnored` — empty env var does not override file value
- `TestLoad_UsesRootConfigPath` — Load reads from root.ConfigPath (not a hardcoded path)

**Dependency added:** `github.com/BurntSushi/toml v1.6.0` in go.mod/go.sum
