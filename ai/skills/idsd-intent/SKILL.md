---
name: idsd-intent
description: Author or refine an ICE intent — what to build and why, never how; also owns .idsd/language.md. Triggers on "intent", "ICE", "IDSD", "plan a feature/project", "pin down a domain term". The feature level; the project's vision and its inherited constraints are idsd-charter's.
argument-hint: "feature/project to plan, or an existing intent to refine"
---

Capture **what** to build and **why** as an **ICE** — Intent · Context · Expectations — never **how**. You author Intent and Expectations (goal, constraints, scenarios); `idsd-build` assembles Context and owns implementation.

## Phase 0 — Detect scope

Pick scope from the request, not repo state: one ticket or one outcome → a **feature**, one ICE; "plan the project" / "map the MVP" / several features → a **project**, a map of linked ICEs each tagged by `milestone`.

At project scope, read `.idsd/charter.md` to ground decomposition; if it is missing, offer once to run `idsd-charter` — never force it.

If refining, read the named intent file, grill only the gaps, and preserve its build-managed `## Follow-ups` checklist.

## Phase 1 — Grill

Invoke `kk-grill` over the parts of `templates/ice-template.md`, which defines each part and its format. Its legwork here is the code, the charter, and the neighbouring intents; the frontier is empty once the goal, constraints, and scenarios are concrete and no open question could still change them.

Cover only what's unclear — the heuristic per part:

1. **Goal** — one outcome. An "and" is a smell, not an automatic split: two facets of one outcome → name that outcome; independently shippable → split into more intents beside this one, never a deeper one. When the goal's outcome word is broader than the symptom the request names, settle which outcome is wanted before approving — in outcome terms, never by reaching for a mechanism.
2. **Constraints** — prefer measurable ones, since `idsd-build` gates those automatically. Each must constrain THIS outcome, not how another component consumes its output (that belongs in their intent).
3. **Scenarios** — one success and one failure scenario are the floor; add one per distinct behaviour, path, or failure mode worth proving — never pad with near-duplicates.

For a **project map**, also decompose into one ICE per independently-shippable slice and tag each `milestone` (`mvp`, `vnext`, …); parked vNext intents are real files at `status: draft`.

**Every slice is vertical** — it cuts through each layer its outcome touches, storage to interface, so one intent is demoable on its own. Horizontal slices ("the schema", then "the API", then "the screen") yield intents nobody can accept, because none of them changes what a user can do.

## Phase 2 — Clarify pass (gate)

Required, even when the grill felt thorough. Re-read the assembled draft as a whole and surface every residual ambiguity that would change what gets written — across every part of the ICE, not only the highest-impact one. Fold each answer into the part it refines; the concrete ICE is the record, so keep no separate log.

Emit one outcome line as the gate's evidence: the residual ambiguities found and where each was folded, or the verbatim `Clarify pass: no residual ambiguities`.

## Phase 3 — Emit

**Precondition:** write no file until Phase 2's outcome line is emitted.

Run `~/.claude/skills/idsd-qualify/scripts/report.sh check-ignore` first (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**). Confirm slug(s) + path(s) once, then write. Slug = kebab-case, ≤5 words. Number = highest existing `NNN` across `.idsd/intents/` **and `.idsd/archive/`**, plus one (zero-padded to 3). Compute it at the moment of write; if a concurrent author already took it, bump to the next free one.

Write each ICE to `.idsd/intents/NNN-<slug>.md` from `templates/ice-template.md` at `status: draft`.

If `.idsd/roadmap.md` exists, or scope is project, (re)generate it from every intent's frontmatter (active + archived): a heading per milestone (`milestone: none` → "Unscheduled"), columns number, title, status. Generated, never hand-edited.

**The charter changes only through `idsd-charter`, whatever the section** — you propose, the human confirms. If `.idsd/charter.md` exists and this planning adds intents, defers them, or puts one outside the current **Scope**, propose a Scope update. If there's no charter, don't create one here.

**Keep `.idsd/language.md` current** — the project's ubiquitous language. One entry per domain term: the term, its meaning in a sentence, and the near-term it must not be confused with. Add every term this ICE coins or uses in a narrowed sense; never invent an entry for a term no artifact uses. **Write it only through `~/.claude/skills/idsd-qualify/scripts/report.sh record {append|bump|revise|evict|admit} language "<text>"`** — the same hazard as the decision log (`~/.claude/skills/idsd-qualify/SKILL.md` → **The decision log**). `~/.kk-flavor/standards/records.md` is the whole delta. **It is pruned here and nowhere else**: a term no artifact uses any longer is deleted here, not left for the audit to find.

## Rules

- Never write code or name implementation (files, classes, libraries) — that's a spec, not an intent.
- **Links rule** — keep each ICE self-contained: every dependency declared in the frontmatter `links:`, none hidden. Direction follows build order: `depends-on` points back at what must ship first; `blocks` points forward at what waits on this one, so never point it at an intent already built; a later intent that adds a constraint to a shipped one `extends` it.
- If the user says "just write it", collapse Phases 1–2 to the fastest pass that still emits the Phase 2 outcome line, then Phase 3 — the gate fires even on the fast path.
- Don't restate the kk-flavor standards or `CLAUDE.md`; they're Context for `idsd-build`.
