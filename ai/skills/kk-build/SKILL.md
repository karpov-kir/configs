---
name: kk-build
description: Take a settled requirement to a green tree — place the work, settle what the requirement leaves open, then code, test and gate it. Use for "build this", "implement the ticket". The build loop, not the quality pass that reads the result afterwards (kk-qualify).
argument-hint: "the requirement to build — a ticket, an issue, a file holding it, or the ask itself"
---

You spawn other skills, so you orchestrate under `~/.kk-flavor/standards/skill-protocol.md`. **The loop is `~/.kk-flavor/standards/building.md`** — read it whole; this file owns only what a standard cannot: where the work is placed, what gets settled before it starts, and who sees the result.

**Your caller names two things** — the requirement set, and the homes that receive what this build produces: a decision it settled, a follow-up it opened, a proposal only a human can accept. **With no home named, they go to your caller** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**), never into a file you chose.

## Place the build

**One requirement = one worktree = one branch**, before any of the reading below. Inherit your caller's worktree if it placed you in one; never nest a second. A lone build in an idle repo may skip the worktree.

## Settle what the requirement leaves open

Only where something is genuinely open. A requirement the code already answers needs no round, and a build that opens one anyway spends the human on a question its own repo settled. **A caller that has already settled one says so, and that round does not run** — it asked, and asking again is the same spend.

**Stack choices — explore spawned, decide in this thread.** `~/.claude/skills/kk-build/technical-round.md` is the whole delta for that path.

- **The exploring half spawns**: read-only and bounded, returning what this repo has already chosen — language, runtime, test runner, datastore, deployment shape, CI. **Skip it on a greenfield tree**: an explorer sent to read an empty repo returns nothing, and greenfield is the case this round exists for.
- **The deciding half stays here, and is interactive.** A choice reaches this round only because reversing it is expensive, which is exactly the class `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** puts live to the human. A subagent has no human: it prepares the question and never answers it.

**Module shape — one subagent that explores and settles.** Non-interactive end to end, returning a plan rather than a question. A shape internal to one module, or additive to an existing one, is cheap to reverse and therefore the agent's to settle: it reads the existing boundaries, drafts two or three shapes as `~/.kk-flavor/standards/architecture/core.md` → **Module depth** requires, chooses on depth and on where change concentrates, and returns the choice with what determined it. Keeping the rejected drafts out of this thread is most of what it buys.

- **The carve-out is narrow.** A surface another slice consumes, or one crossing a process or repo boundary — a published package, an HTTP API, a wire payload — fails the cheap-to-reverse test and is not the subagent's to settle. It returns that as a proposal, and it goes to the human on the stack round's route.
- **Which modules should exist at all is a separate question**, and only worth asking where this build creates a new module boundary. Inside an existing one it is already answered.

**Two traps when spawning either.** Give each spawn its own ledger path (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**) — a shared fixed name is the one two concurrent spawns both pick. And build every prompt from `~/.kk-flavor/templates/spawn-prompt.md`.

## Build it

**Run the loop** — `~/.kk-flavor/standards/building.md` → **The loop**.

**Then the conformance gate**, once the loop is green: `~/.claude/skills/kk-conform/SKILL.md`, per `~/.kk-flavor/standards/quality-pipeline.md` → **Conform it before you review it**. Its requirement set is the one your caller named. Run it **inline** — only this thread reaches the human. A requirement it finds undelivered is a red result you fix and re-run; the rest of its return goes to the checkpoint.

## Checkpoint

Present for human judgment, and stop:

- Diff summary — what changed conceptually, never a line dump.
- **Gate results** — absolute; a red gate blocks (fix or escalate).
- **Test results** — the human approves the behaviour.
- **Scope delta** — what the conformance gate returned, plus every deferral, routed to the home your caller named.
- **Open lanes** — what the loop named, for the pass that will run them.
- **Anything only a human can accept** — a constraint no command can check, a proposal the module round returned.

Approve on outcomes → done. Reject with feedback → back to the loop.

## Parallel builds

Several may run at once, isolated by one worktree each. **You are one of them and cannot see the others**, so each rule is one you hold unilaterally:

- **Your interactive moments are not exclusive.** The human may already be answering another build, so ask once and wait; never open a second question while one is open.
- **Integration is serial, against the current target.** If it advanced since this branch's gates ran, re-run them on the new base first.
- **A drive acquires shared runtime, not just data.** Dev-server ports, one browser instance, an install slot are shared singletons. Isolate them per build (unique ports, a separate profile) or serialize the step; with one shared driver, serialize.
