---
name: quick-spec
description: Fast spec-driven planning. Grill user adaptively until MVP scope is clear, then emit a design doc and MVP tickets as markdown files under ./docs/specs/<slug>/. Use when user wants a lightweight spec, MVP plan, or mentions "quick spec", "spec it", or "plan an MVP".
---

Goal: a minimal spec + MVP ticket set, fast — just enough to start building.

## Phase 1 — Grill (adaptive)

Interview the user one question at a time, giving your recommended answer each time. Ask the fewest questions that lock MVP scope, scaled to it: a one-screen SPA may need one or two; an Amazon-scale alternative needs more.

Cover, in this order, only what's still unclear:

1. **Problem & user** — who hits this, what breaks today
2. **Success criterion** — one observable outcome that proves MVP works
3. **Scope cut** — smallest slice that delivers the criterion; what is explicitly out
4. **Key decision points** — 1-3 design choices that drive implementation (storage, API shape, UI surface, etc.)
5. **Risks/unknowns** — anything that could invalidate the plan

Skip questions answerable by reading the codebase. Read instead.

Stop when the success criterion is concrete, scope is bounded, and you can list the work — but not while an unanswered question could still change the plan. If the user waves off grilling ("just write it"), stop and proceed with what you have.

## Phase 2 — Confirm

Pick a slug from the feature name (kebab-case, ≤4 words); the spec lives under `./docs/specs/<slug>/`.

- If that folder already exists, stop and ask: resume it, overwrite, or pick a new slug. Never silently clobber an existing spec.
- Show the slug and the ordered ticket titles in one message for sign-off — this confirms the breakdown, not just the paths.

## Phase 3 — Write

After sign-off, write every file in one batch.

### `design.md`

```markdown
# <Feature title>

## Problem
<1-3 sentences>

## Success criterion
<one observable outcome>

## MVP scope
- In: <bullets>
- Out: <bullets — defer list>

## Approach
<conceptual solution, no code traces, ≤10 lines. Fold in the 1-3 decisions that shaped it — "chose X over Y because …". Flag a risk inline only if it could invalidate the plan.>

## Tickets
- NN — <title>
```

Lead with insight, not implementation trace. Keep it tight — a few sentences per section. Fill the Tickets list to mirror the signed-off breakdown.

### Tickets — `NN-<ticket-slug>.md` (NN = 01, 02, …)

One ticket per buildable unit — as few as the MVP needs, no filler. A small feature may be one or two tickets; a large one, many. Let scope set the count.

```markdown
# <Ticket title>

**Depends on:** <NN or none>

## Goal
<one sentence>

## Acceptance
- [ ] <observable check>
- [ ] <observable check>

## Notes
<optional: pointers to files, gotchas, ≤5 lines>
```

Order tickets by dependency. First ticket should be runnable end-to-end (vertical slice) when feasible.

## Rules

- Don't write tickets for backlog/nice-to-haves. MVP only — defer the rest in `design.md`'s Out list.
- Don't over-spec. Tickets are prompts for implementation, not contracts.
