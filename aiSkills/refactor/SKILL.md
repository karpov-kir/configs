---
name: refactor
description: Refactor code to follow project code style rules. Use when asked to clean up, refactor, or improve code quality.
argument-hint: "file path, directory, or natural language description of scope"
---

Review every file resolved from `$ARGUMENTS` against CLAUDE.md, one at a time, following the protocol below.

**Rules in scope:** the entire `~/.claude/CLAUDE.md` (already in context) — `# Code Style`, `# Writing Guidelines`, `# Core Principles`, and every other section. If the current project root contains a `PROJECT_CODE_STYLE.md`, read it and merge its rules on top of `# Code Style` — project-level rules override the generic ones on conflict.

**Scope override — cross-file changes:** Core Principle 3 (surgical changes — "touch only what the task requires") does not apply here; refactoring *is* the task, so improving quality is the request, not scope creep. Editing any file is allowed. Actively hunt duplication and generalization rather than waiting to stumble on it: treat each reviewed file's functions, types, and non-trivial logic as search seeds, grep the codebase for the same shape (similar names, copied logic, parallel structures), and fix every site at once — extract a shared helper, generalize a special case, collapse parallel variants. Do this only when it genuinely cuts duplication or complexity; a change that just adds indirection, couples unrelated callers, or chases a one-off resemblance is not an improvement. The one constraint: before touching a file outside the resolved list, describe the change and get the user's confirmation. Files already in the list and new files from your own fixes need none.

## Setup (once)

- Read `~/.claude/CLAUDE.md` and, if present, the project's `PROJECT_CODE_STYLE.md`. Extract every guideline as a numbered list `G1..Gn`. Print the index once so it can be referenced by number. State the active mode on one line: generic-only, or generic + project override (cite the project file path).
- Resolve the file list from `$ARGUMENTS` by intent:
  - File path or directory — that path directly; for a directory, recursively glob source, configuration, and documentation files.
  - **Staged** ("staged files", "what I've staged", "files added to the index", and similar) — `git diff --name-only --cached`.
  - **Unstaged** ("unstaged files", "my current edits", "uncommitted changes", and similar) — `git diff --name-only`.
  - **All changed** ("all changed files", "everything modified", "every changed file", and similar) — `git diff --name-only HEAD`.
  - **Whole project** ("every file in the project", "whole codebase", "all source, config, and documentation", and similar) — every source, configuration, and documentation file under the project root.
- Skip deleted files. For renames, use the new path. Save the list to TodoWrite. The list is the review queue — every file on it gets a verdict. It grows only by appending: new files created by your own fixes, and existing files you pull in to resolve a guideline violation (e.g. the shared helper that absorbs duplication) per **Scope override**. The latter require the user confirmation described there before being added. Never drop a file once queued.

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
- Comment on guidelines that don't apply to the file.
- Merge multiple files into one verdict, skip the verdict, or write anything after the verdict.
- Expand into *unrelated* cleanup — changes that don't trace to a guideline violation. Cross-file changes that *do* fix a guideline violation (e.g. factoring out duplication into a shared helper) are in scope; see **Scope override** above for the confirmation rule on files outside the resolved list.
