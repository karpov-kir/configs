---
name: kk-build
description: Take a settled requirement to a green tree — place the work, plan the stack and the change, then code, test and gate it. Use for "build this", "implement the ticket". The build loop, not the quality pass that reads the result afterwards (kk-qualify).
argument-hint: "the requirement to build — a ticket, an issue, a file holding it, or the ask itself"
---

You spawn other skills, so you orchestrate under `~/.kk-flavor/standards/skill-protocol.md`. **Phase 4 is `~/.kk-flavor/standards/building.md`** — read it whole before you start, since the phases around it exist to feed it. This file owns only what that standard cannot: where the work is placed, what gets planned before it, and who sees the result.

**Your caller names two things** — the requirement set, and the homes that receive what this build produces: a decision it settled, a follow-up it opened, a proposal only a human can accept. **With no home named, they go to your caller** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**), never into a file you chose.

**Two traps for every subagent below.** Give each spawn its own ledger path (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**) — a shared fixed name is the one two concurrent spawns both pick. And build every prompt from `~/.kk-flavor/templates/spawn-prompt.md`.

## Phase 1 — Place the build

**One requirement = one worktree = one branch**, before any reading. Inherit your caller's worktree if it placed you in one; never nest a second. A lone build in an idle repo may skip the worktree.

## Phase 2 — Plan the stack (interactive)

**Only where a choice is genuinely open**, and skipped whole where none is. A requirement the repo already answers needs no round, and opening one anyway spends the human on a question their own code settled. **A caller that has already run this says so, and the phase does not run.**

**Explore, spawned.** Read-only and bounded, returning what this repo has already chosen — language, runtime, test runner, datastore, deployment shape, CI. **Skip it on a greenfield tree**: an explorer sent to read an empty repo returns nothing, and greenfield is the case this phase exists for.

**Decide, here, with the human.** A choice reaches this phase only because reversing it is expensive, which is exactly the class `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** puts live to them. **A subagent has no human**: it prepares the question and never answers it. `~/.claude/skills/kk-build/technical-round.md` is the whole delta — **what earns a question at all**, what one carries, and what closes it. Everything under that bar you settle yourself; a choice the human could reasonably expect you to make is one that spends them for nothing.

## Phase 3 — Plan the change (non-interactive)

**One subagent explores and settles**, returning a plan rather than a question. A shape internal to one module, or additive to an existing one, is cheap to reverse and therefore the agent's to settle — the same rule that made Phase 2 interactive makes this one not.

It reads the existing boundaries, drafts two or three shapes as `~/.kk-flavor/standards/architecture/core.md` → **Module depth** requires, chooses on depth and on where change concentrates, and returns the choice with what determined it. Keeping the rejected drafts out of this thread is most of what it buys.

**It returns a shape, never a procedure** — the boundaries, what each one publishes, and what decided between them. Not the files to add, not the order to write them, not the body of anything. A plan detailed enough to follow line by line has spent the loop's judgement before the loop ran, and Phase 4 is where the body is decided against code this phase never read.

- **Which modules should exist at all** is a further question, and only worth asking where this build creates a new module boundary. Inside an existing one it is already answered.
- **The carve-out is narrow.** A surface another slice consumes, or one crossing a process or repo boundary — a published package, an HTTP API, a wire payload — fails the cheap-to-reverse test and is not the subagent's to settle. It returns that as a proposal, and it goes to the human on Phase 2's route.

## Phase 4 — Build

**Start with `~/.kk-flavor/standards/building.md` → **Before the loop**** — the reading and the gate resolution. Phases 2 and 3 read this repo for what it chose and how it is shaped; this reads the code the requirement itself touches, and turns every gate it carries into a command. Neither earlier read replaces it.

**Then `~/.kk-flavor/standards/building.md` → **The loop****. Its first step is already answered: Phase 3 chose the surface, and the step is where that choice gets written rather than made again.

**Then the conformance gate**, once the loop is green: `~/.claude/skills/kk-conform/SKILL.md`, per `~/.kk-flavor/standards/quality-pipeline.md` → **Conform it before you review it**. Its requirement set is the one your caller named. Run it **inline** — only this thread reaches the human. A requirement it finds undelivered is a red result you fix and re-run; the rest of its return goes to the checkpoint.

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
