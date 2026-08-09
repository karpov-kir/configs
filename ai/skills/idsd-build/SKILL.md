---
name: idsd-build
description: Implement one ICE intent from .idsd/intents/ — code, tests, gates, checkpoint, archive. Use for "build the intent". The implementation loop, not idsd-ship's end-to-end pipeline.
argument-hint: "intent file (NNN-slug), or omit to choose from the unbuilt ones"
---

Turn an ICE intent into merged code: assemble Context, write code and tests, run validation. The human approves **outcomes** at the checkpoint, not code.

Input: an intent file under `.idsd/intents/NNN-<slug>.md` — its parts are defined in [ice-template.md](../idsd-intent/templates/ice-template.md). If unspecified, list the not-yet-built ones (`status: draft` or `approved`) and ask which.

## Phase 1 — Restate & confirm (checkpoint 1)

Read the ICE. Play back in your own words: goal, success/failure scenarios, constraints, reference data, links. Surface any gap now.

Guard before proceeding:
- `collaborative: true` and `approved-by` empty → stop; the intent needs sign-off first.
- A goal term, scenario, or constraint that's missing, vague, or reads two ways → clarify rather than pick a reading. When the answer changes the contract, fold it into the ICE via `idsd-intent` first — the record is the ICE, not this chat.
- A UI or observable-behaviour intent that doesn't pin its **presentation** (surface form, highlighting, loading and empty states) → clarify and fold the choice into the ICE first.
- A `depends-on` link pointing at an intent that isn't `built` → stop; build that one first.

Wait for the human's OK, then set `status: approved`.

## Phase 2 — Assemble Context (progressive)

- **Place the build first: one intent = one worktree = one branch** `idsd/NNN-<slug>`, before any of the reading below. Inherit the caller's worktree if it placed you in one; never nest a second. A lone build in an idle repo may skip the worktree.
- Read `.idsd/charter.md`, `.idsd/constitution.md`, `.idsd/language.md` and `.idsd/playbook.md` if present, plus the project's own `CLAUDE.md`. The language file fixes the names this build uses; the playbook is how this repo is operated, and reading it is what stops you rediscovering a command the human already handed over once. **The playbook is pruned here and nowhere else** — nothing audits it, so an entry you reach for and find wrong is corrected or deleted in the same breath, and one whose subject this build removed goes with it. A playbook nobody prunes teaches the next agent a command that stopped working months ago.
- **In committed repo mode, the project's own `CLAUDE.md` should point at `.idsd/`** — the constitution, the language and the playbook. Nothing else tells an agent working here *outside* an idsd run that any of them exist. Propose that pointer block when it is missing and add it on confirmation; never in throwaway mode, where `CLAUDE.md` is tracked and the mode forbids a traceable edit.
- Read only the parts of the codebase the intent touches; pull more as work reveals need.
- Verify any load-bearing assumption about an existing subsystem in the code, not from its name.

**Resolve gates to commands** — baseline checks (build, lint, test, coverage, perf, …) plus one per measurable constraint in the ICE. Take the commands from the constitution; failing that, from repo tooling (manifest scripts, lint and test config, CI workflow); failing that, the stack's conventional ones. State each before you run it.

A command that *can't run*, that *runs but can't fail* (`idsd-constitution`'s real / runnable / able-to-fail test), or that runs and can fail but *never reads the changed code*, is a **stale gate**, not verification. Fix it, and never read its green as a pass. Read what CI actually invokes, never its stage names. One that genuinely fails is a real red gate → fix the code. A constraint that can't become a command isn't a gate — flag it for human judgment at the checkpoint.

## Phase 3 — Implement & validate (bounded loop)

1. Implement the smallest change that satisfies the goal within the constraints. **Where the change publishes a module surface, settle that surface first** — exports, types, and the contract prose beside them (`~/.kk-flavor/standards/architecture/core.md` → Module depth) — then write the body against it. An ordering, not a gate: don't stop and ask.
2. Encode success/failure scenarios as real acceptance tests; for runtime/UI behaviour that resists a unit test, drive the app directly (a project `verify` skill if there is one, an e2e test, or a manual run). Scenarios are examples, not the whole contract: also cover every constraint no scenario exercises (each supported value, threshold, edge branch), and the non-ASCII / special-character case wherever code lists or round-trips external names. When the deliverable is a mapping, produce the full table (code path → resulting state) and validate every row. Extend hand-written tests; don't clobber them.
3. Run the gates and the scenario tests. On failure, fix and re-run — bounded to a few iterations; if stuck, stop and report rather than thrash.
4. **Exercise it end-to-end, black-box**, wherever the change has observable behaviour. Once gates and scenario tests are green, **spawn a general-purpose subagent** with only the intent's scenarios and how to run the project — **withhold the diff**, so it verifies against the spec, not the implementation. It drives the real path, reports each scenario's observed outcome with evidence, and tears down; a divergence is a red result — fix and re-run. **Drive against a disposable seeded fixture, never live project content**, and for UI or layout behaviour make that fixture representative, not minimal — a toy one renders fine while hiding the overflow that real input triggers. No runnable entrypoint yet is not grounds to skip: a throwaway harness (composition root, built assets) is the expected way, removed afterwards.
5. Before the checkpoint, **spawn `kk-code-review` over the changed files** rather than reviewing them yourself — passing gates don't prove the code is correct. Structure and style are `kk-refactor`'s half of that review: spawn it too where this build moved either. **After code-review, never alongside it** — refactor rewrites the identifiers a concurrent review is still reading (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**).

Re-run the gates yourself after any spawned subagent's edits land.

Capture every decision, loose end and piece of operating knowledge in the artifact that owns it, never only in chat:
- **How to operate this repo** — a command the human hands you that runs it in a mode, seeds a fixture, or drives a tool → `.idsd/playbook.md`, appended without asking. Record what the next agent needs rather than what you were told: the command, what it does, when to reach for it, verified by running it. Gate commands stay the constitution's — point at them. It is agent-maintained where the constitution is curated, and it accumulates across throwaway ships.
- A contract change → its constraint or scenario in the ICE (via `idsd-intent`); ratification also advances `status: approved` / `approved-by`.
- A durable standard the project inherits (a persistence layer, a protocol, a stale constitution gate command) → propose it to the constitution (never auto-edit) **and** record a `## Follow-ups` `- [ ]`, so the Phase 5 gate forces it before archive.
- A change to a contract others consume (an API shape, a shared type, a wire protocol) → a `- [ ]` for **every** consumer, the project's own skills and tooling included — those read the contract from outside the codebase and won't show up in a code search.
- A follow-up, open question, or cross-intent consequence → an unchecked `- [ ]` in the ICE's `## Follow-ups`, naming where it will land. A later build checks it `- [x]` with a one-line resolution — never deletes it.

## Phase 4 — Checkpoint (the 70–90% gate)

Present for human judgment:
- Diff summary — what changed conceptually, not a line dump.
- **Gate results** — absolute; a red gate blocks merge (fix or escalate).
- **Scenario results** — pass/fail; the human approves the behaviour.
- **Observed outcomes** — the end-to-end run's per-scenario results and evidence; a green gate can be vacuous, so never present one as proof on its own.
- **Scope delta** — goal and scenarios versus what shipped, every deferral or descope recorded and routed via `idsd-intent`.
- **Open follow-ups** — every unchecked `- [ ]` and where it will land.

Approve on outcomes → proceed. Reject with feedback → back to Phase 3.

## Phase 5 — Merge & archive

**Address follow-ups first.** Every unchecked `- [ ]` in the ICE's `## Follow-ups`, plus every Phase 4 deferral, must be landed in code, routed to a real home (an intent via `idsd-intent`, a constitution proposal), or declined with a reason — then checked `- [x]` with that resolution; routing to a `draft` intent counts. Don't scan by hand: run `idsd-qualify`'s `scripts/todo-gate.sh <this-intent-file>`, and let a non-zero exit block the archive.

**Then check this intent's `links:`** by the rules `idsd-audit` applies set-wide. A bad link blocks the archive; fix or route it first. Whole-set consistency stays `idsd-audit`'s job.

Set `status: built` **first**, move the file to `.idsd/archive/NNN-<slug>.md` (its resolved checklist travels with it as the record), and regenerate `.idsd/roadmap.md` if it exists — to `idsd-intent`'s format, which owns it. **Then** land everything in one approval-gated commit (`~/.kk-flavor/standards/git.md` → Commits). Committing first leaves the archive move, the status flip and the roadmap uncommitted, with no step that ever commits them.

## Pipeline mode

When `idsd-ship` invokes you, it owns review, refactor, and final approval:

- Run Phases 1–3 unchanged; the interactive gates still fire.
- Skip Phase 3's step 5 — the pipeline's own `kk-code-review` and `kk-refactor` stages replace it.
- Stop when Phase 3 completes — gates green, end-to-end check passed (its evidence is what `idsd-ship` presents as observed outcomes): skip the Phase 4 checkpoint and do **not** enter Phase 5. Hand control back.
- `idsd-ship` re-invokes Phase 5 after its own approval — run it then, unchanged.

## Parallel execution

Several intents may build at once; isolation, not new coordination, makes that safe — Phase 2's one-worktree-per-intent rule is that isolation.

- **The human is one serialized queue; autonomous work overlaps.** Attend interactive moments — Phase 1 confirm, mid-build clarifications, the checkpoint — one build at a time, never N live dialogues. A build that reaches one while you're busy pauses with `blocked: <what it needs>` rather than guessing.
- **Integration is serial, against the current target.** Phase 5's merge, `archive/` move, and roadmap regeneration run one build at a time. If the target advanced since this branch's gates ran, re-run them on the new base first.
- **The end-to-end run acquires shared runtime, not just data.** Dev-server ports, one browser / Chrome-MCP instance, the extension install slot and the like are shared singletons. Isolate them per build (unique ports, a separate browser profile) or serialize the step; with one shared driver, serialize.

## Rules

- Never relax a constraint or edit a scenario to make validation pass. If the intent is wrong, send it back to `idsd-intent`.
- One intent at a time — a missing intent this work reveals, or any deferral or descope of THIS intent's goal or scenarios, goes to `## Follow-ups` and is routed before archive, never silently absorbed.
