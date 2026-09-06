---
name: idsd-reactor
description: "Build several ICE intents at once — take the build order from an audit, launch one session per unblocked intent, then wake as each lands and launch what it unblocked. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". Several intents built across sessions; one intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: every unbuilt intent)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` (→ **Orchestrators — interactive first**). **You launch sessions, relay between them, and react as each lands.** Each intent's grill, worktree, code, gates and merge belong to the session that owns it, through `idsd-ship`.

## 1. Resolve the order

Spawn `idsd-audit` over the intent set — it reads the whole set and writes only a report, so it needs no human.

- **A Blocker stops the launch.** Route each through the skill the audit names, then re-run it. A cyclic or dangling `depends-on` schedules a session onto an intent that never unblocks.
- **The launchable set is every unbuilt intent whose `depends-on` targets are all built** — the audit's first build batch. Narrow it by `<arg>` (a milestone, or named slugs) when one is given.
- Present the schedule: the launchable set, then what each later intent waits on. Say that this session is the reactor's address, and that it launches the later intents only while it stays open.

**Two answers before you launch:** the yes to spend a session per intent, and whether each session archives itself once its intent lands. Self-archiving is an act the human has to have agreed to by name.

Done when the launchable set is named, the schedule is presented, and both answers are in hand.

## 2. Launch — one agent per intent

Spawn one agent per launchable intent, **all in one message** so they draft concurrently, each from `~/.kk-flavor/templates/spawn-prompt.md`. An agent runs `kk-handoff` inline over its one intent and creates that intent's chip. **That keeps `kk-handoff` inline, as `~/.claude/skills/kk-handoff/SKILL.md` requires** — in the drafting agent's thread, over context this step hands it whole. The agent is what makes the chips appear at once rather than one after another.

**Keep the handoff prompt thin.** Its task is one line: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done`, then archive the session. The receiving session reads the ICE, the charter and the constraints itself — a prompt that summarises them drifts from the ICE, and the summary is what gets built.

Fill the template's `Change scope` slot with what no file on disk carries:

- the repository path and the base commit SHA;
- the slugs launching beside it and the branch each cuts (`idsd/NNN-<slug>`), for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation (**Rules**);
- this session's title, as the address to message, and the three messages to send it: a contract change, the merge-gate question, and `done` **in the turn before it archives itself** — archiving stops the session, so anything it has not sent by then is lost.

The human's two answers go in the template's emphasis slot instead, verbatim, for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Licence**.

Done when one chip per launchable intent exists, each titled with its slug.

## 3. React

**Each message from a session arrives in this thread and wakes you — end each turn rather than poll a session for a state its own message carries.** Close each turn by saying which sessions are live and which intents wait on which.

- **A contract change** — an API shape, a shared type, a wire protocol. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge gate** — a session at `idsd-ship done` asks whether the target moved. Answer from what landed since its base, so it re-runs its gates against the real one. **One session merges at a time**: say to wait while another holds the slot, and hand it on when that one's `done` arrives — each build holds serial integration blind, and you are the only party that sees both.
- **`done`** — the session is about to archive. **Confirm it left `list_sessions` before you treat its slot as free**: a session that is pinned, on screen, or still mid-turn refuses to archive and keeps its worktree and its ports. Still listed after its turn ends means the archive did not take — name it to the human and hold its allocation. Gone, and you reclaim the allocation, recompute the launchable set, and return to **2** for whatever its merge unblocked.

**Launch an intent the moment its last dependency lands** — a batch is the starting schedule, never a barrier.

**Check the live sessions at each wake.** One gone from `list_sessions` that never sent `done` died with its intent unbuilt, and nothing else will tell you. Name it to the human and reclaim its allocation.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run is done when every intent in scope is archived — or named to the human as one no live session is building — and every session it launched has left `list_sessions`.

## Rules

- **Shared runtime is yours to hand out**, since only you know how many builds are live: a port range each, and one session at a time holding the browser or any single-slot install. `~/.claude/skills/idsd-build/SKILL.md` → **Parallel execution** is what each session applies to the allocation it gets.
- **You write nothing under the intent set** — every intent, report and record belongs to the session building it.
