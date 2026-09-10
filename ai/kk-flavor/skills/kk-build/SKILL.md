---
name: kk-build
description: Take a settled requirement to a green tree — place the work, plan the stack and the change, then code, test and gate it. Use for "build this", "implement the ticket". The build loop, not the quality pass that reads the result afterwards (kk-qualify).
argument-hint: "the requirement to build — a ticket, an issue, a file holding it, or the ask itself"
---

**Runs:** inline — human

Build in the current coordinator under `~/.kk-flavor/standards/skill-protocol.md`; do not spawn a build wrapper merely to invoke this entry. Apply `~/.kk-flavor/standards/building.md` when Phase 4 starts. Read the selected phase's additional references only when needed. Phase 3's exploration worker dispatches as `kk-build/explore`; every other phase answers `kk-build` (`~/.kk-flavor/standards/model-policy.md`).

**Your caller names two things** — the requirement set, and the homes that receive what this build produces: a decision it settled, a follow-up it opened, a proposal only a human can accept. **With no home named, they go to your caller** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**), never into a file you chose.

**Dispatches:** `explore`

**Two traps for every subagent below.** Give each spawn its own ledger path (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**) — a shared fixed name is the one two concurrent spawns both pick. And build every prompt from `~/.kk-flavor/templates/spawn-prompt.md`.

## Phase 1 — Place the build

**One requirement = one worktree = one branch**, before any reading. Inherit your caller's worktree if it placed you in one; never nest a second. A lone build in an idle repo may skip the worktree.

## Phase 2 — Plan the stack (interactive)

**Only where a choice is genuinely open**, and skipped whole where none is. A requirement the repo already answers needs no round, and opening one anyway spends the human on a question their own code settled. **A caller that has already run this says so, and the phase does not run.**

**Explore only the unanswered choice.** Use the context already held or a bounded read-only worker when discovery is substantial and independent. Return the existing stack facts needed for that choice; do not inventory a known or empty repository.

**Decide, here, with the human.** A choice reaches this phase only because reversing it is expensive, which is exactly the class `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** puts live to them. **A subagent has no human**: it prepares the question and never answers it. `~/.kk-flavor/skills/kk-build/technical-round.md` is the whole delta — **what earns a question at all**, what one carries, and what closes it. Everything under that bar you settle yourself; a choice the human could reasonably expect you to make is one that spends them for nothing.

## Phase 3 — Plan the change (non-interactive)

Plan the change in the implementation context. Reuse an established shape; a local edit does not require another planning agent. Where a new boundary needs substantial exploration, the coordinator may dispatch one bounded worker and carry its decision forward. Read `~/.kk-flavor/standards/architecture/core.md` only for architectural choices; compare alternatives where its module-boundary rules apply.

**An exploration worker returns a shape, never a procedure** — the boundaries, what each one publishes, and what decided between them. Not the files to add, not the order to write them, not the body of anything. A plan detailed enough to follow line by line has spent the loop's judgement before the loop ran, and Phase 4 is where the body is decided against code this phase never read.

- **Which modules should exist at all** is a further question, and only worth asking where this build creates a new module boundary. Inside an existing one it is already answered.
- **The carve-out is narrow.** A surface another slice consumes, or one crossing a process or repo boundary — a published package, an HTTP API, a wire payload — fails the cheap-to-reverse test and is not the subagent's to settle. It returns that as a proposal, and it goes to the human on Phase 2's route.

## Phase 4 — Build

**Start with `~/.kk-flavor/standards/building.md` → **Before the loop**** — the reading and the gate resolution. Phases 2 and 3 read this repo for what it chose and how it is shaped; this reads the code the requirement itself touches, and turns every gate it carries into a command. Reuse facts already verified against unchanged code; inspect the touched implementation and resolve any missing gate commands.

**Then `~/.kk-flavor/standards/building.md` → **The loop****. Carry Phase 3's settled choice into the loop; reopen it only when implementation evidence contradicts it.

**Then the conformance gate**, once the loop is green: `~/.kk-flavor/skills/kk-conform/SKILL.md`, per `~/.kk-flavor/standards/quality-pipeline.md` → **Conform it before you review it**. Its requirement set is the one your caller named. Run it **inline** — only this thread reaches the human. A requirement it finds undelivered is a red result you fix and re-run; the rest of its return goes to the checkpoint.

## Phase 5 — Checkpoint

Present for human judgment, and stop:

- Diff summary — what changed conceptually, never a line dump.
- **Gate results** — absolute; a red gate blocks (fix or escalate).
- **Test results** — the human approves the behaviour.
- **Scope delta** — what the conformance gate returned, plus every deferral, routed to the home your caller named.
- **Open lanes** — what the loop named, for the pass that will run them.
- **Anything only a human can accept** — a constraint no command can check, a proposal Phase 3 returned.

Approve on outcomes → done. Reject with feedback → back to Phase 4.

## Parallel builds

Several may run at once, isolated by one worktree each. **You are one of them and cannot see the others**, so each rule is one you hold unilaterally:

- **Your interactive moments are not exclusive.** The human may already be answering another build, so ask once and wait; never open a second question while one is open.
- **Integration is serial, against the current target.** If it advanced since this branch's gates ran, re-run them on the new base first.
- **A drive acquires shared runtime, not just data.** Dev-server ports, one browser instance, an install slot are shared singletons. Isolate them per build (unique ports, a separate profile) or serialize the step; with one shared driver, serialize.
