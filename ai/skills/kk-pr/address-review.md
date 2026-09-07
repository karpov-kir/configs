# `address-review` — fix what the review asked for, then push

## What you are addressing

Every review comment on the PR that `~/.claude/skills/kk-pr/SKILL.md` → **Continue the round** classified **still open**, plus — under `review-and-address` — every entry in the scratchpad. **Both sets become one change set before you touch a file**, or you push your own fixes while handing the author back their own open questions.

- **A comment you disagree with is still addressed**, and the line stays the reviewer's to reopen.
- **A thread already carrying your `Done <link>` is settled**, so a later invocation skips it and the round continues at the first that is not.

## Fix, gate, push

1. **Fix in the checked-out worktree, one comment at a time**, keeping each change to what that comment asked for. Work beyond it reads as `beyond the ask` on the next round.
2. **Your fixes are unreviewed code** — a behaviour-changing one lands a test per branch it introduces.
3. **Run the repo's gates over the result** (`~/.kk-flavor/standards/quality-pipeline.md` → **The round**): nothing is pushed on red. An untrusted fork's gates do not run, and the landing then says the push went out unverified.
4. **Commit and push per `~/.kk-flavor/standards/git.md`** — your fixes are commits on top of what a reviewer has already read, never a rewrite of it. **The mode is your licence, and it lifts that file's approval before a push** — print what you pushed rather than asking to.

## Answer every thread

- **A thread the push closed gets `Done <link to the commit>` and nothing else.**
- **A thread it did not close gets a reply saying what you decided instead**, or stays open where only the author can settle it.
- **Reply inside the thread** — `gh api repos/{owner}/{repo}/pulls/<N>/comments/<comment-id>/replies -X POST -f body=…`.
- **A comment with no thread to reply into** — a review body carrying no inline comment, a plain timeline comment — is answered in **one** PR comment (`gh pr comment`) that names each comment it answers. One per orphan reads as a new topic each time.

## Hand it back

**Print it, do not run it**: the review can be re-requested with `gh pr edit <N> --add-reviewer <login>`, and that call is the human's. It pulls a person to the PR and nothing takes that back (`~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act**).

The verdict block goes in your closing reply, carrying every question only the author can settle.

**Under `review-and-address` the close is different**: say the PR is ready, then offer the two acts that remain — re-request the review, or post the verdict block as one comment. Neither happens unasked, and the scratchpad is gone by then or its surviving entries are in that reply.
