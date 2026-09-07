---
title: <short title>
milestone: <mvp | vnext | none>
status: draft        # draft → approved (idsd-build's gap rounds closed, every answer landed) → built (at merge)
links:               # neighbouring intents, one edge per line: "extends NNN — why" (relation: extends | depends-on | blocks; drop the why when the relation + linked title make it obvious)
---

# <Goal in one sentence — outcome, no "and">

> Why this matters: <the concrete stakes — value gained, or what specifically breaks without it (lost revenue, blocked launch, churn) — not a restatement of the goal or situation>

## Constraints

Absolute qualities/thresholds the outcome must hold (3–7, business language; violation = failure). This intent inherits `.idsd/constraints.md` — list only the intent-specific ones here.

- <constraint, prefer measurable, e.g. "search returns in < 300ms">

## Success scenarios

```gherkin
Scenario: <name>
  Given <state>
  When <action>
  Then <observable outcome>
```

## Failure scenarios

```gherkin
Scenario: <name — must not happen>
  Given <state>
  When <action>
  Then <the observable bad outcome the caller sees — not the mechanism that prevents it>
```

## Reference data

Optional — include only when the outcome must encode an explicit list or table (allowed values, thresholds, lookup data).

## Follow-ups

Optional, build-managed — open questions and cross-intent consequences surfaced during `idsd-build`, tracked as a checklist.

- [ ] <follow-up>
- [x] <follow-up> — <resolution: fixed … / moved to NNN / declined: …>
