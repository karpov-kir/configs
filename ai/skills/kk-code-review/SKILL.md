---
name: kk-code-review
description: Review the working-tree changes for correctness bugs and standards/CLAUDE.md violations — apply the safe fixes, surface the rest for a human decision. Use when asked to "review the changes/diff", "code review", or when an orchestrator (idsd-qualify) spawns a review of a change set. Not a PR tool — operates on local changes and returns findings as data.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), or natural-language scope"
---

Review every change resolved from `$ARGUMENTS` for **correctness** — bugs, broken logic, violated invariants and constraints, misuse that makes the code do the wrong thing. Apply the fixes you can make correctly; surface the rest for a human decision. This reviews **local working-tree changes** and returns findings as data — it never posts to GitHub.

**Correctness, not quality.** Style, naming, duplication, abstraction, and structure are `/refactor`'s lane — never flag them here. `CLAUDE.md` matters only where a rule encodes a correctness invariant (type-system escape hatches, unchecked assertions, unhandled absence); its style and architecture rules belong to `/refactor`.

**What counts.** Real correctness bugs on the reviewed lines — wrong output, a broken edge case, a violated constraint or invariant, a resource leak, a race. A security rule the project's `CLAUDE.md`/constitution states (path-safety, network-bind, secrets, …) counts too — violating one is a constraint bug. Not: style or structure (→ `/refactor`), broad/generic security auditing (→ `/security-review`), nitpicks a senior engineer wouldn't raise, anything a linter / typechecker / compiler / test catches (assume CI runs them), general quality (coverage, docs) unless `CLAUDE.md` requires it, changes intentional to the broader goal, or issues on lines outside the reviewed changes.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `File`; deltas below.

## Setup (once)

- Inject the kk-flavor if needed (read `~/.kk-flavor/inject.md` when its routing isn't already in context), read the skill protocol (above) and the standards the flavor routes you to; also read the project's own `CLAUDE.md` files — the root one plus any in directories the changes touch — for project-specific conventions.
- Resolve the change set from `$ARGUMENTS`: a git scope (per the protocol), a file path or directory (its current diff against the base), or a natural-language scope (the matching changed files). This reviews *changes* — no whole-project mode by design. Queue per the protocol.

## Review dimensions

Check every changed file against all four; each surfaces candidate findings with the reason flagged:

1. **Standards correctness rules** — violations (in the kk-flavor standards or the project's `CLAUDE.md`) whose breach causes bugs (bypassed type checks, unchecked assertions, swallowed errors, unhandled absence). Skip style and architecture rules — those are `/refactor`'s. (These are author-time guidance; not every rule applies at review time.)
2. **Bug scan** — read only the changed lines; flag large, real bugs, skipping nitpicks and likely false positives.
3. **History** — git blame/log of the file and recent commits touching it; flag bugs visible in that context.
4. **Comments** — code comments in the file; flag changes that violate guidance written there.

Cross-file and interaction bugs: flag on whichever file surfaces them; the final sweep catches the rest.

Surface a finding only when you've verified it's a real bug that will be hit — discard maybes and anything a closer look doesn't confirm. For a standards-flagged finding, confirm the standard (or the project's `CLAUDE.md`) actually calls out that issue specifically.

## Loop deltas

- Check every dimension; surface only verified findings, discarding maybes.
- Apply each surviving finding that is a safe correctness fix (unambiguous, within the changed scope), flagging any that changes behaviour; a finding that needs a human decision (a trade-off, an ambiguous intent, a risky change) goes to your caller.
- The final sweep hunts cross-file and interaction bugs.

## Verdict

- Pass: `File N/M <path> | <lines>L | OK`
- Fail:
  ```
  File N/M <path> | <lines>L | WARN
  <location>: <bug> — fixed | needs human: <decision>
  ```

## Do not

- Post to GitHub or run `gh` — this is a local review.
- Build, typecheck, or run tests — assume CI does.
- Flag nitpicks or anything a linter / typechecker / test catches.
- Fix or block on a pre-existing bug outside the change. If one is serious, surface it once as a separate non-blocking note for the human to route — never fold it into the change's findings or drop it silently. (A pre-existing bug the change makes reachable or worse is in scope — the change introduced that.)
