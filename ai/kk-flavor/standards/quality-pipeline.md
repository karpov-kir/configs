# Quality Pipeline

The stages a quality pass runs over one change set. **Binding on whoever runs a pass or any single stage of one.**

You orchestrate under [skill-protocol.md](skill-protocol.md), which is also the stage subagents' contract. **Code-review always runs, and so does refactor over any changed code**; beyond those two, **which stages run — and whether retro runs at all — is the orchestrator's call**. Each stage below states its own trigger.

**Each scanner lives with the lens it serves**, and you run it from there: `comment-density.sh` under `kk-humanize`, `dup-literals.sh` under `kk-refactor`. **A script's output is evidence only when the script ran** — an exit you did not look at never reaches a spawn prompt as "returned no hits".

## The round

Spawn the round's stages **in one message** so they run concurrently.

**Refactor is the round's serializer** — a fresh spawn after the round, never joining it. The round must have settled *to a decision* first: every blocking finding answered *and applied*, asked of the human as it arrives.

**One subagent per stage.** Every decision, and everything the pass itself writes, stay in this thread. Carry a mid-session deferral into the spawn prompt of every stage that runs afterwards.

**A handoff a stage returns joins this round at the receiving skill's own stage number** ([skill-protocol.md](skill-protocol.md) → **Finish in the lanes your edits opened**).

**Fixes you apply between the round and refactor** get one fresh `kk-code-review` scoped to the files they changed, before refactor.

**Reconcile contradictions empirically** — two stages opposing on one location, or a claim against an observation: re-run the check yourself rather than trust either side's word.

**Settle design once.** Scoping narrows *files*, never which rules a stage applies. A change reworking a **shared or cross-repo primitive** settles its target shape with the human in one pass *before* iterating.

**A stage that hard-fails (red gate, broken build) stops the pipeline.**

## Drive it before you review it

**Use the change the way its user will, wherever it has observable behaviour, and do it before any lens reads it.**

**Spawn `kk-drive`, handed only the scenarios the change was asked to satisfy and how to run the project — withhold the diff.** You own that it ran.

**This is a gate, not a stage** — a divergence stops the pipeline as a red gate does. **A dropped step is carried, with its reason, into the pass's own output.**

**A runtime-behaviour claim the round returns comes back here**: drive the scenario it predicts, or let it land labelled an **unverified inference**.

## The stages

**Decide** and the decision log named below are `idsd-qualify`'s homes for a stage's residue; an orchestrator without them maps each onto its own equivalent.

1. **Code-review** — `kk-code-review` on the change set. Ask live for blocking findings; record the others.
2. **Security-review** — *only if* the change touches a security surface (input handling, filesystem/network/exec, auth or session, secrets, deserialization, or a constitution security invariant).
3. **Tighten & comment pass** — *only if* the change added or changed standalone prose, or touched any comment.
   - **Where the change touched the agents' own instructions** — a skill, standard, prompt, template or `CLAUDE.md` — this stage is `kk-ecosystem` over those files instead. It owns that whole lane and spawns `kk-skillcraft` and `kk-tighten` itself, so queue neither.
   - **Standalone prose** joins the round via `kk-tighten`, scoped to the change set's prose *plus the prose this pass itself wrote*. Prose that reaches no diff-scoped stage — an open PR's body, a working file the orchestrator keeps outside the repo — is **named explicitly in the spawn prompt's scope slot**. `kk-tighten`'s handoff is queued here too, not deferred: spawn `kk-humanize` over the outward-text files it names. Any file it hands off for its **comments** waits for refactor, per the next bullet.
   - **Comment blocks** wait for refactor, then go to `kk-humanize` directly, never `kk-tighten` first. Run `comment-density.sh` **at pass start, not here**; its outliers ride the spawn prompt's tool-output slot.
4. **Refactor** — a loop to compliance in full mode (max 3 iterations), one iteration in fast. Each iteration spawns a **fresh** subagent (never a resume) to run `kk-refactor`, which reports whether the change is compliant; blocked→resume still holds *within* an iteration. Stop the moment one reports compliant; a cap reached without compliance is a **Decide** item with what's open, and duplication deferred under the extract threshold goes to the decision log. Run `dup-literals.sh` before the first iteration and **again after the last**; second-run hits are yours to resolve or record, not a reason for another iteration.
5. **Retro** — always last; how it runs is the orchestrator's.

## Gates

**A full pass verifies the repo's baseline gates are real.** A command that *can't run*, that *runs but can't fail*, or that runs and can fail but *never reads the changed code*, is a **stale gate** rather than verification — never an assumed green. Read what CI actually invokes, never its stage names. What a stale one then becomes is the orchestrator's.
