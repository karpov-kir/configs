---
name: kk-handoff
description: Write the prompt that hands a piece of work to a fresh session, then start the new task. Use for "hand this off", "spawn a session for this", and before starting a separate task. The receiver holds none of this conversation — not a stage subagent spawned inside this run, and not a quality stage naming the next lane for its caller.
argument-hint: "the work to hand off"
---

Hand the work over as one prompt that stands alone, plus the task that starts a session on it.

`~/.kk-flavor/templates/spawn-prompt.md` covers spawning a stage subagent inside this run.

**Runs inline, never spawned** — the context being handed off is context only you hold (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**).

**Hand off scope, never effort.** Work you could finish in this run is work you finish. **Handing off is one of the moves at a phase boundary**, and `~/.kk-flavor/standards/skill-protocol.md` → **Phase boundaries** ranks them — reaching here past a cheaper one writes a file nobody needed.

## 1. Draft it

Fill every slot of `~/.kk-flavor/skills/kk-handoff/handoff-prompt.md`, into a file in the scratch dir (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**).

## 2. Check it

Run `~/.kk-flavor/skills/kk-handoff/scripts/handoff-check.sh <draft> <repo>`.

**A non-zero exit blocks launching.** Exit 1 is the draft: fix what each finding names and re-run until it exits 0, never arguing with one. Exit 2 is your invocation — the check never ran, so the draft is unmeasured and editing it fixes nothing. Read every `declared None:` line back. Each is a slot you chose to send as nothing, and the check cannot tell a deliberate one from a lazy one.

**A dirty tree is the trap the check only warns about.** When the handoff depends on uncommitted work, say so and get your caller's answer before step 3.

## 3. Hand it over

Name the target client explicitly in the draft's **Where it starts** slot. Missing or ambiguous client selection blocks launching.

- **Codex desktop:** use `create_thread` only when the user requested a separate task. Resolve the project with `list_projects`, pass the draft's title and prompt, and follow the tool's checkout/worktree rules. Creation starts work immediately; a returned task ID is the handoff, not a chip waiting for a click. Use subagents for work within the current task.
- **Claude Code:** use the available chip mechanism. The title is the draft's `# ` line, the prompt is everything below it, the working directory is the repository root, and the summary is for the human deciding whether to click.

Keep the target's configured model unless the user selected another. Resolve any role override through `~/.kk-flavor/standards/model-policy.md`; a comparable tier in another provider is not inherited task identity.

**No task or chip mechanism for the selected client?** Return the draft's path and say it is ready to paste.

## Rules

- **One piece of work per task.** Two are two drafts and two tasks.
- **Nothing in the draft is a summary of this conversation.** A receiver cannot act on what you found interesting.
