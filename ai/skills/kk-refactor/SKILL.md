---
name: kk-refactor
description: Review files against the kk-flavor standards and refactor them into compliance, hunting duplication and simplification across the codebase. Use for "refactor" or "clean up". Quality, not correctness (kk-code-review), vulnerabilities (kk-security-review), or prose concision (kk-tighten).
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), whole project, or natural-language scope"
---

Review every file resolved from `$ARGUMENTS` against the kk-flavor standards.

**Quality, not correctness or security.** Functional bugs are `kk-code-review`'s, exploitable weaknesses `kk-security-review`'s, trimming prose for concision `kk-tighten`'s — never flag those here. A true comment attached to the wrong construct *is* yours.

**Scope override — cross-file changes:** `~/.kk-flavor/standards/core-principles.md` → **3. Surgical changes** does not apply here — refactoring *is* the task, so editing any file is in scope — under the gate in `~/.kk-flavor/standards/skill-protocol.md` → **Queue**. Hunt, don't stumble:

- **Duplication, generalization, and simplification reframes** (a remodel that deletes a branch or concept). Treat each reviewed file's functions, types, and non-trivial logic as search seeds, grep the codebase for the same shape, and fix every site at once.
- **Shallowness, on the same footing** (`~/.kk-flavor/standards/architecture/core.md` → **Module depth**): a module whose exports mirror its internal functions, a pass-through forwarding to a same-named method one layer down.
- **A duplicate split across two file kinds** — a doc and a script carrying the same command — is invisible to `dup-literals.sh`: make one point at the other.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `File`; deltas below.

## Setup (once)

- **Seed the duplication hunt with `~/.claude/skills/kk-refactor/scripts/dup-literals.sh`** — bare for the uncommitted changes, or the git revisions to scan — unless your caller passed you its output already.
- Extract every guideline the router's standards state for the reviewed files — plus any project `PROJECT_CODE_STYLE.md` — as a numbered list `G1..Gn`, tagging each **architecture**, **testing**, **project-setup**, or **other**.
- A directory globs source, configuration, and documentation recursively; **whole project** is all of those under the root.

## Confirmation by change kind

Apply every fix directly except the **architecture**, **testing**, and **project-setup** ones Setup tagged: propose each with the files it touches, and apply only after your caller confirms. A declined gated fix is noted and skipped, not retried.

## Loop deltas

- Check every guideline `G1..Gn` against the file; ones that don't apply (code-style rules on a markdown file) still count as checked — don't mention them.

## Verdict

Adds a coverage field — `File N/M <path> | <lines>L | G1..Gn | OK` — counting the guidelines you checked. If you couldn't check them all, list only the ones you did and mark the verdict `WARN`.

Finding line: `<the rule, named in words>: <what failed>` — never `G14`, which nobody outside this pass can resolve.

**Close the run by stating plainly whether the change is now compliant**, and name what is open if it isn't — in the run's closing reply, never left to the per-file verdicts.
