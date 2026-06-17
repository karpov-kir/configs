---
title: <short title>
milestone: <mvp | vnext | none>
status: draft        # draft → approved (at build confirm) → built (at merge)
collaborative: false # true → idsd-build requires approved-by before running
approved-by:         # collaborative sign-off; independent of status, may be set while still draft
links:               # neighbouring intents, one edge per line: "extends NNN — why" (relation: extends | depends-on | blocks; drop the why when the relation + linked title make it obvious)
---

# <Goal in one sentence — outcome, no "and">

> Why this matters: <the concrete stakes — value gained, or what specifically breaks without it (lost revenue, blocked launch, churn) — not a restatement of the goal or situation>

## Constraints

Absolute qualities/thresholds the outcome must hold (3–7, business language; violation = failure). Project-wide NFRs are inherited from the constitution — list only the intent-specific ones here.

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

- <e.g. allowed country codes: AT, DE, CH>

## Follow-ups

Optional, build-managed — open questions and cross-intent consequences surfaced during `idsd-build`, tracked as a checklist: an unchecked item is not yet addressed, a checked one carries a one-line resolution. Every item must be checked before archive; the checklist travels with the archived intent as the record.

- [ ] <follow-up — open, not yet addressed>
- [x] <follow-up> — <resolution: fixed … / moved to NNN / declined: …>
