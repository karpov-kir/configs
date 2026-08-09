# Quality Pipeline

The stages a quality pass runs over one change set, and the discipline that keeps them from colliding. Read by the two orchestrators that run them — `idsd-qualify` over a working tree, `kk-pr-review` over a GitHub PR; each adds its own deltas and owns where a stage's outcome lands.

You orchestrate under [skill-protocol.md](skill-protocol.md), which is also the stage subagents' contract. Code-review always runs; beyond it, **which stages run — and whether retro runs at all — is the orchestrator's call**.

**Each scanner lives with the lens it serves**, and both orchestrators run it from there: `comment-density.sh` under `kk-humanize`, `dup-literals.sh` under `kk-refactor`.

## The round

**The parallel round.** Spawn its stages **in one message** so they run concurrently.

**The round's own stages move the tree under each other.** `kk-code-review` applies safe fixes; `kk-security-review` reads. Run side by side over one change set, the second is reviewing a tree the first is editing — so **a stage re-reads any queued file that changed since it read it, and re-verifies every finding standing in that file, before returning.** Say in the return that the tree moved and against which state the verdicts hold. This is not yours to drop from a spawn prompt: you cannot see which file a stage was mid-way through.

**Refactor is the round's serializer** — a fresh spawn after the round, over the now-changed tree, never joining it. The round must have settled *to a decision* first: every blocking finding answered *and applied*, asked of the human as it arrives.

**Spawn the skill, not your own review.** Each stage gets its own subagent; every decision, and everything the pass itself writes, stay in this thread, which alone has the human. Build each prompt from `~/.kk-flavor/templates/spawn-prompt.md`, filling only its slots. Carry a mid-session deferral into the spawn prompt of every stage that runs afterwards.

**A script's output is evidence only when the script ran.** An exit you did not look at must never reach a spawn prompt as "returned no hits".

**A handoff a stage returns joins this round at the receiving skill's own stage number** ([skill-protocol.md](skill-protocol.md) → **Finish in the lanes your edits opened**) — a code-review handoff out of stage 3 runs before refactor, not after the pass.

**Fixes you apply between the round and refactor are unreviewed code.** Spawn one fresh `kk-code-review` scoped to the files those fixes changed, before refactor.

**Reconcile contradictions empirically** — two stages opposing on one location, or a claim against an observation: re-run the check yourself rather than trust either side's word.

**Settle design once.** Scope each subagent to the changed surface; scoping narrows *files*, never which rules a stage applies. A change reworking a **shared or cross-repo primitive** settles its target shape with the human in one pass *before* iterating.

**A stage that hard-fails (red gate, broken build) stops the pipeline.**

## The stages

**Decide** and the decision log named below are `idsd-qualify`'s homes for a stage's residue; an orchestrator without them maps each onto its own equivalent.

1. **Code-review** — `kk-code-review` on the change set. Ask live for blocking findings; record the others.
2. **Security-review** — *only if* the change touches a security surface (input handling, filesystem/network/exec, auth or session, secrets, deserialization, or a constitution security invariant — a trigger summary; the skill owns the coverage set).
3. **Tighten & comment pass** — *only if* the change added or changed standalone prose or non-trivial comment blocks.
   - **Where the change touched the agents' own instructions** — a skill, standard, prompt, template or `CLAUDE.md` — this stage is `kk-ecosystem` over those files instead. It owns that whole lane and spawns `kk-skillcraft` and `kk-tighten` itself, so queue neither. Nothing else in the pipeline asks whether a rule earns its place, so a run that grows the ecosystem otherwise merges unexamined.
   - **Standalone prose** joins the round via `kk-tighten`, scoped to the change set's prose *plus the prose this pass itself wrote*. Some of that is git-invisible and so reaches no diff-scoped stage — an open PR's body, or a working file the orchestrator keeps outside the repo: **name those files explicitly in the spawn prompt's scope slot**, since no other stage owns them. `kk-tighten`'s outward-text handoff (`human-writing.md`'s set) is queued here too, not deferred: spawn `kk-humanize` over the files it names.
   - **Comment blocks** wait for refactor — it renames the identifiers they name — then go to `kk-humanize` directly, never `kk-tighten` first. Run `comment-density.sh` **at pass start, not here**; its outliers ride the spawn prompt's tool-output slot.
4. **Refactor** — a loop to compliance in full mode (max 3 iterations), one iteration in fast. Each iteration spawns a **fresh** subagent (never a resume) to run `kk-refactor`, which returns gated proposals and reports whether the change is compliant; blocked→resume still holds *within* an iteration. Stop the moment one reports compliant; a cap reached without compliance is a **Decide** item with what's open, and duplication deferred under the extract threshold goes to the decision log. Run `dup-literals.sh` before the first iteration and **again after the last** — refactor rewrites call sites, so it authors duplication a pre-stage scan cannot see; second-run hits are yours to resolve or record, not a reason for another iteration.
5. **Retro** — always last; how it runs is the orchestrator's.

## Gates

**A full pass verifies the gates are real** — the repo's baseline gates, against the stale-gate test in `~/.claude/skills/idsd-build/SKILL.md` → **Phase 2**. A stale one is never an assumed green; what it then becomes is the orchestrator's.
