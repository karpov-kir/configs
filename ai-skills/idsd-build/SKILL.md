---
name: idsd-build
description: Run the IDSD build loop against an ICE intent — restate, gather context, implement, validate against scenarios and gates, checkpoint, merge. Use to implement an intent authored by idsd-intent, or when asked to "build the intent" or "implement this ICE". Code-level work; run by a developer, not a non-dev collaborator.
---

Turn an ICE intent into merged code. You are the harness: you assemble Context, write code and tests, and run validation. The human approves **outcomes** at the checkpoint, not necessarily code.

Where this fits: `idsd-charter` (optional) → `idsd-constitution` (optional) → `idsd-intent` → `idsd-audit` (optional) → **`idsd-build`**.

Input: an intent file under `.idsd/intents/NNN-<slug>.md`. If unspecified, list the not-yet-built ones (`status: draft` or `approved`) and ask which.

A quick vocabulary (full definitions live in `idsd-intent`):
- **Constraint** — an absolute must-hold, authored in the ICE.
- **Gate** — the executable check you run to verify constraints and baselines.
- **Scenario** — behaviour written as Given / When / Then.

## Phase 1 — Restate & confirm (checkpoint 1)

Read the ICE. Play back in your own words: goal, success/failure scenarios, constraints, any reference data, links. Surface any gap now.

Guard before proceeding:
- `collaborative: true` and `approved-by` empty → stop; the intent needs sign-off first.
- A goal term, scenario, or constraint that's missing, vague, or reads two ways → clarify with the human rather than pick a reading and proceed. When the answer changes the contract (a constraint, scenario, or scope), fold it into the ICE via `idsd-intent` before building — the record is the ICE, not this chat.
- A `depends-on` link points to an intent that isn't `built` yet → stop; build that one first.

Wait for the human's OK, then set `status: approved`.

## Phase 2 — Assemble Context (progressive)

Pull the "how" as needed, not all up front:
- Read `.idsd/charter.md` if present (the project's what/why) and `.idsd/constitution.md` if present (baseline NFRs and gate commands).
- Read `CLAUDE.md` if present and follow it.
- Read only the parts of the codebase the intent touches; pull more as work reveals need.

**Resolve gates to commands.** The gate set is:
- baseline checks — build, lint, test, coverage, perf;
- one check per measurable constraint in the ICE.

Get the commands from the constitution. If there is none, discover them from repo tooling — manifest scripts (`package.json`, `Makefile`, `pyproject`, …), lint and test config, CI workflow. If the repo has no tooling either (greenfield), fall back to the stack's conventional commands. State each command before you run it.

A command that *can't run* (missing tool or target), or that *runs but can't fail* (wrong target, no assertion, no server started — green no matter what), is a **stale gate**, not verification — flag and fix it (see *Keep long-term memory honest*); never read its green as a pass. A command that *runs and genuinely fails* is a real red gate → fix the code.

A constraint that can't become a command (e.g. "GDPR compliant") isn't a gate — flag it for human judgment at the checkpoint.

## Phase 3 — Implement & validate (bounded loop)

1. Implement the smallest change that satisfies the goal within the constraints.
2. Encode success/failure scenarios as real acceptance tests (unit/integration/e2e per stack); for runtime/UI behaviour that resists a unit test, drive the app directly — via the `verify` skill if your setup has one, otherwise an e2e test or a manual run. Scenarios are examples, not the whole contract — also cover every constraint no scenario exercises (each supported value, threshold, and edge branch). Extend hand-written tests; don't clobber them.
3. Run the gates and the scenario tests.
4. On failure, fix and re-run. Bound to a few iterations; if stuck, stop and report rather than thrash.
5. Before the checkpoint, self-review the changed files against CLAUDE.md and the standards it points to — passing gates don't prove the code follows the conventions.

Capture every decision and loose end in the artifact that owns it, never only in chat:
- A decision that changes the contract → its constraint or scenario in the ICE (via `idsd-intent`).
- A decision that sets a durable standard the whole project inherits (a persistence layer, a protocol) → propose it to the constitution **and** record a `## Follow-ups` `- [ ]` item (home: the constitution change), so the Phase 5 gate forces it before archive.
- A follow-up, open question, or cross-intent consequence → the ICE's `## Follow-ups` checklist now, an unchecked `- [ ]` item naming where it will land (an intent to create or update, a constitution change). Addressing one in a later build checks it `- [x]` with a one-line resolution — never delete it.

Stay within constraints and links — don't expand into neighbouring intents; when you notice one, it's a follow-up, not this build's work.

## Phase 4 — Checkpoint (the 70–90% gate)

Present for human judgment:
- Diff summary — what changed conceptually, not a line dump.
- **Gate results** — gates are absolute; a red gate blocks merge (fix or escalate).
- **Scenario results** — pass/fail; the human approves the behaviour.
- **Observed outcomes** — drive the real running system and watch each scenario actually happen; a green gate can be vacuous (see *stale gate*, Phase 2), so confirm by observation, never present green as proof on its own.
- **Scope delta** — what the goal and scenarios named versus what shipped: everything delivered, or **each deferral/descope recorded** (routed to the owning intent or a new one via `idsd-intent`). No unrecorded gap reaches merge.
- **Open follow-ups** — the ICE's `## Follow-ups`: every unchecked `- [ ]` item and where it will land.

Approve on outcomes → proceed. Reject with feedback → back to Phase 3.

## Phase 5 — Merge & archive

**Address follow-ups first.** Every unchecked `- [ ]` item in the ICE's `## Follow-ups` — and every scope-delta deferral from Phase 4 — must be addressed: routed to a real home (an intent created or updated via `idsd-intent`, a constitution proposal), landed in code, or declined with a reason. *Addressed* means the item is checked `- [x]` with its one-line resolution; routing to a `draft` intent counts (the work need not be finished). Any unchecked follow-up blocks the archive.

**Then check this intent's links.** Before landing, validate the merging intent's `links:` by the rules `idsd-audit` applies set-wide, scoped to this one — relation known, target resolves, no `depends-on` cycle. A bad link blocks the archive — fix it (or route via `idsd-intent`) first. Whole-set consistency stays `idsd-audit`'s pre-build-round job, not re-run here.

Once approved and every follow-up is checked, land the change via your normal git flow (which asks before commit/push). Then set `status: built`, move the file to `.idsd/archive/NNN-<slug>.md` (its resolved checklist travels with it as the record), and regenerate `.idsd/roadmap.md` if it exists (preserve its layout).

## Keep long-term memory honest

While building, surface drift — propose, never auto-edit; confirmed changes land via `idsd-constitution`. Each proposal here also gets a `## Follow-ups` `- [ ]` item, same as any durable-standard proposal — gated at Phase 5:

- The constitution's gate commands no longer match the repo → flag them as stale and propose the fix.
- A gate or constraint you keep re-deriving that the constitution doesn't capture → propose adding it.

## Pipeline mode

When invoked by `idsd-ship` (not standalone), the boundary shifts — `idsd-ship` owns review, refactor, and final approval:

- Run Phases 1–3 unchanged — the interactive gates (restate/confirm, clarify, gate resolution) still fire; never suppress them.
- Skip Phase 3's self-review step — the dedicated `/code-review` pass replaces it.
- Stop after Phase 3's gates are green: skip the Phase 4 checkpoint and do **not** enter Phase 5. Hand control back to `idsd-ship`.
- `idsd-ship` re-invokes Phase 5 (merge & archive) after its own approval — run it then, unchanged.

## Rules

- Outcomes over instructions: no single prescribed implementation — satisfy the intent, honour the constraints.
- Never relax a constraint or edit a scenario to make validation pass. If the intent is wrong, send it back to `idsd-intent`.
- One intent at a time. If work reveals a missing intent — or defers or descopes any part of THIS intent's goal or scenarios — record it in `## Follow-ups` and route it before archive (Phase 5), never silently absorb or drop it.
