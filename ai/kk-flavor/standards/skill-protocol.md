# Skill Protocol

The execution contract shared by the quality skills — the review/rewrite loops (`code-review`, `security-review`, `refactor`, `tighten`, `humanize`; role names resolve via [../config.yaml](../config.yaml)) and their orchestrators (`idsd-qualify`, `idsd-ship`). Each skill states its lane, lens, and deltas; this file owns the mechanics they all run under.

## Caller

Every skill runs standalone (the user is your caller) or spawned by an orchestrator with no interactive user. Every "ask" / "confirm" resolves to *ask your caller*: interactive → ask directly; spawned → don't apply, return the proposal (or `blocked: <what you need>`) and stop. Never exceed your license — a gated, out-of-scope, or human-decision change — just because you can't ask; and never stop without resolution when returning a proposal would do.

## Queue

- Resolve the target to a file list and save it to TodoWrite — the queue. Every queued file gets a verdict.
- Git scopes: **staged** → `git diff --name-only --cached`; **unstaged** → `git diff --name-only`; **all changed** → `git diff --name-only HEAD`. A path or directory → glob the artifact kinds the skill reviews; a natural-language scope → the matching files. (Which target kinds a skill accepts — changes-only, whole project, literal text — is its own call.)
- Skip deleted files; a rename queues the new path.
- The queue grows only by appending — a sibling pulled in to absorb a fix, a file your own fix created — never by dropping a queued file.

## Loop

- One file per message; read it in full every time, re-reviews included (over 2000 lines: sequential chunks until every line is covered).
- Order inside a message: read, act within your license, verdict last. The verdict describes the state **before** your edits.
- A file that passes moves on; one that warns is re-read from scratch next message and retried.
- Safety stop: an issue that resists three passes → emit `WARN` and ask your caller.
- Once every file has a verdict, run one final sweep with the same rules — where cross-file effects surface. A sweep warning retries per the rule above; passing files stay passed. The loop ends only when a complete sweep produces zero warnings.

## Verdict

The last thing in the message, searchable by its fixed prefix:

- Pass: `<Unit> N/M <path> | <lines>L | OK`
- Fail: the same line with `WARN`, then one line per finding.

`<Unit>` is the skill's counter noun (`File`, `Artifact`). `M` is the current queue length — it grows on append; `N` is the file's stable position. `<lines>L` is the file's real line count. Standalone, the verdicts plus applied changes are your output; spawned, they are your structured return.

## Do not

- Skip files by labeling them — "trivial", "historical", "same as prior", or any shortcut meaning "less attention here".
- Use speed or batch language — "quickly", "batch", "skim", and any phrasing that signals lowering the bar.
- Echo the queue, print progress summaries, or write transition filler; never merge files into one verdict or write anything after the verdict.
- Manufacture findings — `OK` with no edits is correct when nothing earns action.
- Change anything your lens doesn't flag (no rewording for taste).

## Orchestrators — interactive first

Prefer asking the human live over deferring to a digest. A decision you can't settle from the code, the intent, or a sensible default — an ambiguity that changes the verdict or what gets built — is *blocking*: stop and ask now, carrying a **recommended answer you earned by legwork** (the code, intent, charter, constitution, precedent); when it stays open after checking, say what you checked. A subagent's `blocked` return relays the same way, with its recommendation named — answer it, then **resume that same subagent by its ID**, never a fresh spawn, which loses the work and skill state it holds. The digest/report is for what does *not* block — never a place to park a question you should ask.
