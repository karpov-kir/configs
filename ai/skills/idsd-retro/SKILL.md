---
name: idsd-retro
description: Adversarial retrospective on how a run was conducted, never the change it produced. Use standalone, or as idsd-qualify's retro stage.
argument-hint: "path to a run-log, or the run/session to reflect on"
---

Look back at **how a run was conducted**, not at the change it produced — review, refactor and security-review cover that. Read the run cold and never self-grade: whoever ran the work rationalises their own choices. Assume there is something to find.

## Input

- A **factual run-log** — what was asked, what each stage did, where the human corrected course, what was deferred, and any defect the run hit and how it surfaced. **Treat what it says about the run's own mistakes as a claim to verify, not a verdict to inherit.** A caller (e.g. `idsd-qualify`) writes it; standalone, reconstruct it tersely from the session and `git` first.
- The **diff**, and **anything the run touched outside it** — a skill, a standard, a prompt, a script.

## Lenses

Apply each; surface only what the run evidences:

1. **Drift** — agents straying from the user's stated intent.
2. **Lane** — a stage doing another's job (e.g. a correctness pass flagging style).
3. **Docs** — straying from the project's standards or architecture.
4. **Missed-late** — what a stage should have caught that only surfaced later.
5. **Friction** — avoidable churn, rework, or round-trips, communication included: when the human had to re-read a message or ask back about a report, that cost a round. Judge the run's messages against `~/.kk-flavor/standards/writing.md` → **Readability floor**.
6. **Tooling ergonomics** — when the run changed its own tooling (a skill, standard, prompt or shared script), judge it against `~/.kk-flavor/standards/ecosystem.md`. **Bloat the run itself added is a finding here** — the tree's standing bloat is `kk-ecosystem`'s lane, not this lens's.

## The findings file

`findings.md` in **this skill's own directory**, not the project's — the one path that is identical from every repo. Create it on the first run. It is an appended record, so `~/.kk-flavor/standards/records.md` is the whole delta. **Its bound is roughly 50 lines.**

**Absent is not the same as unreadable.** No file means no retro has run yet; a file you cannot read or parse means repeat-detection is unavailable — say so and report every finding with its count *unknown*, never as new.

Read it before you read the run, so you recognise a repeat when you meet one. Write it after the lenses, one line per finding:

```
- 1x | <YYYY-MM-DD> | <repo> | <target> | <the finding in a clause> -> <where it routed>
```

## Output

Bounded, **evidence-backed** findings; the cost of a fix is not a reason to drop one. Each states the improvement, its **target** (the skill / arch doc / prompt / pipeline / constitution / backlog it concerns), **where the fix routes**, **what evidences it**, and its `findings.md` count when this is not the first time.

**Caller.** Spawned by an orchestrator → return the findings as data. Standalone → present them and let the human route. Either way the retro only flags: `findings.md` is the one file it writes, and it never edits the durable record.
