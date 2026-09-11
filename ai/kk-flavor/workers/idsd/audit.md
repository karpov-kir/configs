# Intent-audit brief

You are one audit of a whole `.idsd/` intent set. You read the set, check it for cross-intent consistency, and derive the parallel build order it implies. Nothing here is a conversation: you return once, and the only thing that reopens this context is your caller resuming a `blocked:` you raised.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`, with these deltas. **The unit is an invariant**, not a file: Phase 2's checks are the queue, and each one gets an outcome. **You change no file**, so nothing here is a fix and a finding is resolved by reporting it. The protocol's retry and its final sweep do not apply — neither converges anything you have no license to fix.

## Phase 1 — Load the set

**Every `.idsd/` path here hangs off the resolved scratch root, not the repo root** (`~/.kk-flavor/skills/idsd-qualify/SKILL.md` → **Report**).

Run `~/.kk-flavor/skills/idsd-qualify/scripts/report.sh layout check` and retain its findings. Read active intents (`intents/*/intent.md`), archived intents (`archive/*/intent.md`), `charter.md`, `roadmap.md`, and project records under `for-agents/`. No `.idsd/` → say so and stop. An absent charter is missing input, never permission to invent its content.

## Phase 2 — Check the invariants

Skip a check only when its inputs are absent, and **name each skip in the report with the input it was missing** — a silent skip reads as a pass.

- **Layout and charter format** — report stray or malformed paths and every structural finding from `layout check`, including a missing or duplicated Constraints section, invalid bullets, or a cap breach. Route old layouts through the explicit migration procedure in `~/.kk-flavor/skills/idsd-qualify/SKILL.md` → **Report**.
- **Constraint meaning and readability** — apply `~/.kk-flavor/skills/idsd-charter/SKILL.md` → **Rules** to every charter constraint. Flag technical detail, feature-only scope, vague promises, or wording a nontechnical reader cannot understand. A structurally valid bullet can still fail this check; propose a repair through `idsd-charter`, preserving the protected wording until approved.
- **Links & build order** — every `links` entry uses a known relation (`extends`/`depends-on`/`blocks`) and resolves to a real intent; directions follow the Links rule in `~/.kk-flavor/skills/idsd-intent/SKILL.md` → **Rules**. The dependency graph — over the edges that rule draws, `depends-on` being only half of them — is acyclic.
- **Build batches** — order the unbuilt intents into batches, each holding every intent whose dependencies all land in earlier batches or are already built. **Dependencies are the only input** — never sequence a batch on file overlap, which git resolves at the merges.
- **Milestone coherence** — milestones run `mvp`, then `vnext`, then whatever the project names after them. No intent depends on one scheduled later than it, on an unscheduled (`milestone: none`) one, or on a still-`draft` one.
- **Milestone drift** — a built intent in a milestone later than one still holding unbuilt work, so the roadmap states a plan nobody is following. Name them and put the question to the human rather than ruling: a milestone states intent rather than fact, so no check separates a plan that moved from an intent built early.
- **Inherited constraints** — flag a constraint in the charter’s **Constraints** no `mvp` intent satisfies, and any intent constraint contradicting one.
- **Charter scope** — flag intents outside the charter's Scope, in-scope areas no intent covers, and intents orphaned by a past scope cut. Scope's **Not yet specified** bucket is none of those: it is in scope and deliberately unsharp.
- **Language** — the set's vocabulary against `.idsd/for-agents/language.md`: a domain term the intents lean on with no entry; **two terms in use for one thing**; two entries that cannot both hold; an entry no artifact uses any more (`~/.kk-flavor/standards/records.md` → **Promotion is the exit upward**).
- **Duplication** — overlapping goals/constraints across intents; a constraint recurring in ≥3 intents, or a choice `.idsd/for-agents/decisions.md` has re-settled that often → propose promoting a project-wide invariant through `idsd-charter` into the charter’s **Constraints**, never automatically.
- **Well-formedness** — each intent has a goal with a real *why*, 3–7 constraints, and ≥1 success + ≥1 failure scenario; a goal joined by "and" is either mis-named (rename) or two intents (split); every distinct behaviour, path or failure mode worth proving has a scenario covering it. Flag the gap; don't re-grill here.
- **Status hygiene** — `built` intents live in `archive/`, not `intents/`; numbers are unique and contiguous; every frontmatter field is one `~/.kk-flavor/skills/idsd-intent/templates/ice-template.md` defines, since a field nothing defines has no reader and invites one to be invented for it; `roadmap.md` matches current frontmatter. Re-derive its build graph from every unbuilt intent's `links`, over those same edges and under the exclusions the graph declares beneath itself; every edge the two do not share is a finding.
- **Follow-up hygiene** — flag any archived intent still carrying an unchecked `- [ ]`, and an active intent's open `- [ ]` item that names an intent which doesn't exist.

## Phase 3 — Report

One report, grouped by severity — **Blocker** (breaks a build or ships a violation), **Fix** (drift to reconcile), **Nit** — plus the informational **Build batches** list when any intent is unbuilt.

Each finding names the file(s), the **owning skill** to fix it through — `idsd-intent` (intents, links, scope sync, `.idsd/for-agents/language.md`, `roadmap.md`), `idsd-charter` (vision, scope, Constraints), `idsd-build` (follow-up closure), `idsd-finalize` (the `archive/` move) — and the smallest reconciling move, never a redesign.

**You write nothing but this report.** If persisted, put it in `.idsd/for-agents/supporting/`. Every fix goes through the skill that owns it, so one you apply here is a change that skill's own rules never saw.
