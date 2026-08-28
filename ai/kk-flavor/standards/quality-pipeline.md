# Quality Pipeline

The stages a quality pass runs over one change set. **Binding on whoever runs a pass, any single stage of one, or one of its lanes on its own** — a lane invoked standalone is still that lane, and the rules here that bind it say so.

You orchestrate under [skill-protocol.md](skill-protocol.md), which is also the stage subagents' contract. **Code-review always runs, and so does refactor over any changed code**; beyond those two, **which stages run is the orchestrator's call**. Each stage below states its own trigger.

**Each scanner lives with the lens it serves**, and you run it from there — each lane names its own. **A script's output is evidence only when the script ran** — an exit you did not look at never reaches a spawn prompt as "returned no hits". **A scanner handed a revision skips untracked files**, so a change set holding new ones is seen whole only by the bare form; a change set already committed is seen only by naming the range. **A spawn prompt describes the tree you read, never the tree you intended.**

## The round

Spawn the round's stages **in one message** so they run concurrently.

**Refactor is the round's serializer** — a fresh spawn after the round, never joining it. The round must have settled *to a decision* first: every blocking finding answered *and applied*, asked of the human as it arrives.

**One subagent per stage.** Every decision, and everything the pass itself writes, stay in this thread. Carry a mid-session deferral into the spawn prompt of every stage that runs afterwards.

**A handoff a stage returns joins this round at the receiving skill's own stage number** ([skill-protocol.md](skill-protocol.md) → **Finish in the lanes your edits opened**).

**Fixes you apply between the round and refactor** get one fresh pass of the code-review lane, scoped to the files they changed, before refactor.

**Reconcile contradictions empirically** — two stages opposing on one location, or a claim against an observation: re-run the check yourself rather than trust either side's word. **A claim you cannot verify has an author**: `git log` the line before you replace it with an inference.

**Settle design once.** Scoping narrows *files*, never which rules a stage applies. A change reworking a **shared or cross-repo primitive** settles its target shape with the human in one pass *before* iterating.

**A stage that hard-fails (red gate, broken build) stops the pipeline.**

**The sequence after the round may run streamed instead** — the stages queue patches as they find them and you apply each on arrival, the tier order deciding conflicts rather than the schedule. [streaming.md](streaming.md) is the whole delta for that path.

## Drive it before you review it

**Use the change the way its user will, wherever it has observable behaviour, and do it before any lens reads it.**

**Spawn the drive lane, handed only the scenarios the change was asked to satisfy and how to run the project — withhold the diff.** You own that it ran.

**This is a gate, not a stage** — a divergence stops the pipeline as a red gate does.

**A caller whose own loop builds the thing to drive runs the gate inside that loop instead** — after its work rather than before any lens, and a divergence there is a red result it fixes and re-runs. It says where it departs. Nothing to drive yet is the only reason that holds.

**A step nobody could drive is an ask, and only after they have been asked may it be dropped** — recording it as something the pass is waiting on is that drop taken without them. The drive lane owns the rest of this, including what a dropped step owes its return.

**A runtime-behaviour claim the round returns comes back here**: drive the scenario it predicts, or let it land labelled an **unverified inference**.

## The stages


**The numbering is not the execution order** — stage 3's comment pass runs after stage 4.

**Both review stages are local**: neither posts to GitHub nor runs `gh`. And **a pre-existing defect outside the change is neither fixed nor blocked on** — a serious one is surfaced once, as a separate non-blocking note for the human to route, never folded into the change's findings and never dropped silently. One the change makes reachable or worse is in scope.

1. **Code-review** — the code-review lane on the change set. Ask live for blocking findings; record the others.
2. **Security-review** — *only if* the change touches a security surface (input handling, filesystem/network/exec, auth or session, secrets, deserialization, or an invariant the project's own standards mark security-critical).
3. **Tighten & comment pass** — *only if* the change added or changed standalone prose, or touched any comment.
   - **Standalone prose** joins the round via the **prose lane**, scoped to the change set's prose. Prose that reaches no diff-scoped stage — what this pass itself wrote outside the repo, an open PR's body — is **named explicitly in the spawn prompt's scope slot**. Its handoff goes to the **outward-text lane** over the files it names.
   - **A comment finding splits by placement and content**: a true comment on the wrong construct is the refactor lane's, a false claim about the code is the code-review lane's. Each lane states only its own side.
   - **Comment blocks** wait for refactor, then go to the outward-text lane directly, never the prose lane first. Run the outward-text lane's scanner **at pass start, not here**; its outliers ride the spawn prompt's tool-output slot.
4. **Refactor** — a loop to compliance, iterating where the pass has the budget for it and once where it does not. Each iteration spawns a **fresh** subagent (never a resume) to run the refactor lane, which reports whether the change is compliant; blocked→resume still holds *within* an iteration. Stop the moment one reports compliant; a cap reached without compliance is residue for the human with what's open, and duplication deferred under the extract threshold goes to whatever record the pass appends its settled decisions to. Run the refactor lane's scanner before the first iteration and **again after the last**; second-run hits are yours to resolve or record, not a reason for another iteration.
5. **Retro** — last when it runs; how it runs is the orchestrator's.

**A change to the agents' own instructions is not one of these** — a skill, standard, prompt, template or `CLAUDE.md` goes to the **instruction lane** directly, and a pass that finds one **names it in its return rather than running it**. That lane owns shape and prose itself, so running it from inside a pass queues both a second time.

## Gates

**A full pass verifies the repo's baseline gates are real.** A command that *can't run*, that *runs but can't fail*, or that runs and can fail but *never reads the changed code*, is a **stale gate** rather than verification — never an assumed green. Read what CI actually invokes, never its stage names. What a stale one then becomes is the orchestrator's.
