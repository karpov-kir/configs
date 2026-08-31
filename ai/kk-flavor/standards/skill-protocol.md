# Skill Protocol

## Caller

Every skill runs standalone (the user is your caller) or spawned by an orchestrator with no interactive user. Every "ask" / "confirm" resolves to *ask your caller*: interactive → ask directly; spawned → don't apply the change, return the proposal (or `blocked: <what you need>`) and stop. Never exceed your license — a gated, out-of-scope, or human-decision change — just because you can't ask; and never end blocked where returning a proposal would do. **Committing is the caller's or the human's act.** **Staging is not committing** — a lane may leave a staged tree only where its own contract declares that tree its product, never as a side effect (**Queue**). **An absolute that exists to keep you from deciding something yields to being told plainly to do it** — it bars your initiative, not their instruction, and following that instruction is them acting through you. Agreeing with your findings is not that instruction. **An absolute that holds however it is authorised says so, and says why.** Where the caller named a patch queue, proposals stream instead of waiting for the return: [streaming.md](streaming.md) is the whole delta for that path.

**Invoking another skill spawns it**, with a scope and a return contract; its reading, dead ends, and intermediate state stay out of your context. **Invoke it, never reimplement it** — its own rules still hold, whatever you scoped it to. Run one inline only when it needs the human continuously (they reach only your thread) or when its target is text you already hold; name which when you do.

## Setup

Read this file, the standards the flavor's router (`~/.kk-flavor/inject.md`) points at for what you're reviewing, and the project's own `CLAUDE.md` — the root one and any in a directory your target touches. Further reading, and any index you build from it, is your skill's delta.

## Queue

- Resolve the target to a file list and save it to TodoWrite — the queue. Every queued file gets a verdict.
- Git scopes: **staged** → `git diff --name-only --cached`; **unstaged** → `git diff --name-only`; **all changed** → `git diff --name-only HEAD`. A path or directory → glob the artifact kinds the skill reviews, or, where the skill is scoped to changes, its current diff against the base; a natural-language scope → the matching files. (Which target kinds a skill accepts is its own call.)
- Skip deleted files; a rename queues the new path.
- The queue grows only by appending — a sibling pulled in to absorb a fix — never by dropping a queued file. **Before touching a file outside the resolved list, describe the change and get your caller's confirmation**; a file already queued, or one your own fix created, needs none.
- **A change that starts depending on an in-repo sibling puts that sibling's public surface in the queue.**
- **Undoing an out-of-scope edit is the inverse of that edit, never a checkout.** Restoring the file to `HEAD` discards every other uncommitted change in it — a caller's own deliberate edit included, with nothing to say it went. Copy the file into the scratch dir first and restore from that copy, or apply and undo the edit as a patch — a copy left in the tree moves the fingerprint and discards the ledger by the resume-point rule below.
- **Delete and move with `rm` and `mv`, never `git rm` or `git mv`** — those stage as a side effect, and the index is shared. A bare `git commit` from any other session carries your deletion or rename under their message, and nothing in `git status` says whose it was.
- **Write each verdict to a ledger as you emit it**, at the path your caller names, or `<scratch>/<skill>-queue.md` standalone. One entry per file: its path and its verdict as emitted. The scratch dir is **outside the repository**. On starting, read the ledger if it exists and resume at the first queued file without a verdict.
- **One ledger per spawn, never per skill.** The scratch dir is per *session*, so an orchestrator spawning a skill more than once names a distinct path each time. Reusing it makes the second spawn adopt the first's completed queue, find no unverdicted file, and close having read nothing.
- **A ledger is only a resume point for the tree it was written against.** Head it with the tree's fingerprint plus the resolved file list, and discard a ledger whose head does not match or does not parse. **Fingerprint it with `~/.kk-flavor/scripts/tree-fingerprint.sh`**, never by hand — a half-right hand-rolled form stages the caller's whole tree against their real index, a destructive act with no prompt.

## Loop

- One file per message; read it in full every time, re-reviews included (over 2000 lines: sequential chunks until every line is covered).
- Order inside a message: read, act within your license, verdict last. The verdict describes the state **before** your edits.
- A file that passes moves on; one that warns is re-read from scratch next message and retried.
- **A queued file that changed since you read it is re-read, and every finding standing in it re-verified, before you return.** Another stage may be editing the same change set while you review it. Say in the return that the tree moved and against which state your verdicts hold.
- **And it is never written from the old read.** The change you did not see is the one your edit erases, and that erasure shows in no diff — the file simply holds your version. **Being told a file is yours is not the same as holding it**: agreeing who owns a file settles nothing about who is mid-edit in it right now.
- Safety stop: an issue that resists three passes → emit `WARN` and ask your caller.
- Once every file has a verdict, run one final sweep with the same rules — where cross-file effects surface. A sweep warning retries per the rule above; passing files stay passed. The loop ends when a complete sweep produces zero warnings, or at the safety stop above — and the sweep gets its own ledger line.

## Verdict

The last thing in the message, searchable by its fixed prefix:

- Pass: `<Unit> N/M <path> | <lines>L | OK`
- Fail: the same line with `WARN`, then one line per finding.

`<Unit>` is the skill's counter noun (`File`, `Artifact`). `M` is the current queue length — it grows on append; `N` is the file's stable position. `<lines>L` is the file's real line count. A skill may add a field or fix the shape of the finding line; those are its deltas.

**The caller counts the verdict lines against the file list** — a return that verdicts one file and carries findings for the rest reads as complete, with nothing in it marking the omission. Resume that subagent and point it at **Queue**.

## Your own fixes are unreviewed code

Nothing downstream reviews what a quality pass writes, including a fix the orchestrator applied outside any stage.

- **A behaviour-changing fix lands a test per branch it introduces**, whatever its size.
- **A finding that proposes building a subsystem carries that subsystem's standing cost** — disk, memory, schedule, whatever it will keep consuming.

## Finish in the lanes your edits opened

Name every lane your **own edits** gave work to. Standalone, run each, in [quality-pipeline.md](quality-pipeline.md) → **The stages** order. What your pass opened is a handoff; "it might find more" is not. Spawned, **name the handoff in your return and spawn nothing** — your caller owns the round's order and has that stage queued already, or dropped it on purpose.

A handoff carries **only the files that opened the lane** — the ones you changed — never your entire scope. Where your lens is barred from a whole file kind, it also carries the ones you left untouched for the receiving skill. **A skill reached by a handoff makes no further one**; it returns what it found.

**The caller's half: read every return for a handoff and queue it**, or say why you dropped it. One left unread is work the run created and nothing does. Where it lands in your order is your own skill's delta.

## Do not

- Skip files by labeling them — "trivial", "historical", "same as prior", or any shortcut meaning "less attention here".
- Describe your own pass as quick, batched or skimmed, or use any phrasing that signals lowering the bar.
- Echo the queue, print progress summaries, or write transition filler; never merge files into one verdict or write anything after the verdict **in a file's message**. The run's own closing reply is [writing.md](writing.md) → **Replying to a human**.
- Manufacture findings — `OK` with no edits is correct when nothing earns action.
- Change anything your lens doesn't flag (no rewording for taste).

## Orchestrators — interactive first

Prefer asking the human live over deferring to a digest. Ask a blocking decision (defined below) now. A question carries your recommended answer, the legwork behind it, and a number where the stakes are a size or a duration. A subagent's `blocked` return relays the same way; answer it, then resume **that** subagent by its ID, never a fresh spawn — which re-reads what it already read.

**Before your first write, enumerate the agents live in your tree and tell each which files you hold** — a stage you spawn writes as your own. **Never block waiting on a peer**: announce, then work.

**Build every spawn prompt from `~/.kk-flavor/templates/spawn-prompt.md`**, which states its own constraints. **A licence you received goes into every spawn prompt you build, verbatim** — worded to bind you, it binds the stages acting in your place.

**Most decisions are not blocking, and the default is to settle them.** One blocks only when it is **both** expensive to reverse **and** genuinely unsettled ([core-principles.md](core-principles.md) → **1. Think before coding**). Expensive to reverse means it persists (a schema, a migration, an on-disk or wire format), it crosses a process or repo boundary (a published package, an HTTP API, an event payload), or another slice consumes it. Internal-to-one-module and additive-to-an-existing-shape are cheap. **Unrecallable overrides the second test** — an act with no undo blocks however settled its content ([live-systems.md](live-systems.md) → **Arrange the undo before the act**).

Fail either test — that override aside — and you do not ask: decide it, and record **what determined it**. Not being able to name what determined it is the signal it was never determined, so it becomes an ask. **A decision record is never a home for an open question** — one you still have is a live ask or a report item. **A question you asked and they did not answer is one you still have** — it becomes an item when the pass closes.

**A report item is the blocking test one notch down, not an exemption from it.** Deferring to a digest still spends the human's attention: **their answer has to change what happens next.** Name the branch your recommendation loses to, and what they would have to believe for it to win. Where every answer leads to the same act, you settled it — record what determined it and route the follow-up. **A choice nothing reaches yet is one of those**: it keeps until something calls the code, so it belongs to whatever first does.

**And the next act has to be theirs — both limbs, or it is not an item.** Their answer changing what happens next is not enough when what happens next is your own edit: an item whose recommended branch is *yours* to carry out is one you carry out, then record. What survives is the act that is not yours, whether or not you could perform it: a question for a person, a message someone has to send, an owner someone has to find, a publication, a deletion that loses reasoning — and anything your license bars (**Caller**), which you can do and may not.
