---
name: kk-handoff
description: Write the prompt that hands a piece of work to a fresh session, then create the chip that starts it. Use for "hand this off", "spawn a session for this", and before creating a background-task chip for any reason. The receiver holds none of this conversation — not a stage subagent spawned inside this run, and not a quality stage naming the next lane for its caller.
argument-hint: "the work to hand off"
---

Hand the work over as one prompt that stands alone, plus the chip that starts a session on it.

`~/.kk-flavor/templates/spawn-prompt.md` covers spawning a stage subagent inside this run.

**Runs inline, never spawned** — the context being handed off is context only you hold (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**).

**Hand off scope, never effort.** Work you could finish in this run is work you finish. **Handing off is one of five moves at a phase boundary**, and `~/.kk-flavor/standards/skill-protocol.md` → **Phase boundaries** ranks them — reaching here past a cheaper one writes a file nobody needed.

## 1. Draft it

Fill every slot of `~/.claude/skills/kk-handoff/handoff-prompt.md`, into a file in the scratch dir — **outside the repository** (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**).

## 2. Check it

Run `~/.claude/skills/kk-handoff/scripts/handoff-check.sh <draft> <repo>`.

**A non-zero exit means no chip.** Exit 1 is the draft: fix what each finding names and re-run until it exits 0, never arguing with one. Exit 2 is your invocation — the check never ran, so the draft is unmeasured and editing it fixes nothing. Read every `declared None:` line back. Each is a slot you chose to send as nothing, and the check cannot tell a deliberate one from a lazy one.

**A dirty tree is the trap the check only warns about.** When the handoff depends on uncommitted work, say so and get your caller's answer before step 3.

## 3. Hand it over

Create the chip: the title is the draft's `# ` line, the prompt is everything below it, the working directory is the repository root, and the one-line summary is written for the human deciding whether to click, not for the receiving session.

**No chip mechanism in this session?** Return the draft's path and say it is ready to paste.

## Rules

- **One piece of work per chip.** Two are two drafts and two chips.
- **Nothing in the draft is a summary of this conversation.** A receiver cannot act on what you found interesting.
