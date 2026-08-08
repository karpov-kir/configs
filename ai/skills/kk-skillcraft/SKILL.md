---
name: kk-skillcraft
description: Review and refine skills as skills — how each one is triggered, whether its steps and reference material are split right, whether it steers the agent hard enough, and what in it is a no-op. Use for "review my skills", "why does the agent ignore this skill", "this skill is too big". Shape, not rule economy or prose (kk-tighten) — kk-ecosystem owns that lane and spawns this itself, so invoke it directly only for a skills-only pass.
argument-hint: "a skill dir, several, or the whole skills tree"
---

Judge a skill as a skill. Not whether its rules earn their place or reconcile across skills — that is `kk-ecosystem`, against `~/.kk-flavor/standards/ecosystem.md` — and not whether its prose is tight or cuttable for tokens alone, which is `kk-tighten`. This is the lens those two cannot apply: **is this thing shaped so an agent reaches it at the right moment and then does what it says?**

A big skill is a symptom. Read it for the cause.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Skill` — the unit is a skill **directory**, not a file, since three of the four lenses are about how its files divide. Read every file in it, scripts and reference files included: a lens about how files divide cannot run on `SKILL.md` alone. **This coarser unit replaces `file` throughout the protocol** — one directory per message, one verdict for the directory, and the queue, ledger and `N/M` all count directories.

**Apply the fixes your lenses call for.** A split into two skills is a proposal, not an edit — it changes what the human types; so is a skill that should do *more*. A finding you return as a proposal is resolved by returning it: `WARN` once and move on, because the protocol's retry cannot converge what you have no license to change.

## 1. Trigger — how it gets invoked

Every skill is either **model-invoked** (its description sits in every session's context; the agent decides whether to follow the pointer) or **user-invoked** — `disable-model-invocation: true`, which drops the description from context altogether and leaves `/<name>` as the only way in. The trade is real in both directions:

- Model-invoked costs **context load**. Every description is paid on every request, whether or not the skill runs, and each one is another thing the agent weighs. Descriptions are the cheapest text in an ecosystem to overlook because they do not look like a document — count them.
- Model-invoked also costs **unpredictability**. A perfectly-matched skill the model simply declines to load is a failure no wording fixes, and confirming it fires means running evals.
- User-invoked costs **cognitive load**: the human must remember the skill exists and when it applies.

Judge each skill on how much it costs to *miss*; `~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins** settles which way that falls. A description that never routes anything is pure overhead.

Then check the description does its one job: **route**. It exists to answer "is this the skill for what is happening now", not to summarise the body. It carries a trigger, a target, and — where a near-neighbour exists — a discriminator that keeps the two apart, including the negative kind: the neighbour that owns what this one does not. Too short is a real failure, not a virtue: a description stripped past routing gets the skill invoked at the wrong moment or not at all.

`description` plus `when_to_use` is truncated at 1,536 characters in the listing, so text past that budget is not merely expensive — it is discarded, and a discriminator that lands after the cut does nothing.

## 2. Structure — steps and reference

A skill divides into **steps** (the procedure) and **reference** (templates, definitions, glossaries, mode-specific detail the steps consume). Keep `SKILL.md` to the steps and as small as it will go: it is read in full every time.

Find the **branches** — the conditional paths a run may or may not take. A branch's material does not belong inline; it belongs in a file the skill names at the branch. Three tests, all of which must pass:

1. Would an agent on the common path decide *worse* without having read it? If yes it stays.
2. Is the pointer unmissable, sited exactly where the branch is taken, and does it say the file is the whole delta for that path? A pointer the agent skims past leaves the rule both absent and unenforced.
3. Does the extracted file stand alone, without re-reading the parent?

Fail any one and it stays inline. **A bad extraction is worse than none**, because it converts a rule the agent reads into a rule the agent is merely told about.

## 3. Steering — making it actually comply

When an agent ignores an instruction, argue less and steer harder.

- **Leading words.** A dense term the model already knows beats a paragraph describing the same thing. "Build it as a vertical slice" changes the plan; "don't write it layer by layer" does not — the agent repeats the term back in its own reasoning and inherits everything attached to it. Hunt for paragraphs that describe a concept the language already names, and replace them with the name.
- **Hide the next step.** An agent that can see the goal rushes the step in front of it — which is why a skill told to *ask clarifying questions, then plan* barely asks anything. Split that into two skills. With the plan out of view, the questioning phase is the whole task and gets the whole effort. Where you find one skill whose early phase is chronically thin, this is usually why.

## 4. Pruning — what is not doing anything

- **No-ops.** Text that reads like an instruction but changes no output. Apply the deletion test: cut it, and ask whether a competent agent does the same thing anyway. A paragraph demanding a "clear, detailed commit message" is a no-op. These breed when an agent writes the skill.
- **Sediment.** What accumulates when several people edit one file and nobody dares delete anyone else's rule. It reads as a flat list of equals; it is actually one live procedure plus somebody's old edge case. Move the niche rules into the branch that needs them and kill the stale ones.
- **Repetition.** One home per instruction, within the skill and across its reference files.

Deleting is not the only fix — try the moves in ecosystem.md → **Move it before you cut it** first.

## Verdict

Per the protocol, plus the cause:

- Pass: `Skill N/M <dir> | <SKILL.md lines>L | OK`
- Fail: the same line with `WARN`, then one line per finding, each naming its lens (`trigger` / `structure` / `steering` / `pruning`) and what an agent does wrong today.

Close by stating plainly whether the skills are now sound, and name any you left large on purpose.
