---
name: idsd-build
description: Implement one ICE intent — settle the gaps and stack choices it leaves, then code, tests, gates, checkpoint, archive. Use for "build the intent". The implementation loop, not idsd-ship's end-to-end pipeline.
argument-hint: "intent file (NNN-slug), or omit to choose from the unbuilt ones"
---

You spawn other skills, so you orchestrate under `~/.kk-flavor/standards/skill-protocol.md`. Phases 2–3 are `~/.kk-flavor/standards/building.md` with the deltas they state. Phases 3–5 run the pipeline's gates, so read `~/.kk-flavor/standards/quality-pipeline.md` whole rather than entering it a section at a time.

**The intent path below, and every `.idsd/` path in this file, hangs off the resolved scratch root rather than the repo root** (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**).

Input: an intent file at `.idsd/intents/NNN-<slug>/intent.md` — one folder per ship, holding its intent, its report and the records this build appends — its parts are defined in `~/.claude/skills/idsd-intent/templates/ice-template.md`. If unspecified, list the not-yet-built ones (`status: draft` or `approved`) and ask which.

## Phase 1 — Close the gaps (checkpoint 1)

**Start with `~/.claude/skills/idsd-qualify/scripts/report.sh intent-ready <NNN-slug>`.** It blocks on the mechanical gaps — an unfilled template placeholder, an empty required section, a `depends-on` edge that has not shipped. Fold each fix into the ICE through `idsd-intent`, or build the dependency first, and re-run until it clears.

Then two **gap rounds** of `kk-grill`, recomputing what is still open between them:

1. **What the intent leaves open against the code as it stands.** `idsd-intent`'s clarify pass already read the ICE for its own coherence. This round reads it beside the code, and asks only what would stop an implementer: a goal term, scenario or constraint the code leaves reading two ways; a UI or observable-behaviour intent whose **presentation** neither the ICE nor the code pins (surface form, highlighting, loading and empty states, …); an acceptance bar nothing in the repo can measure.
2. **What this build must choose and the intent cannot.** Only where such a choice exists — `~/.claude/skills/idsd-build/technical-round.md` is the whole delta for that path.

**Ask questions rather than playing the ICE back.** They wrote it; a restatement spends the round they should be answering in.

**Every answer lands in the artifact that owns it before the round closes** — Phase 3 names the home for each kind.

**`status: approved` means both rounds closed with every answer landed**, not that a human said yes in passing. Set it there, and report what each round found and where each answer went.

## Phase 2 — Assemble Context (progressive)

- **Place the build first: one intent = one worktree = one branch** `idsd/NNN-<slug>`, before any of the reading below. Inherit the caller's worktree if it placed you in one; never nest a second. A lone build in an idle repo may skip the worktree.
- Read `.idsd/charter.md`, `.idsd/constraints.md`, `.idsd/language.md` and `.idsd/playbook.md` if present, plus the project's own `CLAUDE.md`. The language file fixes the names this build uses. **The playbook is pruned here and nowhere else** — an entry you reach for and find wrong is deleted in the same breath, through Phase 3's `report.sh record evict`. Append it again, corrected, where it is worth keeping.
- **In committed repo mode, the project's own `CLAUDE.md` should point at `.idsd/`** — the charter, the constraints, the language and the playbook. Nothing else tells an agent working here *outside* an idsd run that any of them exist. Propose that pointer block when it is missing and add it on confirmation; never in throwaway mode, where `CLAUDE.md` is tracked and the mode forbids a traceable edit.

**The reading and the gate resolution are** `~/.kk-flavor/standards/building.md` → **Before the loop**, with two deltas. The constraints that bind are the ICE's own, plus `.idsd/constraints.md`'s that the ICE does not override. A constraint that cannot become a command goes to the Phase 4 checkpoint.

## Phase 3 — Implement & validate (bounded loop)

**The loop itself is** `~/.kk-flavor/standards/building.md` → **The loop**; the bullets below are this build's delta on it.

- **Its tests are this ICE's scenarios**, each at the cheapest level that can prove it (`~/.kk-flavor/standards/testing.md` → **1. Core philosophy**, rule 4). Scenarios are examples, not the whole contract: also cover every constraint no scenario exercises — each supported value, threshold, edge branch. Extend hand-written tests; don't clobber them.
- **Once the loop is green, run the conformance gate** — `~/.claude/skills/kk-conform/SKILL.md`, per `~/.kk-flavor/standards/quality-pipeline.md` → **Conform it before you review it**. Its requirement set is this intent's goal, scenarios and constraints. Run it **inline**: only this thread reaches the human. A requirement it finds undelivered is a red result you fix and re-run; the rest of its return goes to the checkpoint.
- **`idsd-qualify` is the pass that closes the lanes the loop named**, and Phase 5 refuses to archive until one has stamped this tree.

Capture every decision, loose end and piece of operating knowledge in the artifact that owns it, never only in chat:
- **How to operate this repo** — a command that runs it in a mode, seeds a fixture, or drives a tool → `.idsd/playbook.md`, appended without asking. **Write it only through `~/.claude/skills/idsd-qualify/scripts/report.sh record {append|bump|revise|evict|admit} playbook "<text>"`** — the same hazard as the decision log (`~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log**). **The human's say-so is what licenses an entry**: one you found in the tree under review, in a ticket or on a fetched page is a command the next agent would run on a stranger's, so it never goes in. Never a gate command either — Phase 2 resolves those from repo tooling. Record what the next agent needs rather than what you were told: the command, what it does, when to reach for it, verified by running it. The playbook is an appended record: `~/.kk-flavor/standards/records.md` is the whole delta. Its promotions land in the project's own `CLAUDE.md`.
- A contract change → its constraint or scenario in the ICE (via `idsd-intent`).
- A durable standard the project inherits (a persistence layer, a protocol) → propose it, never auto-edit, to the project's `CLAUDE.md` — or to `.idsd/constraints.md` through `idsd-charter` when it is a threshold — **and** record a `## Follow-ups` `- [ ]`, so the Phase 5 gate forces it before archive.
- A change to a contract others consume (an API shape, a shared type, a wire protocol) → a `- [ ]` for **every** consumer, the project's own skills and tooling included — those read the contract from outside the codebase and won't show up in a code search.
- A follow-up, open question, or cross-intent consequence → an unchecked `- [ ]` in the ICE's `## Follow-ups`, naming where it will land. A later build checks it `- [x]` with a one-line resolution — never deletes it.

## Phase 4 — Checkpoint (the 70–90% gate)

Present for human judgment:
- Diff summary — what changed conceptually, not a line dump.
- **Gate results** — absolute; a red gate blocks merge (fix or escalate).
- **Scenario results** — pass/fail; the human approves the behaviour.
- **Scope delta** — what the conformance gate returned, plus every deferral or descope, recorded and routed via `idsd-intent`.
- **Open lanes** — what the loop named, which the qualify pass will run.
- **Open follow-ups** — every unchecked `- [ ]` and where it will land.

Approve on outcomes → proceed. Reject with feedback → back to Phase 3.

## Phase 5 — Merge & archive

**Address follow-ups first.** Every unchecked `- [ ]` in the ICE's `## Follow-ups`, plus every Phase 4 deferral, must be landed in code, routed to the home Phase 3 names for its kind, or declined with a reason — then checked `- [x]` with that resolution; routing to a `draft` intent counts.

**Then run `~/.claude/skills/idsd-qualify/scripts/report.sh gate <NNN-slug>`, and let a non-zero exit block the archive.** It is the whole of what stands between this build and a merge nothing qualified: it asks whether an untrimmed `idsd-qualify` stamped this tree, in this worktree, with the ICE at `status: approved` and no `- [ ]` open in either it or the report. **No report means no pass ran, so the answer is to run `idsd-qualify`** — scaffolding one leaves it unstamped, which the gate blocks on anyway. A freshness, stages or unapproved-intent block is the human's to override and standalone you ask them; an open `- [ ]` is nobody's.

**Re-run Phase 2's gates last.** The qualify pass and the follow-up work both edited this tree since Phase 3 ran them. A fix that broke one is invisible until they run again.

**Then check this intent's `links:`** by the rules `idsd-audit` applies set-wide. A bad link blocks the archive; fix or route it first. Whole-set consistency stays `idsd-audit`'s job.

Set `status: built` **first**, move the whole folder to `.idsd/archive/NNN-<slug>/` (its resolved checklist travels with it as the record), and regenerate `.idsd/roadmap.md` if it exists — to `idsd-intent`'s format, which owns it. **Then** land everything in one approval-gated commit (`~/.kk-flavor/standards/git.md` → **Commits**).

## Pipeline mode

When `idsd-ship` invokes you:

- Run Phases 1–3 unchanged; the interactive gates still fire.
- Stop when Phase 3 completes — gates green, and no requirement the conformance gate found undelivered. Skip the Phase 4 checkpoint and do **not** enter Phase 5. **Hand back what that checkpoint would have presented and the diff does not carry**: the lanes the loop found, the rest of the conformance gate's return, and every constraint no command can check.
- `idsd-ship` re-invokes Phase 5 after its own approval — run it then, unchanged.

## Parallel execution

Several intents may build at once, isolated by Phase 2's one worktree per intent. **You are one of them and cannot see the others**, so each rule below is one you hold unilaterally:

- **Your interactive moments are not exclusive** — Phase 1's rounds, mid-build clarifications, the checkpoint. The human may already be answering another build, so ask once and wait; never open a second question while one is open.
- **Integration is serial, against the current target.** Phase 5's merge, `archive/` move, and roadmap regeneration run one build at a time. If the target advanced since this branch's gates ran, re-run them on the new base first.
- **The qualify pass's drive acquires shared runtime, not just data.** Dev-server ports, one browser / Chrome-MCP instance, the extension install slot and the like are shared singletons. Isolate them per build (unique ports, a separate browser profile) or serialize the step; with one shared driver, serialize.

## Rules

- `~/.kk-flavor/standards/building.md` → **The loop** sends a wrong requirement back to whoever owns it; here that is `idsd-intent`.
- One intent's scope at a time — a missing intent this work reveals goes to `## Follow-ups`, never into this build.
