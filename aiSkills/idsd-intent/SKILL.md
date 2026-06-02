---
name: idsd-intent
description: Author or refine an ICE intent (Intent-Driven Software Development). Grills adaptively, then emits intent files under .idsd/intents/. Use for a new feature, a project plan/roadmap, refining an existing intent, or pairing with a non-developer. Triggers on "intent", "ICE", "IDSD", "plan a feature/project", "what should we build".
---

Capture **what** to build and **why** as an ICE — never **how**. The intent is the durable, reviewable unit of work; implementation belongs to `idsd-build`.

**ICE = Intent · Context · Expectations.** You author Intent and Expectations; `idsd-build` assembles Context at build time. The intent file holds these parts:

- **Goal** — one-sentence outcome, with a one-line *why it matters* (value or cost of inaction).
- **Constraints** — absolute must-hold qualities/thresholds in business language ("p99 < 200ms", "bundle < 200kb", "WCAG 2.1 AA"); violation = failure regardless of execution. Project-wide ones are inherited from the constitution — author only the intent-specific ones.
- **Success / Failure scenarios** — behaviour as Given/When/Then.
- **Links** — neighbouring intents (extends / depends-on / blocks).

A **Gate** — the executable check that verifies constraints plus baselines (build/lint/coverage) — is resolved and run by `idsd-build`, never authored here.

## Phase 0 — Detect scope

Pick scope from the request, not repo state:
- **Feature** — one ticket / one outcome → one ICE (rarely a small handful).
- **Project** — "plan the project", "map the MVP", "multiple features" → a map of linked ICEs, each tagged by `milestone`.

At project scope:
- Read `.idsd/charter.md` if present, to ground decomposition; if absent, offer once to run `idsd-charter` — never force it.
- If `.idsd/constitution.md` is absent, offer once to run `idsd-constitution` — never force it.

If refining, read the named intent file and grill only the gaps.

Where this fits: `idsd-charter` (optional) → `idsd-constitution` (optional) → **`idsd-intent`** → `idsd-build`.

## Phase 1 — Grill (adaptive, one question at a time)

Interview like `grill-me`: one question at a time, each with your recommended answer. Ask the fewest that lock the ICE, scaled to complexity — a tiny feature may need one or two, a project map many more. Skip anything answerable by reading the codebase; read instead. Stop when the goal, constraints, and scenarios are concrete and no open question could still change them.

Cover only what's unclear, in order — the heuristic per part:

1. **Goal** — could two different implementations satisfy it? No "and"; if it needs one, split into two intents (horizontal decomposition: more intents, not deeper).
   - **Why it matters** — the value it delivers or the cost of inaction. Test: if it just rephrases the goal or describes the current situation, it isn't a why — push for the stakes ("…or else what?").
2. **Constraints** — 3–7; prefer measurable ones, since `idsd-build` gates those automatically. Each must constrain THIS outcome, not how another component consumes its output (that belongs in their intent). Exact thresholds are constraints; an explicit list or table the outcome must encode goes in the optional Reference data section.
3. **Success scenarios** — the behaviour that proves it works.
4. **Failure scenarios** — behaviour that must not happen; intent-specific binary limits go here (e.g. "rejects payloads > 1MB").
5. **Links** — what this intent extends, depends on, or blocks.

For a **project map**, also decompose into one ICE per independently-shippable slice and tag each `milestone` (`mvp`, `vnext`, …); parked vNext intents are real files at `status: draft`.

When pairing with a non-developer, the colleague is domain authority over **Goal** and **scenarios** — keep that language plain. You own constraints; Goal is co-authored. Record their sign-off in `approved-by` when they approve.

## Phase 2 — Clarify pass

Before writing, re-read the drafted ICE and surface only the highest-impact ambiguities (≤3 questions). Re-runnable later on an existing file.

## Phase 3 — Emit

Confirm slug(s) + path(s) once, then write. Slug = kebab-case, ≤4 words. Number = highest existing `NNN` across `.idsd/intents/` and `.idsd/archive/`, plus one (zero-padded to 3).

Write each ICE to `.idsd/intents/NNN-<slug>.md` from `templates/ice-template.md` at `status: draft`. Set `collaborative: true` only when authored in a pair session (this activates `idsd-build`'s sign-off gate).

If `.idsd/roadmap.md` exists, or scope is project, (re)generate it from every intent's frontmatter (active + archived), grouped under a heading per milestone, with columns: number, title, status. Intents with `milestone: none` group under an "Unscheduled" heading. Generated, never hand-edited.

If `.idsd/charter.md` exists, keep its **Scope** in sync: when this planning adds or defers intents, or an intent falls outside the current scope, propose a Scope update — confirm it, and never rewrite vision or problem (those change only via `idsd-charter`). If there's no charter, don't create one here.

## Keep long-term memory honest

While authoring, watch for drift and recurrence and surface it — propose, never auto-edit; confirmed changes land via `idsd-charter` / `idsd-constitution`:

- A constraint contradicts a constitution baseline → flag it; don't let both stand.
- The same constraint recurs across three or more intents → propose promoting it to a constitution baseline, so future intents inherit it instead of restating it.
- A domain term keeps recurring → propose adding it to the charter's vocabulary.

## Rules

- Never write code or name implementation (files, classes, libraries) — that's a spec, not an intent.
- Keep each ICE self-contained: declare every dependency in Links, none hidden.
- If the user says "just write it", cut to Phase 3 with current info.
- Don't restate `CLAUDE.md` / `PROJECT_CODE_STYLE.md`; they're Context for `idsd-build`.
