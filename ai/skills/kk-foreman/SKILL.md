---
name: kk-foreman
description: The front door to the kk-* skills and the installed tool skills — say what you want done and this picks them, orders them, and runs them. Use when the work crosses more than one, when you don't know which applies, when it has to land in another system (a ticket, a page), or for a periodic "what does this repo need?". The idsd-* intent workflow is its own door (idsd-ship).
argument-hint: "what you want done (default: look at the working tree and recommend)"
disable-model-invocation: true
---

You **dispatch and do not do the work** — every stage is a skill that already exists, invoked per `~/.kk-flavor/standards/skill-protocol.md`. **Authoring is the exception**: no skill here drafts a PR edit or a ticket body from nothing, so you write the first version and route it.

**This file holds no catalogue of what each skill does.** Their own `description:` fields are that (`~/.kk-flavor/standards/ecosystem.md` → **One home**). Resolve candidates at run time by reading the frontmatter under `~/.claude/skills/*/SKILL.md` — that also finds skills whose `disable-model-invocation: true` keeps them out of your context.

**That mount is the candidate set.** Not every skill you can invoke sits on it: the harness's bundled and plugin skills do not, and several of those are lanes whose triggers nearly duplicate a `kk-*` one. **Off that mount, the human names the skill or you do not use it** — picking one silently is how a run loses the `kk-*` lane's own rules while looking like it ran it.

## 1. Read the state before choosing

**Only when the work touches the agent instruction tree** — skills, standards, prompts, templates, `CLAUDE.md`. The ledger measures that tree and nothing else, so in any other repo there is nothing to read: go straight to **Route**.

Run `~/.claude/skills/kk-reduce/scripts/stats.sh` and read `~/.claude/skills/kk-reduce/stats.md`, which is a level above it. On exit 2, fix what the script names; never treat it as no change. **You read that file; `kk-reduce` writes it** — one row before a campaign and one after, so the rows are the reductions and nothing else.

Decide from the delta, never from a threshold — a number invented here would just teach later passes to trim words until they clear it.

## 2. Route

**Start at the smallest skill that covers the work, and escalate only on evidence.** One `kk-*` skill run directly — whichever description matches — is the common answer, and three touched lines get one of those, not a chain: say which skill and run it, without ceremony.

Escalate on something you can name: the change crosses several lanes at once, a scoped pass already ran and left the problem standing, or the ledger shows drift no single pass reaches. "It might find more" is not evidence — it is how every request becomes the most expensive one.

The chains below are the ones where order is load-bearing and an agent choosing one skill at a time gets it wrong:

| The work | The chain |
|---|---|
| Code or tooling changed | `kk-drive`, then `kk-code-review` and `kk-security-review`, then `kk-refactor`, then `kk-humanize` over the comments. That order, each stage's own trigger, and what may be dropped are `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it** and → **The stages**; **`kk-refactor` is never dropped.** |
| A PR | `kk-pr-review`, alone — it spawns the pipeline's stages itself, so queuing the code row over the same diff reviews it twice. |
| Changes were requested on a PR | Make the edits, or name `idsd-ship` if the change is intent-shaped, then run the code row over what you changed, before you push. **`kk-pr-review` answers nothing**: it is never how a comment gets addressed, and exactly how the edits get re-reviewed once they are in. |
| Something has to happen in another system — a ticket, a page, a message | The tool skill that owns it does the acting; you order the `kk-*` work around it (**Tool skills**, below). |
| Prose changed | `kk-tighten`, `kk-humanize`, or **both**, tighten first so its handoff reaches humanize — their own descriptions split which prose is whose. Neither needs an orchestrator. |
| Skills, standards, prompts or templates changed | `kk-ecosystem` over the diff, alone — it owns that lane end to end and spawns `kk-skillcraft` and `kk-tighten` itself, so queuing either beside it runs them twice and out of order. |
| The tree has grown well past its last reduction | `kk-reduce` — a campaign, not a pass. |
| A plan or a decision, with nothing built yet | `kk-grill`, alone. Every row above reviews something that exists; this is the only one that reaches a choice while it is still cheap to change. |
| Nothing named, or a periodic check | Recommend from what changed, plus the ledger's trend where step 1 read one. Recommending nothing is a valid outcome. |

**The `idsd-*` suite is a workflow the human enters deliberately and stays inside.** When the work is plainly intent-shaped (an ICE to author, an intent to build, a change heading for that pipeline's merge gate), say so and name `idsd-ship` as its door, then stop. Do not sequence its stages, and do not substitute a `kk-*` chain for it: `idsd-qualify` writes a report and stamps a merge gate, which no chain here reproduces.

**More than one row will often match** — a change set that touches skills and their scripts matches two. Run them as one chain rather than one after the other: stages whose file sets are disjoint go concurrently, and where the sets overlap the rows keep their own order. Two full chains run back to back review the same files twice and let the second undo the first.

**When rows merge:** a stage another row's skill already spawns is not queued again — it runs there, in that skill's own order. Where two rows queue the same skill, the row listed first above fixes its position.

### Tool skills

**A tool skill acts in a system this repo does not own** — a tracker, a wiki, an API. You spot one by a name on that mount that is neither `kk-*` nor `idsd-*`, and you route it by its own `description:` like any other. When a skill owns a system, never reach for a raw API call of your own — reads included.

**An MCP server acts in an outside system too, and is not a skill.** Its tools are that system's sanctioned interface, not the raw call the line above rules out. Where a skill and an MCP server both reach one system, **the human names which**.

**The send goes last**, by a skill or an MCP tool, and the ordering is `~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act** — a create is an external write, which is that file's trigger. Here it means draft the ticket body, run the prose lane, show the human, then create.

**A tool skill is an action; the `kk-*` skills are lanes over its text.** Its return is text you now own, and it re-enters **Route** like any other prose.

## 3. Run

Spawn each stage per the protocol's default, in the order **Route** resolved. Relay a stage's blocking question to the human live — you hold the thread and it does not.

**A handoff a stage returns is placed by Route like any other stage** (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**): one naming a skill already queued merges into that row rather than running twice.

## Rules

- **Recommend before you run anything expensive.** Anything that will spawn several agents gets named, with what it will cost, and started only on a yes.
- Never narrow a stage's **lens**. Scope its files to the change set — that is the one narrowing the pipeline requires (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**).
- A chain that stops early stops the chain. A later stage running on the output of a stage that failed is worse than not running it.
