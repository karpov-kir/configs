---
name: review
description: Review the working-tree changes for correctness bugs and CLAUDE.md violations — apply the safe fixes, surface the rest for a human decision. Use when asked to "review the changes/diff", "code review", or when an orchestrator (idsd-ship) spawns a review of a build. Not a PR tool — operates on local changes and returns findings as data.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), or natural-language scope"
---

Review every change resolved from `$ARGUMENTS` for **correctness** — bugs, broken logic, violated invariants and constraints, misuse that makes the code do the wrong thing. Apply the fixes you can make correctly; surface the rest for a human decision. This reviews **local working-tree changes** and returns findings as data — it never posts to GitHub.

**Correctness, not quality.** Style, naming, duplication, abstraction, and structure are `/refactor`'s lane — never flag them here. `CLAUDE.md` matters only where a rule encodes a correctness invariant (type-system escape hatches, unchecked assertions, unhandled absence); its style and architecture rules belong to `/refactor`.

**Caller.** You run either standalone (the user is your caller) or spawned by an orchestrator with no interactive user. Every "ask" / "confirm" below resolves to *ask your caller*: interactive → ask directly; spawned → don't apply, return the proposal (or `blocked: <what you need>`) and stop. Never apply a fix that needs a human decision just because you can't ask.

**What counts.** Real correctness bugs on the reviewed lines — wrong output, a broken edge case, a violated constraint or invariant, a resource leak, a race. A security rule the project's `CLAUDE.md`/constitution states (path-safety, network-bind, secrets, …) counts too — violating one is a constraint bug. Not: style or structure (→ `/refactor`), broad/generic security auditing (→ `/security-review`), nitpicks a senior engineer wouldn't raise, anything a linter / typechecker / compiler / test catches (assume CI runs them), general quality (coverage, docs) unless `CLAUDE.md` requires it, changes intentional to the broader goal, or issues on lines outside the reviewed changes.

## Setup (once)

- Read `CLAUDE.md` (already in context) and the in-scope standards docs it points to. List the relevant `CLAUDE.md` files — the root one plus any in directories the changes touch.
- Resolve the change set from `$ARGUMENTS` by intent:
  - File path or directory — its current diff against the base.
  - **Staged** / **unstaged** / **all changed** → `git diff` with `--cached` / nothing / `HEAD` (names via `--name-only`).
  - Natural-language scope — the matching changed files.
- Save the changed-file list to TodoWrite — the review queue; every file gets a verdict. Skip deleted files; for a rename, use the new path.

## Review dimensions

Check every changed file against all four; each surfaces candidate findings with the reason flagged:

1. **`CLAUDE.md` correctness rules** — violations of rules whose breach causes bugs (bypassed type checks, unchecked assertions, swallowed errors, unhandled absence). Skip style and architecture rules — those are `/refactor`'s. (`CLAUDE.md` is author-time guidance; not every rule applies at review time.)
2. **Bug scan** — read only the changed lines; flag large, real bugs, skipping nitpicks and likely false positives.
3. **History** — git blame/log of the file and recent commits touching it; flag bugs visible in that context.
4. **Comments** — code comments in the file; flag changes that violate guidance written there.

Cross-file and interaction bugs: flag on whichever file surfaces them; the final sweep catches the rest.

Surface a finding only when you've verified it's a real bug that will be hit — discard maybes and anything a closer look doesn't confirm. For a `CLAUDE.md`-flagged finding, confirm the `CLAUDE.md` actually calls out that issue specifically.

## Loop

- Review one changed file per message.
- Read the full file every time, including re-reviews. For files over 2000 lines, read in sequential chunks until every line is covered.
- Check every dimension; surface only verified findings (per **Review dimensions**), discarding maybes.
- Apply each surviving finding that is a safe correctness fix (unambiguous, within the changed scope), flagging any that changes behaviour; leave a finding that needs a human decision (a trade-off, an ambiguous intent, a risky change) for the caller, per **Caller**.
- Order inside a message: read the file, apply safe fixes, then emit the verdict last. The verdict describes the state **before** the fix.
- If the file passes, move on. If a finding resists three passes, emit a `WARN` verdict and ask the caller.
- Once every file has a verdict, do a final sweep with the same dimensions for cross-file and interaction bugs; a file that warns in the sweep retries next message, passing files stay passed.

## Verdict format

Always the last thing in the message, searchable via `File N/M `:

- Pass: `File N/M <path> | <lines>L | OK`
- Fail:
  ```
  File N/M <path> | <lines>L | WARN
  <location>: <bug> — fixed | needs human: <decision>
  ```

`M` is the current queue length, not a frozen total; `N` is the file's stable position. Standalone, the verdicts plus applied fixes are your output; spawned, that set is your structured return for the caller to record.

## Do not

- Post to GitHub or run `gh` — this is a local review.
- Build, typecheck, or run tests — assume CI does.
- Flag nitpicks or anything a linter / typechecker / test catches.
- Fix or block on a pre-existing bug outside the change. If one is serious, surface it once as a separate non-blocking note for the human to route — never fold it into the change's findings or drop it silently. (A pre-existing bug the change makes reachable or worse is in scope — the change introduced that.)
- Apply a fix that needs a human decision just because you can't ask (per **Caller**).
