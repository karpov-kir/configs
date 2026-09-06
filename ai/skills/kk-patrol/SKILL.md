---
name: kk-patrol
description: Run refinement as a standing loop rather than a pass — a scout finds one thing wrong and dies, a fixer lands it on main and dies, round after round, until the human stops it or every angle comes back empty against a tree that has not moved. Use for "keep refining", "patrol the tree", "run until there is nothing left to fix". Built to run cheaply for a long time, not quickly. The loop above kk-foreman, which dispatches one piece of work; kk-reduce is a bounded campaign with a single lens.
argument-hint: "the tree to patrol, and any angle to start from (default: this repo, every angle)"
disable-model-invocation: true
---

One thing found, one thing landed, again — for as long as the human leaves it running.

**You orchestrate and you never read the work.** Two spawned agents do each round and neither survives it. What lets this run for hours is that *your* context grows by a line a round while theirs is thrown away — so the moment you start reading a diff to check it, the loop's budget is gone. **Verify from the ref, never from the report.**

## A round

1. **Pick the angle** — the ledger's least-recently-swept row (**The ledger**, below).
2. **Spawn the scout**, handing it `~/.claude/skills/kk-patrol/SCOUT.md` verbatim, the angle, and the ledger path. It returns the path of one written finding, or nothing, and stops.
3. **Empty?** Stamp the angle swept and go to 1. In a healthy tree this is the common case, and it costs one cheap agent.
4. **Spawn the fixer**, handing it `~/.claude/skills/kk-patrol/FIXER.md` verbatim and that path. Never the scout's prose, and never a second finding.
5. **Confirm it landed where it says.** `git fetch` and check the SHA it named is on `origin/main`. A report is not evidence: a working-tree edit, a local commit and a pushed one read identically in a summary, and the difference is the whole of whether the round happened.
6. **Stamp the ledger** and report the round in one line: angle, finding, SHA or `held`, cost.

**One finding a round.** A round carrying two cannot be stopped in the middle of, and a loop with no safe stopping point is one the human cannot afford to start.

**Start from a committed tree.** Git is this loop's only undo, and on a dirty tree the human's own work reverts with a round's.

## Why they are thrown away

A scout carrying last round's conclusions is not reading the tree, it is reading its own notes — and it finds the shape it found before. **A fresh context each round is what keeps the loop converging on the tree being right rather than on one agent's taste.**

Their separation is load-bearing too. The scout is never handed the brief that licenses editing, so it cannot slide from noticing into fixing and report the fix as the finding. The fixer is handed one finding and not the angle, so it cannot go looking.

**What crosses between them is a file, never a message through you.** The scout writes the finding; the fixer gets its path. Prose relayed through your context is prose you now hold — and an interrupted round leaves a file recoverable where it leaves a message gone.

## It improves anything, including itself

An unbounded loop that only fixes instances grinds the same ground forever. **A finding an instrument could have caught is not finished when the instance is fixed — the class goes into the instrument** (`~/.kk-flavor/standards/ecosystem.md` → **Prefer the mechanism**). Later rounds get it free, and that is the difference between a loop that compounds and one that re-reads the tree until the budget is gone.

**This skill is in the tree it patrols, so it is a target like any other file** — a vague angle, a bar that lets noise through, a guard that fires on the wrong thing. `SCOUT.md` carries **the patrol itself** as an angle, and the ledger is its evidence: after a few dozen rounds, which angles never find anything, which findings get held, which rounds get reverted. That is data about the loop nothing else has, and it is the honest input for sharpening it.

**One asymmetry, and the fixer holds it.** Adding to this skill — a new angle, a sharper bar, a guard the loop lacked — is an ordinary round. **Loosening what constrains the loop is not**: a rule cut, a stopping condition widened, a landing condition relaxed. The loop cannot check its own supervision, because the thing checking is what changed. Those go to the human, named, and the loop carries on with the rest.

A round that edits this skill says so in the ledger, and moves the fingerprint — which expires every stamp, so the loop re-sweeps under its own new rules rather than trusting sweeps taken under the old ones.

## What the licence still does not hand you

`~/.claude/skills/kk-foreman/SKILL.md` → **The argument may hand you the run** is what unattended lifts, plus the merge and push this skill's own **Landing** grants. Nothing else. **Hold it, name it, keep going** — a blocker is a line in the report, never the reason the loop stopped:

- A cap, a threshold or a bound nobody has measured. Inventing one teaches every later round to trim until it clears.
- **Keeping something the standards say to replace**, which `~/.kk-flavor/standards/core-principles.md` → **2. Simplicity first** makes the human's call and not yours.
- Anything else whose undo has to be arranged first (`~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act**).

## Landing

**The fixer merges to `main` and pushes** — the human grants that here, for this loop, so that a round which cannot be stopped halfway also cannot leave work stranded on a branch nobody merges. `FIXER.md` holds the conditions, and they are not yours to relax.

**A round that changes what agents read is integrated, not just applied** — `kk-foreman`'s route sends it to `kk-ecosystem`, and the fixer lands only what comes back from there. Every finding argues for text, each one defensible alone; **a loop that adds a rule a round and never asks whether it earns its place is a bloat engine with good intentions**, and the tree it patrols ends up needing `kk-reduce` to undo the patrol. That skill's `stats.md` is where the answer shows: a tree growing steadily under a long patrol is the loop failing, whatever the rounds claimed.

**A push cannot be taken back** (`~/.kk-flavor/standards/git.md`), so the undo is arranged instead of skipped: every round is one small commit, fast-forward only, gated green immediately before, and revertible on its own. **A fixer that reports a push you cannot find on `origin/main` ends the loop** rather than the round — one agent misreporting a landing is a bug in the round, and a loop that keeps running on unverified landings publishes them.

## Oscillation is the failure only an unbounded loop has

Round N deletes what round N+8 restores, with the reasoning that wrote it the first time. Both rounds were right on what they could see, and neither can see the other — that is what throwing the context away costs, and this is what pays it.

**A decision worth keeping goes into the file it binds, never into the ledger** — one line saying why the obvious edit is wrong. `~/.kk-flavor/standards/ecosystem.md` → **No evidence in a rule file** lifts its ban for exactly this: a rule a later agent would otherwise override. Both briefs make a finding that contradicts such a line a hold, never a re-decision.

## The ledger

`${XDG_STATE_HOME:-~/.local/state}/kk-flavor/patrol/<repo>.md` — the loop's whole memory, since no agent's survives it. One row per sweep: angle, fingerprint, outcome, and the commit where there is one. Seed it from `SCOUT.md`'s angles on the first run.

**Machine-local, never inside the tree under patrol** — a ledger there is a dirty working tree that travels to everyone on the next commit, the reason `~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins** puts machine-local values outside it. It holds only what is in flight: sweeps, and findings held for the human. Anything durable was written into the file it binds.

Take the fingerprint with `~/.kk-flavor/scripts/tree-fingerprint.sh`. **A fingerprint that could not be taken is not a fingerprint** — stamp the failure rather than a guess, or a stale row reads as a valid resume point.

## Stopping

Two ways, and no third:

- **The human says so**, at any point. Report rounds run, what landed, what is held and why.
- **Every angle stamped empty at the current fingerprint.** Say so and stop; do not lower the bar to keep the loop alive. Reaching the end is the outcome, not the failure.

**The tree moving expires every stamp**, which is the whole mechanism: a rule added tomorrow makes yesterday's exhaustion untrue, and the loop restarts with that rule as its first angle.

A red gate stops the round, not the loop (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**). Revert the round or send a fresh fixer, then carry on.

## Rules

- **Resume, do not restart.** The first act of a new session is to read the ledger; a sweep already stamped at the current fingerprint is not repeated.
- **Say what a round cost.** The human starts this against spare budget and can only spend it deliberately if the burn is visible.
- Peers may be patrolling the same tree. Coordinate as `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first** requires, and never spawn a fixer onto a file another session is holding.
