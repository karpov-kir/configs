---
name: kk-skillcraft
description: Review and refine skills as skills — triggering, how steps and reference material are split, steering strength, and what in them is a no-op. Use for "why does the agent ignore this skill", "this skill is too big". Shape, not rule economy (kk-ecosystem) or prose (kk-tighten).
argument-hint: "a skill dir, several, or the whole skills tree"
---

Judge a skill as a skill: **is this thing shaped so an agent reaches it at the right moment and then does what it says?** Rule economy is `kk-ecosystem`'s lens and prose is `kk-tighten`'s; neither applies this one.

A big skill is a symptom. Read it for the cause.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Skill` — the unit is a skill **directory**, not a file. Read every file in it: reference files whole, and a script by its header, usage and call sites — the lens is how files divide, which a script's contract answers and its body does not. **This coarser unit replaces `file` throughout the protocol** — one unit per message, one verdict for the unit, and the queue, ledger and `N/M` all count units.

**A queued artifact that is not a skill** — a standard, a prompt, a template, a `CLAUDE.md` — is one unit as a single file, with no frontmatter and no file set of its own, so §1 and §2 do not reach it. Apply §3 and §4.

**Apply the fixes your lenses call for.** A split into two skills is a proposal, not an edit — it changes what the human types; so is a skill that should do *more*. A finding you return as a proposal is resolved by returning it: `WARN` once and move on, because the protocol's retry cannot converge what you have no license to change. **A defect outside your lenses is named, never edited and never dropped** — `~/.kk-flavor/standards/skill-protocol.md` → **Do not** bars the edit, and silence loses what only this pass saw.

## 1. Trigger — how it gets invoked

Judge each skill on how much it costs to *miss*; `~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins** settles whether it is model-invoked or user-invoked. A description that never routes anything is pure overhead.

Then check the description does its one job: **route**. It exists to answer "is this the skill for what is happening now", not to summarise the body. It carries a trigger, a target, and — where a near-neighbour exists — a discriminator that keeps the two apart. **Too short is a failure, not a virtue** — stripped past routing, a description gets the skill invoked at the wrong moment or not at all.

The `description` is truncated at 1,536 characters in the listing, so text past that budget is not merely expensive — it is discarded, and a discriminator that lands after the cut does nothing.

## 2. Structure — steps and reference

A skill divides into **steps** (the procedure) and **reference** (templates, definitions, glossaries, mode-specific detail the steps consume). Keep `SKILL.md` to the steps and as small as it will go: it is read in full every time.

Find the **branches** — the conditional paths a run may or may not take. A branch's material does not belong inline; it belongs in a file the skill names at the branch, on the terms `~/.kk-flavor/standards/ecosystem.md` → **Move it before you cut it** sets for a **Split**, plus one this lens adds: the extracted file stands alone, without re-reading the parent.

Fail any one and it stays inline. **A bad extraction is worse than none**, because it converts a rule the agent reads into a rule the agent is merely told about.

## 3. Steering — making it actually comply

When an agent ignores an instruction, argue less and steer harder.

- **Leading words.** A dense term the model already knows beats a paragraph describing the same thing — hunt those paragraphs and replace them with the name. **Prefer a word the model was pretrained on over one we coin**: a coined term recruits no priors, so we pay in definition tokens what an existing word gives free. Grade the word as **Pruning** below grades prose — one too weak to move the agent off its default is a no-op, and a stronger word is the fix.
- **State the target, not the ban.** A prohibition leaves the behaviour more available, not less — the negation is a weak modifier over a strongly activated concept. Write what the agent should do, so the other behaviour is never spoken. A ban earns its place where it is a guardrail with no positive phrasing, and there it carries the target beside it.
- **Completion criteria.** Every step ends on the condition that says it is done, and each is judged twice: can the agent tell done from not-done, and **how much does the bound demand**? "Every changed model accounted for" drives legwork that "produce a change list" never asks for. **The demand binds flat reference as well as steps** — "every rule applied" is how a skill that is all reference still carries an exhaustiveness bar. **A bound the file hands to its caller is already stated.** Filling the number in authors a rule, and that is a proposal, never your edit (`~/.claude/skills/kk-ecosystem/SKILL.md` → **Rules**).
- **Hide the next step.** An agent that can see the goal rushes the step in front of it — a skill told to *ask clarifying questions, then plan* barely asks. Split that skill in two, so the early phase is the whole task. Where you find one skill whose early phase is chronically thin, this is usually why. **Splitting hides only what a real context boundary hides**: an invocation that runs inline leaves the later steps in context and clears nothing.

## 4. Pruning — what is not doing anything

- **No-ops.** Text that reads like an instruction but changes no output; `~/.kk-flavor/standards/ecosystem.md` → **Earn the place** holds the test.
- **Sediment.** What accumulates when several people edit one file and nobody dares delete anyone else's rule. It reads as a flat list of equals; it is actually one live procedure plus somebody's old edge case. Move the niche rules into the branch that needs them and kill the stale ones.

**A rule a second file acts on is neither a no-op nor sediment.** You hold one skill at a time, so that file is never in front of you. Name the apparent restatement for `kk-ecosystem`, which scans the whole root, and leave the text standing.

Deleting is not the only fix — try the moves in `~/.kk-flavor/standards/ecosystem.md` → **Move it before you cut it** first.

## Verdict

Per the protocol, plus the cause:

- Pass: `Skill N/M <dir> | <SKILL.md lines>L | OK`
- Fail: the same line with `WARN`, then one line per finding, each naming its lens — one of `trigger` / `structure` / `steering` / `pruning`, or the lens that owns a defect outside them — and what an agent does wrong today.
- A non-skill artifact takes `Artifact` in place of `Skill`, and its own path and line count.

Close by stating plainly whether the skills and artifacts you reviewed are now sound, and name any you left large on purpose.
