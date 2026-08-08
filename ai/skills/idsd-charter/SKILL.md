---
name: idsd-charter
description: Write or edit .idsd/charter.md — an IDSD project's what & why (vision, problem, users, scope). Use for "set the project vision/charter", "define scope". Safe with a non-technical collaborator; the technical how is idsd-constitution's.
---

Write `.idsd/charter.md` — the project's **what & why**, the level-0 intent above the feature intents. Don't list features (that's the roadmap), restate principles or standards (the constitution), or detail behaviour (the intents) — link to them.

## Phase 1 — Inventory what exists

- If editing, read the current `.idsd/charter.md`.
- If a roadmap or intents already exist, read them for grounding.
- If this edit removes something from scope, scan active and archived intents for ones covering the removed area — they are now off-mission. Name them and recommend retiring them: a removal intent via `idsd-intent` → `idsd-build`, or deleting the obsolete code.

## Phase 2 — Grill the gaps only

Invoke `kk-grill` at project scope, over the sections of `templates/charter-template.md`, which says what belongs in each. Its legwork here is Phase 1's inventory and the code. Cover only what's unclear, and push until the boundaries are sharp — hardest on what is explicitly **out** for now.

## Phase 3 — Emit

Run `idsd-qualify`'s `scripts/report.sh check-ignore` first (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**), then confirm the path once and write `.idsd/charter.md` from that template. In "See also", link only to artifacts that exist: drop the constitution line when there's no constitution, the roadmap line until intents have been drafted, and the language line until there is a `language.md`. Domain terms go there and never here — `idsd-intent` owns that file and `idsd-audit` checks only that file, so a term pinned in the charter is a term nothing audits.

## Rules

Curated, not generated: humans own the wording. `idsd-intent` may refine the scope section as features evolve, but vision and problem change only here, on purpose.
