---
name: idsd-audit
description: Audit the .idsd/ intent set for cross-intent consistency — links, build order, milestones, charter scope. Read-only; each fix routes to its owning skill.
---

Check the invariants that surface **across** intents; a single intent's own ambiguities are `idsd-intent`'s clarify pass, not this.

## Phase 1 — Load the set

Read the intent set under `.idsd/`: active intents (`intents/`), built ones (`archive/`), `charter.md`, `constitution.md`, `language.md`, `roadmap.md`. No `.idsd/` → say so and stop.

## Phase 2 — Check the invariants

Skip a check only when its inputs are absent.

- **Links & build order** — every `links` entry uses a known relation (`extends`/`depends-on`/`blocks`, nothing else) and resolves to a real intent; the `depends-on` graph is acyclic; directions follow `idsd-intent`'s **Links rule**.
- **Build batches (parallel schedule)** — group the unbuilt intents into batches whose members share no unbuilt dependency; each batch is safe to build concurrently, one worktree per intent.
- **Milestone coherence** — no `mvp` intent depends on a `vnext`/unscheduled or still-`draft` one.
- **Constitution coverage** — flag a baseline NFR no `mvp` intent satisfies, and any constraint contradicting a baseline.
- **Charter scope** — flag intents outside the charter's Scope, in-scope areas no intent covers, and intents orphaned by a past scope cut.
- **Language** — the set's vocabulary against `language.md`: a domain term the intents lean on with no entry; **two terms in use for one thing** (`~/.kk-flavor/standards/writing.md` → **Readability floor**); two entries that cannot both hold (`~/.kk-flavor/standards/writing.md` → **Density**); an entry no artifact uses any more (`~/.kk-flavor/standards/records.md` → **Deletion is not eviction**).
- **Duplication** — overlapping goals/constraints across intents; a constraint recurring in ≥3 intents → propose promoting it to a constitution baseline.
- **Well-formedness** — each intent has a goal with a real *why*, 3–7 constraints, and ≥1 success + ≥1 failure scenario; a goal joined by "and" is either mis-named (rename) or two intents (split); scenario coverage shouldn't be thin for the intent's surface. Flag the gap; don't re-grill here.
- **Status hygiene** — `built` intents live in `archive/`, not `intents/`; numbers are unique and contiguous; flag `collaborative: true` intents missing `approved-by`; `roadmap.md` matches current frontmatter.
- **Follow-up hygiene** — flag any archived intent still carrying an unchecked `- [ ]`, and an active intent's open `- [ ]` item that names an intent which doesn't exist.

## Phase 3 — Report

One report, grouped by severity — **Blocker** (breaks a build or ships a violation), **Fix** (drift to reconcile), **Nit** — plus the informational **Build batches** list when any intent is unbuilt.

Each finding names the file(s), the **owning skill** to fix it through — `idsd-intent` (intents, links, scope sync, `language.md`), `idsd-charter` (vision/scope), `idsd-constitution` (baselines, gate commands) — and the smallest reconciling move, never a redesign.
