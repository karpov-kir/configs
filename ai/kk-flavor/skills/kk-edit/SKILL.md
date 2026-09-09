---
name: kk-edit
description: The edit lane for authored or revised outward artifacts before approval or delivery, including PR text, commit messages, docs and code comments. Also use for "tighten", "humanize", "de-AI this", or "make this readable". Preserves meaning; ordinary session replies and structured worker returns use writing rules directly. Rule and skill-structure changes belong to their own lanes.
argument-hint: "text, file, directory, or git scope such as staged or the changes"
---

Improve the resolved text in one pass. Stop with the edited artifact or, for pasted text, return the rewrite without touching files.

This skill fills the **edit lane** required by `~/.kk-flavor/standards/human-writing.md` → **Edit pass**.

Run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`. Use the `edit` role from `~/.kk-flavor/standards/model-policy.md`.

## Scope

Resolve the requested files or text and its audience. A directory scope includes prose documents; a code scope includes comments and docstrings only. For the outward-artifact trigger, scope the pass to the artifact being prepared. Preserve quoted material, code blocks, command output and code behavior.

Agent instructions may receive wording-only edits here. Deleting an obligation, changing a trigger, resolving conflicting rules or restructuring a skill goes to the instruction or skill-structure lane before mutation. Those lanes can apply this prose pass inline after settling semantics.

## Edit

Read `~/.kk-flavor/standards/writing.md` for clarity and density. For outward communication and code comments, also apply `~/.kk-flavor/standards/human-writing.md` for voice and its artifact-specific form. The audience selects the guidance; the human need not choose a mode.

Cut repetition and recoverable filler. Use concrete words and readable sentences. Keep deliberate repetition where a reader needs a constraint at its point of use. Preserve required facts, numbers, names, negation, exceptions, tense, conditionality, commitments, severity and unresolved questions. Keep the source's wording for unresolved status if paraphrasing would attribute knowledge, intent, a response or a decision to someone. A future commitment is not a present fact. If a cut may change meaning, keep it or return a proposal.

For code comments, run `~/.kk-flavor/skills/kk-edit/scripts/comment-density.sh --bar` with the caller's revisions, or no revisions for uncommitted changes. Use its findings to prioritize reading; density alone never licenses deletion. Comment truth belongs to correctness review and placement to refactor.

## Verify and return

Compare the rewrite with the source for lost meaning and accidental code changes. Check links or syntax affected by the edit. Account for the scoped artifacts and return only unresolved proposals and the protocol's required evidence. A wording edit creates no second editorial lane.
