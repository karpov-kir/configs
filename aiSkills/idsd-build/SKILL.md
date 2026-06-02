---
name: idsd-build
description: Run the IDSD build loop against an ICE intent — restate, gather context, implement, validate against scenarios and gates, checkpoint, merge. Use to implement an intent authored by idsd-intent, or when asked to "build the intent", "implement this ICE", "ship NNN-slug". Code-level work; run by a developer, not a non-dev collaborator.
---

Turn an ICE intent into merged code. You are the harness: you assemble Context, write code and tests, and run validation. The human approves **outcomes** at the checkpoint, not necessarily code.

Where this fits: `idsd-charter` (optional) → `idsd-constitution` (optional) → `idsd-intent` → **`idsd-build`**.

Input: an intent file under `.idsd/intents/NNN-<slug>.md`. If unspecified, list the not-yet-built ones (`status: draft` or `approved`) and ask which.

A quick vocabulary (full definitions live in `idsd-intent`):
- **Constraint** — an absolute must-hold, authored in the ICE.
- **Gate** — the executable check you run to verify constraints and baselines.
- **Scenario** — behaviour written as Given / When / Then.

## Phase 1 — Restate & confirm (checkpoint 1)

Read the ICE. Play back in your own words: goal, success/failure scenarios, constraints, any reference data, links. Surface any gap now.

Guard before proceeding:
- `collaborative: true` and `approved-by` empty → stop; the intent needs sign-off first.
- Scenarios or constraints missing/vague → send back to `idsd-intent`, don't guess.
- A `depends-on` link points to an intent that isn't `built` yet → stop; build that one first.

Wait for the human's OK, then set `status: approved`.

## Phase 2 — Assemble Context (progressive)

Pull the "how" as needed, not all up front:
- Read `.idsd/charter.md` if present (the project's what/why) and `.idsd/constitution.md` if present (baseline NFRs and gate commands).
- Read `CLAUDE.md` / `PROJECT_CODE_STYLE.md` if present and follow them.
- Read only the parts of the codebase the intent touches; pull more as work reveals need.

**Resolve gates to commands.** The gate set is:
- baseline checks — build, lint, test, coverage, perf;
- one check per measurable constraint in the ICE.

Get the commands from the constitution. If there is none, discover them from repo tooling — `package.json` / `Makefile` / `pyproject` scripts, lint and test config, CI workflow. If the repo has no tooling either (greenfield), fall back to the stack's conventional commands. State each command before you run it.

A command that *can't run* (missing tool or target) is a **stale gate**, not a failing check — flag and fix it (see *Keep long-term memory honest*); don't mistake it for a code failure and thrash. A command that *runs and fails* is a real red gate → fix the code.

A constraint that can't become a command (e.g. "GDPR compliant") isn't a gate — flag it for human judgment at the checkpoint.

## Phase 3 — Implement & validate (bounded loop)

1. Implement the smallest change that satisfies the goal within the constraints.
2. Encode success/failure scenarios as real acceptance tests (unit/integration/e2e per stack); for runtime/UI behaviour that resists a unit test, drive the app directly — via the `verify` skill if your setup has one, otherwise an e2e test or a manual run. Scenarios are examples, not the whole contract — also cover every constraint no scenario exercises (each supported value, threshold, and edge branch). Extend hand-written tests; don't clobber them.
3. Run the gates and the scenario tests.
4. On failure, fix and re-run. Bound to a few iterations; if stuck, stop and report rather than thrash.

Stay within constraints and links — don't expand into neighbouring intents.

## Phase 4 — Checkpoint (the 70–90% gate)

Present for human judgment:
- Diff summary — what changed conceptually, not a line dump.
- **Gate results** — gates are absolute; a red gate blocks merge (fix or escalate).
- **Scenario results** — pass/fail; the human approves the behaviour.

Approve on outcomes → proceed. Reject with feedback → back to Phase 3. Code review is optional and the human's — offer a code review (the `code-review` skill, if your setup has one); never force or silently skip it.

## Phase 5 — Merge & archive

Once approved, land the change via your normal git flow (which asks before commit/push). Then set `status: built`, move the file to `.idsd/archive/NNN-<slug>.md`, and regenerate `.idsd/roadmap.md` if it exists (preserve its layout).

## Keep long-term memory honest

While building, surface drift — propose, never auto-edit; confirmed changes land via `idsd-constitution`:

- The constitution's gate commands no longer match the repo → flag them as stale and propose the fix.
- A gate or constraint you keep re-deriving that the constitution doesn't capture → propose adding it.

## Rules

- Outcomes over instructions: no single prescribed implementation — satisfy the intent, honour the constraints.
- Never relax a constraint or edit a scenario to make validation pass. If the intent is wrong, send it back to `idsd-intent`.
- One intent at a time. If work reveals a missing intent, note it; don't silently absorb it.
