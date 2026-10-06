---
name: kk-edit
description: The edit lane for outward artifacts before approval or delivery, yours or another's, including PR text, commit messages, docs and code comments. Also use for "tighten", "humanize", "de-AI this", or "make this readable". Preserves meaning; ordinary session replies and structured worker returns use writing rules directly. Rule and skill-structure changes belong to their own lanes.
argument-hint: "text, file, directory, or git scope such as staged or the changes"
---

**Runs:** dispatched

Improve the resolved text in one pass. Stop with the edited artifact or, for pasted text, return the rewrite without touching files.

This skill fills the **edit lane** required by `~/.kk-flavor/standards/human-writing.md` → **Edit pass**.

Run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`. Dispatch as `kk-edit` under `~/.kk-flavor/standards/model-policy.md`.

## Scope

Resolve the requested files or text and its audience. A directory scope includes prose documents; a code scope includes comments and docstrings only. For the outward-artifact trigger, scope the pass to the artifact being prepared. Preserve quoted material, code blocks, command output and code behavior.

Agent instructions may receive wording-only edits here. Deleting an obligation, changing a trigger, resolving conflicting rules or restructuring a skill goes to the instruction or skill-structure lane before mutation. Those lanes can apply this prose pass inline after settling semantics.

## Edit

Read `~/.kk-flavor/standards/writing.md` for clarity and density. For outward communication and code comments, also apply `~/.kk-flavor/standards/human-writing.md` for voice and its artifact-specific form. For a body, a comment or a reply you are preparing, run `~/.kk-flavor/skills/kk-edit/scripts/voice-check.sh --profile=prose` over it and make the edits it names. Add `--kind=pr-body` for a PR body and `--kind=ticket` for a ticket, so the run reads the author's sentences and leaves the template's alone. A description is cut by sentence, by the test `~/.kk-flavor/standards/human-writing.md` → **Change descriptions (PRs)** states, and never to a length. The audience selects the guidance; the human need not choose a mode.

Cut repetition and recoverable filler. Use concrete words and readable sentences. Keep deliberate repetition where a reader needs a constraint at its point of use. Preserve required facts, numbers, names, negation, exceptions, tense, conditionality, commitments, severity and unresolved questions. Keep the source's wording for unresolved status if paraphrasing would attribute knowledge, intent, a response or a decision to someone. A future commitment is not a present fact. If a cut may change meaning, keep it or return a proposal.

For code comments, run `~/.kk-flavor/skills/kk-edit/scripts/comment-pass.sh --base=<the change's base>` from the change's tree. One model call per changed file decides each comment the change touched, and each declaration it added or changed that has none, under `~/.kk-flavor/standards/comments.md`. The same call writes the comment text, and the tool applies it. A comment the change left alone is not touched. The tool gates each written comment on width, length and markdown, and a failure stops the pass with the text shown. Put review findings on comments in a notes file, one `<path>:<line> <what is wrong>` line each, and pass it as `--notes=<file>`. A file whose page, prompt and model are unchanged reuses its last reply. Then run the project's own formatter, linter and tests, and commit. The lane returns no comment decision to the human.

## Verify and return

Compare the rewrite with the source for lost meaning and accidental code changes. Check links or syntax affected by the edit. Account for the scoped artifacts and return only unresolved proposals and the protocol's required evidence. A wording edit creates no second editorial lane.
