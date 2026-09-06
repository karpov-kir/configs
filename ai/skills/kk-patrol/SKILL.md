---
name: kk-patrol
description: Run refinement as a standing loop rather than a pass — each round spawns a scout that finds one defect and dies, then a fixer that lands it on main and dies. Use for "keep refining", "patrol the tree", "run until there is nothing left to fix". Built to run cheaply for a long time, not quickly. The loop above kk-foreman, which dispatches one piece of work; a bounded campaign that shrinks a tree of instructions is a different shape again.
argument-hint: "the tree to patrol, and any angle to start from (default: this repo, every angle)"
disable-model-invocation: true
---

**You orchestrate and you never read the work.** Your context grows by a line a round; each round's two agents are thrown away with everything they read, and that is what buys the hours — so the moment you read a diff to check it, the loop's budget is gone. **Verify from the ref, never from the report.**

**A round spawns fresh agents and never resumes an earlier round's**, departing from `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**. A scout carrying last round's conclusions reads its own notes rather than the tree, and finds the shape it found before; a fresh context is what keeps the loop converging on the tree being right rather than on one agent's taste.

## A round

1. **Pick the angle** — the ledger's least-recently-swept row (**The ledger**, below).
2. **Spawn the scout**, handing it `~/.claude/skills/kk-patrol/SCOUT.md` verbatim, the angle, and the ledger path. It returns the path of one written finding and the path of the file that finding is about, or nothing. The second path is what keeps two patrols off one file (**Rules**, below).
3. **Empty?** Stamp the angle swept and go to 1 — in a healthy tree, the common case.
4. **Spawn the fixer**, handing it `~/.claude/skills/kk-patrol/FIXER.md` verbatim and that path. **Never the scout's prose** — prose relayed through you is prose you now hold, and an interrupted round leaves a file recoverable where it leaves a message gone. **Never a second finding**: a round carrying two cannot be stopped or reverted in the middle, and a loop with no safe stopping point is one the human cannot afford to start.
5. **Confirm it landed where it says.** `git fetch` and check the SHA it named is on `origin/main`. A working-tree edit, a local commit and a pushed one read identically in a summary, and the difference is the whole of whether the round happened.
6. **Stamp the ledger** — the sweep, plus the path of any finding the fixer wrote beside its own — and report the round in one line: angle, finding, SHA or `held`, cost.

**Start from a committed tree.** Git is this loop's only undo, and on a dirty tree the human's own work reverts with a round's.

## Landing

Your licence is `~/.claude/skills/kk-foreman/SKILL.md` → **The argument may hand you the run**, plus the merge and push granted here. **Nothing else.**

**The fixer merges to `main` and pushes** — the human grants that for this loop, so a round that cannot be stopped halfway also cannot strand work on a branch nobody merges. `FIXER.md` holds the conditions, and they are not yours to relax.

**A fixer that reports a push you cannot find on `origin/main` ends the loop** rather than the round — one agent misreporting a landing is a bug in the round, and a loop that keeps running on unverified landings publishes them.

## The ledger

`${XDG_STATE_HOME:-~/.local/state}/kk-flavor/patrol/<repo>.md` — the loop's whole memory, since no agent's survives it. One row per sweep: the angle, the fingerprint and `main` SHA it was read at, the outcome, and the commit where a fix landed. Seed it from `SCOUT.md`'s angles on the first run.

**Machine-local, never inside the tree under patrol** — a ledger there is a dirty working tree that travels to everyone on the next commit. It holds only what is in flight: sweeps, findings a fixer wrote and nobody took, and findings held for the human. Anything durable was written into the file it binds.

**Rewritten, not appended** (`~/.kk-flavor/standards/records.md`) — which is why it carries no cap. The angle table holds one row per angle in `SCOUT.md` and is overwritten in place; a held finding is deleted the round it lands or the human answers it. **That deletion is the pruning point, and it is here.** Nothing in the file grows without a bound already set somewhere else.

Take the fingerprint with `~/.kk-flavor/scripts/tree-fingerprint.sh`; where it refuses, **stamp the failure rather than a guess**, or a stale row reads as a valid resume point.

## Stopping

Two ways, and no third:

- **The human says so**, at any point. Report rounds run, what landed, what is held and why.
- **Every angle stamped empty at the current fingerprint.** Say so and stop; do not lower the bar to keep the loop alive. Reaching the end is the outcome, not the failure.

**The tree moving expires every stamp**: a rule added tomorrow makes yesterday's exhaustion untrue, and the loop restarts with that rule as its first angle.

A red gate stops the round, not the loop (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**). Revert the round or send a fresh fixer, then carry on.

## What it has done

Asked what the patrol has changed over some period, **answer and run no rounds**.

**What landed comes from git**, which is durable, shared, and holds rounds this machine never ran:

```bash
git log --since='<period>' --grep='^Patrol-angle:' --format='%h %s%n  %(trailers:key=Patrol-angle,valueonly)'
```

`FIXER.md` puts that trailer on every round's commit. It is a trailer and not a subject prefix because the type — `fix:`, `docs:`, `refactor:` — says what the change *is*, and a round that fixes a bug is still a `fix:`; provenance is a second fact about the same commit.

**What was held, and what swept empty, comes from the ledger** — neither leaves a commit. **Report both**: from git alone this reads as a list of wins with the holds invisible, which flatters the loop exactly where it did least. Say so plainly when the ledger is missing, rather than offering the git half as the whole.

**A reverted round is reported as reverted, never counted.** And **rounds are activity, not outcome** — for whether the tree actually improved, point at `~/.claude/skills/kk-reduce/stats.md`, where a tree growing steadily under a long patrol shows the loop failing whatever its rounds claimed.

## Rules

- **Resume, do not restart.** The first act of a new session is to read the ledger; a sweep already stamped **empty** at the current fingerprint is not repeated. **A held angle is swept again** — `SCOUT.md` bars raising a finding the ledger already holds, so the next sweep finds something else or comes back empty. Skipping it instead deadlocks the loop: the angle can never reach `empty`, and **Stopping** never fires.
- **Say what a round cost.** The human starts this against spare budget and can only spend it deliberately if the burn is visible.
- Peers may be patrolling the same tree. Coordinate as `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** requires, and never spawn a fixer onto a file another session is holding.
