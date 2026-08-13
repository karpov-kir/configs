---
name: idsd-qualify
description: Run the quality pipeline over the working tree in fast or full (the pre-merge verdict) mode. Use for "qualify the changes". Several stages, not one review — a single pass over local changes is kk-code-review's; a GitHub PR is kk-pr-review's.
argument-hint: "[fast|full]"
---

Run the quality stages over one change set and leave only the residue that needs the human. Callers: standalone, or `idsd-ship`'s quality pass, which runs this skill **inline** — the named exception to spawning — and relays subagent pauses to the human.

**The round, the stages and the gate check are `~/.kk-flavor/standards/quality-pipeline.md`** — read it; everything below is this skill's delta, and the target is always the working tree.

## Modes

**Fast iterates; full is the verdict** — the pass that must precede a merge: `idsd-ship <intent>` runs it, and `idsd-ship done` refuses to merge on a stamp carrying a `(fast)` entry. Bare `idsd-qualify` is fast.

## Running a pass

1. **`scripts/report.sh check-ignore`** (this skill's dir), before anything else — it keeps the report out of the tree fingerprint, and its exit 1 blocks the pass (**Report**).
2. **Set the base — the report this pass appends to.** **First pass, no report** — `report.sh init "<NNN-slug>"`, or `init "review: <description>"` for a standalone review; a report `idsd-ship` already initialized for this change set is the same report, so keep appending. **A report recording a different intent** — run `report.sh carry` first and route every open item it prints, then `init --force`. **Re-qualify** — keep it and reconcile per the carry rule (**Structure**). `report.sh` resolves the repo from the shell's cwd, so confirm the path `init` prints is the change set's repo, or the cwd has drifted.
3. **`report.sh invalidate`**, once the base is set — `stamp` refuses until you have, because until then the stamp and the stage markers standing there are the previous pass's.

**Take one stage's return at a time**: `report.sh stage-returned <stage>` before you read its findings, then record its items in the report — or `report.sh no-items <stage>` when it surfaced nothing — and only then pick up the next stage's return. Every stage that ran, refactor and retro included, not just the round's. The script holds you to that order.

**Retro runs here, and only when your caller or the human asks for one** — never on the pass's own initiative, and the asking happens **before** the pass starts, so the retro is a stage like any other and the stamp records it truthfully. Read [RETRO-STAGE.md](RETRO-STAGE.md) (this skill's dir) and follow it — the run-log, the spawn, and what to do with the return live there and nowhere else. Its decision-log line is written before `report.sh stamp`, with the rest.

**A stale gate is a Decide item** (`quality-pipeline.md` → **Gates**), never an assumed green, and gate verification precedes the stamp. Under `idsd-ship`, `idsd-build`'s Phase 2 already resolved them.

When all stages complete, stamp: `report.sh stamp "<stage entries>"` — its usage string carries the entry vocabulary. A stage that ran is never stamped skipped, or vice versa: by design, a fast pass whose every stage ran to completion stamps with no `(fast)` entry and passes the merge gate.

**A skip's recorded reason follows *why* it was skipped, not the mode.** Stamp `retro:skipped(not-applicable)` when nobody asked for a retro; stamp `retro:skipped(fast)` when one that *was* asked for is skipped for turnaround. **Stage 3 stamps under `tighten` whichever skill ran it** — `kk-tighten`, `kk-ecosystem`, or `kk-humanize` for the comment pass. **A refactor loop cut short by fast mode is `refactor:partial(fast)`**; `(cap)` is only full mode's iteration cap — in fast mode the single iteration *is* the cap.

**A human's "don't re-qualify" binds the tree it was said about, not the session** — once `report.sh state` prints `re-qualify`, the refusal has expired and you ask again rather than infer consent. Do not compare a bare `git write-tree`: that hashes the human's index, not the fingerprint `report.sh` computes against a throwaway one.

**Report post-processing.** After stamping, when the report has items, apply `kk-tighten`'s lens inline over `.idsd/ship-report.md` before presenting — inline because its target is text you already hold, the exception that `skill-protocol.md` → Caller names. Its **lossless** license is the one that applies: every item and its stakes survive, and a `- [ ]` is never dropped or softened. `kk-humanize` does not apply — the audience is the human mid-run, so house style holds. (The report is check-ignored, so this never invalidates the stamp.)

## Report

`ship-report.md` **always** lives at `.idsd/ship-report.md` and **persists across runs** — the working digest. `scripts/report.sh` (this skill's dir) owns the path and every deterministic operation on it, `idsd-ship`'s lifecycle ones included — never done by hand.

**Before the first write into `.idsd/` — any file, by any skill — run `report.sh check-ignore`.** It is what keeps the directory out of the human's `git add -A`, and in a repo that has never shipped, nothing else has run it; a charter or an intent written first would sit there visible. Its exit 1 blocks the write however its message is worded.

**Two repo modes, decided by whether `.idsd/` is tracked in git** (`report.sh repo-mode` prints which): **committed** — `.idsd/` is part of the durable record; **throwaway** — the whole `.idsd/`, intents and report alike, leaves zero traces, and is committed only if the human promotes it (`idsd-ship promote`). Either way **never commit the report, the copy `init --force` keeps beside it, or a `.gitignore` diff for either**.

### The decision log

`.idsd/decisions.md`, beside the report. **Written for the next agent, not the human** — nothing here is presented, and the chat message never summarises it. It holds **decisions a stage settled without asking**, each with what determined it, and **standing observations** — monitor-only notes and pointers to follow-ups routed out of the report. `~/.kk-flavor/standards/skill-protocol.md` owns the companion rule, **a decision record is never a home for an open question**: that is a `Decide` item or a live ask.

It is durable and tracked **in committed mode only**; in throwaway mode it is local scratch that `done` discards with the rest of `.idsd/`, so anything that must outlive the ship is routed out of it rather than into it. Either way **write it before `report.sh stamp`** — content added afterwards moves the tree out from under `reviewed-tree`, and the merge gate reads the pass as stale. **Read it at pass start**: its standing observations are re-evaluated each pass, and nothing else opens it.

### Structure

```markdown
# Read

- `<path>` — <what it is: surface, scenario, migration, security edge> <optional line range>

<one closing line: files and lines left unread, the surfaces behind them, the gauge covering them>

# Decide

<optional: context several items share, stated once here, never per item>

- [ ] **<the decision, one line>** — recommend: <one clause>.
  <the case: what it is, why it matters, the evidence>
```

The report is **only the residue that needs the human — not a record of the run**:

- **At most two groups**, each only when it has items: **Read** (what to open) and **Decide** (`- [ ]` actions) — no per-stage sections, no summary. **The test: if the human takes no action, it is not in the report.** Reading is an action, noticing is not, so a monitor-only observation goes to the decision log and there is **never a monitor-only group**. Shrinking to nothing is the success case, with two exceptions: a fix the human might want to *reverse*, recorded to ratify or revert, and **a claim this pass falsified** — a `Decide` item, not a tidy-up, since someone planned against it.
- **Order Decide by decision kind**: **forks** (a genuine choice still open), then **ratifications**, then **pending evidence** (blocked on a named signal). A blocking question is asked live, never recorded — except one you asked live and the human did not answer, which becomes a `Decide` item when the pass closes.
- **Open each item with its kind and the action; the evidence follows** — inverting `~/.kk-flavor/standards/writing.md` → **Density**'s item form, because an item here is an action still open, not a change already made. `**Fork —** escape the filter values before interpolating`, then why. The reader decides from that line and reads on only to check it. Last in a dense paragraph, the action is found by whoever reads every line, which on a long report is nobody — and a `Recommendation:` on every item labels nothing, since the kind is what varies and what says whether a choice is theirs to make or a yes to give. **Having no recommendation is itself an opening** — `**Fork —** no recommendation; this is a product call` — and it beats a hedge dressed as advice, which costs them the analysis and gives no action back.
- **Every item stands on its own within the report** — someone who never saw the run understands what it is and why it matters, and can act. Cut run-narration and command strings, never the stakes; when an item needs detail it can't carry, that detail belongs in a durable home, linked.
- **A Read item answers *what must you open*, never *what did the pass read*.** One file, what it is, and the delta — which exports arrived, went or changed shape. Not a summary of it, not why the pass chose it, and **never a list of what went unread**, which is coverage accounting and produces no action. **A finding noticed while reading is a finding**: it goes to `Decide`, or to the stage that owns it. Left inside a Read item, the one line the human had to act on reads as background — which is how a missing assertion ships. **Build the list from the reading tiers code-review returns** (`~/.claude/skills/kk-code-review/SKILL.md` → **Verdict**), demoting nothing on your own, and **promote across stages**: a file `kk-security-review` touched is `Read` whatever tier `kk-code-review` gave it.
- **On re-qualify every unresolved `- [ ]` carries forward verbatim** (`report.sh carry` lists them) — dropped only on positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area. The `# Read` list is rebuilt each pass, never carried.

## After the pass

The closing message points at the report (`~/.kk-flavor/standards/writing.md` → **Replying to a human**): **one status line** — mode, repo mode, what there is to read, item count by decision kind — plus at most one live blocking question; no per-stage verdicts, no retro narrative. After an `idsd-ship` build, surface `idsd-build`'s checkpoint evidence too; in throwaway mode add one line — `.idsd/` is local scratch this run, `/idsd-ship promote` to keep it.

## Rules

- **Never commits or pushes** — fixes stay in the tree; committing is the caller's or the human's act. Any verification that reads a ref runs against the stamped `reviewed-tree`, not the remote.
- **While a stage is live, the tree it was told to read must not move under it** — no rebase, cherry-pick, reset, branch switch or base change until it returns. Either finish the stage, or abandon it, do the maintenance, then `report.sh invalidate` and spawn it fresh against the new HEAD; a separate worktree is the only way to overlap the two safely.
