---
name: kk-foreman
description: The front door to the kk-* skills — say what you want done and this picks them, orders them, and runs them. Use when the work crosses more than one, when you don't know which applies, or for a periodic "what does this repo need?". Keeps the size ledger. The idsd-* intent workflow is its own door (idsd-ship).
argument-hint: "what you want done (default: look at the working tree and recommend)"
---

Turn an intent into the right skills, in the right order, and run them. You **dispatch and do not do the work** — every stage is a skill that already exists, invoked per `~/.kk-flavor/standards/skill-protocol.md`.

**This file holds no catalogue of what each skill does.** Their own `description:` fields are that, and a second copy here would drift (`~/.kk-flavor/standards/ecosystem.md` → **One home**). Resolve candidates at run time by reading the frontmatter under `~/.claude/skills/*/SKILL.md` — that also finds skills whose `disable-model-invocation: true` keeps them out of your context.

What this skill carries is only what a description cannot: **the chains, the order inside them, and when not to run something.**

## 1. Read the state before choosing

**Only when the work touches the agent instruction tree** — skills, standards, prompts, templates, `CLAUDE.md`. The ledger measures that tree and nothing else, so in any other repo there is nothing to read: go straight to **Route**.

Run `scripts/stats.sh` (this skill's dir) and read `history.md` beside it. Exit 2 means nothing was measured — fix the invocation rather than treat it as no change.

Those two answer the question a description cannot: *has this grown since it was last cut, and by how much?* Decide from the delta, never from a threshold — a number invented here would just teach later passes to trim words until they clear it.

## 2. Route

**Start at the smallest skill that covers the work, and escalate only on evidence.** One `kk-*` skill run directly — whichever description matches — is the common answer, and three touched lines get one of those, not a chain: say which skill and run it, without ceremony.

Escalate on something you can name: the change crosses several lanes at once, a scoped pass already ran and left the problem standing, or the ledger shows drift no single pass reaches. "It might find more" is not evidence — it is how every request becomes the most expensive one.

The chains below are the ones where order is load-bearing and an agent choosing one skill at a time gets it wrong:

| The work | The chain |
|---|---|
| Code or tooling changed | `kk-code-review`, plus `kk-security-review` where the change touches a security surface — concurrently, they are independent lenses → then `kk-refactor`, which serializes on the tree they leave. Drop the middle one for a change that touches no such surface; drop refactor where no structure moved. |
| A PR | `kk-pr-review`. It drafts, and posts nothing until the human approves. |
| Prose changed | `kk-tighten` or `kk-humanize` — their own descriptions split which prose is whose. Neither needs an orchestrator. |
| Skills, standards, prompts or templates changed | `kk-ecosystem` over the diff, alone — it owns that lane end to end and spawns `kk-skillcraft` and `kk-tighten` itself, so queuing either beside it runs them twice and out of order. |
| The tree has grown well past its last reduction | `kk-reduce` — a campaign, not a pass. |
| A plan or a decision, with nothing built yet | `kk-grill`, alone. Every row above reviews something that exists; this is the only one that reaches a choice while it is still cheap to change. |
| Nothing named, or a periodic check | Recommend from what changed, plus the ledger's trend where step 1 read one. Recommending nothing is a valid outcome. |

**You route `kk-*` skills only.** The `idsd-*` suite is a workflow the human enters deliberately and stays inside, carrying its own orchestrator and its own order. When the work is plainly intent-shaped (an ICE to author, an intent to build, a change heading for that pipeline's merge gate), say so and name `idsd-ship` as its door, then stop. Do not sequence its stages, and do not substitute a `kk-*` chain for it: `idsd-qualify` writes a report and stamps a merge gate, which no chain here reproduces.

**More than one row will often match** — a change set that touches skills and their scripts matches two. Run them as one chain rather than one after the other: stages whose file sets are disjoint go concurrently, and where the sets overlap the rows keep their own order. Two full chains run back to back review the same files twice and let the second undo the first.

Two fixed points when rows merge. **A stage another row's skill already spawns is not queued again** — it runs there, in that skill's own order. And **`kk-humanize` over code comments always runs after `kk-refactor`**, which renames the identifiers those comments name (`~/.kk-flavor/standards/quality-pipeline.md` → The stages).

## 3. Run and record

Spawn each stage per the protocol's default, in the order **Route** resolved. Relay a stage's blocking question to the human live — you hold the thread and it does not.

When the chain finishes and the instruction tree changed, `scripts/stats.sh --append "<what ran>"`. That row is what the next invocation reads, and skipping it is how the ledger stops being able to answer anything.

## Rules

- **Recommend before you run anything expensive.** Anything that will spawn several agents gets named, with what it will cost, and started only on a yes.
- Never narrow a stage's **lens**. Scope its files to the change set — that is the one narrowing the pipeline requires (`~/.kk-flavor/standards/quality-pipeline.md` → The round).
- A chain that stops early stops the chain. A later stage running on the output of a stage that failed is worse than not running it.
