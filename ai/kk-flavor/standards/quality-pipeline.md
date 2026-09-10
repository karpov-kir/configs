# Quality Pipeline

The stages a quality pass runs over one change set. **Binding on whoever runs a pass, any single stage of one, or one of its lanes on its own** — the rules here that bind a standalone lane say so.

You orchestrate under [skill-protocol.md](skill-protocol.md), which is also the stage subagents' contract. **Code-review always runs, and so does refactor over changed code.** Evaluate the other triggers against the actual change set and record why each absent lane is not applicable. A standalone pass may trim only on an explicit caller request for turnaround; a pass a merge waits on runs every triggered lane. Unknown scope or uncertain applicability means inspect further or run the lane, never assume a skip.

**Each scanner lives with the lens it serves**, and you run it from there. **A script's output is evidence only when the script ran** — an exit you did not look at never reaches a spawn prompt as "returned no hits". **A scanner handed a revision skips untracked files**, so a change set holding new ones is seen whole only by the bare form; a change set already committed is seen only by naming the range. **A spawn prompt describes the tree you read, never the tree you intended.** **Reuse a scanner result only while its inputs and tool are unchanged.** Rerun after changes to those inputs; a hit still standing is resolved or returned as residue. Retain full failure logs outside the context window.

## The round

**One caller owns the round and dispatches leaf workers directly.** Reading a build or qualify entry does not spawn another coordinator. Resolve each leaf's model through [model-policy.md](model-policy.md) by its task name; a stage whose green a human trusts stays on its assigned tier however cheap the dispatch is.

**Settle scope and write ownership before dispatch.** Run independent code and security reviews in separate contexts against the same candidate. Parallelize other applicable leaves where inputs and writes do not conflict, within the actual worker limit. At capacity, wait or reuse a completed worker for a valid continuation; do not retry failed spawns through proxy agents.

Where streaming pays, [streaming.md](streaming.md) supplies patch ordering. Otherwise reviews return proposals for one caller to apply, and refactor follows the settled round. Fixes made after a review invalidate that review over the changed files and affected dependencies; dispatch that scoped review before accepting the candidate.

**Refactor needs a fresh independent compliance check after repairs settle**, including streamed repairs. Reuse unaffected evidence, but recheck any refactor obligation opened by later edits. A structural or behavioral repair also takes independent correctness review and, if it touches a security surface, security review.

If independent review cannot run, report the missing independence. Standalone passes may return that limitation; a merge-gating pass waits for independent workers. No substitute review by the author is silently accepted.
**A handoff a stage returns joins this round at the receiving skill's own stage number** ([skill-protocol.md](skill-protocol.md) → **Finish in the lanes your edits opened**).

**Reconcile contradictions empirically** — two stages opposing on one location, or a claim against an observation: re-run the check yourself rather than trust either side's word. **A claim you cannot verify has an author**: `git log` the line before you replace it with an inference.

**Settle design once.** Scoping narrows *files*, never which rules a stage applies. A change reworking a **shared or cross-repo primitive** settles any genuinely open, expensive-to-reverse choice with the human before iterating. Existing authorization and settled requirements remain binding.

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
3. **Edit** — only if the change adds or changes prose or comments. Scope it to those artifacts, including prose outside the repository that the pass itself produces. The audience determines the wording guidance in this one lane; do not send the same prose through separate concision and humanization passes. False comments remain correctness findings, and misplaced comments remain refactor findings. Apply comment edits after changes to the code they describe settle.
4. **Refactor** — required over changed code. Run the scanner, apply actionable repairs, then obtain the final compliance check. A changed candidate reopens affected files and dependencies, not the whole passed queue. Stop when compliant; an unresolved warning or safety cap is reported rather than stamped clean. A scanner hit by itself does not justify repeatedly spawning a fresh full-scope agent.

**Agent instructions go to the instruction lane**, which owns rule semantics, shape and wording in that order. The shared caller dispatches one scoped instruction worker, or records an already active owner. That worker applies the ordered checks to the target it holds; it does not spawn another orchestration stack or hand the same files to edit again. A broader ecosystem audit requires its own scope.

**A retrospective is not one of these either** — no pass runs, offers or schedules one, and the human starts it.

## Gates

**Verify the repo's baseline gates are real.** A command that *can't run*, that *runs but can't fail*, or that runs and can fail but *never reads the changed code*, is a **stale gate** rather than verification — never an assumed green. Read what CI actually invokes, never its stage names. What a stale one then becomes is the orchestrator's.

Freeze the final candidate before expensive gates. A required repair reopens only evidence whose inputs it changes, with whole-tree gates retaining their own freshness rules. Optional wording work does not keep a settled candidate moving.
