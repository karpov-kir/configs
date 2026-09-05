---
name: idsd-charter
description: Write or edit .idsd/charter.md — an IDSD project's what & why — and .idsd/constraints.md, the thresholds every intent inherits. Use for "seed the project", "set the project vision/charter", "define scope", "add a project-wide constraint". The level-0 files; one feature's intent is idsd-intent's.
---

Write `.idsd/charter.md` and `.idsd/constraints.md`. Don't list features (that's the roadmap), detail behaviour (the intents), or restate the project's `CLAUDE.md` — link to them.

**The two files are independent.** A repo with no charter still takes constraints, and a request that names only one touches only that one — a project can be worth constraining before anyone can say what it is for.

## Phase 1 — Inventory what exists

- If editing, read the current `.idsd/charter.md` and `.idsd/constraints.md`.
- If a roadmap or intents already exist, read them for grounding.
- If this edit removes something from scope, scan active and archived intents for ones covering the removed area — now off-mission. Name them and recommend retiring them: a removal intent via `idsd-intent` → `idsd-build`, or deleting the obsolete code.

## Phase 2 — Grill the gaps only

Invoke `kk-grill` **inline**, at project scope, over the sections of `templates/charter-template.md` and over the constraints. Its legwork here is Phase 1's inventory and the code. Cover only what's unclear.

**On the Scope boundaries, the test between Out and Not yet specified is sharpness, not certainty.** Ask whether the question can be *phrased* precisely now — never whether it can be answered.

**Grill the constraints hardest** — every intent inherits them, so they are what rots. Beyond the **Rules** test that governs the charter, each constraint takes one of its own: a command resolved from the repo's own tooling must exit non-zero when that constraint is breached. One that can't become such a command is an aspiration — it belongs in the charter's Vision or nowhere.

## Phase 3 — Emit

Run `~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore` first (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**), then confirm the path once. Where the request reaches the charter, write `.idsd/charter.md` from that template. In "See also", link only to artifacts that exist. Domain terms go in `language.md`, never here — a term pinned in the charter is a term nothing audits.

**Write each constraint through `~/.claude/skills/idsd-qualify/scripts/report.sh record {append|bump|revise|evict|admit} constraints "<text>"`**, never by hand — the same hazard as the decision log (`~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log**). `~/.kk-flavor/standards/records.md` is the whole delta. **It is pruned here and nowhere else.**

Run `~/.kk-flavor/scripts/bloat-judge.sh record-entry` over the constraints; what it names is a proposal to the human (**Rules**).

A constraint promoted from `.idsd/decisions.md` arrives as a proposal, never as the entry itself: the human owns the threshold. Once they accept it, delete the source entry with `~/.claude/skills/idsd-qualify/scripts/report.sh record evict decisions "<text identifying it>"` (`~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log**).

## Rules

Curated, not generated: humans own the wording. **So you open no prose-lane handoff for either file.**

`charter.md` takes promotions, so `~/.kk-flavor/standards/records.md` → **Deletion is not eviction**'s test applies to every line on every edit. Nothing else there reaches it: this is prose, not counted entries — no cap.
