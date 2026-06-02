---
title: <short title>
milestone: <mvp | vnext | none>
status: draft        # draft → approved (at build confirm) → built (at merge)
collaborative: false # true → idsd-build requires approved-by before running
approved-by:         # collaborative sign-off; independent of status, may be set while still draft
links:               # neighbouring intents: "extends NNN", "depends-on NNN", "blocks NNN"
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

## Links

- <extends / depends-on / blocks> NNN-<slug> — <why>
