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

Read `~/.kk-flavor/standards/writing.md` for clarity and density. For outward communication and code comments, also apply `~/.kk-flavor/standards/human-writing.md` for voice and its artifact-specific form. For a body, a comment or a reply you are preparing, run `~/.kk-flavor/skills/kk-edit/scripts/comment-density.sh --voice --profile=prose` over it and make the edits it names. The audience selects the guidance; the human need not choose a mode.

Cut repetition and recoverable filler. Use concrete words and readable sentences. Keep deliberate repetition where a reader needs a constraint at its point of use. Preserve required facts, numbers, names, negation, exceptions, tense, conditionality, commitments, severity and unresolved questions. Keep the source's wording for unresolved status if paraphrasing would attribute knowledge, intent, a response or a decision to someone. A future commitment is not a present fact. If a cut may change meaning, keep it or return a proposal.

For code comments, write every block again from the code. A block refined in place is judged against the block it replaces, and three rounds of that produced comments no reader could follow. The lane runs four steps over each changed source file, in this order, and a block ships only as `keep` or not at all.

1. **Strip.** Run `~/.kk-flavor/scripts/bloat-judge.sh --strip=<dir> --changed=<the caller's revisions> comment <file>`, with `<dir>` a directory of your own under the scratch dir, empty. It removes every comment block the change set touched, writes each to a facts file headed by its site, and prints the sites. A comment the toolchain reads stays, and the strip names it on stderr.
2. **Write.** Dispatch `~/.kk-flavor/workers/comment-writer.md` as `comment-writer` under `~/.kk-flavor/standards/model-policy.md`, with the stripped file, the printed sites and the facts directory. It writes a block from the code or `none` per site, and returns `stale`, `carried by`, `rename` and `for the PR body` lines. Count its `Block` lines against the sites.
3. **Verdict.** Run `~/.kk-flavor/scripts/bloat-judge.sh --changed=<the same revisions> comment-verdict <file>`, `JUDGE_PROVIDER` set explicitly, over the written file. It prints one verdict per block, `<file>:<line>: <verdict>`, from the closed set `keep`, `obvious`, `padded`, `unclear`, `coined`, `carried`, `stale`. Count its lines against the blocks. **Exit 2 is a judge that did not run**, and a run with no verdict lines is not a clean file. Act on each verdict once:

   | Verdict | Action |
   |---|---|
   | `keep` | leave it |
   | `obvious` | delete the block |
   | `padded`, `unclear`, `coined` | strip that block again and dispatch the writer for it once more; a second verdict other than `keep` deletes the block and returns its facts as `for the PR body` |
   | `carried` | return `carried by` for the refactor lane, then delete the block |
   | `stale` | return the block as a correctness finding, then delete it |

4. **Voice.** Run `~/.kk-flavor/skills/kk-edit/scripts/comment-density.sh --voice` over the caller's revisions. **Every finding it prints is an edit you make**: strip that block and dispatch the writer for it. The shapes it names are the **House voice** group of `~/.kk-flavor/standards/human-writing.md` → **AI tells**, and the form it holds you to is `~/.kk-flavor/standards/code-style.md` → **Comments**. A finding that must stand goes in the repository's `comment-voice.conf` with its reason, and the caller decides that entry.

Then run `~/.kk-flavor/skills/kk-edit/scripts/comment-density.sh --bar` over the same revisions and report its figure. The figure gates no edit. Return the writer's and the verdict's routed lines with it: `carried by` and `rename` to the refactor lane, `stale` to correctness review, `for the PR body` to the caller. Comment truth belongs to correctness review and placement to refactor.

## Verify and return

Compare the rewrite with the source for lost meaning and accidental code changes. Check links or syntax affected by the edit. Account for the scoped artifacts and return only unresolved proposals and the protocol's required evidence. A wording edit creates no second editorial lane.
