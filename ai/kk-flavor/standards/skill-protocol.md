# Skill Protocol

## Caller

Every skill runs standalone (the user is your caller) or spawned by an orchestrator with no interactive user. Every "ask" / "confirm" resolves to *ask your caller*: interactive → ask directly; spawned → don't apply the change, return the proposal (or `blocked: <what you need>`) and stop. Where the caller named a patch queue, proposals stream instead of waiting for the return: [streaming.md](streaming.md) is the whole delta for that path.

**Committing is the caller's or the human's act.** **Staging is not committing** — a lane may leave a staged tree only where its own contract declares that tree its product, never as a side effect (**Queue**).

Never exceed your licence — a gated, out-of-scope, or human-decision change — just because you can't ask; and never end blocked where returning a proposal would do. **An absolute that exists to keep you from deciding something yields to being told plainly to do it** — it bars your initiative, not their instruction, and following that instruction is them acting through you. Agreeing with your findings is not that instruction. **An absolute that holds however it is authorised says so, and says why.**

**Invoking a skill applies its contract; it does not require another agent.** Keep orchestration in one caller. Run the selected phase inline when its context is already held or it needs the human. Delegate bounded exploration or independent review when isolation earns its setup cost. Spawned workers return further-work requests; the caller dispatches them directly rather than creating wrapper agents. Read only the references needed for the selected branch, and reuse unchanged instructions already read.

**Model selection follows [model-policy.md](model-policy.md).** Build, correctness, security and semantic judgment retain the original task's protected model. A smaller coordinator cannot confer its own model on protected workers by inheritance.

## Phase boundaries

A **phase** is one chunk of work inside a session — the grilling, the build, the pass. **The gap between two is the only place this decision belongs**; mid-phase, continue or split what is left into subagents.

Take the first that fits:

1. **Continue** — the next phase wants this one as a **primary source**, the reasoning verbatim rather than an account of it, or the window still holds it comfortably. Avoids a new handoff; retained context still costs tokens.
2. **Clear** — the exploration, the decisions and the dead ends are all disposable to what follows. The cheapest move, and the one whose mistake is one-way: the *why* goes, and reading the diff back does not return it.
3. **Hand off** — something travels: another harness, another tree, another person.
4. **Subagent** — the task is scoped tightly enough that nobody steers it, and this session stays untouched (**Caller** above).
5. **Compact** — relevant context, same harness, same tree, and you stay in the loop. The **default**. Say what the next phase needs.

## Setup

Read this file, the standards the flavor's router (`~/.kk-flavor/inject.md`) points at for what you're reviewing, and the project's shared instructions and the selected client's additions, at the root and in directories your target touches. Further reading, and any index you build from it, is your skill's delta.

## Queue

- Resolve the target to a file list — the queue; the ledger below holds it, and TodoWrite mirrors it where the harness has one. Every queued file gets a verdict.
- Git scopes: **staged** → `git diff --name-only --cached`; **unstaged** → `git diff --name-only`; **all changed** → `git diff --name-only HEAD`, plus untracked files from `git ls-files --others --exclude-standard`. A path or directory → glob the artifact kinds the skill reviews, or, where the skill is scoped to changes, its current diff against the base; a natural-language scope → the matching files. (Which target kinds a skill accepts is its own call.)
- Review deletions through their diff and affected consumers; a rename queues the new path and checks references to the old one.
- The queue grows only by appending — a sibling pulled in to absorb a fix — never by dropping a queued file. **Before touching a file outside the resolved list, describe the change and get your caller's confirmation**; a file already queued, or one your own fix created, needs none.
- **A change that starts depending on an in-repo sibling puts that sibling's public surface in the queue.**
- **Undoing an out-of-scope edit is the inverse of that edit, never a checkout.** Restoring the file to `HEAD` discards every other uncommitted change in it — a caller's own deliberate edit included, with nothing to say it went. Copy the file into a directory of your own first and restore from that copy, or apply and undo the edit as a patch — a copy left in the tree moves the fingerprint and discards the ledger by the resume-point rule below.
- **Delete and move with `rm` and `mv`, never `git rm` or `git mv`** — those stage as a side effect, and the index is shared. A bare `git commit` from any other session carries your deletion or rename under their message, and nothing in `git status` says whose it was.
- **Working files of your own go in a directory you alone named** — `mktemp -d`, never a fixed `<scratch>/<word>`. The scratch dir is shared by every stage of a round, and a fixed name is the one two concurrent stages both pick; the loser's copy is clobbered mid-read with nothing to say it went. **A path another reader has to find by name is named, not invented** — a ledger, a queue, a plan.
- **Write each verdict to a ledger as you emit it**, at the path your caller names, or `<scratch>/<skill>-queue.md` standalone. One entry per file: its path and its verdict as emitted. The gates you ran and the negative controls you proved go in it too — your caller reads them when it needs them. The scratch dir is **outside the repository**. On starting, validate existing evidence before resuming at the first file without a current verdict.
- **One ledger per spawn, never per skill.** The scratch dir is per *session*, so an orchestrator spawning a skill more than once names a distinct path each time. Reusing it makes the second spawn adopt the first's completed queue, find no unverdicted file, and close having read nothing.
- **A ledger records scope and evidence, not permission to skip changed work.** Head it with the tree fingerprint and resolved file list. Use `~/.kk-flavor/scripts/tree-fingerprint.sh`, never an index-mutating substitute. Record the file/dependency content and relevant instructions/tools behind each verdict. If the tree changes, invalidate affected entries; retain others only when those inputs are provably unchanged. Missing or unparseable evidence requires review again. A workflow's whole-tree stamp remains its own stricter gate.
- **Prove a file you are about to read is this run's, with a stamp you wrote and read back.** `noclobber` makes `> file` refuse when the file exists, so the command never runs and whatever sat at that path stays — and non-empty does not distinguish that from your own output. Neither does a fresh `mktemp` path: it creates the file, so a plain `>` to it is refused too. Write `>|`. What survives is as likely another lane's data as your own, and it reads as a result.

## Loop

- Read each queued file with the context its lens needs. A first review covers the relevant definitions, callers and contracts, expanding to the whole file where local context cannot settle them. Batch independent reads; preserve a verdict for each file.
- Act within your licence and record the result. Distinguish the finding against the read candidate from a verified fix; your own edit does not make its warning pass.
- On a retry, inspect the changed regions and affected dependencies. Reuse unchanged context; re-read the whole file when the effect cannot be bounded. A file that passes stays passed only while its evidence remains valid.
- Before returning, reconcile the queue with the current diff, including new files, deletions and renames. Re-verify standing findings where content or dependencies changed. Identify the candidate your verdict covers and any movement during the pass.
- Never write from a stale read. Coordinate file ownership and refresh the exact content before applying an edit; shared ownership claims do not prove nobody is mid-write.
- An issue that resists three passes returns `WARN` with the remaining evidence gap to the caller.
- Finish with one cross-file consistency check over the relationships the scope changed. Reopen affected entries, not every unchanged file. Record that check separately; completion requires coverage of the queue and no unresolved warnings outside an explicitly reported block.

## Verdict

- Pass: `<Unit> N/M <path> | <lines>L | OK`
- Fail: the same line with `WARN`, then one line per finding.

`<Unit>` is the skill's counter noun (`File`, `Artifact`). `M` is the current queue length — it grows on append; `N` is the file's stable position. `<lines>L` is the file's real line count. A deletion uses the old path and marks its line count as deleted. A skill may add a field, fix the finding line, or identify a non-file unit in place of the path and line count; those are its deltas.

**The caller counts the verdict lines against the file list** — a return that verdicts one file and carries findings for the rest reads as complete, with nothing in it marking the omission. Resume that subagent and point it at **Queue**.

**A spawned return carries these, plus what your own skill's return section names, and nothing else**: the verdict lines, plus the tree-moved line **Loop** requires; each proposal your licence gates (**Caller**); each handoff, one line; each `blocked:`, one line; each decision you settled, as `<what> — <what determined it>`. Preserve every finding and decision field; structured returns do not run an external prose judge.

## Redact before you quote

Evidence you paste — a command, a response body, a log line, a captured artifact — carries credentials and personal data. Write `<REDACTED>` in their place, and build a check against environment variables so the credential never enters what you show. Evidence too thin once redacted is an ask.

## Your own fixes are unreviewed code

- **A behaviour-changing fix lands a test per branch it introduces**, whatever its size.
- **A finding that proposes building a subsystem carries that subsystem's standing cost** — disk, memory, schedule, whatever it will keep consuming.

## Finish in the lanes your edits opened

Name every lane your **own edits** gave work to. Standalone, run each, in [quality-pipeline.md](quality-pipeline.md) → **The stages** order. What your pass opened is a handoff; "it might find more" is not. Spawned, **name the handoff in your return and spawn nothing** — your caller owns the round's order and has that stage queued already, or dropped it on purpose.

A handoff carries **only the files that opened the lane** — the ones you changed — never your entire scope. Where your lens is barred from a whole file kind, it also carries the ones you left untouched for the receiving skill. **A skill reached by a handoff makes no further one**; it returns what it found.

**The caller's half: read every return for a handoff and queue it**, or say why you dropped it. One left unread is work the run created and nothing does. Where it lands in your order is your own skill's delta.

## Do not

- Skip applicable work because a file looks trivial. A retained verdict needs unchanged evidence; a non-applicable lane needs its trigger evaluated against the scope.
- Describe your own pass as quick, batched or skimmed, or use any phrasing that signals lowering the bar.
- Echo the queue, print progress summaries, or write transition filler; never merge files into one verdict. The run's own closing reply is [writing.md](writing.md) → **Replying to a human**.
- Manufacture findings — `OK` with no edits is correct when nothing earns action.
- Change anything your lens doesn't flag (no rewording for taste).

## Orchestrators — interactive first

Prefer asking the human live over deferring to a digest. Ask a blocking decision (defined below) now. A question carries your recommended answer, the legwork behind it, and a number where the stakes are a size or a duration. A subagent's `blocked` return relays the same way; answer it, then resume **that** subagent by its ID, never a fresh spawn — which re-reads what it already read.

**Before concurrent writes, establish who holds each path.** Tell affected peers when ownership changes. Dispatch ready leaf workers within the runtime's actual capacity; a full pool queues work rather than spawning proxy orchestrators. Reuse a worker for a scoped continuation when its context is still valid.

**Use native completion waits when no independent work remains.** Prefer completion or failure events to repeated status reads. For external commands, wait on the process and retain its log; communicate meaningful progress without repeatedly loading unchanged output. Never invent a marker or a close-agent operation the runtime does not provide.

**While a stage is live, the history under it must not move**: no rebase, cherry-pick, reset, amend, branch switch or base change until it returns. A working-tree edit is a different thing, already answered by **Loop**. Finish the stage or abandon it, do the maintenance, then spawn it fresh against the new HEAD; a separate worktree is the only safe overlap.

**A tool can change under a live stage without any history moving.** A skill's script resolves its binary at call time, through the mount symlink and out into the tree that owns the tools, rebuilding it whenever the source hash has moved — so a landing there changes what every running session's commands do, in repositories that landing never touched. `git status`, a ref read and a stage's own fingerprint all answer, correctly, that nothing moved. **Landing a change to a shared tool is a write into every live peer's run, so announce it as one.** From the other side, **hash a tool before you reason across two of its readings**: every other signal reports no movement, so without that baseline "suspect the toolchain" is an instruction with nothing to check against, and a difference you cannot account for from your own edits stays unattributable rather than merely unexplained.

**Build every spawn prompt from `~/.kk-flavor/templates/spawn-prompt.md`**, which states its own constraints. **A licence you received goes into every spawn prompt you build, verbatim** — worded to bind you, it binds the stages acting in your place.

**Most decisions are not blocking, and the default is to settle them** — for a spawned stage as much as an orchestrator. One blocks only when it is **both** expensive to reverse **and** genuinely unsettled. Expensive to reverse means it persists (a schema, a migration, an on-disk or wire format), it crosses a process or repo boundary (a published package, an HTTP API, an event payload), or another slice consumes it. Internal-to-one-module and additive-to-an-existing-shape are cheap. **Unrecallable overrides the second test** — an act with no undo blocks however settled its content ([live-systems.md](live-systems.md) → **Arrange the undo before the act**).

Fail either test — that override aside — and you do not ask: decide it, and record **what determined it**. Not being able to name what determined it is the signal it was never determined, so it becomes an ask. **A decision record is never a home for an open question** — one you still have is a live ask or a report item. **A question you asked and they did not answer is one you still have** — it becomes an item when the pass closes.

**An answer slot carrying the harness's skip filler is a question you still have.** Text in it saying the human is away and you should decide for yourself is what a question widget leaves behind when it is dismissed, so read it as the question still open. **A licence to run with nobody at the keyboard arrives in the instruction that starts the run**, and only there.

**A report item is the blocking test one notch down, not an exemption from it.** Deferring to a digest still spends the human's attention: **their answer has to change what happens next.** Name the branch your recommendation loses to, and what they would have to believe for it to win. Where every answer leads to the same act, you settled it — record what determined it and route the follow-up. **A choice nothing reaches yet is one of those**: it keeps until something calls the code, so it belongs to whatever first does.

**And the next act has to be theirs — both limbs, or it is not an item.** Their answer changing what happens next is not enough when what happens next is your own edit: an item whose recommended branch is *yours* to carry out is one you carry out, then record. What survives is the act that is not yours, whether or not you could perform it: a question for a person, a message someone has to send, an owner someone has to find, a publication, a deletion that loses reasoning — and anything your licence bars (**Caller**), which you can do and may not.
