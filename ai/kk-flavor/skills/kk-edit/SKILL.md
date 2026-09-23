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

For code comments, write every block again from the code. The lane decides each block itself, and it returns no decision about a comment to the human. Read the refactor lane's comment lines by script. `grep -E '^Comment [0-9]+/[0-9]+ ' <the return>` counts them against the blocks. `grep -E '^Comment .* \| (blocked:|stays: .*(licen|scope|confirm))' <the return>` finds the ones to refuse. Refuse a line with any verdict outside `carried by` and `stays:` as well. Refuse a `blocked:` on a comment block. Refuse a `stays:` too where its reason names a licence, a scope rule or a confirmation. Dispatch the lane again for those blocks with `~/.kk-flavor/standards/code-style.md` → **Comments** quoted, twice at most. A block still refused after that is a tool finding in the run's report, by its line, and never a question to the human. A block refined in place is judged against the block it replaces, and three rounds of that produced comments no reader could follow. The lane runs three steps over each changed source file, in this order.

1. **Strip.** Run `~/.kk-flavor/skills/kk-edit/scripts/comment-strip.sh --facts=<dir> --changed=<the caller's revisions> <file>`, with `<dir>` a directory of your own under the scratch dir, empty. It removes every comment block the change set touched, writes each to a facts file headed by its site, and prints the sites. It calls no model. A comment the toolchain reads stays, and the strip names it on stderr.
2. **Write.** Dispatch `~/.kk-flavor/workers/comment-writer.md` as `comment-writer` under `~/.kk-flavor/standards/model-policy.md`, with the stripped file, the printed sites and the facts directory. It writes a block from the code or `none` per site, and `none` is its default. Count its `Block` lines against the sites. It returns `stale`, `carried by`, `rename`, `belongs at`, `does not fit` and `for the PR body` lines. A `belongs at <file>:<identifier>: <fact>` line is a fact whose bearing is at a caller in another file of the change set. Hand it to that file's writer as a record, under `bears_on: <identifier>`, in the same round where that file is in the set. Where the set holds no such file, it goes to the caller with the `does not fit` lines. Then grep the spawn's transcript for `git diff`, `git show` and `git log`. The strip clears the tree and leaves the history, so a writer that read the range saw the blocks it was sent to replace. A hit counts where it stands before the writer's first write to the file. Its output also has to hold a comment line. A diff the writer runs after writing shows its own blocks beside the removed ones, which it has already replaced. Run 9 re-stripped three writers for that and lost 45 sites. A hit that counts is a tainted run: strip those sites again and dispatch a fresh writer for them.
3. **Voice.** Run `~/.kk-flavor/skills/kk-edit/scripts/voice-check.sh` over the caller's revisions. **Every finding it prints is an edit you make**: strip that block and dispatch the writer for it. The shapes it names are the **House voice** group of `~/.kk-flavor/standards/human-writing.md` → **AI tells**, and the form it holds you to is `~/.kk-flavor/standards/code-style.md` → **Comments**. No finding is kept by a list. One that must stand is a defect in the check. The check is measured on the reviewed set and fixed there, or it is demoted to report-only.

Three bounds, and a model votes on none of them. The writer's `none` default and its two-rewrites-then-`none` rule bound how much gets written. The voice check bounds the register. The refactor lane's `carried by` line bounds placement. A judge kind that labelled every block was measured against this lane and dropped. `ai/tools/reader-judge/eval_test.go` holds what it scored.

Then run `~/.kk-flavor/skills/kk-edit/scripts/voice-check.sh --density` over the same revisions and report its figure. The figure gates no edit. Return the writer's routed lines with it. `carried by` and `rename` go to the refactor lane, and `stale` to correctness review. `for the PR body` goes to the caller for the change's description, and `does not fit` to the caller for the human. A `does not fit` fact is about the world, and it never becomes a paragraph of the body. A `carried by` fact the refactor lane leaves unmade comes back as `for the PR body` with its carrier named as pending. A fact the tree holds nowhere has been lost. Comment truth belongs to correctness review and placement to refactor.

## Verify and return

Compare the rewrite with the source for lost meaning and accidental code changes. Check links or syntax affected by the edit. Account for the scoped artifacts and return only unresolved proposals and the protocol's required evidence. A wording edit creates no second editorial lane.
