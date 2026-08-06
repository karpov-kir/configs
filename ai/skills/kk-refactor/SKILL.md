---
name: kk-refactor
description: Review files against the kk-flavor standards (code style, architecture, testing, project setup) and refactor them into compliance — apply routine fixes directly, propose structural ones for confirmation, hunt duplication and simplification across the codebase. Use when asked to "refactor", "clean up", "improve code quality", "apply the standards", or when idsd-qualify spawns its refactor stage.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), whole project, or natural-language scope"
---

Review every file resolved from `$ARGUMENTS` against the kk-flavor standards, one at a time.

**Quality, not correctness or security.** Structure, style, naming, duplication, abstraction, and architecture compliance are this lane. Functional bugs are `/code-review`'s; exploitable weaknesses are `/security-review`'s; trimming prose for concision (in comments or docs) is `/tighten`'s — never flag those here. A true comment attached to the wrong construct *is* yours, since it is placement rather than wording; a comment whose content is false is `/code-review`'s, because it marks a defect. (Skill names here are roles resolved through `~/.kk-flavor/config.yaml` → `roles`.)

**Rules in scope:** the kk-flavor standards, loaded via its router — inject the flavor if needed (read `~/.kk-flavor/inject.md` when its routing isn't already in context) and pull each doc its triggers point to for the reviewed files: `code-style.md`/`writing.md`/`core-principles.md` generally; the **architecture** docs for source code; the **testing** standard for tests; the **project-setup** standard for env, scripts, or Docker/local-dev config. A project `PROJECT_CODE_STYLE.md` (and its `CLAUDE.md`), when present, layers on top — its rules win on conflict.

**Scope override — cross-file changes:** Core Principle 3 (surgical changes) does not apply here — refactoring *is* the task, so editing any file is in scope. Actively hunt duplication, generalization, and simplification reframes (a remodel that deletes a branch or concept) rather than waiting to stumble on them: treat each reviewed file's functions, types, and non-trivial logic as search seeds, grep the codebase for the same shape (similar names, copied logic, parallel structures), and fix every site at once — extract a shared helper, generalize a special case, collapse parallel variants, or remodel state so a conditional disappears. Do this only when it genuinely cuts duplication or complexity; a change that just adds indirection, couples unrelated callers, or chases a one-off resemblance is not an improvement. The one constraint: before touching a file outside the resolved list, describe the change and get your caller's confirmation. Files already in the list and new files from your own fixes need none.

**Confirmation by change kind.** Apply every fix directly except architecture, testing, and project-setup ones (subject to the cross-file rule above). **Architecture**, **testing**, and **project-setup** fixes are structural and opinionated — propose each, with any files it touches, and apply only after your caller confirms; routine refactoring is never gated. A declined gated fix is noted and skipped, not retried.

**Prefer the proper fix.** Take the root-cause fix over a cheaper workaround that only masks the symptom — gated by the rules above when it spans files or turns structural. Root-cause means the quality root — not a functional bug: a logic error or broken behaviour is `/code-review`'s lane.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `File`; deltas below.

## Setup (once)

- Load the in-scope standards (per **Rules in scope**, via `~/.kk-flavor/inject.md`) plus the skill protocol (above) and any project `PROJECT_CODE_STYLE.md`/`CLAUDE.md`, once the file list is resolved (next bullet). Extract every guideline as a numbered list `G1..Gn`, tagging each **architecture**, **testing**, **project-setup**, or **other** — the tag drives **Confirmation by change kind**. Print the index once so it can be referenced by number. State the active mode on one line: which standards are live.
- Resolve the file list from `$ARGUMENTS`: a git scope (per the protocol), a file path or directory (for a directory, recursively glob source, configuration, and documentation files), **whole project** — every source, configuration, and documentation file under the root — or a natural-language scope. Queue per the protocol; appends here are new files from your own fixes and files pulled in per **Scope override**.

## Loop deltas

- Check every guideline `G1..Gn` against the file. Guidelines that don't apply to this file (e.g. code-style rules on a markdown file) still count as checked — just don't mention them.

## Verdict

- Pass: `File N/M <path> | <lines>L | G1..Gn | OK`
- Fail:
  ```
  File N/M <path> | <lines>L | G1..Gn | WARN
  <Gx>: <one-line description of what failed>
  <Gy>: <one-line description of what failed>
  ```

If you couldn't check every guideline, list only the ones you did check and mark the verdict `WARN`.

## Do not

- Expand into *unrelated* cleanup — changes that don't trace to a guideline violation.
