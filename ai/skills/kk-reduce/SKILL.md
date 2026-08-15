---
name: kk-reduce
description: Shrink and refine a whole ecosystem of agent instructions as a multi-agent campaign — over-cut, adjudicate, fan out scoped passes, reconcile, converge, repair, drive. Use for "shrink the ecosystem", "cut this in half", "de-bloat everything". The campaign above kk-ecosystem, which is one scope's pass; expensive, so run it deliberately.
argument-hint: "the ecosystem root (default: the kk-flavor + skills tree)"
disable-model-invocation: true
---

Cut an ecosystem of agent instructions hard — Phase 0 sets how hard — without losing what steers an agent. This is a **campaign** — many agents, each with a clean context, coordinated by you.

**You orchestrate and do not edit.** The scoped agents apply their own cuts; you set scopes, arbitrate what crosses them, and own the accounting. Read `~/.kk-flavor/standards/skill-protocol.md` (Caller and orchestrator rules) and `~/.kk-flavor/standards/ecosystem.md` (the bar every agent judges against) before Phase 1.

**Every agent below is spawned**, per the protocol's default — a second opinion from a context that already holds your conclusions is not a second opinion. Hand `~/.claude/skills/kk-reduce/AGENT-BRIEF.md` verbatim to every agent that **edits** — Phases 3–5, and only those; it carries their invariants and return contract, so your prompt carries only that agent's scope and delta.

## 0. Baseline

**Start from a committed tree** — git is this campaign's undo, and on a dirty tree the human's own work reverts with the cuts. Dirty → they commit or stash before Phase 1. A clean start also scopes Phase 6's comment scan, which reads the diff against `HEAD`.

**Open the ledger row** — `~/.claude/skills/kk-reduce/scripts/stats.sh --append "<what is about to run>, start"`. Phase 7 closes it; `stats.md` owns everything else about that file.

**Take Phase 1's target from the ledger**, not from a number you invent. How far the tree drifted since the last closing row is what there is to give back. Reach for ≥40% only where the ledger holds no comparable pair.

## 1. Over-cut

One agent, told plainly that **quality is not its job** and that a reviewer will restore what it takes too far. Give it Phase 0's target and instruct it that under-cutting is the only way it fails. Ask for structural collapses — whole files, merged skills, relocated sections — not just line edits, and require a real word count per cut and enough quoted text to locate each one.

## 2. Adjudicate

A second agent turns that list into the plan. Its bias must be stated as explicitly as the cutter's: **default to accept**, because a review that rescues most of the list has failed exactly as badly as a cutter that proposed nothing.

Three things it must do:

- **Verify against the real files.** The cutter will have misquoted, inflated counts, double-counted spans, and named passages that do not exist.
- **Apply the rescue test** — *name the specific wrong action an agent takes without this text*. Not "this is true", not "this is useful". A passage that cannot fill that sentence loses.
- **Catch de-duplication to zero** — the pair of items that each delete a rule citing the other as its home. This is the failure mode of the whole method.

It returns themed scopes, ordered so an earlier theme never invalidates a later one, and writes the plan to `<scratch>/reduce-plan.md` — **outside the repository**, like every campaign file here (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**).

**The plan marks every entry `Accepted`, `Modified`, or `Rescued`**, because `AGENT-BRIEF.md` binds Phase 3 agents to exactly those three labels — a rescue argued only in prose reaches them as nothing. `Modified` carries the verified numbers; `Rescued` names the file the passage must survive in.

## 3. Fan out

One agent per theme, each running `kk-ecosystem` over its own scope.

**Partition by file, not by topic.** Two agents that share a file clobber each other; two with disjoint files run concurrently however related their themes. Sequence only where a real dependency exists — a fold that must land before the file it folds into is deleted, a hoist that must precede the skills it hoists from.

**`check.sh` over the root is yours, not theirs** (`AGENT-BRIEF.md` → **Scope discipline**). You run the wiring check between phases and at the end.

**You own the cross-scope ledger**, at `<scratch>/reduce-cross-scope.md`. Agents return an edit another agent's file needs in their `CROSS-SCOPE:` line; you append them, relay each to the owning agent if it is still running, and **drain the ledger in Phase 4** — every entry applied, or recorded with why it was declined. A *handoff* is not a cross-scope entry: it names a lane rather than an edit, and Phase 6 drains it (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**).

**Keep a campaign record beside it** — the plan path, each theme's scope, whether its agent finished, and every return verbatim. Without it an interrupted campaign leaves a half-cut tree with no way to tell which themes landed.

## 4. Reconcile

The checks below need the whole tree at once, which no Phase 3 agent had.

- **One home, and no home.** Reconcile rules living in several files. Then grep-verify **every `DELETED:` line every agent returned** — open the file each names as still covering the rule and confirm the text is there — not only what the plan rescued, since two agents can each confirm the other's copy exists and each delete its own. A line whose named home no longer holds it is a rule deleted twice; restore one copy.
- **Stale claims.** A wiring check proves the path and the heading a citation names; nothing proves that section still says the thing. Read every citation into a file that was heavily cut.
- **Prefer the mechanism.** Move prose a script can assert into the script (ecosystem.md → **Prefer the mechanism**).
- **Trace the real runs.** Walk each end-to-end path as the agent would, loading files in order, and find where the instruction runs out: a step naming something no file defines, an invocation whose receiving skill no longer expects what the sender sends, a contract with one half deleted.
- **Skill shape**, where skills were cut — invoke `kk-skillcraft`. Run it here and not in Phase 3: what to extract depends on what survived.

## 5. Converge

Fresh agents over the largest remaining scopes, told nothing of what was already cut — only what is protected (**Rules**, below). **Tell them a short return is the correct answer to a converged tree** — an agent measured on findings will manufacture them and undo work that was already argued.

Stop when a round returns little and says so. Two rounds is usually enough; a third produces noise.

## 6. Repair and verify

Cutting damages prose: it stitches sentences together, strands pronouns, leaves terms used before the line defining them, and compresses a rule past the point where its constraint survives. Spawn `kk-tighten` **last**, pointed at the readability floor rather than at volume, and hand it that list — expect it to *add* words.

**Run the tests beside every script the campaign touched, per that script's own header** — it names the case and the mutation run that proves the case can fail. **A header naming no `-test.sh` states `# untested: <why>` instead** (`~/.kk-flavor/standards/ecosystem.md` → **Prefer the mechanism**). Read that reason: this step covers such a script with nothing. A script the campaign changed owes its case in this phase (`~/.kk-flavor/standards/skill-protocol.md` → **Your own fixes are unreviewed code**).

**Then run `~/.claude/skills/kk-humanize/scripts/comment-density.sh`** with no arguments, which scans the diff against `HEAD` — the campaign's own edits exactly, given Phase 0's clean start. A comment's bar is `~/.kk-flavor/standards/human-writing.md` → **Code comments**.

**Then drain the handoffs the phases returned** (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**) — nothing else here reads a script as code.

## 7. Drive it, then close the row

**A gate, not a phase** (`~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**). Every judgment above was reached by reading; this is the campaign's only evidence that the tree still steers. That file orders it first; here it runs last, because the lenses above *are* the change and `kk-tighten` rewrites prose after all of them.

**You write the scenarios; `kk-drive` is handed those and the Phase 4 paths, never the plan or the cut list.** Each of Phase 2's rescues already names the wrong action an agent takes without its passage, so a scenario is a real task plus that action not happening — a driver told what was cut looks for it instead of using the tree.

Its deltas: the entrypoint is a fresh agent reading the shrunk tree, and what it watches is what that agent does — the file it loads, the skill it routes to, the rule it applies.

A `DIVERGED` scenario stops the campaign as a red gate does. Restore the instruction it names, **re-enter Phase 6 over the files you touched** — a restoration made here is otherwise unrepaired and unscanned — then re-drive.

**Close the ledger row** — `~/.claude/skills/kk-reduce/scripts/stats.sh --append "<what ran>"`. **An open item does not live in the note.** Put it where whoever trips over it will read it — a comment at the site, a rule in the file that owns it — and let the note name it in a clause.

## Rules

- **The target is an aim, not a quota** (`AGENT-BRIEF.md` → **On the target**) — accept the first return, and never send an agent back for the number alone.
- **Report the honest total.** Scripts grow when prose becomes enforcement; show that separately rather than netting it out. A comment is not that growth — it is prose in a `.sh` and belongs in the prose total, so only executable lines count as a win.
- Deliberately-argued restorations accumulate across phases. Carry them into every later brief as protected, or the next agent cuts them again with the same reasoning that cut them the first time.
