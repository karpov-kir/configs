---
name: idsd-audit
description: Audit the whole IDSD intent set for cross-intent consistency — links & build order, milestones, charter scope, constitution coverage, duplication, well-formedness. Read-only: reports findings and routes each fix to its owning skill. Use on a maturing project or before a build round; triggers on "idsd-audit", "audit the intents", "check the intent set", "is the plan consistent".
---

A read-only sweep over the whole `.idsd/` set, checking the invariants that only surface **across** intents — `idsd-intent` enforces these while authoring one; this runs them over the whole set. It writes nothing: every fix is proposed and routed to the skill that owns the file.

Where this fits: `idsd-charter` / `idsd-constitution` (optional) → `idsd-intent` → **`idsd-audit`** (optional) → `idsd-build`.

## Phase 1 — Load the set

Read everything under `.idsd/`: active intents (`intents/`), built ones (`archive/`), `charter.md`, `constitution.md`, `roadmap.md`. Parse each intent's frontmatter (`title`, `milestone`, `status`, `collaborative`, `approved-by`, `links`) and body (goal, constraints, scenarios). No `.idsd/` → say so and stop.

## Phase 2 — Check the invariants

Run every applicable check; skip a dimension only when its inputs are absent (no constitution → skip baseline coverage).

- **Links & build order** — every `links` entry uses a known relation (`extends`/`depends-on`/`blocks`, nothing else) and resolves to a real intent; the `depends-on` graph is acyclic; directions follow `idsd-intent`'s Links rule (extend a built/foundational intent, don't depend backward onto it).
- **Milestone coherence** — no `mvp` intent depends on a `vnext`/unscheduled or still-`draft` one (it could never ship in the MVP); roadmap groupings match each intent's `milestone`.
- **Constitution coverage** — every baseline NFR is enforced by, or at least not contradicted by, the `mvp` set; flag a baseline no `mvp` intent satisfies (ships in standing violation) and any constraint contradicting a baseline.
- **Charter scope** — every active intent sits inside the charter's Scope; flag off-mission intents, in-scope areas no intent covers, and intents orphaned by a past scope cut.
- **Duplication** — overlapping goals/constraints across intents; a constraint recurring in ≥3 intents → propose promoting it to a constitution baseline.
- **Well-formedness** — each intent has a goal with a real *why*, 3–7 constraints, and ≥1 success + ≥1 failure scenario; a goal joined by "and" is either mis-named (one outcome → rename) or two intents (→ split); scenario coverage shouldn't be thin for the intent's surface (a many-path outcome on the bare floor). Flag the gap and route to `idsd-intent`; don't re-grill here.
- **Status hygiene** — `built` intents live in `archive/` (not `intents/`); numbers are unique and contiguous; `collaborative: true` intents missing `approved-by` are flagged before they reach `idsd-build`; `roadmap.md` matches current frontmatter.
- **Follow-up hygiene** — flag any archived intent still carrying an unchecked `- [ ]`, and an active intent's open `- [ ]` item that names an intent which doesn't exist.

## Phase 3 — Report

One report, grouped by severity:

- **Blocker** — breaks a build or ships a violation: link cycle, dangling link, an `mvp` intent that can't ship, an unsatisfied baseline.
- **Fix** — drift to reconcile: off-mission intent, scope gap, duplication, stale roadmap.
- **Nit** — numbering, polish.

Each finding names the file(s), the **owning skill** to fix it through — `idsd-intent` (intents, links, scope sync), `idsd-charter` (vision/scope), `idsd-constitution` (baselines, gate commands) — and the smallest reconciling move.

## Rules

- Read-only — the one IDSD skill that never writes to `.idsd/`; fixes land through the owning skill, on confirmation.
- Work at the **set** level — a single intent's ambiguities are `idsd-intent`'s clarify pass, not this.
- A finding states the invariant broken and where, not a redesign. Stay at audit altitude.
