---
name: kk-tighten
description: Tighten prose artifacts (docs, skills, standards, prompts, comments) for the context window — cut redundancy and inferable filler, fix closed taxonomies and contradictions, enforce the Writing Guidelines, losslessly (every rule and fact survives). Use when asked to "tighten", "de-dup the docs", "make it concise", "cut redundancy", or to review changed prose for bloat. Triggers on "tighten", "tighten the changes", "de-duplicate".
argument-hint: "file, directory, or natural-language scope (e.g. \"the changes\", \"staged\")"
---

Tighten the prose in every artifact resolved from `$ARGUMENTS`, losslessly: cut what costs context-window tokens without earning them.

**Why.** These artifacts load into the model's context; redundant or inferable text dilutes attention and drifts the model. Leaner prose, same accuracy.

**Scope.** Prose only — standalone docs (markdown, standards, skills, prompts, tickets, PR/commit text) and the prose *inside* code (comments, docstrings). Never code logic or behaviour; that's `/refactor` and `/simplify`. If a comment is unclear only because the code is, note it — don't rewrite the code.

**The invariant — lossless on substance and clarity.** Cut only what's recoverable from surrounding context: adjacent text, the code it documents, types, sibling artifacts, the diff. Never drop a rule, fact, constraint, or example carrying unique information. Some redundancy earns its keep — a rule restated at its point of use, a safety-critical repeat, an example that speeds comprehension; that's signal, not filler. A cut wins only when it nets positive for the reader (AI and human) — never trade real clarity for a few tokens. Unsure a cut loses meaning → keep it.

**Outward text and code comments need more than lossless.** When a resolved artifact's audience is a person reading communication (the set `~/.kk-flavor/standards/human-writing.md` defines — PR/ticket text, chat, email, …), lossless tightening is necessary but not sufficient: after your pass, hand it to the `humanize` role (`~/.kk-flavor/config.yaml`) for voice, AI-tell scrubbing, and the lossy reader-action budget. Changed code comments hand off the same way — humanize's comment form (per that standard) is lossy where this pass is not: it cuts true, unique detail that doesn't change how the next engineer edits the code. Interactive → invoke it; spawned → note the handoff in your return instead.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

## Setup (once)

- Inject the kk-flavor if needed (read `~/.kk-flavor/inject.md` when its routing isn't already in context), then read the skill protocol (above) and the writing standard the flavor routes you to (`standards/writing.md`) — what you tighten against.
- Resolve the artifact list from `$ARGUMENTS`: a path or directory (recursively glob prose + commented source), a git scope (per the protocol), or **whole project** — every doc and commented source under the root. Queue it per the protocol; the one append source here is a sibling pulled in to absorb a duplication.

## The lens

Check every artifact against all five:

1. **Redundancy** — the same fact/rule stated twice, within the artifact or across siblings. Keep one home; cross-reference from the rest.
2. **Inferable filler** — text recoverable from context, plus backstory, hedging, and justification ("we tried…", "importantly"). Cut.
3. **Closed taxonomy** — an enumerated list implying completeness where the domain is open. Open it, or keep only if the set is genuinely fixed.
4. **Contradiction** — two statements that can't both hold. Reconcile to one.
5. **Writing Guidelines** — lead with *why*; one abstraction level; group by purpose; reference down to the artifact's own altitude and link to other layers rather than restate them.

## Look around

Judge each artifact in context, never in isolation. For every non-trivial claim, scan its siblings — other skills in the dir, other sections of the same standard, the docs it links — for the same claim stated elsewhere. Editing a sibling to absorb a duplication is in scope: append it to the queue; if it's outside the resolved list, describe the change and confirm first.

## Loop deltas

- Apply cuts directly — tightening is routine; the one exception is the substance invariant: flag a doubtful cut for your caller instead of guessing.

## Verdict

- Pass: `Artifact N/M <path> | <lines>L | OK`
- Fail:
  ```
  Artifact N/M <path> | <lines>L | WARN
  <each redundancy / inferable / closed-taxonomy / contradiction — one line>
  ```
