---
name: idsd-qualify
description: Run the quality pipeline over the working tree in fast or full mode. Use for "qualify the changes". Several stages, not one review — one pass over local changes is kk-code-review's, a GitHub PR kk-pr-review's.
argument-hint: "[fast|full]"
---

Run the quality stages over one change set and leave only the residue that needs the human. Callers: standalone, or `idsd-ship`'s quality pass, which runs this skill **inline**.

**The round, the stages and the gate check are `~/.kk-flavor/standards/quality-pipeline.md`** — read it; everything below is this skill's delta, and the target is always the working tree.

## Lanes

That file names **lanes**, never skills (`~/.kk-flavor/standards/ecosystem.md` → **One home**). These are the skills filling them, and the scanner each one owns — by full path, per `~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins**:

| Lane | Skill | Its scanner |
|---|---|---|
| drive | `kk-drive` | — |
| code-review | `kk-code-review` | — |
| security-review | `kk-security-review` | — |
| prose | `kk-tighten` | — |
| outward-text | `kk-humanize` | `~/.claude/skills/kk-humanize/scripts/comment-density.sh` |
| instruction | `kk-ecosystem` | `~/.claude/skills/kk-ecosystem/scripts/check.sh` |
| refactor | `kk-refactor` | `~/.claude/skills/kk-refactor/scripts/dup-literals.sh` |
| retro | `idsd-retro` | — |

## Modes

**Fast iterates; full is the verdict** — the pass that must precede a merge, and what `idsd-ship <intent>` runs. Bare `idsd-qualify` is fast.

## Running a pass

1. **`~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore`**, before anything else (**Report**).
2. **Set the base — the report this pass appends to.** With none for this intent, `report.sh init "<NNN-slug>"`, or `init "review: <description>"` for a standalone review. Over an existing one `init` refuses and prints the routing; follow it. `report.sh` resolves the repo from the shell's cwd, so confirm the path `init` prints is the change set's repo — a different path means the cwd drifted.
3. **`report.sh invalidate <intent>`**, once the base is set — `stamp` refuses until you have.

**Take one stage's return at a time.** Run `report.sh stage-returned <stage> <intent>` before you read its findings, then record its items — or `report.sh no-items <stage> <intent>` when it surfaced nothing — and only then pick up the next stage's return. Every stage that ran, refactor and retro included, not just the round's.

**Retro runs last, and only when your caller or the human asks for one.** **Offer one once every other stage has returned, and before `report.sh stamp`**: earlier, the human decides holding nothing the pass found; later, the stamp already reads `retro:skipped` for a stage that then runs. Read [RETRO-STAGE.md](RETRO-STAGE.md) (this skill's dir) and follow it — the whole delta for a retro.

**A stale gate is a Decide item** (`quality-pipeline.md` → **Gates**), and gate verification precedes the stamp. Under `idsd-ship`, `idsd-build`'s Phase 2 already resolved them.

When all stages complete, stamp: `report.sh stamp "<stage entries>" <intent>` — its usage string carries the entry vocabulary. A stage that ran is never stamped skipped, or vice versa: by design, a fast pass whose every stage ran to completion stamps with no `(fast)` entry and passes the merge gate.

**A skip's recorded reason follows *why* it was skipped, not the mode.** Stamp `retro:skipped(not-applicable)` when nobody asked for a retro; stamp `retro:skipped(fast)` when one that *was* asked for is skipped for turnaround. **Stage 3 stamps under `tighten` whichever skill ran it** — `kk-tighten`, `kk-ecosystem`, or `kk-humanize` for the comment pass. **A refactor loop cut short by fast mode is `refactor:partial(fast)`**; `(cap)` is only full mode's iteration cap — in fast mode the single iteration *is* the cap.

**A human's "don't re-qualify" binds the tree it was said about, not the session** — once `report.sh state <intent>` prints `re-qualify`, the refusal has expired and you ask again rather than infer consent.

**Report post-processing.** After stamping, when the report has items, apply `kk-tighten`'s lens **inline** over this pass's report file before presenting — the exception that `skill-protocol.md` → **Caller** names. **Lossless**: every item and its stakes survive, and a `- [ ]` is never dropped or softened. `kk-humanize` does not apply — the audience is the human mid-run, so house style holds. (The report is check-ignored, so this never invalidates the stamp.)

## Report

**One report per intent**, at `.idsd/qualify-reports/<intent>-qualify-report.md`, **persisting across runs** — the working digest. `~/.claude/skills/idsd-qualify/scripts/report.sh` owns the path and every deterministic operation on it, `idsd-ship`'s lifecycle ones included — never done by hand. **Every subcommand that reads a report takes the intent as its last argument**, omitted only while one report is open — so pass it whenever you know it.

**Two intents ship in parallel only in separate worktrees.** The freshness stamp fingerprints the **whole tree**, not the intent's files, so two ships in one worktree each stamp a tree holding the other's edits and invalidate each other's gate on every save.

**Every standalone `review: <description>` shares the one `review` stem**, so a worktree holds one open standalone review at a time.

**A landed ship's report is retired, not left standing** — `report.sh close <intent>`, which `idsd-ship done` runs. **A standalone review has no `done`** — `report.sh close review` retires it, and unsaid, `report.sh list` offers it as work in flight for good.

**Before the first write into `.idsd/` — any file, by any skill — run `report.sh check-ignore`**: it is what keeps the directory out of the human's `git add -A`, and nothing else runs it. Its exit 1 blocks the write however its message is worded.

**Two repo modes, decided by whether `.idsd/` is tracked in git** (`report.sh repo-mode` prints which): **committed** — `.idsd/` is part of the durable record; **throwaway** — the whole `.idsd/`, intents and report alike, leaves zero traces, and is committed only if the human promotes it (`idsd-ship promote`). Either way **never commit a report**.

### The decision log

`.idsd/decisions.md`. **Written for the next agent, not the human** — nothing here is presented. It holds **decisions a stage settled without asking**, each with what determined it, and **standing observations** — monitor-only notes and pointers to follow-ups routed out of the report. `~/.kk-flavor/standards/skill-protocol.md` owns the companion rule, **a decision record is never a home for an open question**.

It is an appended record, so `~/.kk-flavor/standards/records.md` is the whole delta. **Its bound is roughly 40 lines.**

Tracked in committed mode only; in throwaway mode `done` discards it, so route out anything that must outlive the ship. **Write it before `report.sh stamp`** — content added afterwards moves the tree out from under `reviewed-tree`, and the merge gate reads the pass as stale. **Read it at pass start**: its standing observations are re-evaluated each pass.

### Structure

```markdown
# Decide

<optional: context several items share, stated once here, never per item>

- [ ] **<Falsified | Fork | Pending evidence> —** <the action, one line>
  <the case: what it is, why it matters, the evidence>
  **Recommend:** <the answer, and the branch it loses to>
```

The report is **only the residue that needs the human — not a record of the run**:

- **One group, `Decide`, holding `- [ ]` actions** — no per-stage sections, no summary, and no reading list: **if the human takes no action, it is not in the report.** A monitor-only observation goes to the decision log, and there is **never a monitor-only group**. Shrinking to nothing is the success case.
- **An item earns its place by the report-item test** in `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** — read it. **An applied fix is not an item.** The diff is its record. Where the fix traded something they may weigh differently — behaviour they relied on, a cost, a constraint they wrote — the open question is *which way*. That is a fork, and its default is what the tree now does. **Where the losing branch names no belief the human could hold, it is not a fork either** — keep-it-or-revert clears the test on a technicality: the two answers do lead to different acts, but it is no choice at all.
- **Order Decide by decision kind**: **falsified** (a claim this pass disproved) first, because it can change the answer to everything under it; then **forks** (a genuine choice still open); then **pending evidence** (blocked on a named signal). **A falsified item has no branch to lose to**, so its line closes on what the human must now re-decide. Where that is nothing, or where the re-deciding is a correction you can make, it is a tidy-up and there is no item. A blocking question is asked live, never recorded — except one you asked live and the human did not answer, which becomes a `Decide` item when the pass closes.
- **The item's shape is the block above** — inverting `~/.kk-flavor/standards/writing.md` → **Density**'s item form, because an item here is an action still open, not a change already made. **The recommendation closes the item on its own line.** **Having none is itself an opening**: `**Recommend:** nothing — this is a product call` beats a hedge dressed as advice.
- **Every item stands on its own within the report** — someone who never saw the run understands what it is and why it matters, and can act. Cut run-narration and command strings, never the stakes; when an item needs detail it can't carry, that detail belongs in a durable home, linked.
- **On re-qualify every unresolved `- [ ]` carries forward verbatim** (`report.sh carry <intent>` lists them) — dropped only on positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area.

## After the pass

The closing message points at the report (`~/.kk-flavor/standards/writing.md` → **Replying to a human**): **one status line** — mode, repo mode, item count by decision kind — plus at most one live blocking question; at most one line for a tooling improvement the retro proposed ([RETRO-STAGE.md](RETRO-STAGE.md) routes it here); no per-stage verdicts, no retro narrative. After an `idsd-ship` build, surface `idsd-build`'s checkpoint evidence too; after a standalone review, one line saying `report.sh close review` retires it (**Report**); in throwaway mode add one line — `.idsd/` is local scratch this run, `/idsd-ship promote` to keep it.

## Rules

- **Never commits or pushes** — fixes stay in the tree; committing is the caller's or the human's act. Any verification that reads a ref runs against the stamped `reviewed-tree`, not the remote.
- **While a stage is live, the history under it must not move** — until it returns, nothing that rewrites or repoints it: rebase, cherry-pick, reset, amend, branch switch, base change. A working-tree edit is a different thing, already answered by `~/.kk-flavor/standards/skill-protocol.md` → **Loop**, and it is what lets a caller apply patches beneath a live stage (`~/.kk-flavor/standards/streaming.md`). Finish the stage, or abandon it, do the maintenance, then `report.sh invalidate <intent>` and spawn it fresh against the new HEAD; a separate worktree is the only safe overlap.
