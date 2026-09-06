---
name: idsd-reactor
description: "Build several ICE intents at once — one session per unblocked intent, launching each as its dependencies land. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". One intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: every unbuilt intent)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` (→ **Orchestrators — interactive first**). **You write nothing under the intent set** — every intent's work, from its grill to its merge, belongs to the session that owns it and runs `idsd-ship`.

## 1. Resolve the order

**Ask whether to audit first**, and recommend it. `idsd-audit` catches a cycle or a dangling `depends-on`, and either one otherwise shows up as an intent that silently never launches. Skipping it costs that check alone — `report.sh intent-ready` still refuses each build whose dependency has not shipped.

- **Audited** — the launchable set is the audit's first build batch, and **a Blocker stops the launch**: route each through the skill the audit names, then re-run it.
- **Unaudited** — the launchable set is every unbuilt intent whose `depends-on` targets are all built, read from the intents' own frontmatter.

**`depends-on` decides what launches together; overlapping files do not.** Two intents touching the same code still launch at once — the overlap resolves at their merges, and **3**'s relay keeps them from surprising each other. Hold a pair back only for an overlap big enough that they would be redoing each other's work, and say so — how big that is depends on the two intents, never on a count of shared files.

Narrow that set either way by `<arg>` (a milestone, or named slugs). Drop every intent whose `idsd/NNN-<slug>` branch or worktree already exists (`git branch --list 'idsd/*'`, `git worktree list`) — a second chip on one intent puts two sessions on one branch. Then present the schedule: the launchable set, and what each later intent waits on. Say that this session is the reactor's address and launches the later intents only while it stays open.

**The human alone says whether each session archives itself once its intent lands** — no licence to act unattended supplies that answer, and unanswered no session archives itself.

## 2. Launch — one agent per intent

Spawn one agent per launchable intent, **all in one message**, so they draft in parallel. Each agent then runs `kk-handoff` **inline**. `~/.claude/skills/kk-handoff/SKILL.md` bars spawning that skill, not spawning an agent that runs it over context you handed it — so you spawn the agent, never `kk-handoff`.

**Keep each prompt thin.** Its task is one line: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done` — then archive the session, where the human agreed to that. The receiving session reads the ICE, the charter and the constraints itself. A prompt that summarises them drifts, and the summary is what gets built.

Hand each agent what no file on disk carries:

- the branch each sibling cuts (`idsd/NNN-<slug>`), for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation — a port range, plus the browser and any single-slot install held by one session at a time;
- this session's `sessionId`, which `get_session "self"` prints — a title is not unique enough to address;
- the three messages to send here (**3**), each sent **before** the session archives itself, since archiving stops it and loses anything unsent;
- any licence you run under, verbatim — nothing else lets a session commit its own merge.

Done when one chip per launchable intent exists. **With nobody at the keyboard the run ends here** — a chip starts a session only when the human clicks it, so nothing wakes **3**. Name the intents left waiting on a click.

## 3. React

End each turn rather than wait on a session's message — it wakes you. Close each turn with the live sessions, and what each waiting intent waits on.

- **A contract change** — an API shape, a shared type, a wire protocol. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge slot** — a sibling that hit the slot refusal asks whether the holder is still alive, and you hold the live-session list that answers it (`~/.claude/skills/idsd-finalize/SKILL.md` → **2. Take the slot**). Match the worktree the refusal names against your live sessions: gone, and the waiting sibling may `--force`; otherwise it waits.
- **`done`** — the intent landed and its allocation frees. Recompute the launchable set and return to **2** for whatever its merge unblocked.

**Launch an intent the moment its last dependency lands** — a batch is the starting schedule, never a barrier.

**Check `list_sessions` at each wake** and name each mismatch to the human. One gone that never sent `done` died with its intent unbuilt: reclaim its allocation. Where the sessions archive themselves, one that sent `done` and is still listed did not archive: hold its allocation until it goes.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run ends when every intent in scope has landed and every session it launched has sent `done`. Name to the human each one that will not.
