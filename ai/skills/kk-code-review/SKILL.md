---
name: kk-code-review
description: Review the working-tree changes for correctness bugs — apply the safe fixes, surface the rest. Use for "code review". One pass, not a pipeline (kk-qualify); a GitHub PR is kk-pr-review's; style is kk-refactor's lane, vulnerabilities kk-security-review's. Judges against the kk-flavor standards, unlike the same-named bundled reviewer.
argument-hint: "file, directory, diff selector (staged/unstaged/all changed), or natural-language scope"
---

Review every change resolved from `$ARGUMENTS` for **correctness** — bugs, broken logic, violated invariants and constraints, leaks, races, misuse that makes the code do the wrong thing.

**Correctness, not quality.** Style, naming, duplication, abstraction, and structure are `kk-refactor`'s lane — never flag them here; broad security auditing is `kk-security-review`'s. A security rule the project's `CLAUDE.md`/constitution states is in scope — violating one is a constraint bug.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `File`; deltas below. This reviews *changes* — no whole-project mode by design.

## Review dimensions

Check every changed file against all five:

1. **Standards correctness rules** — violations (kk-flavor standards or the project's `CLAUDE.md`) whose breach causes bugs: bypassed type checks, unchecked assertions, swallowed errors, unhandled absence.
   - Also yours: **a declaration that permits violating an invariant the code states in prose** — a parameter whose wrong value is unsafe, an optional that cannot legitimately be absent. Flag the mismatch and name the fact that makes it unsafe; a fix bigger than narrowing a type is `kk-refactor`'s.
2. **Bug scan** — read the changed lines; flag real bugs.
3. **History** — git blame/log of the file and recent commits touching it; flag bugs visible in that context.
4. **Comments** — flag changes that violate guidance written in a comment, and check each factual claim a comment makes against the code, schema or migration it describes: a false comment is itself a finding.
5. **A changed behaviour no test exercises** — an added or changed body whose behaviour no test reaches at any level. The one absence CI cannot report: a run proves what it covers, never what it omits, and a coverage percentage passes with the new branch untested. Name the behaviour that is unguarded; writing the test is `kk-refactor`'s gated testing lane.

## Loop deltas

- Surface a finding only once you've verified it is a real bug that will be hit, and — for a standards-flagged one — that the standard names that issue specifically.
- Apply each surviving finding that is a safe correctness fix — unambiguous, within the changed scope, and making the code do what its stated contract already claims. **A fix that would oblige you to write a test is not a safe one** (`~/.kk-flavor/standards/skill-protocol.md` → **Your own fixes are unreviewed code**): a changed return, a new error path, a widened or narrowed signature. Flag those instead.
- Once a finding is confirmed, grep the interface or module it belongs to for the same shape before moving on, and report what the sweep found.
- **A finding that predicts how a device, browser or external service behaves at runtime is unconfirmed until you check what actually happened.** Recorded results — a test run, a log, an earlier session's output — outrank platform documentation: read them where they exist and name what you read; where they don't, label the finding an unverified inference.
- The final sweep hunts cross-file and interaction bugs.

## Verdict

Finding line: `<location>: <bug> — fixed | needs human: <decision>`

## Do not

- Post to GitHub, run `gh`, or fix or block on a pre-existing defect outside the change — both rules are `~/.kk-flavor/standards/quality-pipeline.md`'s, and they bind every stage, this one run standalone included.
- Build, typecheck, run tests, or flag nitpicks and anything a linter / typechecker / test catches — assume CI runs them.
