# Skill Protocol

## Caller

Every skill runs standalone (the user is your caller) or spawned by an orchestrator with no interactive user. Every "ask" / "confirm" resolves to *ask your caller*: interactive → ask directly; spawned → don't apply the change, return the proposal (or `blocked: <what you need>`) and stop. Never exceed your license — a gated, out-of-scope, or human-decision change — just because you can't ask; and never stop without resolution when returning a proposal would do.

**Invoking another skill spawns it**, with a scope and a return contract; its reading, dead ends, and intermediate state stay out of your context. **Invoke it, never reimplement it** — its own rules still hold, whatever you scoped it to. Run one inline only when it needs the human continuously (they reach only your thread) or when its target is text you already hold; name which when you do.

## Setup

Read this file, the standards the flavor's router (`~/.kk-flavor/inject.md`) points at for what you're reviewing, and the project's own `CLAUDE.md` — the root one and any in a directory your target touches. Further reading, and any index you build from it, is your skill's delta.

## Queue

- Resolve the target to a file list and save it to TodoWrite — the queue. Every queued file gets a verdict.
- Git scopes: **staged** → `git diff --name-only --cached`; **unstaged** → `git diff --name-only`; **all changed** → `git diff --name-only HEAD`. A path or directory → glob the artifact kinds the skill reviews, or, where the skill is scoped to changes, its current diff against the base; a natural-language scope → the matching files. (Which target kinds a skill accepts — changes-only, whole project, literal text — is its own call.)
- Skip deleted files; a rename queues the new path.
- The queue grows only by appending — a sibling pulled in to absorb a fix, a file your own fix created — never by dropping a queued file. **Before touching a file outside the resolved list, describe the change and get your caller's confirmation**; a file already queued, or one your own fix created, needs none.
- **A change that starts depending on an in-repo sibling puts that sibling's public surface in the queue.**
- **Undoing an out-of-scope edit is the inverse of that edit, never a checkout.** Restoring the file to `HEAD` discards every other uncommitted change in it — a caller's own deliberate edit included, with nothing to say it went.
- **Write each verdict to a ledger as you emit it**, at the path your caller names, or `<scratch>/<skill>-queue.md` standalone. One line per file, its path and its verdict. The scratch dir is **outside the repository**. On starting, read the ledger if it exists and resume at the first queued file without a verdict.
- **One ledger per spawn, never per skill.** The scratch dir is per *session*, so an orchestrator spawning a skill more than once names a distinct path each time. Reusing it makes the second spawn adopt the first's completed queue, find no unverdicted file, and close having read nothing.
- **A ledger is only a resume point for the tree it was written against.** Head it with the tree's fingerprint plus the resolved file list, and discard a ledger whose head does not match or does not parse. Fingerprint it with **`GIT_INDEX_FILE` pointed at a `mktemp` path you then delete, on both commands** — `add -A`, then `write-tree`. Deleting it first is not optional: `mktemp` leaves a 0-byte file behind and git rejects that as an index (`index file smaller than expected`).

## Loop

- One file per message; read it in full every time, re-reviews included (over 2000 lines: sequential chunks until every line is covered).
- Order inside a message: read, act within your license, verdict last. The verdict describes the state **before** your edits.
- A file that passes moves on; one that warns is re-read from scratch next message and retried.
- **A queued file that changed since you read it is re-read, and every finding standing in it re-verified, before you return.** Another stage may be editing the same change set while you review it. Say in the return that the tree moved and against which state your verdicts hold.
- Safety stop: an issue that resists three passes → emit `WARN` and ask your caller.
- Once every file has a verdict, run one final sweep with the same rules — where cross-file effects surface. A sweep warning retries per the rule above; passing files stay passed. The loop ends when a complete sweep produces zero warnings, or at the safety stop above — and the sweep gets its own ledger line.

## Verdict

The last thing in the message, searchable by its fixed prefix:

- Pass: `<Unit> N/M <path> | <lines>L | OK`
- Fail: the same line with `WARN`, then one line per finding.

`<Unit>` is the skill's counter noun (`File`, `Artifact`). `M` is the current queue length — it grows on append; `N` is the file's stable position. `<lines>L` is the file's real line count. A skill may add a field or fix the shape of the finding line; those are its deltas.

## Your own fixes are unreviewed code

Nothing downstream reviews what a quality pass writes, including a fix the orchestrator applied outside any stage.

- **A behaviour-changing fix lands a test per branch it introduces**, whatever its size.
- **A finding that proposes building a subsystem carries that subsystem's standing cost** — disk, memory, schedule, whatever it will keep consuming.

## Finish in the lanes your edits opened

Name every lane your **own edits** gave work to. Standalone, run each, in [quality-pipeline.md](quality-pipeline.md) → **The stages** order. What your pass opened is a handoff; "it might find more" is not. Spawned, **name the handoff in your return and spawn nothing** — your caller owns the round's order and has that stage queued already, or dropped it on purpose.

A handoff carries **only the files that opened the lane** — the ones you changed — never your entire scope. Where your lens is barred from a whole file kind, it also carries the ones you left untouched for the receiving skill. **A skill reached by a handoff makes no further one**; it returns what it found. A caller spawning one therefore **says in the prompt that it is reached by a handoff** — unsaid, the receiver cannot know.

**The caller's half: read every return for a handoff and queue it**, or say why you dropped it. One left unread is work the run created and nothing does. Where it lands in your order is your own skill's delta.

## Do not

- Skip files by labeling them — "trivial", "historical", "same as prior", or any shortcut meaning "less attention here".
- Describe your own pass as quick, batched or skimmed, or use any phrasing that signals lowering the bar.
- Echo the queue, print progress summaries, or write transition filler; never merge files into one verdict or write anything after the verdict **in a file's message**. The run's own closing reply is [writing.md](writing.md) → **Replying to a human**.
- Manufacture findings — `OK` with no edits is correct when nothing earns action.
- Change anything your lens doesn't flag (no rewording for taste).

## Orchestrators — interactive first

Prefer asking the human live over deferring to a digest. Ask a blocking decision (defined below) now, and define every identifier you cite — they hold none of your context. A question carries your recommended answer, the legwork behind it, and a number where the stakes are a size or a duration. A subagent's `blocked` return relays the same way; answer it, then resume **that** subagent by its ID, never a fresh spawn.

**Build every spawn prompt from `~/.kk-flavor/templates/spawn-prompt.md`**, which states its own constraints.

**Most decisions are not blocking, and the default is to settle them.** One blocks only when it is **both** expensive to reverse **and** genuinely unsettled (Core Principle 1). Expensive to reverse means it persists (a schema, a migration, an on-disk or wire format), it crosses a process or repo boundary (a published package, an HTTP API, an event payload), or another slice consumes it; internal-to-one-module and additive-to-an-existing-shape are cheap. **Unrecallable overrides the second test** — an act with no undo blocks however settled its content ([live-systems.md](live-systems.md) → **Arrange the undo before the act**).

Fail either test — that override aside — and you do not ask: decide it, and record **what determined it**. Not being able to name what determined it is the signal it was never determined, so it becomes an ask. **A decision record is never a home for an open question** — one you still have is a live ask or a report item.
