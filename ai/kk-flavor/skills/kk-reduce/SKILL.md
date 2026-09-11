---
name: kk-reduce
description: Shrink and refine a whole ecosystem of agent instructions as a multi-agent campaign — over-cut, arbitrate, fan out scoped passes, reconcile, converge, repair, drive. Use for "shrink the ecosystem", "cut this in half", "de-bloat everything". The campaign above kk-ecosystem, which is one scope's pass; expensive, so run it deliberately.
argument-hint: "the ecosystem root (default: the kk-flavor + skills tree)"
disable-model-invocation: true
audience: maintainer
---

**Runs:** inline — human

Cut an ecosystem of agent instructions hard — Phase 0 sets how hard — without losing what steers an agent.

**You orchestrate and do not edit.** The scoped agents apply their own cuts; you set scopes, arbitrate what crosses them, and own the accounting. You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**); read it and `~/.kk-flavor/standards/ecosystem.md`, the bar every agent judges against, before Phase 1.

**This campaign uses independent workers for the phases below** — a second opinion from a context that already holds your conclusions is not a second opinion. Each one's prompt is its own file, so yours carries only that agent's scope and delta. **Hand `~/.kk-flavor/skills/kk-reduce/AGENT-BRIEF.md` verbatim to Phase 3's agents**: they run `kk-ecosystem`, whose contract does not name the brief, and the brief is where every editing agent's invariants and return contract live. Phases 4 and 5 read it from their own prompts. **Never hand it to `kk-skillcraft` or `kk-edit`**, which carry their own.

## 0. Baseline

**Start from a committed tree** — git is this campaign's undo, and on a dirty tree the human's own work reverts with the cuts. Dirty → they commit or stash before Phase 1.

**Open the `stats.md` row** — `~/.kk-flavor/skills/kk-reduce/scripts/stats.sh --agent="${ECO_AGENT:?choose claude or codex explicitly}" --append "<what is about to run>, start"`. Phase 7 closes it; `stats.md` owns everything else about that file.

**Take Phase 1's target from `stats.md`**, not from a number you invent. How far the tree drifted since the last closing row is what there is to give back. Reach for ≥40% only where that file holds no comparable pair.

## 1. Over-cut

One agent, `~/.kk-flavor/workers/reduce/over-cut.md`. Give it the tree, Phase 0's target, and every passage an earlier round argued back in as protected (**Rules**). Its bias, its bar and what it owes per cut are that file's.

## 2. Arbitrate

A second agent — the **arbiter**, `~/.kk-flavor/workers/reduce/arbitrate.md` — turns that list into the plan. Give it the cutter's list and the plan path, `<scratch>/reduce-plan.md`. Its bias is stated there as explicitly as the cutter's, and so is what it has to verify before accepting a cut.

**Read the plan yourself before Phase 3.** Its scope order is what decides which themes may run at once, and its labels are the vocabulary `AGENT-BRIEF.md` → **The plan's authority** binds every agent below to (`~/.kk-flavor/workers/reduce/arbitrate.md` → **What you return**).

## 3. Fan out

One agent per theme, each running `kk-ecosystem` over its own scope and dispatched as `reduce/fan-out`, so the campaign accounts for that pass under its own row (`~/.kk-flavor/standards/model-policy.md`).

**The plan's scopes are already partitioned by file** (`~/.kk-flavor/workers/reduce/arbitrate.md` → **What you return**), so run them concurrently. Sequence only where a real dependency exists — a fold that must land before the file it folds into is deleted, a hoist that must precede the skills it hoists from.

**The wiring check over the root is yours, not theirs** (`AGENT-BRIEF.md` → **Scope discipline**) — `~/.kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent="${ECO_AGENT:?choose claude or codex explicitly}"`. Run it between phases and at the end.

**You own the cross-scope queue**, at `<scratch>/reduce-cross-scope/` — where an agent files the edit another agent's file needs, as a patch (`AGENT-BRIEF.md` → **Scope discipline**). **Drain it as each patch arrives, not in Phase 4**: apply it or record why you declined, then resume its owner with what landed. A patch outliving its author can only be repaired by hand, which `~/.kk-flavor/standards/streaming.md` → **The caller's half** forbids. Phase 4 takes only what arrived after its owner finished. A *handoff* is not a cross-scope entry: it names a lane rather than an edit, and Phase 6 drains it (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**).

**Keep a campaign record beside it** — the plan path, each theme's scope, whether its agent finished, and every return verbatim. Without it an interrupted campaign leaves a half-cut tree with no way to tell which themes landed.

## 4. Reconcile

The reading and the arbitration are yours; `~/.kk-flavor/workers/reduce/reconcile.md` applies each fix (**You orchestrate and do not edit**, above).

- **One home, and no home.** Reconcile rules living in several files. Then grep-verify **every `DELETED:` line every agent returned** — open the file each names as still covering the rule and confirm the text is there — not only what the plan rescued: **de-duplication to zero** survives the plan when two agents each delete their own copy. A line whose named home no longer holds it is a rule deleted twice; restore one copy.
- **Stale claims.** A wiring check proves the path and the heading a citation names; nothing proves that section still says the thing. Read every citation into a file that was heavily cut.
- **Prefer the mechanism.** Move prose a script can assert into the script (`~/.kk-flavor/standards/ecosystem.md` → **Prefer the mechanism**).
- **Trace the real runs.** Walk each end-to-end path as the agent would, loading files in order, and find where the instruction runs out: a step naming something no file defines, an invocation whose receiving skill no longer expects what the sender sends, a contract with one half deleted.
- **Skill shape**, where skills were cut — invoke `kk-skillcraft`. Run it here and not in Phase 3: what to extract depends on what survived.

## 5. Converge

Fresh agents over the largest remaining scopes, `~/.kk-flavor/workers/reduce/converge.md`. Give each its scope and the protected list (**Rules**, below), and **nothing about what was already cut** — that omission is the phase, so a prompt that helpfully summarises the campaign destroys it.

Stop when a round returns little and says so.

## 6. Repair and verify

Cutting damages prose: it stitches sentences together, strands pronouns, leaves terms used before the line defining them, and compresses a rule past the point where its constraint survives. Spawn `kk-edit` **last**, as `reduce/repair`, pointed at the readability floor rather than at volume, and hand it that list — expect it to *add* words.

**Run the tests beside every script the campaign touched, per that script's own header** — it names the case and the mutation run that proves the case can fail. A header stating `# untested:` instead (`~/.kk-flavor/standards/ecosystem.md` → **Prefer the mechanism**) leaves this step covering that script with nothing; read its reason. A script the campaign changed owes its case in this phase (`~/.kk-flavor/standards/skill-protocol.md` → **Your own fixes are unreviewed code**).

Include comments from the campaign's own edits in that same `kk-edit` pass; do not spawn a second editor.

**Then drain the handoffs the phases returned** (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**) — nothing else here reads a script as code.

## 7. Drive it, then close the row

**A gate, not a phase** (`~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**). Every judgment above was reached by reading; this is the campaign's only evidence that the tree still steers. That file orders it first; here it runs last, because the lenses above *are* the change and `kk-edit` rewrites prose after all of them.

**You write the scenarios; `kk-drive` is handed those and the Phase 4 paths, never the plan or the cut list.** Each of Phase 2's rescues already names the wrong action an agent takes without its passage, so a scenario is a real task plus that action not happening — a driver told what was cut looks for it instead of using the tree.

Its deltas: the entrypoint is a fresh agent reading the shrunk tree, and what it watches is what that agent does — the file it loads, the skill it routes to, the rule it applies.

A `DIVERGED` scenario stops the campaign as a red gate does. Restore the instruction it names, **re-enter Phase 6 over the files you touched** — a restoration made here is otherwise unrepaired and unscanned — then re-drive.

**Close the `stats.md` row** — `~/.kk-flavor/skills/kk-reduce/scripts/stats.sh --agent="${ECO_AGENT:?choose claude or codex explicitly}" --append "<what ran>"`. **An open item does not live in the note** (`~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**). Put it where whoever trips over it will read it — a comment at the site, a rule in the file that owns it — and let the note name it in a clause.

## Rules

- **The target is an aim, not a quota** (`AGENT-BRIEF.md` → **On the target**) — accept the first return, and never send an agent back for the number alone.
- **Report the honest total.** Scripts grow when prose becomes enforcement; show that separately rather than netting it out. A comment is not that growth — it is prose in a `.sh`, so only executable lines count as a win.
- Deliberately-argued restorations accumulate across phases. Carry them into every later brief as protected, or the next agent cuts them again with the same reasoning that cut them the first time.
