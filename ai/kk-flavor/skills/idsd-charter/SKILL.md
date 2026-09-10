---
name: idsd-charter
description: Write or edit .idsd/charter.md — an IDSD project's what & why, including the constraints every intent inherits. Use for "seed the project", "set the project charter", "add a project-wide constraint". The level-0 charter; one feature's intent is idsd-intent's.
argument-hint: "the vision, scope or constraint to set, or omit to seed the charter"
---

**Runs:** inline — human

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

Present the exact proposed charter edit and obtain explicit user approval before writing it; existing authorization for that exact edit suffices. For a promotion, write the accepted constraint before removing a fully promoted source decision through `report.sh record evict` with the appropriate scope. A rejected or unanswered proposal leaves both the charter and source decision intact.

Re-read the charter immediately before applying the approved edit. Preserve unrelated changes; if the affected wording changed since approval, present the revised proposal instead of overwriting it with the older copy.

Before proposing a promotion, extract the enduring project-wide obligation from the decision and rewrite it against **Rules** below. Do not copy the decision entry into the charter. Preserve its meaning without expanding its scope; keep implementation details and rationale in the agent records. If the decision contains no project-wide obligation, leave it there. If only part is promoted, retain the remaining useful detail instead of evicting the whole entry.

Run `~/.kk-flavor/skills/idsd-qualify/scripts/report.sh layout check` after the edit and correct its structural findings within the approved wording.

## Rules

Curated, not generated: humans own the wording. **Open no prose-lane handoff for the charter.**

**Constraints are protected against every mutation:** adding, rewording, replacing, combining or deleting a bullet requires explicit user approval of the resulting change. Permission to build, finalize, merge, clean up or run unattended does not grant that approval. Rejecting a promotion rejects that proposal, not an existing constraint. An explicit user instruction to remove a named existing constraint authorizes only that removal. Preserve all other constraints, including during template regeneration, migration and conflict resolution.

Exactly one `## Constraints` section holds at most 30 concise, single-line, top-level `- ` bullets: project-wide invariants only, with no dates or usage counts. Each must rule out something no existing constraint already rules out. An intent may add stricter requirements but cannot silently override these invariants; resolve a conflict through an explicitly approved charter edit before building against it.

**A nontechnical reader must understand each constraint without opening code or a glossary.** State one enduring outcome, promise or boundary for the whole project in plain language. Name who or what it protects and what must remain true. Exclude implementation choices, internal identifiers, code paths, protocols, unexplained abbreviations and feature-level instructions; keep those in intents or agent records. A business limit may use a number and familiar unit when the limit itself matters to users. High-level does not mean vague: the reader must be able to recognize a violation. Check this meaning and readability on every constraint edit; `layout check` validates structure, not these judgments.

Scope and principle lines also must rule out something no existing line rules out.

The cap applies only to Constraints; the rest of the charter is prose. At the cap, propose a consolidation or removal alongside the addition. The human approves the resulting wording before any edit.
