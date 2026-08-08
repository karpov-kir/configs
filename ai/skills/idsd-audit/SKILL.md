---
name: idsd-audit
description: Audit the whole .idsd/ intent set for cross-intent consistency — links, build order, milestones, charter scope. Read-only — routes each fix to its owning skill.
---

A **read-only** sweep over the whole `.idsd/` set: it writes nothing — every fix is proposed and routed to the skill that owns the file, and lands there on confirmation. Check the invariants that surface **across** intents; a single intent's own ambiguities are `idsd-intent`'s clarify pass, not this.

## Phase 1 — Load the set

Read the intent set under `.idsd/`: active intents (`intents/`), built ones (`archive/`), `charter.md`, `constitution.md`, `language.md`, `roadmap.md`. Parse each intent's frontmatter (`title`, `milestone`, `status`, `collaborative`, `approved-by`, `links`) and body (goal, constraints, scenarios). No `.idsd/` → say so and stop.

## Phase 2 — Check the invariants

Run every applicable check; skip a dimension only when its inputs are absent.

- **Links & build order** — every `links` entry uses a known relation (`extends`/`depends-on`/`blocks`, nothing else) and resolves to a real intent; the `depends-on` graph is acyclic; directions follow `idsd-intent`'s **Links rule**.
- **Build batches (parallel schedule)** — from the `depends-on`/`blocks` graph over the not-yet-`built` intents, derive the batches whose members share no unbuilt dependency: each batch is safe to build concurrently, one worktree per intent. Informational, not a finding — an orchestrator schedules parallel `idsd-ship`/`idsd-build` runs from it.
- **Milestone coherence** — no `mvp` intent depends on a `vnext`/unscheduled or still-`draft` one.
- **Constitution coverage** — every baseline NFR is enforced by, or at least not contradicted by, the `mvp` set; flag a baseline no `mvp` intent satisfies and any constraint contradicting a baseline.
- **Charter scope** — every active intent sits inside the charter's Scope; flag off-mission intents, in-scope areas no intent covers, and intents orphaned by a past scope cut.
- **Language** — the set's vocabulary against `language.md`: a domain term the intents lean on with no entry, two entries that cannot both hold, an entry no artifact uses any more, and — the finding that matters most — **two terms in use for one thing**, which is the drift the file exists to stop.
- **Duplication** — overlapping goals/constraints across intents; a constraint recurring in ≥3 intents → propose promoting it to a constitution baseline.
- **Well-formedness** — each intent has a goal with a real *why*, 3–7 constraints, and ≥1 success + ≥1 failure scenario; a goal joined by "and" is either mis-named (rename) or two intents (split); scenario coverage shouldn't be thin for the intent's surface. Flag the gap; don't re-grill here.
- **Status hygiene** — `built` intents live in `archive/`, not `intents/`; numbers are unique and contiguous; `collaborative: true` intents missing `approved-by` are flagged before they reach `idsd-build`; `roadmap.md` matches current frontmatter.
- **Follow-up hygiene** — flag any archived intent still carrying an unchecked `- [ ]`, and an active intent's open `- [ ]` item that names an intent which doesn't exist.

## Phase 3 — Report

One report, grouped by severity — **Blocker** (breaks a build or ships a violation), **Fix** (drift to reconcile), **Nit** — plus the informational **Build batches** list when any intent is unbuilt.

Each finding names the file(s), the **owning skill** to fix it through — `idsd-intent` (intents, links, scope sync, `language.md`), `idsd-charter` (vision/scope), `idsd-constitution` (baselines, gate commands) — and the smallest reconciling move, never a redesign.
