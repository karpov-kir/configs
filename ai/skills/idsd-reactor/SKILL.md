---
name: idsd-reactor
description: "Build several ICE intents at once — one session per unblocked intent, launching each as its dependencies land. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". One intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: ask which milestone)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` (→ **Orchestrators — interactive first**). **You write nothing under the intent set** — every intent's work, from its grill to its merge, belongs to the session that owns it and runs `idsd-ship`.

## 1. Resolve the order

**Ask whether to audit first**, and recommend it. `idsd-audit` catches a cycle or a dangling `depends-on`, and either one otherwise shows up as an intent that silently never launches. Skipping it costs that check alone — `report.sh intent-ready` still refuses each build whose dependency has not shipped.

**One milestone at a time, and any milestone can be the one** — `vnext` after `mvp` lands is the next wave. `<arg>` names it, or names slugs outright; with neither, list the milestones still holding unbuilt intents and ask which — recommend the one whose unbuilt intents depend on nothing outside it. **Nothing outside the chosen one launches, however unblocked it is** — that is the parked work. `milestone: none` is unplanned rather than a milestone, so it is never the chosen one; naming slugs is the only route to one of those.

- **Audited** — the launchable set is that milestone's share of the audit's first build batch, and **a Blocker touching that set stops the launch**: route each through the skill the audit names, then re-run the audit. Parked drafts carry Blockers as a matter of course, and none of those reaches this launch.
- **Unaudited** — the launchable set is every unbuilt intent in that milestone whose `depends-on` targets are all built, read from the intents' own frontmatter.

**`depends-on` decides what launches together; overlapping files do not.** The overlap resolves at their merges, and **3**'s relay keeps them from surprising each other. Hold a pair back only for an overlap big enough that they would be redoing each other's work, and say so — how big that is depends on the two intents, never on a count of shared files.

Drop from that set every intent whose `idsd/NNN-<slug>` branch or worktree already exists (`git branch --list 'idsd/*'`, `git worktree list`) — a second chip on one intent puts two sessions on one branch. Then present the schedule: the launchable set, what each later intent waits on, and what you are leaving parked. **Count the `draft` intents in that set and say so.** Each opens its own grill thread once its session reaches `idsd-build`'s gap rounds, so a wave of drafts grills the human from several threads at once. Say that this session is the reactor's address and launches the later intents only while it stays open.

**Read the launchable set back by name and launch what the human confirms.** Nothing in the repo marks an intent as parked — `status: draft` fits a fresh intent and a shelved one alike, and one milestone holds both — so the ask is the only place that knowledge enters.

**The human alone says whether each session archives itself once its intent lands** — no licence to act unattended supplies that answer, and unanswered no session archives itself.

## 2. Launch — one agent per intent

Spawn one agent per launchable intent, **every spawn in a single message**, so they draft at once. Each agent then runs `kk-handoff` **inline**. `~/.claude/skills/kk-handoff/SKILL.md` bars spawning that skill, not spawning an agent that runs it over context you handed it — so you spawn the agent, never `kk-handoff`.

**Keep each prompt thin.** Its task is one line: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done` — then archive the session, where the human agreed to that. The receiving session reads the ICE, the charter and the constraints itself. A prompt that summarises them drifts, and the summary is what gets built.

Hand each agent what no file on disk carries:

- the branch each sibling cuts (`idsd/NNN-<slug>`), for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation — a port range, plus the browser and any single-slot install held by one session at a time;
- this session's `sessionId`, which `get_session "self"` prints — a title is not unique enough to address;
- the three messages to send here (**3**), each sent **before** the session archives itself, since archiving stops it and loses anything unsent;
- any licence you run under, verbatim — nothing else lets a session commit its own merge.

**An agent whose session has no chip mechanism returns a draft path instead** (`~/.claude/skills/kk-handoff/SKILL.md` → **3. Hand it over**). Create those chips yourself once the agents are back, again in one message — drafting is the slow part, so the run stays parallel either way.

Done when one chip per launchable intent exists, off one round of spawns rather than a sequence of them. **With nobody at the keyboard the run ends here** — a chip starts a session only when the human clicks it, so nothing wakes **3**. Name the intents left waiting on a click.

## 3. React

End each turn rather than wait on a session's message — it wakes you. Close each turn with the live sessions, and what each waiting intent waits on.

- **A contract change** — an API shape, a shared type, a wire protocol. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge slot** — a sibling that hit the slot refusal asks whether the holder is still alive, and you hold the live-session list that answers it (`~/.claude/skills/idsd-finalize/SKILL.md` → **2. Take the slot**). Match the worktree the refusal names against your live sessions: gone, and the waiting sibling may `--force`; otherwise it waits.
- **`done`** — the intent landed and its allocation frees. Recompute the launchable set and return to **2** at once for whatever this merge unblocked. **A batch is the starting schedule, never a barrier.**

**Check `list_sessions` at each wake** and name each mismatch to the human. One gone that never sent `done` died with its intent unbuilt: reclaim its allocation. Where the sessions archive themselves, one that sent `done` and is still listed did not archive: hold its allocation until it goes.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run ends when every intent in scope has landed and every session it launched has sent `done`. Name to the human each one that will not.
