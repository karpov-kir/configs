---
name: refactor
description: Refactor code to follow project code style rules. Use when asked to clean up, refactor, or improve code quality.
argument-hint: "file path, directory, or natural language description of scope"
---

Review every file resolved from `$ARGUMENTS` against CLAUDE.md, one at a time.

**Rules in scope:** all of `CLAUDE.md` (already in context — every section) plus every standards doc it points to. Pull the linked docs in by relevance: a project code-style override if the project has one (its rules win over the generic ones on conflict); the architecture standard when the reviewed files include source code; the testing standard when they include tests.

**Scope override — cross-file changes:** Core Principle 3 (surgical changes) does not apply here — refactoring *is* the task, so editing any file is in scope. Actively hunt duplication and generalization rather than waiting to stumble on it: treat each reviewed file's functions, types, and non-trivial logic as search seeds, grep the codebase for the same shape (similar names, copied logic, parallel structures), and fix every site at once — extract a shared helper, generalize a special case, collapse parallel variants. Do this only when it genuinely cuts duplication or complexity; a change that just adds indirection, couples unrelated callers, or chases a one-off resemblance is not an improvement. The one constraint: before touching a file outside the resolved list, describe the change and get the user's confirmation. Files already in the list and new files from your own fixes need none.

**Confirmation by change kind.** Apply every fix directly except architecture and testing ones (subject to the cross-file rule above). **Architecture** and **testing** fixes are structural and opinionated — propose each, with any files it touches, and apply only after the user confirms; routine refactoring is never gated. A declined arch/test fix is noted and skipped, not retried.

## Setup (once)

- Read `CLAUDE.md` and the in-scope standards docs it points to (per **Rules in scope**), once the file list is resolved (next bullet). Extract every guideline as a numbered list `G1..Gn`, tagging each **architecture**, **testing**, or **other** — the tag drives **Confirmation by change kind**. Print the index once so it can be referenced by number. State the active mode on one line: which standards are live.
- Resolve the file list from `$ARGUMENTS` by intent:
  - File path or directory — that path directly; for a directory, recursively glob source, configuration, and documentation files.
  - **Staged** ("staged files", "what I've staged", "files added to the index", and similar) — `git diff --name-only --cached`.
  - **Unstaged** ("unstaged files", "my current edits", "uncommitted changes", and similar) — `git diff --name-only`.
  - **All changed** ("all changed files", "everything modified", "every changed file", and similar) — `git diff --name-only HEAD`.
  - **Whole project** ("every file in the project", "whole codebase", "all source, config, and documentation", and similar) — every source, configuration, and documentation file under the project root.
- Skip deleted files. For renames, use the new path. Save the list to TodoWrite. The list is the review queue — every file on it gets a verdict. It grows only by appending: new files created by your own fixes, and existing files you pull in to resolve a guideline violation (e.g. the shared helper that absorbs duplication) per **Scope override**. Never drop a file once queued.

## Loop

- Review one file per assistant message.
- Read the full file every time, including when re-reviewing after a fix. For files over 2000 lines, read in sequential chunks until every line is covered.
- Check every guideline `G1..Gn` against the file. Guidelines that don't apply to this file (e.g. code-style rules on a markdown file) still count as checked — just don't mention them.
- Order inside a message: read the file, apply fixes if needed, then emit the verdict as the last thing in the message. The verdict describes the state **before** the fix.
- If the file passes, move on. If it fails, the next message re-reads the same file from scratch and tries again.
- Safety stop: if the same guideline fails on the same file three messages in a row, emit a `WARN` verdict, stop the loop, and ask the user for help.
- Once every file has an `OK` verdict, do a final sweep with the same rules. If any file warns during the final sweep, void all prior `OK`s and restart from file 1.
- The loop ends only when a complete final sweep produces zero warnings.

## Verdict format

Always the last thing in the message, easy to search via `File N/M `:

- Pass: `File N/M <path> | <lines>L | G1..Gn | OK`
- Fail:
  ```
  File N/M <path> | <lines>L | G1..Gn | WARN
  <Gx>: <one-line description of what failed>
  <Gy>: <one-line description of what failed>
  ```

- Use the exact prefix `File N/M ` — no variants like `[1/78]` or `File 1 of 78`. `M` is the current queue length at the time of the message, not a frozen total; it grows when files are appended per **Scope override**, so a later `M` may exceed an earlier one. `N` is the file's stable position in the queue.
- `<lines>L` is the file's actual line count.
- If you couldn't check every guideline, list only the ones you did check and mark the verdict `WARN`.

## Do not

- Skip files by labeling them — "historical", "trivial", "obvious", "already covered", "auto-pass", "same as prior", and any similar shortcut that means "this one needs less attention".
- Use speed or batch language — "efficiently", "faster", "quickly", "batch", "lump", "skim", "move faster", and any similar phrasing that signals lowering the bar.
- Echo the TodoWrite queue or print queue summaries like "remaining: X" or "completed: 1-N".
- Write setup or transition filler — "setup done", "continuing", "next file", and similar.
- Merge multiple files into one verdict, skip the verdict, or write anything after the verdict.
- Expand into *unrelated* cleanup — changes that don't trace to a guideline violation.
