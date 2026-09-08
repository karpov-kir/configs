---
name: idsd-charter
description: Write or edit .idsd/charter.md — an IDSD project's what & why, including the constraints every intent inherits. Use for "seed the project", "set the project charter", "add a project-wide constraint". The level-0 charter; one feature's intent is idsd-intent's.
argument-hint: "the vision, scope or constraint to set, or omit to seed the charter"
---

Write `.idsd/charter.md`, including its protected `## Constraints` section. Don't list features (that's the roadmap), detail behaviour (the intents), or restate the project's agent instructions — link to them.

A request naming one section touches only that section. A missing charter is missing input: obtain its content from the human rather than inventing project purpose to house a constraint.

## Phase 1 — Inventory what exists

- If editing, read the current `.idsd/charter.md`.
- If a roadmap or intents already exist, read them for grounding.
- If this edit removes something from scope, scan active and archived intents for ones covering the removed area — now off-mission. Name them and recommend retiring them: a removal intent via `idsd-intent` → `idsd-build`, or deleting the obsolete code.

## Phase 2 — Grill the gaps only

Invoke `kk-grill` at project scope, over the sections of `templates/charter-template.md` and over the constraints. Its legwork here is Phase 1's inventory and the code. Cover only what's unclear.

**On the Scope boundaries, the test between Out and Not yet specified is sharpness, not certainty.** Ask whether the question can be *phrased* precisely now — never whether it can be answered.

**Grill the constraints hardest** — every intent inherits them. Each must be a project-wide invariant with a concrete violation, not a feature detail or implementation decision. Prefer measurable constraints; an invariant needing human judgment still belongs here when it passes the **Rules** test.

## Phase 3 — Emit

Run `~/.kk-flavor/skills/idsd-qualify/scripts/report.sh check-ignore` first (`~/.kk-flavor/skills/idsd-qualify/SKILL.md` → **Report**), then confirm the path once. Prepare `.idsd/charter.md` from the template. In "See also", link only to artifacts that exist. Domain terms go in `.idsd/for-agents/language.md`.

Present the exact proposed charter edit and obtain explicit approval before writing it; existing authorization for that edit suffices. For a promotion, write the accepted constraint before removing its source decision through `report.sh record evict` with the appropriate scope. A declined or unanswered proposal leaves the source intact.

Run `~/.kk-flavor/skills/idsd-qualify/scripts/report.sh layout check` after the edit and correct its structural findings within the approved wording.

## Rules

Curated, not generated: humans own the wording. **Open no prose-lane handoff for the charter.**

Exactly one `## Constraints` section holds at most 30 concise, single-line, top-level `- ` bullets: project-wide invariants only, with no dates or usage counts. Each must rule out something no existing constraint already rules out. An intent may add stricter requirements but cannot silently override these invariants; resolve a conflict through an explicitly approved charter edit before building against it.

Scope and principle lines also must rule out something no existing line rules out.

The cap applies only to Constraints; the rest of the charter is prose. At the cap, propose a consolidation or removal alongside the addition. The human approves the resulting wording before any edit.
