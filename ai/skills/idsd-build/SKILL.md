---
name: idsd-build
description: Implement one ICE intent — code, tests, gates, checkpoint, archive. Use for "build the intent". The implementation loop, not idsd-ship's end-to-end pipeline.
argument-hint: "intent file (NNN-slug), or omit to choose from the unbuilt ones"
---

You spawn other skills, so you orchestrate under `~/.kk-flavor/standards/skill-protocol.md`. Phases 3–5 run the pipeline's gates and its drive, so read `~/.kk-flavor/standards/quality-pipeline.md` whole rather than entering it a section at a time.

Input: an intent file under `.idsd/intents/NNN-<slug>.md` — its parts are defined in `~/.claude/skills/idsd-intent/templates/ice-template.md`. If unspecified, list the not-yet-built ones (`status: draft` or `approved`) and ask which.

## Phase 1 — Restate & confirm (checkpoint 1)

Read the ICE. Play back in your own words: goal, success/failure scenarios, constraints, reference data, links.

Guard before proceeding:
- `collaborative: true` and `approved-by` empty → stop; the intent needs sign-off first.
- A goal term, scenario, or constraint that's missing, vague, or reads two ways → clarify rather than pick a reading. When the answer changes the contract, fold it into the ICE via `idsd-intent` first — the record is the ICE, not this chat.
- A UI or observable-behaviour intent that doesn't pin its **presentation** (surface form, highlighting, loading and empty states) → clarify and fold the choice into the ICE first.
- A `depends-on` link pointing at an intent that isn't `built` → stop; build that one first.

Wait for the human's OK, then set `status: approved`.

## Phase 2 — Assemble Context (progressive)

- **Place the build first: one intent = one worktree = one branch** `idsd/NNN-<slug>`, before any of the reading below. Inherit the caller's worktree if it placed you in one; never nest a second. A lone build in an idle repo may skip the worktree.
- Read `.idsd/charter.md`, `.idsd/constitution.md`, `.idsd/language.md` and `.idsd/playbook.md` if present, plus the project's own `CLAUDE.md`. The language file fixes the names this build uses. **The playbook is pruned here and nowhere else** — an entry you reach for and find wrong is corrected or deleted in the same breath.
- **In committed repo mode, the project's own `CLAUDE.md` should point at `.idsd/`** — the constitution, the language and the playbook. Nothing else tells an agent working here *outside* an idsd run that any of them exist. Propose that pointer block when it is missing and add it on confirmation; never in throwaway mode, where `CLAUDE.md` is tracked and the mode forbids a traceable edit.
- Read only the parts of the codebase the intent touches; pull more as work reveals need.
- Verify any load-bearing assumption about an existing subsystem in the code, not from its name.

**Resolve gates to commands** — baseline checks (build, lint, test, coverage, perf, …) plus one per measurable constraint in the ICE. Take the commands from the constitution; failing that, from repo tooling (manifest scripts, lint and test config, CI workflow); failing that, the stack's conventional ones. State each before you run it.

**A gate whose green proves nothing is a stale gate** (`~/.kk-flavor/standards/quality-pipeline.md` → **Gates**) — here you fix it rather than route it. A constraint that can't become a command isn't a gate — flag it for human judgment at the checkpoint.

## Phase 3 — Implement & validate (bounded loop)

1. Implement the smallest change that satisfies the goal within the constraints. **Where the change publishes a module surface, settle that surface first** — exports, types, and the contract prose beside them (`~/.kk-flavor/standards/architecture/core.md` → **Module depth**) — then write the body against it. An ordering, not a gate: don't stop and ask.
2. Encode success/failure scenarios as real acceptance tests, each at the cheapest level that can prove it (`~/.kk-flavor/standards/testing.md` → **1. Core philosophy**, rule 4). Scenarios are examples, not the whole contract: also cover every constraint no scenario exercises — each supported value, threshold, edge branch. Cover the non-ASCII / special-character case wherever code lists or round-trips external names. When the deliverable is a mapping, produce the full table (code path → resulting state) and validate every row. Extend hand-written tests; don't clobber them.
3. Run the gates and the scenario tests. On failure, fix and re-run — bounded to a few iterations; if stuck, stop and report rather than thrash.
4. **Drive it**, once the gates and scenario tests are green — `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**, handing `kk-drive` this intent's scenarios. Here a divergence is a red result you fix and re-run, not a stop.
5. Before the checkpoint, **spawn `kk-code-review` over the changed files** rather than reviewing them yourself. Structure and style are `kk-refactor`'s half of that review: spawn it too on any build that wrote code (`~/.kk-flavor/standards/quality-pipeline.md` → **The stages**), **after code-review and never alongside it** (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**).

Re-run the gates yourself after any spawned subagent's edits land. **Then close the lanes this build's own edits opened** — the comments and the prose it wrote included — before the checkpoint (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**).

Capture every decision, loose end and piece of operating knowledge in the artifact that owns it, never only in chat:
- **How to operate this repo** — a command the human hands you that runs it in a mode, seeds a fixture, or drives a tool → `.idsd/playbook.md`, appended without asking. Record what the next agent needs rather than what you were told: the command, what it does, when to reach for it, verified by running it. Gate commands stay the constitution's — point at them. It accumulates across throwaway ships, so it is an appended record: `~/.kk-flavor/standards/records.md` is the whole delta, and **its bound is roughly 40 lines**. Its promotions land in the constitution or the project's own `CLAUDE.md`.
- A contract change → its constraint or scenario in the ICE (via `idsd-intent`); ratification also advances `status: approved` / `approved-by`.
- A durable standard the project inherits (a persistence layer, a protocol, a stale constitution gate command) → propose it to the constitution (never auto-edit) **and** record a `## Follow-ups` `- [ ]`, so the Phase 5 gate forces it before archive.
- A change to a contract others consume (an API shape, a shared type, a wire protocol) → a `- [ ]` for **every** consumer, the project's own skills and tooling included — those read the contract from outside the codebase and won't show up in a code search.
- A follow-up, open question, or cross-intent consequence → an unchecked `- [ ]` in the ICE's `## Follow-ups`, naming where it will land. A later build checks it `- [x]` with a one-line resolution — never deletes it.

## Phase 4 — Checkpoint (the 70–90% gate)

Present for human judgment:
- Diff summary — what changed conceptually, not a line dump.
- **Gate results** — absolute; a red gate blocks merge (fix or escalate).
- **Scenario results** — pass/fail; the human approves the behaviour.
- **Observed outcomes** — the drive's per-scenario results and evidence.
- **Scope delta** — goal and scenarios versus what shipped, every deferral or descope recorded and routed via `idsd-intent`.
- **Open follow-ups** — every unchecked `- [ ]` and where it will land.

Approve on outcomes → proceed. Reject with feedback → back to Phase 3.

## Phase 5 — Merge & archive

**Address follow-ups first.** Every unchecked `- [ ]` in the ICE's `## Follow-ups`, plus every Phase 4 deferral, must be landed in code, routed to a real home (an intent via `idsd-intent`, a constitution proposal), or declined with a reason — then checked `- [x]` with that resolution; routing to a `draft` intent counts. Don't scan by hand: run `~/.claude/skills/idsd-qualify/scripts/todo-gate.sh <this-intent-file>`, and let a non-zero exit block the archive.

**Then check this intent's `links:`** by the rules `idsd-audit` applies set-wide. A bad link blocks the archive; fix or route it first. Whole-set consistency stays `idsd-audit`'s job.

Set `status: built` **first**, move the file to `.idsd/archive/NNN-<slug>.md` (its resolved checklist travels with it as the record), and regenerate `.idsd/roadmap.md` if it exists — to `idsd-intent`'s format, which owns it. **Then** land everything in one approval-gated commit (`~/.kk-flavor/standards/git.md` → **Commits**).

## Pipeline mode

When `idsd-ship` invokes you:

- Run Phases 1–3 unchanged; the interactive gates still fire.
- Skip Phase 3's step 5 and the lane closing after it — `idsd-ship` owns both; name the lanes your edits opened in your return instead.
- Stop when Phase 3 completes — gates green, the drive passed (its evidence is what `idsd-ship` presents as observed outcomes): skip the Phase 4 checkpoint and do **not** enter Phase 5. Hand control back.
- `idsd-ship` re-invokes Phase 5 after its own approval — run it then, unchanged.

## Parallel execution

Several intents may build at once, isolated by Phase 2's one worktree per intent. **You are one of them and cannot see the others**, so each rule below is one you hold unilaterally:

- **Your interactive moments are not exclusive** — Phase 1 confirm, mid-build clarifications, the checkpoint. The human may already be answering another build, so ask once and wait; never open a second question while one is open.
- **Integration is serial, against the current target.** Phase 5's merge, `archive/` move, and roadmap regeneration run one build at a time. If the target advanced since this branch's gates ran, re-run them on the new base first.
- **The drive acquires shared runtime, not just data.** Dev-server ports, one browser / Chrome-MCP instance, the extension install slot and the like are shared singletons. Isolate them per build (unique ports, a separate browser profile) or serialize the step; with one shared driver, serialize.

## Rules

- Never relax a constraint or edit a scenario to make validation pass. If the intent is wrong, send it back to `idsd-intent`.
- One intent's scope at a time — a missing intent this work reveals goes to `## Follow-ups`, never into this build.
