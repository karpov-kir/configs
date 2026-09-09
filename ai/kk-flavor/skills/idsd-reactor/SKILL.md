---
name: idsd-reactor
description: "Build several ICE intents at once — one session per unblocked intent, launching each as its dependencies land. Use for \"build the mvp\", \"ship these intents in parallel\", \"start the next wave\". One intent end-to-end is idsd-ship's, and the order or the consistency report on its own is idsd-audit's."
argument-hint: "milestone or intent slugs to build (default: ask which milestone)"
---

You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**. **You write nothing under the intent set** — every intent's work, grill to merge, belongs to the session that owns it and runs `idsd-ship`.

## Client mechanics

Require the target client explicitly before launching and carry it into every handoff.

- **Codex desktop:** the reactor prepares and launches checked prompts through `kk-handoff` inline. Track returned task IDs. Use `wait_threads` for completion, `send_message_to_thread` for authorized sibling reports, and `list_threads` for inventory. Use this task's ID from runtime context as the return address; never guess it from a title. Tasks start immediately. Ending a turn does not schedule a future check.
- **Claude Code:** use the available chip mechanism, `get_session "self"` for the return address, and `list_sessions` for inventory. Chips wait for a click; incoming session messages wake the reactor.

Without the selected client's task tools, return checked drafts instead of claiming a launch.

## 1. Resolve the order

**Settle the landing process at startup, before launching work.** Ask whether this run should merge branches directly or open PRs and wait for them to merge. Reuse an explicit choice already given for this run. Confirm the target branch and the authorized commit, push and landing actions with the schedule; distinguish waiting for a PR merge from permission to perform it. Until answered, prepare the schedule but launch nothing. Carry the user's instruction verbatim into every initial and later handoff; quality gates still apply.

**Ask whether to audit first**, and recommend it. Without it, a cycle or a dangling `depends-on` surfaces only as an intent that silently never launches: `report.sh intent-ready` refuses a build whose prerequisite is unbuilt in either direction, and one whose own `blocks` or `extends` names an intent that is nowhere — but it catches no other Blocker.

**One milestone at a time, and any milestone can be the one.** `<arg>` names it, or names slugs outright; with neither, list the milestones still holding unbuilt intents and ask which — recommend the one whose unbuilt intents depend on nothing outside it. **Nothing outside the chosen one launches, however unblocked it is** — that is the parked work. `milestone: none` is unplanned rather than a milestone, so it is never the chosen one; naming slugs is the only route to one of those.

The launchable set:

- **Audited** — that milestone's share of the audit's first build batch.
- **Unaudited** — every unbuilt intent in that milestone that points at no unbuilt intent, over the dependency edges the Links rule draws (`~/.kk-flavor/skills/idsd-intent/SKILL.md` → **Rules**), `depends-on` being only half of them.

**Those edges decide what launches together; overlapping files do not.** The overlap resolves at their merges, and **3**'s relay keeps them from surprising each other. Hold a pair back only where the overlap is big enough that they would redo each other's work, and say so — that size depends on the two intents, never on a count of shared files.

Drop from that set every intent whose `idsd/NNN-<slug>` branch or worktree already exists (`git branch --list 'idsd/*'`, `git worktree list`) — a second task for one intent puts two sessions on one branch.

**At most 10 intents in flight**, counted as authored-but-unlanded plus building. Count each intent once across authoring and build sessions; an exploration worker does not add an intent. What the cap protects is not machine load: every authoring session regenerates `.idsd/roadmap.md`, so authors collide there and serialise through rebase-and-retry. A regeneration off a stale tree also drops edges without reddening any gate. **Finalize contention is the sharper limit**: a ship forced to re-qualify holds the merge slot across that whole pass (`~/.kk-flavor/skills/idsd-finalize/SKILL.md` → **2. Take the slot**), and every other finalize waits it out. Over the cap, keep the intents others wait on and drop the rest from the set, parked — **2** launches one session per intent still in it.

**Present the schedule and launch only what the human confirms.** Say the cap and what it currently counts; it is theirs to change for the run. Read the launchable set back by name, say what each later intent waits on and what you are leaving parked, and count the `draft` intents in the set — each grills the human in its own thread at `idsd-build`'s gap rounds. Nothing on disk marks an intent as parked, since `status: draft` fits a fresh intent and a shelved one alike, so the ask is the only place that knowledge enters. Say that this session is the reactor's address: it launches the later intents only while it stays open. **After an audit, a Blocker touching what they confirmed stops the launch**: route each through the skill the audit names, then re-run the audit.

**The human alone says whether each session archives itself once its intent lands** — no licence to act unattended supplies that answer, and until they do, none archives. **They are answering about the sessions you launch, never about you.** Archive yourself and every later `done` lands nowhere: nothing schedules what the last merge unblocked, and the sessions still working are reporting to a session that is gone. You run until the human stops you, and stopping you is theirs.

**One intent set has one reactor.** A second recomputes the same graph and cannot see the branch the first is about to cut, so the branch check above does not reach it — that check is one reactor's view of what exists, never of what another is deciding. The sessions fare worse than the branches: each was given one address for `done`, the slot question and contract changes, so half the run reports to a reactor that does not know what the other half was told. **Succession is stop then start** — the human archives the incumbent, then starts the successor — so **never launch your own replacement while you are live**, however certain you are that you are going.

## 2. Launch — one task per intent

Prepare each confirmed intent's handoff in this coordinator through `kk-handoff` inline. Reuse verified shared facts while their inputs remain unchanged; keep each draft's slug, branch, allocation and licence specific to its task. Delegate bounded discovery only when a handoff needs substantial context not already held, within the available worker capacity. A discovery worker returns facts to this coordinator, which owns the checked draft and launch.

**Keep the handoff prompt thin.** It states one task: run `idsd-ship <NNN-slug>` in this repo through `idsd-ship done` — then archive the session, where the human agreed to that. The receiving session reads the ICE, the charter and the constraints itself. A prompt that summarises them drifts, and the summary is what gets built.

**Include independent quality workers in the schedule the human approves**, then carry that authorization into each handoff. The receiving coordinator dispatches applicable leaves under `~/.kk-flavor/standards/quality-pipeline.md`, within its worker capacity. It retains independent reviews and protected model roles without assigning a worker to every inline phase.

Each prompt also carries what no file on disk holds:

- the landing process, target branch and user authorization settled in **1**;
- the branch each sibling cuts (`idsd/NNN-<slug>`), for `~/.kk-flavor/skills/kk-handoff/handoff-prompt.md` → **Where it starts**;
- its shared-runtime allocation — a port range, plus the browser and any single-slot install held by one session at a time;
- this session's return address, resolved by **Client mechanics**;
- the three messages to send here (**3**), each sent **before** the session archives itself, since archiving stops it and loses anything unsent.

**Launch each confirmed intent as soon as its handoff passes the check and its allocation is available.** Independent checks may run concurrently; an unfinished sibling draft does not hold a ready launch. After a partial launch, reconcile returned task IDs with the branch/worktree and client inventories before retrying, so a retry does not create a duplicate task. Without the selected client's launch mechanism, return the checked draft paths.

Done when one task per confirmed launchable intent exists, or each remaining draft has a stated blocker. **Deferred tasks need a human click**; with nobody at the keyboard, name those still waiting and end the turn. Immediately started tasks proceed to **3**.

## 3. React

Use the selected client's waiting mechanism from **Client mechanics**. Close each turn with the live sessions and what each waiting intent waits on.

**Every message is a report about a state that has moved since.** A session writes, then keeps working, and you read minutes later — so check the repo before acting on what one told you.

- **A contract change** — an API shape, a shared type, a wire protocol. Forward it to every live sibling whose ICE consumes it, so that sibling rebases instead of colliding.
- **The merge slot** — a sibling that hit the slot refusal asks whether the holder is still alive, and only your live-session list answers (`~/.kk-flavor/skills/idsd-finalize/SKILL.md` → **2. Take the slot**). Match the worktree the refusal names against it: gone, and the sibling may `--force`; otherwise it waits.
- **`done`** — verify the merge on the agreed target and the completed archive before freeing the allocation. A PR still waiting to merge stays in flight; an archive on its feature branch does not unblock dependents. Recompute the launchable set and return to **2** at once for whatever this merge unblocked. **A batch is the starting schedule, never a barrier.**
- **An ask with no intent behind it** — the human wants work the set does not carry, or a ship surfaced something nobody wrote down. **You are the address for that ask, and you do not answer it yourself**: launch a session scoped to `idsd-intent` and nothing else — author the ICE, stop, build none of it — and hand it its number. It goes the way a ship does, in the same order: **its ICE on the target branch, then its `done` to you, then it archives itself** where the human agreed to that. Until it lands, an ICE on a branch nothing merged is not merely unlanded — it is lost with the session that wrote it. Authoring is where a brief gets corrected — that session reads the base and finds what your account of it had wrong — so an ICE written from your summary ships your errors into the set as requirements.
- **A new intent** — a session split its own, or the human authored one mid-run. Add it and recompute: it launches once its `depends-on` targets are built, and those may still be in flight, so work authored now is scheduled behind the wave rather than held back until the wave ends. **You hand out its `NNN`** — `idsd-intent` bumps off a number already written, which does not separate two sessions writing at the same moment. **A number you promised is not a number taken**: an edge naming it, written before that session wrote anything, gates on nothing if the session landed on another — and gates on unrelated work as soon as something else takes the one you named. Read the edge back against what the authoring session actually wrote. A session that split still owes one `done` per piece it became; take the new slugs when it tells you.

**Check the live-session inventory at each wake** and name each mismatch to the human. One gone that never sent `done` proves nothing — a late message and one that never comes look alike — so **read the repo before calling it dead**: its intent in `.idsd/archive/` with its merge on the target means it landed, and you treat that as its `done`. For a PR route, check its recorded PR first: an open PR is waiting, and a merged PR missing its local archive needs finalize resumed. Neither is a dead, unbuilt intent. Reclaim an abandoned allocation only after reconciling its branch, PR and target state. **`done` frees the allocation, not the departure.** A session that sent `done` and landed its work has finished; where the human asked for self-archiving, one still listed simply did not archive — a row for them to clear, named once and not waited on. Holding its allocation until it goes stalls the run behind an archive that may never take: the call reports success from inside the turn that issued it, and only a later inventory says whether it took.

**A question you cannot see is not yours to hold** — the human answers each session in its own thread.

The run ends when every intent in scope, additions included, has landed and every session it launched has sent `done`. Name each one that will not.
