---
name: idsd-intent
description: Author or refine an ICE intent (Intent-Driven Software Development). Grills adaptively, then emits intent files under .idsd/intents/. Use for a new feature, a project plan/roadmap, refining an existing intent, or pairing with a non-developer. Triggers on "intent", "ICE", "IDSD", "plan a feature/project", "what should we build".
argument-hint: "feature/project to plan, or an existing intent to refine"
---

Capture **what** to build and **why** as an ICE — never **how**; implementation belongs to `idsd-build`.

**ICE = Intent · Context · Expectations.** You author Intent and Expectations; `idsd-build` assembles Context at build time. The intent file holds these parts:

- **Goal** — one-sentence outcome, with a one-line *why it matters* (value or cost of inaction).
- **Constraints** — absolute must-hold qualities/thresholds in business language ("p99 < 200ms", "bundle < 200kb", "WCAG 2.1 AA"); violation = failure regardless of execution. Project-wide ones are inherited from the constitution — author only the intent-specific ones.
- **Success / Failure scenarios** — behaviour as Given/When/Then.
- **Links** — neighbouring intents (extends / depends-on / blocks), declared in the frontmatter `links:`.

A **Gate** — the executable check that verifies constraints plus baselines (build/lint/coverage) — is resolved and run by `idsd-build`, never authored here.

A `## Follow-ups` section may also appear — build-time bookkeeping (open questions, cross-intent consequences surfaced during `idsd-build`), not contract; preserve it when refining. It's a checklist — see the template for the format.

**Goal** and **scenarios** are the plain-language contract (shapeable by a non-dev collaborator); **constraints** are the technical must-holds.

**Authoring gate.** Re-reading the assembled set surfaces cross-cutting gaps a one-at-a-time grill structurally can't — so Phase 2 (the clarify pass) is a required checkpoint, not optional polish, run even when the grill felt thorough. It gates Phase 3 (below).

## Phase 0 — Detect scope

Pick scope from the request, not repo state:
- **Feature** — one ticket / one outcome → one ICE (rarely a small handful).
- **Project** — "plan the project", "map the MVP", "multiple features" → a map of linked ICEs, each tagged by `milestone`.

At project scope:
- Read `.idsd/charter.md` if present, to ground decomposition; if absent, offer once to run `idsd-charter` — never force it.
- If `.idsd/constitution.md` is absent, offer once to run `idsd-constitution` — never force it.

If refining, read the named intent file and grill only the gaps.

Where this fits: `idsd-charter` (optional) → `idsd-constitution` (optional) → **`idsd-intent`** → `idsd-audit` (optional) → `idsd-build`.

## Phase 1 — Grill (adaptive, one question at a time)

Grill like `grill-me`: one question at a time, each with your recommended answer. Ask the fewest that lock the ICE, scaled to complexity — a tiny feature may need one or two, a project map many more. Skip anything answerable by reading the codebase; read instead. Stop when the goal, constraints, and scenarios are concrete and no open question could still change them.

Cover only what's unclear, in order — the heuristic per part:

1. **Goal** — one outcome. Could two different implementations satisfy it? An "and" is a smell, not an automatic split: if both halves are facets of one outcome, name that outcome; if they're independently shippable, split (horizontal decomposition — more intents, not deeper).
   - **Why it matters** — the value it delivers or the cost of inaction. Test: if it just rephrases the goal or describes the current situation, it isn't a why — push for the stakes ("…or else what?").
   - **Outcome breadth** — when the goal's outcome word is broader than the symptom or fix the request names ("warmed up" vs "powered on"), pin down which outcome is actually wanted before approving. The broader reading usually implies a different contract, and resolving it after the build is the costly rework — settle it here, in outcome terms, never by reaching for a mechanism.
2. **Constraints** — 3–7; prefer measurable ones, since `idsd-build` gates those automatically. Each must constrain THIS outcome, not how another component consumes its output (that belongs in their intent). Exact thresholds are constraints; an explicit list or table the outcome must encode goes in the optional Reference data section.
3. **Success scenarios** — the behaviour that proves it works. One is the floor; add one per distinct behaviour or path worth proving — never pad with near-duplicates.
4. **Failure scenarios** — behaviour that must not happen; intent-specific binary limits go here (e.g. "rejects payloads > 1MB"). Same scaling, per distinct failure mode.
5. **Links** — what this intent extends, depends on, or blocks; in the frontmatter `links:`, one edge per line, why inline only when not obvious.

For a **project map**, also decompose into one ICE per independently-shippable slice and tag each `milestone` (`mvp`, `vnext`, …); parked vNext intents are real files at `status: draft`.

## Phase 2 — Clarify pass (gate)

Re-read the assembled draft as a whole — the one view the one-at-a-time grill can't give you. Surface every residual ambiguity that would change what gets written — across every part of the ICE, not only the highest-impact one. Scale to what's left: a tidy feature often yields nothing. No cap. Fold each answer into the part it refines — no separate log; the concrete ICE is the record. Re-runnable later on an existing file.

Emit one outcome line as the gate's evidence: either the residual ambiguities found (and where each was folded), or the verbatim `Clarify pass: no residual ambiguities`.

## Phase 3 — Emit

**Precondition (Phase 2 gate):** write no file until Phase 2's outcome line is emitted; if it isn't, run Phase 2 first.

Confirm slug(s) + path(s) once, then write. Slug = kebab-case, ≤5 words. Number = highest existing `NNN` across `.idsd/intents/` and `.idsd/archive/`, plus one (zero-padded to 3).

Write each ICE to `.idsd/intents/NNN-<slug>.md` from `templates/ice-template.md` at `status: draft`. Set `collaborative: true` only when authored in a pair session (this activates `idsd-build`'s sign-off gate); record the collaborator's sign-off in `approved-by` when they approve.

If `.idsd/roadmap.md` exists, or scope is project, (re)generate it from every intent's frontmatter (active + archived), grouped under a heading per milestone, with columns: number, title, status. Intents with `milestone: none` group under an "Unscheduled" heading. Generated, never hand-edited.

If `.idsd/charter.md` exists, keep its **Scope** in sync: when this planning adds or defers intents, or an intent falls outside the current scope, propose a Scope update — confirm it, and never rewrite vision or problem (those change only via `idsd-charter`). If there's no charter, don't create one here.

## Keep long-term memory honest

While authoring, watch for drift and recurrence and surface it — propose, never auto-edit; confirmed changes land via `idsd-charter` / `idsd-constitution`:

- A constraint contradicts a constitution baseline → flag it; don't let both stand.
- A constitution baseline that no `mvp` intent will satisfy (e.g. an auth baseline with auth parked in vNext) → flag it; the product would ship in standing violation. Pull an enforcing intent into the mvp, or mark the baseline deferred.
- The same constraint recurs across three or more intents → propose promoting it to a constitution baseline; on promotion, strip the now-redundant restatement from the contributing intents too (keep only what's genuinely intent-specific), so it lives in exactly one place.
- A domain term keeps recurring → propose adding it to the charter's vocabulary.

## Rules

- Never write code or name implementation (files, classes, libraries) — that's a spec, not an intent.
- Keep each ICE self-contained: declare every dependency in the frontmatter `links:`, none hidden — and keep it consistent with build order: never `block` or `depend-on` an intent that is foundational to this one or already built (that's backwards). A later intent that adds a constraint to a shipped one `extends` it.
- If the user says "just write it", collapse Phases 1–2 to the fastest pass that still emits the Phase 2 outcome line, then Phase 3 — the gate fires even on the fast path.
- Don't restate `CLAUDE.md`; it's Context for `idsd-build`.
