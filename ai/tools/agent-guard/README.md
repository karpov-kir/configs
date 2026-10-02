# agent-guard

The PreToolUse hook bootstrap registers on Claude's Agent tool. It refuses a dispatch to
`general-purpose`, or one naming no type, when the prompt has no `Needs: <tool>` line. The rule it
enforces is `skill-protocol.md` → **Caller**. `agent-guard-hook.sh` runs it and passes only its own
refusal on. Any other failure, such as a guard that cannot build, lets the dispatch run.

## Codex takes no guard

Bootstrap writes nothing for Codex and says so on every Codex install. A Codex session verified
the attempt on 2026-10-02, with a light-worker role layer and this hook installed:

- `collaboration.spawn_agent` takes `task_name`, `message`, `fork_turns`, `model` and
  `reasoning_effort`. It has no role argument, so a worker with the light-worker role could not be
  spawned at all.
- A spawn with no role and no `Needs:` line ran and replied. The guard refused nothing, and
  whether Codex ran the hook at all is unverified.
- A spawn with a `Needs:` line opened at 29,572 input tokens, against a median 29.8k over 18 earlier
  Codex spawns on this machine.

The only lever found is a role's `[agents.<name>.config_file]` layer, and nothing in the exposed spawn
tool selects a role. Until Codex exposes a role on spawn, or a hook that provably fires on one, a Codex
worker opens at about 30k and nothing here changes that. The guard still reads Codex's spawn payload
(`agent_type`, `role`, `message`), so a real install is one bootstrap step once Codex offers one.
