---
name: kk-qualify
description: Run the multi-stage quality pipeline over a change set, in any repo. Use for "qualify the changes", "run a quality pass". Several stages, not one review — one pass over local changes is kk-code-review's, a GitHub PR kk-pr-review's; the `.idsd` report and merge stamp are idsd-qualify's, which stacks on this.
argument-hint: "[fast|full] [scope: a path, a diff selector, or natural language] [score threshold]"
---

**The round, the stages and the gate check are `~/.kk-flavor/standards/quality-pipeline.md`** — read it; everything below is this skill's delta. The target is the working tree unless your caller names another.

**No persisted state.** No report file, no stamp, no `.idsd/`. The residue reaches the human in your closing reply and nowhere else. **A caller that needs it to outlive the run owns that home and says so** — `idsd-qualify` is the one that does.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**); the per-file queue and loop belong to the subagents you spawn.

## Lanes

`~/.kk-flavor/standards/quality-pipeline.md` names **lanes**, never skills (`~/.kk-flavor/standards/ecosystem.md` → **One home**). These are the skills filling them, and the scanner to run for each:

| Lane | Skill | Scanner to run |
|---|---|---|
| drive | `kk-drive` | — |
| code-review | `kk-code-review` | — |
| security-review | `kk-security-review` | — |
| prose | `kk-tighten` | — |
| outward-text | `kk-humanize` | `~/.claude/skills/kk-humanize/scripts/comment-density.sh` |
| refactor | `kk-refactor` | `~/.claude/skills/kk-refactor/scripts/dup-literals.sh` |

**This table is the map for any caller running a lane by name.**

**Two lanes are deliberately absent.** The agents' own instruction tree is `kk-ecosystem`'s, never a stage here (`~/.kk-flavor/standards/quality-pipeline.md` → **The stages**). A retro needs a home for its residue, so it belongs to a caller that persists one (`idsd-qualify`). **A change set holding either names it in your return** and you run neither.

## Modes

**Fast iterates; full is the verdict** — the pass that must precede a merge. Bare invocation is fast.

## The residue

**Only what needs the human, never a record of the run.** If they take no action, it is not residue: shrinking to nothing is the success case, and there is never a monitor-only group.

- **An item earns its place by the report-item test** in `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** — read it. **An applied fix is not an item**; the diff is its record. Where the fix traded something the human may weigh differently, the open question is *which way* — that is a **fork**, and its default is what the tree now does. **Where the losing branch names no belief the human could hold, it is not a fork either.**
- **Score what clears that test** (`~/.kk-flavor/standards/writing.md` → **Score what survives**) — here the reader's need is how much it takes their own eyes. At or below the lane's threshold it is one line in the closing reply.
- **Order by score, highest first** — the kind rides as a label, never a group: **falsified** (a claim this pass disproved), **fork** (a genuine choice still open), **pending evidence** (blocked on a named signal that does not exist yet). A falsified claim is not more urgent than a fork, it is a prerequisite, and the item's own text carries that: **a falsified item has no branch to lose to**, so it closes on what the human must now re-decide; where that is nothing, it is a tidy-up and there is no item.
- **Number the items in the order they land**, so the human can name one. The number is positional and good only for the pass in front of them — say so, because the next pass renumbers and "item 3" then means something else.
- **A blocking question is asked live and never recorded** — except one you asked and they did not answer, which becomes an item when the pass closes. **A drive step the human dropped is neither** — it is one line in the closing reply (**After the pass**); asking before dropping is `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**.
- **Each item stands alone** — someone who never saw the run understands what it is, why it matters, and can act. Cut run-narration and command strings, never the stakes.
- **Roughly 60 words an item, and the recommendation closes it on its own line** — an item inverts `~/.kk-flavor/standards/writing.md` → **Density**'s one-line form, because it is an action still open rather than a change already made. Over the bound the surplus is the case restated for a reader who has just read it. **Having no recommendation is itself an opening**: "nothing — this is a product call" beats a hedge dressed as advice.

## After the pass

`~/.kk-flavor/standards/writing.md` → **Replying to a human** owns the shape. **One status line** — mode and item count by decision kind — then the items in the order above, then **one line for everything the threshold cut** and every drive step the human dropped when asked, named and not argued. No per-stage verdicts.

## Rules

- **Never commits or pushes** — fixes stay in the tree; committing is the caller's or the human's act.
- **While a stage is live, the history under it must not move**: no rebase, cherry-pick, reset, amend, branch switch or base change until it returns. A working-tree edit is a different thing, already answered by `~/.kk-flavor/standards/skill-protocol.md` → **Loop**. Finish the stage or abandon it, do the maintenance, then spawn it fresh against the new HEAD; a separate worktree is the only safe overlap.
