---
name: kk-patrol
description: Run refinement as a standing loop rather than a pass — each round spawns a scout that finds one defect and dies, then a fixer that commits it to the patrol's branch and dies. Use for "keep refining", "patrol the tree", "run until there is nothing left to fix". Built to run cheaply for a long time, not quickly. The loop above kk-foreman, which dispatches one piece of work; a bounded campaign that shrinks a tree of instructions is a different shape again.
argument-hint: "the tree to patrol, and any angle to start from (default: this repo, every angle)"
disable-model-invocation: true
---

**You orchestrate and you never read the work.** Your context grows by a line a round: each round's two agents are thrown away with everything they read, and that is what buys the hours. Read a diff to check it and the loop's budget is gone. **Verify from the ref, never from the report.**

**A round spawns fresh agents and never resumes an earlier round's**, departing from `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**. A scout carrying last round's conclusions reads its own notes rather than the tree, and finds the shape it found before; a fresh context is what keeps the loop converging on the tree being right rather than on one agent's taste.

## A round

1. **Fetch, then merge `origin/main` into the branch.** A branch that drifts gates against a tree nobody has; merging a stale `origin/main` leaves it drifting. **A conflict stops the round and asks the human**: resolving another session's work is not the loop's call.
2. **Pick the angle** — the ledger's least-recently-swept row (**The ledger**, below).
3. **Spawn the scout**, handing it `~/.claude/skills/kk-patrol/SCOUT.md` verbatim, the angle, and the ledger path. It returns the path of one written finding and the path of the file that finding is about, or nothing. The second path is what keeps two patrols off one file (**Rules**, below).
4. **Empty?** Stamp the angle swept and go to 1 — in a healthy tree, the common case.
5. **Spawn the fixer**, handing it `~/.claude/skills/kk-patrol/FIXER.md` verbatim and that path. **Never the scout's prose** — prose relayed through you is prose you now hold, and an interrupted round leaves a file recoverable where it leaves a message gone. **Never a second finding**: a round carrying two cannot be stopped or reverted in the middle, and a loop with no safe stopping point is one the human cannot afford to start.
6. **Confirm it landed where it says.** Check the SHA it named is on the patrol's branch. A working-tree edit and a commit read identically in a summary, and the difference is the whole of whether the round happened.
7. **Stamp the ledger** — the sweep, plus the path of any finding the fixer wrote beside its own — and report the round in one line: angle, finding, SHA or `held`, cost.

**Start from a committed tree, on the branch the ledger names.** Git is this loop's only undo, and on a dirty tree the human's own work reverts with a round's. Where the ledger names no branch, create `patrol/<slug>` and record it there. The loop commits without asking, so a session that starts on `main` lands every round straight onto it, taking the merge out of the human's hands.

## Landing

Your licence is `~/.claude/skills/kk-foreman/SKILL.md` → **The argument may hand you the run**, **minus the push that licence lifts**. **Nothing else.**

**A round ends at a commit on the patrol's own branch**; the merge is the human's. `FIXER.md` holds the conditions, and they are not yours to relax.

**An unmerged fix to an instruction is not in effect** (`~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins**) — a round that fixes `SCOUT.md`, `FIXER.md` or a standard does not change what the next round runs. The ledger's commit column is what stops a later round re-finding it.

**A fixer that reports a commit you cannot find on the branch ends the loop** rather than the round — one agent misreporting a landing is a bug in the round, and a loop that keeps running on unverified landings builds on them.

## The ledger

`${XDG_STATE_HOME:-~/.local/state}/kk-flavor/patrol/<repo>.md` — the loop's whole memory, since no agent's survives it. **It names the patrol's branch**, which is how a later session resumes onto it rather than opening a second one and orphaning every round on the first. One row per sweep: the angle, the fingerprint and branch SHA it was read at, the outcome, and the commit where a fix landed. Seed it from `SCOUT.md`'s angles on the first run.

**Machine-local, never inside the tree under patrol** — a ledger there is a dirty working tree that travels to everyone on the next commit. It holds only what is in flight: sweeps, findings a fixer wrote and nobody took, and findings held for the human. Anything durable was written into the file it binds.

**Rewritten, not appended** (`~/.kk-flavor/standards/records.md`) — which is why it carries no cap. The angle table holds one row per angle in `SCOUT.md` and is overwritten in place; a held finding is deleted the round it lands or the human answers it. **That deletion is the pruning point, and it is here.** Nothing in the file grows without a bound already set somewhere else.

Take the fingerprint with `~/.kk-flavor/scripts/tree-fingerprint.sh`; where it refuses, **stamp the failure rather than a guess**, or a stale row reads as a valid resume point.

## Stopping

Two ways, and no third:

- **The human says so**, at any point. Report rounds run, what landed, what is held and why.
- **Every angle stamped empty at the current fingerprint.** Say so and stop; do not lower the bar to keep the loop alive. Reaching the end is the outcome, not the failure.

**The tree moving expires every stamp**: a rule added tomorrow makes yesterday's exhaustion untrue, and the loop restarts with that rule as its first angle.

**Either stop names the branch and the commits on it waiting to be merged.** A round delivers nothing past the branch, so a report of rounds run that never says where the work sits strands it.

A red gate stops the round, not the loop (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**). Revert the round or send a fresh fixer, then carry on.

## What it has done

Asked what the patrol has changed over some period, **answer and run no rounds**.

**What landed comes from git**, read on the patrol's branch — durable, and carrying every round anyone ran on it:

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
