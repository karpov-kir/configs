---
name: idsd-charter
description: Write or edit .idsd/charter.md — an IDSD project's what & why, plus the constraints every intent inherits. Use for "seed the project", "set the project vision/charter", "define scope", "add a project-wide constraint". The level-0 file; one feature's intent is idsd-intent's.
---

Write `.idsd/charter.md`. Don't list features (that's the roadmap), detail behaviour (the intents), or restate the project's `CLAUDE.md` — link to them.

## Phase 1 — Inventory what exists

- If editing, read the current `.idsd/charter.md`.
- If a roadmap or intents already exist, read them for grounding.
- If this edit removes something from scope, scan active and archived intents for ones covering the removed area — now off-mission. Name them and recommend retiring them: a removal intent via `idsd-intent` → `idsd-build`, or deleting the obsolete code.

## Phase 2 — Grill the gaps only

Invoke `kk-grill` **inline**, at project scope, over the sections of `templates/charter-template.md`. Its legwork here is Phase 1's inventory and the code. Cover only what's unclear, and push until the boundaries are sharp — hardest on what is explicitly **out** for now.

**Grill the Constraints section hardest** — every intent inherits it, so it is the section that rots. Beyond the **Rules** test that governs the whole file, each constraint takes one of its own: a command resolved from the repo's own tooling must exit non-zero when that constraint is breached. One that can't become such a command is an aspiration — it belongs in Vision or nowhere.

## Phase 3 — Emit

Run `~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore` first (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**), then confirm the path once and write `.idsd/charter.md` from that template. In "See also", link only to artifacts that exist. Domain terms go in `language.md`, never here — a term pinned in the charter is a term nothing audits.

Score the Constraints section through `~/.kk-flavor/scripts/score.sh cut instruction "a constraint whose breach fails the build"`. The two tests above have already taken the weak ones, so `--kept-all <why>` is the ordinary ending here rather than a failure.

## Rules

Curated, not generated: humans own the wording. **So you open no prose-lane handoff for this file.**

This file is a promotion target, so `~/.kk-flavor/standards/records.md` is the whole delta: its test bounds the file, applied to every line on every edit, never a line count.

A constraint promoted from `.idsd/decisions.md` arrives as a proposal, never as the entry itself: the human owns the threshold.
