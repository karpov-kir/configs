---
name: idsd-charter
description: Write or edit .idsd/charter.md — an IDSD project's what & why. Use for "set the project vision/charter", "define scope". Safe with a non-technical collaborator; the technical how is idsd-constitution's.
---

Write `.idsd/charter.md` — the project's **what & why**, the level-0 intent above the feature intents. Don't list features (that's the roadmap), restate principles or standards (the constitution), or detail behaviour (the intents) — link to them.

## Phase 1 — Inventory what exists

- If editing, read the current `.idsd/charter.md`.
- If a roadmap or intents already exist, read them for grounding.
- If this edit removes something from scope, scan active and archived intents for ones covering the removed area — now off-mission. Name them and recommend retiring them: a removal intent via `idsd-intent` → `idsd-build`, or deleting the obsolete code.

## Phase 2 — Grill the gaps only

Invoke `kk-grill` at project scope, over the sections of `templates/charter-template.md`. Its legwork here is Phase 1's inventory and the code. Cover only what's unclear, and push until the boundaries are sharp — hardest on what is explicitly **out** for now.

## Phase 3 — Emit

Run `~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore` first (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**), then confirm the path once and write `.idsd/charter.md` from that template. In "See also", link only to artifacts that exist. Domain terms go in `language.md`, never here — a term pinned in the charter is a term nothing audits.

## Rules

Curated, not generated: humans own the wording. **So the prose lane does not run over this file** — a tightener rewriting the human's vision prose is the one edit "they own the wording" rules out, whatever it buys in words.

**The charter receives promotions from the records below it, so it is not bounded by size** — `~/.kk-flavor/standards/records.md` → **The promotion targets carry a test, not a cap** holds instead.
