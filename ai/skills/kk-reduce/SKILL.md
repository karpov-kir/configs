---
name: kk-reduce
description: Shrink and refine a whole ecosystem of agent instructions as a multi-agent campaign — over-cut, adjudicate, fan out scoped passes, reconcile, converge, repair. Use for "shrink the ecosystem", "cut this in half", "de-bloat everything". The campaign above kk-ecosystem, which is one scope's pass; expensive, so run it deliberately.
argument-hint: "the ecosystem root (default: the kk-flavor + skills tree)"
disable-model-invocation: true
---

Cut an ecosystem of agent instructions roughly in half without losing what steers an agent. One pass cannot do this: a single agent holding the whole tree runs out of attention long before it runs out of files, and an agent scoped to one file cannot see the rule stated in two. So this is a **campaign** — many agents, each with a clean context, coordinated by you.

**You orchestrate and do not edit.** The scoped agents apply their own cuts; you set scopes, arbitrate what crosses them, and own the accounting. Read `~/.kk-flavor/standards/skill-protocol.md` (Caller and orchestrator rules) and `~/.kk-flavor/standards/ecosystem.md` (the bar every agent judges against) before Phase 1.

**Every agent below is spawned**, per the protocol's default — a second opinion from a context that already holds your conclusions is not a second opinion. Hand `AGENT-BRIEF.md` (this skill's dir) verbatim to every agent that **edits** — Phases 3–5, and only those; it carries their invariants and return contract, so your prompt carries only that agent's scope and delta. Phases 1 and 2 return a list and a plan, not edits, and take their instructions from their own sections below.

## 1. Over-cut

One agent, told plainly that **quality is not its job** and that a reviewer will restore what it takes too far. Target ≥40% and instruct it that under-cutting is the only way it fails. Ask for structural collapses — whole files, merged skills, relocated sections — not just line edits, and require a real word count per cut and enough quoted text to locate each one.

## 2. Adjudicate

A second agent turns that list into the plan. Its bias must be stated as explicitly as the cutter's: **default to accept**, because a review that rescues most of the list has failed exactly as badly as a cutter that proposed nothing.

Three things it must do, none of which the cutter did:

- **Verify against the real files.** The cutter will have misquoted, inflated counts, double-counted spans, and named passages that do not exist.
- **Apply the rescue test** — *name the specific wrong action an agent takes without this text*. Not "this is true", not "this is useful". A passage that cannot fill that sentence loses.
- **Catch de-duplication to zero** — the pair of items that each delete a rule citing the other as its home. This is the failure mode of the whole method; hunt it by name.

It returns themed scopes, ordered so an earlier theme never invalidates a later one, and writes the plan to `<scratch>/reduce-plan.md` — **outside the repository**, like every campaign file here (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**).

**The plan marks every entry `Accepted`, `Modified`, or `Rescued`**, because `AGENT-BRIEF.md` binds Phase 3 agents to exactly those three labels. A rescue argued only in prose reaches them as nothing, and "anything the plan did not consider is still yours to cut" then licenses an agent carrying a word target to cut the passage this phase fought to keep. `Modified` carries the verified numbers; `Rescued` names the file the passage must survive in.

## 3. Fan out

One agent per theme, each running `kk-ecosystem` over its own scope.

**Partition by file, not by topic.** Two agents that share a file will clobber each other; two agents with disjoint files can run concurrently however related their themes. Sequence only where a real dependency exists — a fold that must land before the file it folds into is deleted, a hoist that must precede the skills it hoists from.

**`check.sh` over the root is yours, not theirs.** `kk-ecosystem` opens and closes with it, but a Phase 3 agent cannot fix what it finds — the files belong to other agents, and with several editing at once each one's in-flight state reads to the others as dangling links. `AGENT-BRIEF.md` → **Scope discipline** tells them to skip both steps, and its return contract routes what they hit. You run the wiring check between phases and at the end.

**You own the handoff ledger**, at `<scratch>/reduce-handoff.md`. Agents return handoffs in their `HANDOFF:` line; you append them, relay each to the owning agent if it is still running, and **drain the ledger in Phase 4** — every entry applied, or recorded with why it was declined. An entry nobody applies is a change the campaign promised and dropped.

**Keep a campaign record beside it** — the plan path, each theme's scope, whether its agent finished, and every return verbatim. This is the most destructive procedure here, and without that record an interrupted campaign leaves a half-cut tree with no way to tell which themes landed.

## 4. Reconcile

The partition that made Phase 3 safe is what leaves this work: no agent could see across its own boundary. These need the whole tree at once.

- **One home, and no home.** Reconcile rules living in several files. Then grep-verify **every `DELETED:` line every agent returned** — open the file each names as still covering the rule and confirm the text is there. Verifying only what the plan rescued misses the case that actually happens: two agents concurrently confirming each other's copy exists and each deleting its own, which no plan ever scheduled. A line whose named home no longer holds it is a rule deleted twice; restore one copy.
- **Stale claims.** A wiring check proves a cited path exists; nothing proves the target still says the thing. Check every citation into a file that was heavily cut.
- **Prefer the mechanism.** Prose a script can assert (ecosystem.md → **Prefer the mechanism**). This is the pass that converts words into enforcement instead of deleting them, and it is worth more than any single cut.
- **Trace the real runs.** Walk each end-to-end path as the agent would, loading files in order, and find where the instruction runs out: a step naming something no file defines, a handoff whose receiving skill no longer expects what the sender sends, a contract with one half deleted. Nothing else in this campaign checks that the system still works as a system.
- **Skill shape**, where skills were cut — invoke `kk-skillcraft`. Run it here and not in Phase 3: what to extract depends on what survived.

## 5. Converge

Fresh agents, no knowledge of what was already cut, over the largest remaining scopes. **Tell them a short return is the correct answer to a converged tree** — an agent measured on findings will manufacture them and undo work that was already argued.

Stop when a round returns little and says so. Two rounds is usually enough; a third produces noise.

## 6. Repair

Cutting damages prose: it stitches sentences together, strands pronouns, and leaves terms used before the line defining them. Spawn `kk-tighten` **last**, pointed at the readability floor rather than at volume, and expect it to *add* words. Where scripts changed, `kk-code-review` then `kk-refactor` — in that order, refactor being the serializer.

**Close the campaign by recording the result** — `kk-foreman`'s `scripts/stats.sh --append "<what ran>"`. A campaign changes the tree more than anything else and is always started directly, so nothing else will write that row.

## Rules

- **The target is an aim, not a quota** (`AGENT-BRIEF.md` → **On the target**) — accept the first return, and never send an agent back for the number alone.
- **Report the honest total.** Scripts grow when prose becomes enforcement; count that as a win and show it separately rather than netting it out.
- Deliberately-argued restorations accumulate across phases. Carry them into every later brief as protected, or the next agent cuts them again with the same reasoning that cut them the first time.
