---
name: idsd-charter
description: Set up or edit the charter for an IDSD project — the project's what & why (vision, problem, users, scope boundaries) that idsd-build reads as Context. Optional and run rarely (project seeding or a direction change). Use when asked to "set the project vision/charter", "what is this project about", "define scope". Safe to run solo or with a non-technical collaborator.
---

Write `.idsd/charter.md` — the project's **what & why**: the level-0 intent that sits above the feature intents. It is the home for the vision the rest of the suite assumes but doesn't store.

Optional. Feature work and even project planning run without it; create one when the project's purpose is worth stating for collaborators and for `idsd-build`'s Context.

Living memory: it's refined as the project teaches you more. `idsd-intent` and `idsd-build` surface drift and recurring terms as you build; you make the change here. It's self-*auditing*, not self-*writing* — edits are deliberate.

Where this fits: **`idsd-charter` (optional) → `idsd-constitution` (optional) → `idsd-intent` → `idsd-build`**. The charter is the *what/why*, the constitution the *how*; `idsd-intent` keeps the charter's scope current as features evolve.

## Phase 1 — Inventory what exists

- If editing, read the current `.idsd/charter.md`.
- If a roadmap or intents already exist, read them for grounding — but the charter links to them, it doesn't copy them.
- If this edit removes something from scope, scan the intents (active and archived) for ones covering the removed area — they are now off-mission and must be reconciled (see Rules).

## Phase 2 — Grill the gaps only

Grill like `grill-me` for project-level scope — one question at a time, each with your recommended answer; push until the boundaries are sharp. The charter is non-technical by nature, so it reads plainly whether you're solo or pairing. Cover only what's unclear:

1. **Vision** — what the project is, in 1–2 sentences. An outcome for users, not a feature list.
2. **Problem & users** — who it's for and what's broken without it (concrete stakes, not a generic harm).
3. **Scope boundaries** — what's in and, just as important, what's explicitly out for now.
4. **Shared vocabulary** *(optional)* — domain terms worth pinning so intents use them consistently. Add as they're clarified, not up front.

## Phase 3 — Emit

Write `.idsd/charter.md` from `templates/charter-template.md`. Confirm the path once before writing. Keep it lean — link out for detail. In "See also", link only to artifacts that exist — drop the constitution line if there's no constitution, and the roadmap line until intents have been drafted.

## Rules

- Charter holds the *what & why* (plus the supporting vocabulary), nothing else. Don't list features (that's the roadmap) or restate principles/standards (that's the constitution) or detail behaviour (that's the intents) — link to them.
- Curated, not generated: humans own the wording. `idsd-intent` may refine the scope section as features evolve, but vision and problem change only here, on purpose.
- A scope cut that orphans built or active intents must be flagged: name them and recommend retiring them — open a removal intent via `idsd-intent` → `idsd-build`, or delete the obsolete code. Never leave the charter and the codebase contradicting each other.
- Keep it at altitude: vision, problem, scope, shared vocabulary — never feature detail (intents) or architecture (code). It may grow as the project teaches you more, but link out rather than copy detail in.
