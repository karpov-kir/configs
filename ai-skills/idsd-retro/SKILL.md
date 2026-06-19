---
name: idsd-retro
description: Adversarial retrospective on how a run was conducted — a fresh agent reads a factual run-log plus the diff and touched files, and surfaces where agents drifted from intent, lane, or docs, what a stage missed, what was avoidable friction, and (when the run changed its own tooling) whether that tooling is sound. Returns bounded, routed, evidence-backed findings. Use standalone to reflect on a run, or spawned by idsd-ship's Retro stage.
argument-hint: "path to a run-log, or the run/session to reflect on"
---

Look back at **how a run was conducted** — not at the change it produced (review, refactor, and security-review cover the change). The point is to learn: where the work drifted, stalled, or cost more than it should, and the concrete change that prevents the repeat.

**Fresh and adversarial.** Whoever ran the work rationalises their own choices, so this never self-grades. Read the run cold and hunt for what went wrong or wasted effort — assume there's something to learn, the more so on a run that needed course-correction.

## Input

- A **factual run-log** — what was asked, what each stage/step did, where the human corrected course, what was deferred. Decisions and events only, no self-assessment. A caller (e.g. idsd-ship) writes it; standalone, reconstruct it tersely from the session and `git` first.
- The **diff**, and any **skill / doc / prompt / script the run touched**.

## Lenses

Apply each; surface only what the run evidences:

1. **Drift** — agents straying from the user's stated intent.
2. **Lane** — a stage doing another's job (e.g. a correctness pass flagging style).
3. **Docs** — straying from the project's standards or architecture.
4. **Missed-late** — what a stage should have caught that only surfaced later.
5. **Friction** — avoidable churn, rework, or round-trips in how the work was done.
6. **Tooling ergonomics** — when the run changed its own tooling (a skill, a pipeline, a shared script): is it sound, or does it lean on prose where code would be reliable, is it heavier than it earns, is it itself overdue a retrospective? A tooling change that ships without one is a blind spot.

## Output

Bounded, **evidence-backed** findings — never a vague essay; cost is not a reason to narrow them. Each states: the improvement, the concrete **target** (a skill / arch doc / prompt / the pipeline / the constitution / a backlog), the **durable home** its fix routes to, and **what evidences it**.

**Caller.** Spawned by an orchestrator → return the findings as data; don't apply or route them (the orchestrator records, the human ratifies). Standalone → present them and let the human route. Either way the retro only flags — it never edits the durable record itself.
