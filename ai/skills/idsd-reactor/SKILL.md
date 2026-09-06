---
name: idsd-reactor
description: "Build several ICE intents at once — one session per unblocked intent, launching each as its dependencies land. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". Several intents built across sessions; one intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: every unbuilt intent)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` (→ **Orchestrators — interactive first**). **You write nothing under the intent set**: each intent's grill, worktree, code, gates, records and merge belong to the session that owns it, through `idsd-ship`.

## 1. Resolve the order

Spawn `idsd-audit` over the intent set.

- **A Blocker stops the launch.** Route each through the skill the audit names, then re-run it. A cyclic or dangling `depends-on` schedules a session onto an intent that never unblocks.
- **The launchable set is the audit's first build batch**, narrowed by `<arg>` (a milestone, or named slugs) when one is given. Drop every intent whose `idsd/NNN-<slug>` branch or worktree already exists (`git branch --list 'idsd/*'`, `git worktree list`) — a second chip on one intent puts two sessions on one branch.
- Present the schedule: the launchable set, then what each later intent waits on. Say that this session is the reactor's address and launches the later intents only while it stays open.

**Two answers before you launch:** the yes to spend a session per intent, and whether each session archives itself once its intent lands. **The human gives the second by name, however the run is authorised** — a licence to act unattended does not supply it, and **2** says what archiving costs.

## 2. Launch — one agent per intent

Spawn one agent per launchable intent, **all in one message**, so the chips appear together rather than one draft after another. Each agent runs `kk-handoff` **inline** in its own thread, over the context this step hands it whole — which is the case `~/.claude/skills/kk-handoff/SKILL.md` keeps inline, so the agent goes around it rather than in place of it.

**Keep each prompt thin.** Its task is one line: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done` — then archive the session, where the human agreed to that. The receiving session reads the ICE, the charter and the constraints itself — a prompt that summarises them drifts from the ICE, and the summary is what gets built.

Hand each agent what no file on disk carries:

- the repository path, the base commit SHA, and the branch each sibling cuts (`idsd/NNN-<slug>`), for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation (**Rules**);
- this session's `sessionId` — `get_session "self"` prints it, and titles are not unique enough to address — plus the three messages to send here: a contract change, the merge-gate question, and `done` once its intent lands — sent **before** it archives itself, since archiving stops the session and loses anything unsent;
- the human's two answers, verbatim, as the draft's **Licence**.

Done when one chip per launchable intent exists. **With nobody at the keyboard the run ends here**, since a chip starts a session only when the human clicks it and **3** never wakes — name the intents left waiting on a click.

## 3. React

A session's message wakes you, so end each turn rather than wait on one. Close each turn by saying which sessions are live and which intents wait on which.

- **A contract change** — as `~/.claude/skills/idsd-build/SKILL.md` → **Phase 3 — Build** defines one. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge gate** — `idsd-finalize` holds the merge slot itself and refuses a second with exit 4, naming the holder's intent and worktree. **Your part is the one call it cannot make: whether that holder is still alive.** Match the worktree against your live sessions; where its session is gone, tell the waiting one to `--force`, and otherwise to wait. A session that dies between `report.sh merge-slot take` and `finalize` leaves a slot held that nothing else will free.
- **`done`** — the intent landed and its allocation frees. **Where the sessions archive themselves, confirm it left `list_sessions` first**: still listed after its turn ends means the archive did not take — name it to the human and hold its allocation until it goes. Then recompute the launchable set and return to **2** for whatever its merge unblocked.

**Launch an intent the moment its last dependency lands** — a batch is the starting schedule, never a barrier.

**Check the live sessions at each wake.** One gone from `list_sessions` that never sent `done` died with its intent unbuilt, and nothing else will tell you. Name it to the human and reclaim its allocation.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run is done when every intent in scope is archived — or named to the human as one no live session is building — and every session it launched has sent `done`, or been named as one that never will.

## Rules

- **Shared runtime is yours to hand out**, since only you know how many builds are live: a port range each, and one session at a time holding the browser or any single-slot install. `~/.claude/skills/kk-build/SKILL.md` → **Parallel builds** is what each session applies to the allocation it gets.
