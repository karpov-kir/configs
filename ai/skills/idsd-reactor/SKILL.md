---
name: idsd-reactor
description: "Build several ICE intents at once — one session per unblocked intent, launching each as its dependencies land. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". One intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: every unbuilt intent)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` (→ **Orchestrators — interactive first**). **You write nothing under the intent set** — every intent's work, from its grill to its merge, belongs to the session that owns it and runs `idsd-ship`.

## 1. Resolve the order

Spawn `idsd-audit` over the intent set.

- **A Blocker stops the launch.** Route each through the skill the audit names, then re-run it. A cyclic or dangling `depends-on` schedules a session onto an intent that never unblocks.
- **The launchable set is the audit's first build batch**, narrowed by `<arg>` (a milestone, or named slugs) when one is given. Drop every intent whose `idsd/NNN-<slug>` branch or worktree already exists (`git branch --list 'idsd/*'`, `git worktree list`) — a second chip on one intent puts two sessions on one branch.
- Present the schedule: the launchable set, then what each later intent waits on. Say that this session is the reactor's address and launches the later intents only while it stays open.

**Two answers shape the launch:** the yes to spend a session per intent, and whether each session archives itself once its intent lands. **The human gives the second by name, however the run is authorised** — a licence to act unattended does not supply it. **Unanswered, launch anyway and no session archives itself.**

## 2. Launch — one agent per intent

Spawn one agent per launchable intent, **all in one message**, so they draft in parallel. Each agent runs `kk-handoff` **inline** in its own thread. You spawn the agent, never `kk-handoff` itself — the agent holds the handed-off context whole, which is the condition `~/.claude/skills/kk-handoff/SKILL.md` gives for running it inline.

**Keep each prompt thin.** Its task is one line: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done` — then archive the session, where the human agreed to that. The receiving session reads the ICE, the charter and the constraints itself — a prompt that summarises them drifts from the ICE, and the summary is what gets built.

Hand each agent what no file on disk carries:

- the branch each sibling cuts (`idsd/NNN-<slug>`), for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation (**Rules**);
- this session's `sessionId`, which `get_session "self"` prints — a title is not unique enough to address;
- the three messages to send here: a contract change, the merge-slot question, and `done` once its intent lands. Each goes **before** the session archives itself, since archiving stops it and loses anything unsent;
- the human's two answers, plus any licence you run under, verbatim, for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Licence** — nothing else lets a session commit its own merge.

Done when one chip per launchable intent exists. **With nobody at the keyboard the run ends here** — a chip starts a session only when the human clicks it, so nothing wakes **3**. Name the intents left waiting on a click.

## 3. React

A session's message wakes you, so end each turn rather than wait on one. Close each turn with the live sessions, and what each waiting intent waits on.

- **A contract change** — an API shape, a shared type, a wire protocol. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge slot** — a sibling refused the slot asks whether its holder is still alive. **`report.sh merge-slot` cannot answer that.** Its refusal names the holder's worktree (`~/.claude/skills/idsd-finalize/SKILL.md` → **2. Take the slot**); match that worktree against your live sessions. Gone, and the waiting one may `--force`; otherwise it waits.
- **`done`** — the intent landed and its allocation frees. **Where the sessions archive themselves, confirm the sender left `list_sessions` first**: still listed at your next wake means the archive did not take — name it to the human and hold its allocation until it goes. Then recompute the launchable set and return to **2** for whatever its merge unblocked.

**Launch an intent the moment its last dependency lands** — a batch is the starting schedule, never a barrier.

**Check the live sessions at each wake.** One gone from `list_sessions` that never sent `done` died with its intent unbuilt, and nothing else will tell you. Name it to the human and reclaim its allocation.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run ends when every intent in scope has landed and every session it launched has sent `done`. Name to the human each one that will not: an intent nothing live is building, a session that died.

## Rules

- **Shared runtime is yours to hand out**, since only you know how many builds are live: a port range each, and one session at a time holding the browser or any single-slot install.
