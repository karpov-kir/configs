---
name: kk-qualify
description: Run the multi-stage quality pipeline over a change set, in any repo. Use for "qualify the changes", "run a quality pass". Several stages, not one review — one pass over local changes is kk-code-review's, a GitHub PR kk-pr-review's. A caller that needs the pass written down with a merge stamp layers that on top of this one.
argument-hint: "[scope: a path, a diff selector, or natural language]"
---

**The round, the stages and the gate check are `~/.kk-flavor/standards/quality-pipeline.md`** — read it; everything below is this skill's delta. The target is the working tree unless your caller names another. **Nothing waits on your pass unless your caller says it does**: bare, you may trim for turnaround, and you say what you trimmed.

**No persisted state.** No report file, no stamp, no directory of your own — a run's scratch ledgers and patch queue are not that. The residue reaches the human in your closing reply and nowhere else. **A caller that needs it to outlive the run owns that home and says so.**

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**); the per-file queue and loop belong to the subagents you spawn.

## Lanes

`~/.kk-flavor/standards/quality-pipeline.md` names **lanes**, never skills. These are the skills filling them, and the scanner to run for each:

| Lane | Skill | Scanner to run | Tier |
|---|---|---|---|
| diagnosis | `kk-diagnose` | — | — |
| drive | `kk-drive` | — | — |
| code-review | `kk-code-review` | — | `code-review` |
| security-review | `kk-security-review` | — | `security` |
| prose | `kk-tighten` | — | — |
| outward-text | `kk-humanize` | — | `comments` |
| refactor | `kk-refactor` | `~/.claude/skills/kk-refactor/scripts/dup-literals.sh` | `refactor` |

**This table is the map for any caller running a lane by name.**

**Diagnosis is a destination, never a stage of the round** (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**).

**Two absences here are deliberate, and neither is yours to run** (`~/.kk-flavor/standards/quality-pipeline.md` → **The stages**): the **instruction lane**, which `kk-ecosystem` fills; and a retrospective, which is no lane at all and belongs to `kk-retro`.

**You are the streamed path's caller** — `~/.kk-flavor/standards/streaming.md` is the whole delta for it, and **its test, not this table, decides whether a given pass streams at all**. Where it does, **every tiered row goes out in the round's one message — `kk-refactor` and `kk-humanize` alongside the reviews, never after them**. The **Tier** column is what each one's spawn prompt carries in its patch-queue slot. A lane with no tier runs unstreamed: prose in the round with the rest, drive still the gate before it.

## The residue

**Only what needs the human, never a record of the run.** If they take no action, it is not residue, and there is never a monitor-only group.

- **An item earns its place by the report-item test** in `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** — read it. **An applied fix is not an item**; the diff is its record. Where the fix traded something the human may weigh differently, the open question is *which way* — that is a **fork**, and its default is what the tree now does. **Where the losing branch names no belief the human could hold, it is not a fork either.**
- **Order by how much each takes the human's own eyes, most first** — the kind rides as a label, never a group: **falsified** (a claim this pass disproved), **fork** (a genuine choice still open), **pending evidence** (blocked on a named signal that does not exist yet). A falsified claim is not more urgent than a fork, it is a prerequisite, and the item's own text carries that: **a falsified item has no branch to lose to**, so it closes on what the human must now re-decide; where that is nothing, it is a tidy-up and there is no item.
- **Number the items in the order they land**, so the human can name one. The number is positional and good only for the pass in front of them — say so, because the next pass renumbers.
- **A blocking question is asked live and never recorded**, except the unanswered one (`~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**). **A drive step the human dropped is neither** — it is one line in the closing reply (**After the pass**); asking before dropping is `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**.
- **Each item stands alone** — someone who never saw the run understands what it is, why it matters, and can act. Cut run-narration and command strings, never the stakes.
- **Roughly 60 words an item, and the recommendation closes it on its own line** — this skill's bound on the exception `~/.kk-flavor/standards/writing.md` → **Density** licenses. Over the bound the surplus is the case restated for a reader who has just read it. **Having no recommendation is itself an opening**: "nothing — this is a product call" beats a hedge dressed as advice.
- **Last, with the items written, run `~/.kk-flavor/scripts/bloat-judge.sh report` over the residue's text** — what it names is deleted; the items stay, since each is a decision the human owes.

## After the pass

`~/.kk-flavor/standards/writing.md` → **Replying to a human** owns the shape. **One status line** — what the pass ran, and item count by decision kind — then the items in the order above, then **one line for every drive step the human dropped when asked**, named and not argued. No per-stage verdicts.

## Rules

- **Never commits or pushes** — fixes stay in the tree (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**).
- **While a stage is live, the history under it must not move**: no rebase, cherry-pick, reset, amend, branch switch or base change until it returns. A working-tree edit is a different thing, already answered by `~/.kk-flavor/standards/skill-protocol.md` → **Loop**. Finish the stage or abandon it, do the maintenance, then spawn it fresh against the new HEAD; a separate worktree is the only safe overlap.
