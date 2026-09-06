# Quality Pipeline

The stages a quality pass runs over one change set. **Binding on whoever runs a pass, any single stage of one, or one of its lanes on its own** — the rules here that bind a standalone lane say so.

You orchestrate under [skill-protocol.md](skill-protocol.md), which is also the stage subagents' contract. **Code-review always runs, and so does refactor over any changed code**; beyond those two, **which stages run is the orchestrator's call**. Each stage below states its own trigger. **A pass a merge waits on** has no such latitude: every stage its trigger fires runs.

**Each scanner lives with the lens it serves**, and you run it from there. **A script's output is evidence only when the script ran** — an exit you did not look at never reaches a spawn prompt as "returned no hits". **A scanner handed a revision skips untracked files**, so a change set holding new ones is seen whole only by the bare form; a change set already committed is seen only by naming the range. **A spawn prompt describes the tree you read, never the tree you intended.** **Every scanner runs again after the lane it serves returns** — a hit still standing is resolved, or goes to the human as residue.

## The round

**Settle the path before you spawn**, then spawn the round's stages **in one message** so they run concurrently. Where the path pays, the pass streams: **the comment pass and refactor go out in that message with the reviews**, none waiting on another's return. [streaming.md](streaming.md) → **A quality pass's tiers** is the whole delta for the path.

**Batched, refactor is a fresh spawn after the round**, which must have settled *to a decision* first: every blocking finding answered *and applied*, asked of the human as it arrives. Fixes you applied in that interval get one fresh pass of the code-review lane, scoped to the files they changed, before refactor.

**One subagent per stage.** Every decision, and everything the pass itself writes, stay in this thread. Carry a mid-session deferral into the spawn prompt of every stage that runs afterwards.

**Before accepting that it cannot spawn, a session tests the bar** ([skill-protocol.md](skill-protocol.md) → **Caller**) — the bar an agent would rather not test is the one this rule exists to catch. Where the session still cannot spawn, the stages run inline and the closing status names which of these happened: the author reviewed their own work, each lens read the diff with the build's reasoning already in the window, no file got a fresh read. Every lens still runs and every control is still proved. **A pass a merge waits on runs only from a session that can spawn.**

**A handoff a stage returns joins this round at the receiving skill's own stage number** ([skill-protocol.md](skill-protocol.md) → **Finish in the lanes your edits opened**).

**Reconcile contradictions empirically** — two stages opposing on one location, or a claim against an observation: re-run the check yourself rather than trust either side's word. **A claim you cannot verify has an author**: `git log` the line before you replace it with an inference.

**Settle design once.** Scoping narrows *files*, never which rules a stage applies. A change reworking a **shared or cross-repo primitive** settles its target shape with the human in one pass *before* iterating.

**A stage that hard-fails (red gate, broken build) stops the pipeline.**

**A symptom whose cause nobody has reproduced goes to the diagnosis lane, never to a review stage.** A divergence the drive gate returns, a red the build leaves, an intermittent failure — a lens reading the diff will produce a theory, and a theory is what the pipeline then treats as a finding.

## Conform it before you review it

**Hold the change set against the ask it was given, before any lens reads it** — every requirement delivered, nothing delivered beyond them, and no contradiction inside the change. This is the **conformance lane**, and it is **a gate, not a stage**: a stage is handed a scope, never the ask, so unasked-for work reads to every lens as correct code and passes.

**Whoever holds the ask runs it.** A caller above the pass runs it inside its own loop; **where the pass is itself the top, it runs the gate before the round**. **A pass that reaches the stages with no conformance gate behind it says so in its closing status** — a green pass otherwise reads as covering scope.

**Its findings split by who can resolve them.** A requirement not delivered is a red result the caller fixes and re-runs. **Delivery beyond the ask, and every contradiction, go to the human** — deleting unasked-for work is a decision, not a fix ([skill-protocol.md](skill-protocol.md) → **Orchestrators — interactive first**). Say plainly when the gate found neither.

**A change set stating no ask and linking none cannot be checked against one** — say so and ask, before the pass is spent.

## Drive it before you review it

**Use the change the way its user will, wherever it has observable behaviour, and do it before any lens reads it.**

**Spawn the drive lane, handed only the scenarios the change was asked to satisfy and how to run the project — withhold the diff.** You own that it ran.

**This is a gate, not a stage** — a divergence stops the pipeline as a red gate does.

**A step nobody could drive is an ask, and only after they have been asked may it be dropped** — recording it as something the pass is waiting on is that drop taken without them. The drive lane owns the rest of this, including what a dropped step owes its return.

**A runtime-behaviour claim the round returns comes back here**: drive the scenario it predicts, or let it land labelled an **unverified inference**.

## The stages

**The numbering is not the execution order** — **The round** above sets it.

**Both review stages are local**: neither posts to GitHub nor runs `gh`. And **a pre-existing defect outside the change is neither fixed nor blocked on** — a serious one is surfaced once, as a separate non-blocking note for the human to route, never folded into the change's findings and never dropped silently. One the change makes reachable or worse is in scope.

1. **Code-review** — the code-review lane on the change set. Ask live for blocking findings; record the others.
2. **Security-review** — *only if* the change touches a security surface (input handling, filesystem/network/exec, auth or session, secrets, deserialization, or an invariant the project's own standards mark security-critical).
3. **Prose & comment pass** — *only if* the change added or changed standalone prose, or touched any comment.
   - **Standalone prose** joins the round via the **prose lane**, scoped to the change set's prose. Prose that reaches no diff-scoped stage — what this pass itself wrote outside the repo, an open PR's body — is **named explicitly in the spawn prompt's scope slot**. Its handoff goes to the **outward-text lane** over the files it names.
   - **A comment finding splits by placement and content**: a true comment on the wrong construct is the refactor lane's, a false claim about the code is the code-review lane's. Each lane states only its own side.
   - **Comment blocks** go to the outward-text lane directly, never the prose lane first, and their fixes land after refactor's.
4. **Refactor** — batched, a loop to compliance; a pass trimming for turnaround runs one iteration instead. Each iteration spawns a **fresh** subagent (never a resume) to run the refactor lane; blocked→resume still holds *within* an iteration. Stop the moment one reports compliant; a cap reached without compliance is residue for the human with what's open, and duplication deferred under the extract threshold goes to whatever record the pass appends its settled decisions to. Run the refactor lane's scanner before the first iteration; its second run is never a reason for another iteration.

**A change to the agents' own instructions is not one of these** — a skill, standard, prompt, template or `CLAUDE.md` goes to the **instruction lane** directly, and a pass that finds one **names it in its return rather than running it**. That lane owns shape and prose itself, so running it from inside a pass queues both a second time.

**A retrospective is not one of these either** — no pass runs, offers or schedules one, and the human starts it.

## Gates

**Verify the repo's baseline gates are real.** A command that *can't run*, that *runs but can't fail*, or that runs and can fail but *never reads the changed code*, is a **stale gate** rather than verification — never an assumed green. Read what CI actually invokes, never its stage names. What a stale one then becomes is the orchestrator's.
