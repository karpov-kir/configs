---
name: idsd-qualify
description: Run the quality pipeline over the working tree with a persisted report and a merge stamp, in fast or full mode. Use for "qualify the changes" inside an IDSD project. The `.idsd` report layer over kk-qualify — the same pass without a report is that skill's, and its description discriminates the rest.
argument-hint: "[fast|full] [score threshold]"
---

Callers: standalone, or `idsd-ship`'s quality pass, which runs this skill **inline**. You orchestrate, so read `~/.kk-flavor/standards/skill-protocol.md` whole.

**The pass is `~/.claude/skills/kk-qualify/SKILL.md`** — read it and run it **inline**, since it needs the human continuously and they reach only your thread (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). That skill ends at a closing reply; this one adds the report, the stamp and the lifecycle around it. **Every heading below that it also carries is the delta over its section of that name**, and the rest is this skill's own.

## Lanes

Adds the lane it leaves out: **retro** → `idsd-retro`.

## Modes

Bare `idsd-qualify` is fast; `idsd-ship <intent>` runs full, which the merge gate requires.

## Running a pass

1. **`~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore`**, before anything else (**Report**).
2. **Set the base — the report this pass appends to.** With none for this intent, `report.sh init "<NNN-slug>"`, or `init "review: <description>"` for a standalone review. Over an existing one `init` refuses and prints the routing; follow it. `report.sh` resolves the repo from the shell's cwd, so confirm the path `init` prints is the change set's repo — a different path means the cwd drifted.
3. **`report.sh invalidate <intent>`**, once the base is set — `stamp` refuses until you have.

**Take one stage's return at a time.** Run `report.sh stage-returned <stage> <intent>` before you read its findings, then record its items — or `report.sh no-items <stage> <intent>` when it surfaced nothing — and only then pick up the next stage's return. Every stage that ran, refactor and retro included, not just the round's.

**Retro runs only when your caller or the human asks for one.** **Offer one once every other stage has returned, and before `report.sh stamp`**: earlier, the human decides holding nothing the pass found; later, the stamp already reads `retro:skipped` for a stage that then runs. Read [RETRO-STAGE.md](RETRO-STAGE.md) (this skill's dir) and follow it — the whole delta for a retro.

**A stale gate is a Decide item** (`~/.kk-flavor/standards/quality-pipeline.md` → **Gates**), and gate verification precedes the stamp. Under `idsd-ship`, `idsd-build`'s Phase 2 already resolved them.

When all stages complete, stamp: `report.sh stamp "<stage entries>" <intent>` — its usage string carries the entry vocabulary. A stage that ran is never stamped skipped, or vice versa: by design, a fast pass whose every stage ran to completion stamps with no `(fast)` entry and passes the merge gate.

**A skip's recorded reason follows *why* it was skipped, not the mode.** Stamp `retro:skipped(not-applicable)` when nobody asked for a retro; stamp `retro:skipped(fast)` when one that *was* asked for is skipped for turnaround. **Stage 3 stamps under `tighten` whichever skill ran it** — `kk-tighten`, or `kk-humanize` for the comment pass. **A refactor loop cut short by fast mode is `refactor:partial(fast)`**; `(cap)` is only full mode's iteration cap.

**A human's "don't re-qualify" binds the tree it was said about, not the session** — once `report.sh state <intent>` prints `re-qualify`, the refusal has expired and you ask again rather than infer consent.

**Report post-processing.** After stamping, when the report has items, apply `kk-humanize`'s lens **inline** over this pass's report file before presenting — the exception that `~/.kk-flavor/standards/skill-protocol.md` → **Caller** names. **The report is outward text**: a person reads it to decide, so `~/.kk-flavor/standards/human-writing.md` binds it and `kk-tighten`'s internal-prose lens does not. **Lossless, against `kk-humanize`'s lossy license**: every item and its stakes survive, and a `- [ ]` is never dropped or softened. (The report is check-ignored, so this never invalidates the stamp.)

## Report

**One report per intent**, at `.idsd/qualify-reports/<intent>-qualify-report.md`, **persisting across runs** — the working digest. `~/.claude/skills/idsd-qualify/scripts/report.sh` owns the path and every deterministic operation on it, `idsd-ship`'s lifecycle ones included — never done by hand. **Every subcommand that reads a report takes the intent as its last argument**, omitted only while one report is open — so pass it whenever you know it.

**Two intents ship in parallel only in separate worktrees.** The freshness stamp fingerprints the **whole tree**, not the intent's files, so two ships in one worktree each stamp a tree holding the other's edits and invalidate each other's gate on every save.

**A landed ship's report is retired, not left standing** — `report.sh close <intent>`, which `idsd-ship done` runs. **A standalone review has no `done`** — `report.sh close review` retires it, and unsaid, `report.sh list` offers it as work in flight for good.

**Before the first write into `.idsd/` — any file, by any skill — run `report.sh check-ignore`**: it is what keeps the directory out of the human's `git add -A`, and nothing else runs it. Its exit 1 blocks the write however its message is worded.

**Two repo modes, decided by whether `.idsd/` is tracked in git** (`report.sh repo-mode` prints which): **committed** — `.idsd/` is part of the durable record; **throwaway** — the whole `.idsd/`, intents and report alike, leaves zero traces, and survives only if the human promotes it (`idsd-ship promote`). Either way **never commit a report — however that is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): no route commits one, `promote` included, which keeps `qualify-reports/` ignored on its way to committed mode. A stamped report asserts a merge gate over a tree fingerprint, and committed it carries that assertion past the tree it was taken from.

### The decision log

`.idsd/decisions.md`. **Written for the next agent, not the human** — nothing here is presented. It holds **decisions a stage settled without asking**, each with what determined it, and **standing observations** — monitor-only notes and pointers to follow-ups routed out of the report. The companion rule is `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**.

It is an appended record, so `~/.kk-flavor/standards/records.md` is the whole delta. **Its bound is roughly 40 lines.**

Tracked in committed mode only; in throwaway mode `done` discards it, so route out anything that must outlive the ship. **Write it before `report.sh stamp`** — content added afterwards moves the tree out from under `reviewed-tree`, and the merge gate reads the pass as stale. **Read it at pass start**: its standing observations are re-evaluated each pass.

### Structure

```markdown
# Decide

<optional: context several items share, stated once here, never per item>

- [ ] **<N> · <Falsified | Fork | Pending evidence> · <score>/10 —** <the action, one line>
  <the case: what it is, why it matters, the evidence>
  **Recommend:** <the answer, and the branch it loses to>
```

**What earns an item, how it is scored, and the order it lands in are `kk-qualify` → **The residue**.** This file adds only where that residue is written:

- **One group, `Decide`, holding `- [ ]` actions** — no per-stage sections, no summary, and no reading list. A monitor-only observation goes to the decision log.
- **Score on the `report` lane.** What the threshold cuts gets no `- [ ]` — it goes in the closing message as one line (**After the pass**). Not the decision log: a question you still hold is barred from it by the rule **The decision log** already cites.
- **The item's shape is the block above** — inverting `~/.kk-flavor/standards/writing.md` → **Density**'s item form, because an item here is an action still open, not a change already made.
- **On re-qualify every unresolved `- [ ]` carries forward verbatim** (`report.sh carry <intent>` lists them) — dropped only on positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area.

## After the pass

The closing message is `~/.kk-flavor/standards/writing.md` → **Replying to a human**, plus: the **repo mode** on its status line; at most one line for a tooling improvement the retro proposed ([RETRO-STAGE.md](RETRO-STAGE.md) routes it here), and no retro narrative. After an `idsd-ship` build, surface `idsd-build`'s checkpoint evidence too. After a standalone review, one line saying `report.sh close review` retires it (**Report**). In throwaway mode add one line — `.idsd/` is local scratch this run, `/idsd-ship promote` to keep it.

## Rules

- **Any verification that reads a ref runs against the stamped `reviewed-tree`**, not the remote.
- **Resuming after the history moved** is `report.sh invalidate <intent>`, then a fresh spawn against the new HEAD.
