---
name: idsd-qualify
description: Run the quality pipeline over the working tree with a merge stamp. Use for "qualify the changes" inside an IDSD project. The `.idsd` report layer over kk-qualify — the same pass without a report is that skill's.
---

Callers: standalone, or `idsd-ship`'s quality pass, which runs this skill **inline**. You orchestrate, so read `~/.kk-flavor/standards/skill-protocol.md` whole.

**The pass is `~/.claude/skills/kk-qualify/SKILL.md`** — read it and run it **inline**, since it needs the human continuously and they reach only your thread (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). That skill ends at a closing reply; this one adds the report, the stamp and the lifecycle around it. **Every heading below that it also carries is the delta over its section of that name**, and the rest is this skill's own.

## Running a pass

1. **`~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore`**, before anything else (**Report**).
2. **Set the base — the report this pass appends to.** With none for this intent, `report.sh init "<NNN-slug>"`, or `init "review: <description>"` for a standalone review. Over an existing one `init` refuses and prints the routing; follow it. `report.sh` resolves the repo from the shell's cwd, so confirm the path `init` prints is the change set's repo.
3. **`report.sh invalidate <intent>`**, once the base is set — `stamp` refuses until you have.

**Take one stage's return at a time.** Run `report.sh stage-returned <stage> <intent>` before you read its findings, then record its items — or `report.sh no-items <stage> <intent>` when it surfaced nothing — and only then pick up the next stage's return. Every stage that ran takes this, refactor included. **Streamed, a patch is not a return** (`~/.kk-flavor/standards/streaming.md`): cases are read and applied as they arrive, while `stage-returned` still waits for the stage's own verdict.

**A stale gate is a Decide item** (`~/.kk-flavor/standards/quality-pipeline.md` → **Gates**), and gate verification precedes the stamp. Under `idsd-ship`, `idsd-build`'s Phase 2 already resolved them.

When all stages complete, stamp: `report.sh stamp "<stage entries>" <intent>` — **its usage string is the entry vocabulary's only home**, and a copy in this file would drift from the tool that validates it. A stage that ran is never stamped skipped, or vice versa.

**Stage 3 stamps under `tighten` whichever skill ran it** — `kk-tighten`, or `kk-humanize` for the comment pass.

**A human's "don't re-qualify" binds the tree it was said about, not the session** — once `report.sh state <intent>` prints `re-qualify`, the refusal has expired and you ask again rather than infer consent.

**Report post-processing.** After stamping, when the report has items, apply `kk-humanize`'s lens over this pass's report file before presenting — **inline**, since it is text you already hold (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). **The report is outward text**, so `kk-tighten`'s internal-prose lens never runs over it. **Lossless, where `kk-humanize` deletes**: every item and its stakes survive, and a `- [ ]` is never dropped or softened. (The report is check-ignored, so this never invalidates the stamp.)

## Report

**`.idsd/` in this suite means the resolved scratch root, not a path in the repo.** `report.sh root` prints it, and it is the only way to learn it. In **committed** mode it is `<repo>/.idsd/`. In **throwaway** mode it is outside the working tree — shared by every branch and worktree of the clone — so a skill that joins `.idsd/` onto the repo root writes where no other worktree will look, and where the next `git add -A` can see it. Every `.idsd/<file>` below, and in every skill of this suite, is relative to what that subcommand printed.

**One report per intent**, at `.idsd/intents/<intent>/qualify-report.md`, **persisting across runs** — the working digest. `~/.claude/skills/idsd-qualify/scripts/report.sh` owns the path and every deterministic operation on it, `idsd-ship`'s lifecycle ones included — never done by hand. **Every subcommand that reads a report takes the intent as its last argument**, omitted only while one report is open — so pass it whenever you know it.

**Two intents ship in parallel only in separate worktrees.** The freshness stamp fingerprints the **whole tree**, not the intent's files, so two ships in one worktree each stamp a tree holding the other's edits and invalidate each other's gate on every save.

**A landed ship's report is retired, not left standing** — `report.sh close <intent>`, which `idsd-ship done` runs. **A standalone review has no `done`** — `report.sh close review` retires it, and unsaid, `report.sh list` offers it as work in flight for good.

**Before the first write into `.idsd/` — any file, by any skill — run `report.sh check-ignore`**: it is what keeps the directory out of the human's `git add -A`, and nothing else runs it. Its exit 1 blocks the write however its message is worded.

**Two repo modes, decided by whether `.idsd/` is tracked in git** (`report.sh repo-mode` prints which): **committed** — `.idsd/` is part of the durable record; **throwaway** — the whole `.idsd/`, intents and report alike, leaves zero traces, and survives only if the human promotes it (`idsd-ship promote`). Either way **never commit a report — however that is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): no route commits one, `promote` included, which keeps every ship's scratch ignored on its way to committed mode. A stamped report asserts a merge gate over a tree fingerprint, and committed it carries that assertion past the tree it was taken from.

### The decision log

`.idsd/decisions.md`. **Written for the next agent, not the human** — nothing here is presented. It holds **decisions a stage settled without asking**, each with what determined it, and **standing observations** — monitor-only notes and pointers to follow-ups routed out of the report. The companion rule is `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**.

It is an appended record, so `~/.kk-flavor/standards/records.md` is the whole delta.

**Write it only through `report.sh record {append|bump|revise|evict|admit} decisions "<text>"`.** Two hand-run read-modify-writes leave the file holding whichever landed second, with nothing in any diff to say the other's entries went. In throwaway mode every worktree of the clone races for that one copy (**Report**).

Tracked in committed mode only; in throwaway mode `done` discards it, so route out anything that must outlive the ship. **Write it before `report.sh stamp`** — content added afterwards moves the tree out from under `reviewed-tree`, and the merge gate reads the pass as stale.

**Read it at pass start and re-evaluate every entry against the tree — the log is pruned here and nowhere else.** An entry whose subject is gone from the code, or which a later one supersedes, goes now: `record evict`, whatever its count. One still binding on what this pass examines takes `record bump`. **Then `report.sh decisions-reviewed <intent>`, after this pass's `invalidate`** — `stamp` refuses until you have, and `invalidate` clears it, so no pass inherits another's reading of the log.

### The residue

```markdown
# Decide

<optional: context several items share, stated once here, never per item>

- [ ] **<N> · <Falsified | Fork | Pending evidence> —** <the action, one line>
  <the case: what it is, why it matters, the evidence>
  **Recommend:** <the answer>
```

**What earns an item and the order it lands in are `~/.claude/skills/kk-qualify/SKILL.md` → **The residue**.** This file adds only where that residue is written:

- **One group, `Decide`** — no per-stage sections, no summary, and no reading list. A monitor-only observation goes to the decision log.
- **On re-qualify every unresolved `- [ ]` carries forward verbatim** (`report.sh carry <intent>` lists them) — dropped only on positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area.

## After the pass

The closing message adds to `~/.claude/skills/kk-qualify/SKILL.md` → **After the pass**. The **repo mode** rides its status line. After an `idsd-ship` build, surface `idsd-build`'s checkpoint evidence too. After a standalone review, one line saying `report.sh close review` retires it (**Report**). In throwaway mode add one line — `.idsd/` is local scratch this run, `/idsd-ship promote` to keep it.

## Rules

- **Any verification that reads a ref runs against the stamped `reviewed-tree`**, not the remote.
- **Resuming after the history moved** is `report.sh invalidate <intent>`, then a fresh spawn against the new HEAD.
