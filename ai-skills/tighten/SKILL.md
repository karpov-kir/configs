---
name: tighten
description: Tighten prose artifacts (docs, skills, standards, prompts, comments) for the context window — cut redundancy and inferable filler, fix closed taxonomies and contradictions, enforce the Writing Guidelines, losslessly (every rule and fact survives). Use when asked to "tighten", "de-dup the docs", "make it concise", "cut redundancy", or to review changed prose for bloat. Triggers on "tighten", "tighten the changes", "de-duplicate".
argument-hint: "file, directory, or natural-language scope (e.g. \"the changes\", \"staged\")"
---

Tighten the prose in every artifact resolved from `$ARGUMENTS`, losslessly: cut what costs context-window tokens without earning them.

**Why.** These artifacts load into the model's context; redundant or inferable text dilutes attention and drifts the model. Leaner prose, same accuracy.

**Scope.** Prose only — standalone docs (markdown, standards, skills, prompts, tickets, PR/commit text) and the prose *inside* code (comments, docstrings). Never code logic or behaviour; that's `/refactor` and `/simplify`. If a comment is unclear only because the code is, note it — don't rewrite the code.

**The invariant — lossless on substance.** Cut only what's recoverable from surrounding context: adjacent text, the code it documents, types, sibling artifacts, the diff. Never drop a rule, fact, constraint, or example carrying unique information. Unsure a cut loses meaning → keep it.

## Setup (once)

- Read CLAUDE.md's **Writing Guidelines** (already in context) — the standard you tighten against.
- Resolve the artifact list from `$ARGUMENTS`: a path or directory (recursively glob prose + commented source); **staged** / **unstaged** / **all changed** → `git diff --name-only` with `--cached` / nothing / `HEAD`; **whole project** → every doc and commented source under the root.
- Skip deleted files; for a rename use the new path. Save the list to TodoWrite — the queue. It grows only by appending a sibling pulled in to absorb a duplication; never drop a queued artifact.

## The lens

Check every artifact against all five:

1. **Redundancy** — the same fact/rule stated twice, within the artifact or across siblings. Keep one home; cross-reference from the rest.
2. **Inferable filler** — text recoverable from context, plus backstory, hedging, and justification ("we tried…", "importantly"). Cut.
3. **Closed taxonomy** — an enumerated list implying completeness where the domain is open. Open it, or keep only if the set is genuinely fixed.
4. **Contradiction** — two statements that can't both hold. Reconcile to one.
5. **Writing Guidelines** — lead with *why*; one abstraction level; group by purpose; reference down to the artifact's own altitude and link to other layers rather than restate them.

## Look around

Judge each artifact in context, never in isolation. For every non-trivial claim, scan its siblings — other skills in the dir, other sections of the same standard, the docs it links — for the same claim stated elsewhere. Editing a sibling to absorb a duplication is in scope: append it to the queue; if it's outside the resolved list, describe the change and confirm first.

## Loop

- One artifact per message; read it in full each time (re-read after editing).
- Apply cuts directly — tightening is routine; the one exception is the substance invariant — flag a doubtful cut instead of guessing.
- Order per message: read, cut, then the verdict last.
- Safety stop: if an issue resists three passes, emit `WARN` and ask.
- Once every artifact is `OK`, run one final sweep; if any warns, restart from the first.

## Verdict (the last thing in the message)

- Pass: `Artifact N/M <path> | <lines>L | OK`
- Fail:
  ```
  Artifact N/M <path> | <lines>L | WARN
  <each redundancy / inferable / closed-taxonomy / contradiction — one line>
  ```

`M` is the current queue length (grows as you append siblings). The verdict describes the state before your edits.

## Do not

- Reword for taste — change only what a lens item flags.
- Echo the queue, write transition filler, or merge artifacts into one verdict.
