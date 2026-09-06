---
name: kk-reduce
description: Shrink and refine a whole ecosystem of agent instructions as a multi-agent campaign — over-cut, arbitrate, fan out scoped passes, reconcile, converge, repair, drive. Use for "shrink the ecosystem", "cut this in half", "de-bloat everything". The campaign above kk-ecosystem, which is one scope's pass; expensive, so run it deliberately.
argument-hint: "the ecosystem root (default: the kk-flavor + skills tree)"
disable-model-invocation: true
---

Cut an ecosystem of agent instructions hard — Phase 0 sets how hard — without losing what steers an agent.

**You orchestrate and do not edit.** The scoped agents apply their own cuts; you set scopes, arbitrate what crosses them, and own the accounting. You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**); read it and `~/.kk-flavor/standards/ecosystem.md`, the bar every agent judges against, before Phase 1.

**Every agent below is spawned**, per the protocol's default — a second opinion from a context that already holds your conclusions is not a second opinion. Hand `~/.claude/skills/kk-reduce/AGENT-BRIEF.md` verbatim to every scoped agent that **edits** — Phases 3–5, and only those, never a skill you invoke, which carries its own contract. It carries their invariants and return contract, so your prompt carries only that agent's scope and delta.

## 0. Baseline

**Start from a committed tree** — git is this campaign's undo, and on a dirty tree the human's own work reverts with the cuts. Dirty → they commit or stash before Phase 1.

**Open the `stats.md` row** — `~/.claude/skills/kk-reduce/scripts/stats.sh --append "<what is about to run>, start"`. Phase 7 closes it; `stats.md` owns everything else about that file.

**Take Phase 1's target from `stats.md`**, not from a number you invent. How far the tree drifted since the last closing row is what there is to give back. Reach for ≥40% only where that file holds no comparable pair.

## 1. Over-cut

One agent, told plainly that **quality is not its job** and that a reviewer will restore what it takes too far. Give it Phase 0's target and instruct it that under-cutting is the only way it fails. Ask for structural collapses — whole files, merged skills, relocated sections — not just line edits, and require a real word count per cut and enough quoted text to locate each one.

## 2. Arbitrate

A second agent — the **arbiter** — turns that list into the plan. Its bias must be stated as explicitly as the cutter's: **default to accept**, because a review that rescues most of the list has failed exactly as badly as a cutter that proposed nothing.

Three things it must do:

- **Verify against the real files.** The cutter will have misquoted, inflated counts, double-counted spans, and named passages that do not exist.
- **Apply the rescue test** — *name the specific wrong action an agent takes without this text*. Not "this is true", not "this is useful". A passage that cannot fill that sentence loses.
- **Catch de-duplication to zero** (`AGENT-BRIEF.md` → **The one failure mode that matters**) — here, the pair of *items* that each delete a rule citing the other as its home.

It returns themed scopes, ordered so an earlier theme never invalidates a later one, and writes the plan to `<scratch>/reduce-plan.md`.

**The plan marks every entry `Accepted`, `Modified`, or `Rescued`**, because `AGENT-BRIEF.md` binds Phase 3 agents to exactly those three labels — a rescue argued only in prose reaches them as nothing. `Modified` carries the verified numbers; `Rescued` names the file the passage must survive in.

## 3. Fan out

One agent per theme, each running `kk-ecosystem` over its own scope.

**Partition by file, not by topic.** Two agents that share a file clobber each other; two with disjoint files run concurrently however related their themes. Sequence only where a real dependency exists — a fold that must land before the file it folds into is deleted, a hoist that must precede the skills it hoists from.

**The wiring check over the root is yours, not theirs** (`AGENT-BRIEF.md` → **Scope discipline**) — `~/.claude/skills/kk-ecosystem/scripts/check.sh`. Run it between phases and at the end.

**You own the cross-scope queue**, at `<scratch>/reduce-cross-scope/` — where an agent files the edit another agent's file needs, as a patch (`AGENT-BRIEF.md` → **Scope discipline**). **Drain it as each patch arrives, not in Phase 4**: apply it or record why you declined, then resume its owner with what landed. A patch outliving its author can only be repaired by hand, which `~/.kk-flavor/standards/streaming.md` → **The caller's half** forbids. Phase 4 takes only what arrived after its owner finished. A *handoff* is not a cross-scope entry: it names a lane rather than an edit, and Phase 6 drains it (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**).

**Keep a campaign record beside it** — the plan path, each theme's scope, whether its agent finished, and every return verbatim. Without it an interrupted campaign leaves a half-cut tree with no way to tell which themes landed.

## 4. Reconcile

The reading and the arbitration are yours; an agent handed `AGENT-BRIEF.md` applies each fix (**You orchestrate and do not edit**, above).

- **One home, and no home.** Reconcile rules living in several files. Then grep-verify **every `DELETED:` line every agent returned** — open the file each names as still covering the rule and confirm the text is there — not only what the plan rescued: **de-duplication to zero** survives the plan when two agents each delete their own copy. A line whose named home no longer holds it is a rule deleted twice; restore one copy.
- **Stale claims.** A wiring check proves the path and the heading a citation names; nothing proves that section still says the thing. Read every citation into a file that was heavily cut.
- **Prefer the mechanism.** Move prose a script can assert into the script (`~/.kk-flavor/standards/ecosystem.md` → **Prefer the mechanism**).
- **Trace the real runs.** Walk each end-to-end path as the agent would, loading files in order, and find where the instruction runs out: a step naming something no file defines, an invocation whose receiving skill no longer expects what the sender sends, a contract with one half deleted.
- **Skill shape**, where skills were cut — invoke `kk-skillcraft`. Run it here and not in Phase 3: what to extract depends on what survived.

## 5. Converge

Fresh agents over the largest remaining scopes, told nothing of what was already cut — only what is protected (**Rules**, below). **Tell them a short return is the correct answer to a converged tree** — an agent measured on findings will manufacture them and undo work that was already argued.

Stop when a round returns little and says so.

## 6. Repair and verify

Cutting damages prose: it stitches sentences together, strands pronouns, leaves terms used before the line defining them, and compresses a rule past the point where its constraint survives. Spawn `kk-tighten` **last**, pointed at the readability floor rather than at volume, and hand it that list — expect it to *add* words.

**Run the tests beside every script the campaign touched, per that script's own header** — it names the case and the mutation run that proves the case can fail. A header stating `# untested:` instead (`~/.kk-flavor/standards/ecosystem.md` → **Prefer the mechanism**) leaves this step covering that script with nothing; read its reason. A script the campaign changed owes its case in this phase (`~/.kk-flavor/standards/skill-protocol.md` → **Your own fixes are unreviewed code**).

**Then spawn `kk-humanize` over the campaign's own edits** — the diff against `HEAD`, exactly, given Phase 0's clean start. It runs its own comment scan over them.

**Then drain the handoffs the phases returned** (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**) — nothing else here reads a script as code.

## 7. Drive it, then close the row

**A gate, not a phase** (`~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**). Every judgment above was reached by reading; this is the campaign's only evidence that the tree still steers. That file orders it first; here it runs last, because the lenses above *are* the change and `kk-tighten` rewrites prose after all of them.

**You write the scenarios; `kk-drive` is handed those and the Phase 4 paths, never the plan or the cut list.** Each of Phase 2's rescues already names the wrong action an agent takes without its passage, so a scenario is a real task plus that action not happening — a driver told what was cut looks for it instead of using the tree.

Its deltas: the entrypoint is a fresh agent reading the shrunk tree, and what it watches is what that agent does — the file it loads, the skill it routes to, the rule it applies.

A `DIVERGED` scenario stops the campaign as a red gate does. Restore the instruction it names, **re-enter Phase 6 over the files you touched** — a restoration made here is otherwise unrepaired and unscanned — then re-drive.

**Close the `stats.md` row** — `~/.claude/skills/kk-reduce/scripts/stats.sh --append "<what ran>"`. **An open item does not live in the note** (`~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**). Put it where whoever trips over it will read it — a comment at the site, a rule in the file that owns it — and let the note name it in a clause.

## Rules

- **The target is an aim, not a quota** (`AGENT-BRIEF.md` → **On the target**) — accept the first return, and never send an agent back for the number alone.
- **Report the honest total.** Scripts grow when prose becomes enforcement; show that separately rather than netting it out. A comment is not that growth — it is prose in a `.sh`, so only executable lines count as a win.
- Deliberately-argued restorations accumulate across phases. Carry them into every later brief as protected, or the next agent cuts them again with the same reasoning that cut them the first time.
