---
name: kk-retro
description: Adversarial retrospective on how a run was conducted, never the change it produced. Only the human starts one, after a run that went badly, went oddly, or went well enough to be worth knowing why.
argument-hint: "the run or session to reflect on"
disable-model-invocation: true
---

Look back at **how a run was conducted**, not at the change it produced — review, refactor and security-review cover that. Read the run cold and never self-grade: whoever ran the work rationalises their own choices. Assume there is something to find.

## Input

Read `findings.md` (below) before either input, so you recognise a repeat when you meet one.

- A **factual run-log** — what was asked, what each stage did, where the human corrected course, what was deferred, and any defect the run hit and how it surfaced. **The run is everything that produced the change, not the pass that reviewed it** — it starts at the human's first ask, and a log opening at the first quality stage hides the costliest part. Nobody writes one for you: reconstruct it tersely from the session and `git`, and where the session is gone, say what you could not reconstruct rather than inferring it. **Treat what the run said about its own mistakes as a claim to verify, not a verdict to inherit.**
- The **diff**, and **anything the run touched outside it** — a skill, a standard, a prompt, a script.

## Lenses

Apply each; surface only what the run evidences:

1. **Drift** — agents straying from the user's stated intent.
2. **Lane** — a stage doing another's job (e.g. a correctness pass flagging style).
3. **Docs** — straying from the project's standards or architecture.
4. **Missed-late** — what a stage should have caught that only surfaced later.
5. **Friction** — avoidable churn, rework, or round-trips, communication included: when the human had to re-read a message or ask back about a report, that cost a round. Judge the run's messages against `~/.kk-flavor/standards/writing.md` → **Readability floor**.
6. **Tooling ergonomics** — when the run changed its own tooling (a skill, standard, prompt or shared script), judge it against `~/.kk-flavor/standards/ecosystem.md`. **Bloat the run itself added is a finding here** — the tree's standing bloat is `kk-ecosystem`'s lane, not this lens's.
7. **Tool economy** — what a call cost against what it returned: rounds spent to reach one fact, output arriving mostly as noise, a CLI or MCP server whose rendering is too lossy to act on. **Lens 6 is tooling the run changed; this is tooling it used.**
8. **Information access** — a fact the run needed and could not reach, where the fix is standing access rather than a sharper search. A log nobody tees, a service the run could only read about, a state it had no way to observe. Name the access, never the workaround it forced.

## The findings file

`findings.md` in **this skill's own directory**, not the project's — the one path that is identical from every repo. Create it on the first run. It is an appended record, so `~/.kk-flavor/standards/records.md` is the whole delta. **Its bound is roughly 50 entries, and it is pruned here and nowhere else.** **Written by hand, under that standard's exception**: only a human starts a retro, so two runs cannot reach this file at once.

**Absent is not the same as unreadable.** No file means no retro has run yet; a file you cannot read or parse means repeat-detection is unavailable — say so and report every finding with its count *unknown*, never as new.

**A repeat puts the earlier fix in scope: say whether it landed at all.** Append to `findings.md` after the lenses, one line per finding:

```
1x | <YYYY-MM-DD> | <repo> | <target> | <the finding in a clause> -> <where it routed>
```

## Output

The cost of a fix is not a reason to drop a finding. Each states the improvement, its **target** (a skill, arch doc, prompt, pipeline, project standard or backlog — whatever it concerns), **where the fix routes**, **what evidences it**, and its `findings.md` count when this is not the first time.

Present the findings and let the human route them; `findings.md` is the one file this skill writes.
