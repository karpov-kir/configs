---
name: idsd-reactor
description: "Build several ICE intents at once — one session per unblocked intent, launching each as its dependencies land. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". One intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: ask which milestone)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**. **You write nothing under the intent set** — every intent's work, grill to merge, belongs to the session that owns it and runs `idsd-ship`.

## 1. Resolve the order

**Ask whether to audit first**, and recommend it. Without it, a cycle or a dangling `depends-on` surfaces only as an intent that silently never launches: `report.sh intent-ready` refuses a build whose dependency is unbuilt, and catches no other Blocker.

**One milestone at a time, and any milestone can be the one.** `<arg>` names it, or names slugs outright; with neither, list the milestones still holding unbuilt intents and ask which — recommend the one whose unbuilt intents depend on nothing outside it. **Nothing outside the chosen one launches, however unblocked it is** — that is the parked work. `milestone: none` is unplanned rather than a milestone, so it is never the chosen one; naming slugs is the only route to one of those.

The launchable set:

- **Audited** — that milestone's share of the audit's first build batch.
- **Unaudited** — every unbuilt intent in that milestone whose `depends-on` targets are all built, read from the intents' own frontmatter.

**`depends-on` decides what launches together; overlapping files do not.** The overlap resolves at their merges, and **3**'s relay keeps them from surprising each other. Hold a pair back only where the overlap is big enough that they would redo each other's work, and say so — that size depends on the two intents, never on a count of shared files.

Drop from that set every intent whose `idsd/NNN-<slug>` branch or worktree already exists (`git branch --list 'idsd/*'`, `git worktree list`) — a second chip on one intent puts two sessions on one branch.

**Present the schedule and launch only what the human confirms.** Read the launchable set back by name, say what each later intent waits on and what you are leaving parked, and count the `draft` intents in the set — each grills the human in its own thread at `idsd-build`'s gap rounds. Nothing on disk marks an intent as parked, since `status: draft` fits a fresh intent and a shelved one alike, so the ask is the only place that knowledge enters. Say that this session is the reactor's address: it launches the later intents only while it stays open. **After an audit, a Blocker touching what they confirmed stops the launch**: route each through the skill the audit names, then re-run the audit.

**The human alone says whether each session archives itself once its intent lands** — no licence to act unattended supplies that answer, and until they do, none archives. **They are answering about the sessions you launch, never about you.** Archive yourself and every later `done` lands nowhere: nothing schedules what the last merge unblocked, and the sessions still working are reporting to a session that is gone. You run until the human stops you, and stopping you is theirs.

**One intent set has one reactor.** A second recomputes the same graph and cannot see the branch the first is about to cut, so the branch check above does not reach it — that check is one reactor's view of what exists, never of what another is deciding. The sessions fare worse than the branches: each was given one address for `done`, the slot question and contract changes, so half the run reports to a reactor that does not know what the other half was told. **Succession is stop then start** — the human archives the incumbent, then starts the successor — so **never chip your own replacement while you are live**, however certain you are that you are going.

## 2. Launch — one agent per intent

Spawn one agent per launchable intent, **every spawn in a single message**, so the drafting — the slow part — happens at once. Each agent then runs `kk-handoff` **inline**. That skill bars spawning *it*, not spawning an agent that runs it over context you handed it — so you spawn the agent, never `kk-handoff`.

**Keep the chip prompt thin** — the one the agent drafts. It states one task: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done` — then archive the session, where the human agreed to that. The receiving session reads the ICE, the charter and the constraints itself. A prompt that summarises them drifts, and the summary is what gets built.

Hand each agent what no file on disk carries, for that prompt:

- the branch each sibling cuts (`idsd/NNN-<slug>`), for `~/.claude/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation — a port range, plus the browser and any single-slot install held by one session at a time;
- this session's `sessionId`, which `get_session "self"` prints — a title is not unique enough to address;
- the three messages to send here (**3**), each sent **before** the session archives itself, since archiving stops it and loses anything unsent.

**An agent whose session has no chip mechanism returns a draft path instead.** Create those chips yourself once the agents are back, again in one message.

Done when one chip per launchable intent exists, off that single round of spawns. **With nobody at the keyboard the run ends here** — a chip starts a session only when the human clicks it, so nothing wakes **3**. Name the intents left waiting on a click.

## 3. React

End each turn rather than wait on a session's message — it wakes you. Close each with the live sessions and what each waiting intent waits on.

**Every message is a report about a state that has moved since.** A session writes, then keeps working, and you read minutes later — so check the repo before acting on what one told you.

- **A contract change** — an API shape, a shared type, a wire protocol. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge slot** — a sibling that hit the slot refusal asks whether the holder is still alive, and only your live-session list answers (`~/.claude/skills/idsd-finalize/SKILL.md` → **2. Take the slot**). Match the worktree the refusal names against it: gone, and the sibling may `--force`; otherwise it waits.
- **`done`** — the intent landed and its allocation frees. Recompute the launchable set and return to **2** at once for whatever this merge unblocked. **A batch is the starting schedule, never a barrier.**
- **An ask with no intent behind it** — the human wants work the set does not carry, or a ship surfaced something nobody wrote down. **You are the address for that ask, and you do not answer it yourself**: chip a session scoped to `idsd-intent` and nothing else — author the ICE, stop, build none of it — and hand it its number. It goes the way a ship does, in the same order: **its ICE on the target branch, then its `done` to you, then it archives itself** where the human agreed to that. Until it lands, an ICE on a branch nothing merged is not merely unlanded — it is lost with the session that wrote it. Authoring is where a brief gets corrected — that session reads the base and finds what your account of it had wrong — so an ICE written from your summary ships your errors into the set as requirements.
- **A new intent** — a session split its own, or the human authored one mid-run. Add it and recompute: it launches once its `depends-on` targets are built, and those may still be in flight, so work authored now is scheduled behind the wave rather than held back until the wave ends. **You hand out its `NNN`** — `idsd-intent` bumps off a number already written, which does not separate two sessions writing at the same moment. **A number you promised is not a number taken**: an edge naming it, written before that session wrote anything, gates on nothing if the session landed on another — and gates on unrelated work as soon as something else takes the one you named. Read the edge back against what the authoring session actually wrote. A session that split still owes one `done` per piece it became; take the new slugs when it tells you.

**Check `list_sessions` at each wake** and name each mismatch to the human. One gone that never sent `done` proves nothing — a late message and one that never comes look alike — so **read the repo before calling it dead**: its intent in `.idsd/archive/` with its merge on the target means it landed, and you treat that as its `done`. Neither, and it died with its intent unbuilt: reclaim its allocation. **`done` frees the allocation, not the departure.** A session that sent `done` and landed its work has finished; where the human asked for self-archiving, one still listed simply did not archive — a row for them to clear, named once and not waited on. Holding its allocation until it goes stalls the run behind an archive that may never take: the call reports success from inside the turn that issued it, and only a later `list_sessions` says whether it took.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run ends when every intent in scope, additions included, has landed and every session it launched has sent `done`. Name each one that will not.
