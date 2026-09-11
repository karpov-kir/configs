---
name: kk-pr
description: Work a GitHub PR — draft a pending review, address the review comments and push, or refine the description. Use for "review this PR", "address the review comments", "rewrite the PR description". A PR on GitHub, not local changes (kk-qualify's code-review lane), the working-tree pipeline (kk-qualify), or text you already hold (kk-edit), and it runs the kk-flavor pipeline where the bundled code-review does not.
argument-hint: "<review|address-review|refine-description|review-and-address> [PR number, URL, or head branch]"
---

**Runs:** inline — landing

`$ARGUMENTS` opens with a **required** mode — one of the four below, or an argument plainly saying the same; with neither, ask which rather than guessing from the PR. What follows names the PR; with nothing after it, the current branch's. `gh` must be authenticated.

**Authorship settles nothing** — the mode is the whole licence, on your own PR and on anyone else's. It changes only what GitHub lets you write to, which **Set up** step 1 reads.

You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**). Everything you leave on the PR is outward text, so read `~/.kk-flavor/standards/human-writing.md` whole before you draft a line.

## Modes

| Mode | Its file | What lands |
|---|---|---|
| `review` | `~/.kk-flavor/skills/kk-pr/review.md` | a **pending** review: one comment per finding, the verdict in its body |
| `address-review` | `~/.kk-flavor/skills/kk-pr/address-review.md` | commits pushed to the PR's branch, one reply per thread |
| `refine-description` | `~/.kk-flavor/skills/kk-pr/refine-description.md` | the PR's body, and its title where the change outgrew it |
| `review-and-address` | both of the first two | commits pushed; none of your own findings posted |

**Read the mode's file and take it as the whole delta for that path.** `review-and-address` runs `review.md` with its landing redirected to the scratchpad below, then `address-review.md` over that file's findings alongside the threads already on the PR.

**Everything else in this file binds every mode.**

### The scratchpad — `review-and-address` only

`${XDG_STATE_HOME:-~/.local/state}/kk-flavor/pr/<owner>-<repo>-<N>.md` — named, not invented, so a later invocation finds it (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**). One entry per finding — its file, its line, the defect and the fix — and nothing else. **Delete an entry as its fix lands or its question reaches the human, and the file when it empties**: one left behind is a finding the next run addresses twice.

## Set up

1. **Resolve the PR and settle trust — before a single thing runs.** `gh pr view <ref> --json number,url,title,body,baseRefName,headRefName,headRefOid,headRepositoryOwner,author,maintainerCanModify`. **A PR whose `headRepositoryOwner` differs from the base repo's owner is untrusted code**: run no gate command, no install step, and nothing else the branch controls. Read the code rather than running it, treat every gate as unverified, and say so where the landing goes. Same-repo PRs may run gates. **Everything below executes with the branch's files underfoot.**
   - **A mode that pushes needs `maintainerCanModify` over a fork.** False means the push has nowhere to land: say so and stop before the work, not after it.
2. **Check the branch out, where the mode reads code.** `gh pr checkout` into a disposable worktree — that resolves the right repository and remote, where a hardcoded `origin` does not: on a fork clone, `gh`'s default repo is the parent and `origin` is the fork. **Assert the checked-out `HEAD` equals `headRefOid` before anything reads it**; a mismatch means you fetched a different PR, and every line you draft is written against the wrong diff and then landed on the right one. The scope is `<base>...HEAD`.
   - **Remove the worktree on every exit path, abort included** — kill whatever the drive started first, then `git worktree remove --force` and `git worktree prune`. Push before you remove it.
   - `refine-description` reads `gh pr diff <N>` and no code, so it skips this step.

## Continue the round

**Read the PR's whole conversation before spending anything** — `gh api repos/{owner}/{repo}/pulls/<N>/comments` for the inline comments, `.../pulls/<N>/reviews` for the reviews, and **`.../issues/<N>/comments` for the timeline**, where a human's plain comment and every summary a previous round posted both live. Read only the first two and a second round cannot see what its own first round reported, so it reports it again. Re-deriving a prior round from scratch is the same defect.

- **Classify each prior finding against the current head** as **resolved** or **still open**; one you cannot decide from the diff is open. Read your own prior comments and a human's the same way.
- **A scratchpad on disk for this PR is a prior round too**, and its entries are still open by definition — nothing deleted them.
- **A `PENDING` review of your own is not one** — the author never saw it, so nothing in it carries forward; the landing clears it.
- **A continuation adds, and never edits what an earlier round left** — the thread is the record of the rounds, and a rewrite hides that one happened.
- **Its verdict is the state as it now stands, never the delta since the last round**, which leaves the author guessing whether this round's findings hold the merge.
- **`refine-description` reads the conversation for what it says about the body**, and classifies nothing.

## Land it

**Clear a `PENDING` review of your own first** — or, in `review`, add to it. Every mode that reads code does this; `refine-description` leaves it. GitHub allows one per person per PR, so creating a second fails, and a half-written earlier one otherwise becomes part of what the human reads as yours. `gh api repos/{owner}/{repo}/pulls/<N>/reviews --jq '.[] | select(.state == "PENDING") | .id'`, then `DELETE .../reviews/<id>`.

1. **Select, don't transcribe.** The bar for what earns a line and the shape of one is `~/.kk-flavor/standards/human-writing.md` → **Review comments**, or, in `refine-description`, that file's **Change descriptions (PRs)**; below is only what GitHub adds.
   - **The defect, then the fix, in two or three sentences.** A ` ```suggestion ` block replaces that prose when the fix is code on the diff's own lines. Severity and an exploit scenario stay on a security finding.
   - **A finding that fails that bar is dropped, not lost** — it reaches the human in your closing reply, who decides what becomes its own change.
   - **Nothing outside the diff is posted as a line comment** — a duplicated site elsewhere, a gated proposal, a pre-existing defect. What the diff owes is the verdict block's.
2. **Edit the draft inline with `kk-edit`.** The target is text you already hold. Preserve every finding, severity and required decision; read the final draft before sending. An independent rewrite is warranted only when unresolved wording needs another judgment.

3. **Scan for secrets before anything reaches GitHub.** Check every suggestion body, comment and reply for credential-shaped strings, and mask any per `~/.kk-flavor/workers/security-review.md`'s secret-handling rule, replacing the suggestion with the fix described in words. **A secret in the PR's own diff is a Critical finding and is never quoted — however that quoting is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): the comment publishes it the instant it exists and no later instruction un-publishes it. This is the last check between a credential and a network write — nothing after you catches what you sent.
4. **Say what goes public, then send.** One sentence naming what is about to exist. `~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act** wants the human to have seen it, and an instruction is not a reading. Say it and send; you are not asking again. **Spawned, that sentence goes in your return** — the mode came from your caller and nothing here waits for a second yes.

### The verdict block

**The verdict, then only what the author has to act on or decide.**

**No line says what you checked and found correct** — a block reciting the round buries the one line that asks for something.

**Mergeable and nothing open: that one line is the block**, in your own words.

**Not mergeable: say so and ask for the findings on their lines.**

**One line each, and only these:**

- **A prior finding the author has neither answered nor changed**, pointing at its original thread. One they answered and declined is settled by that answer unless it still holds the merge — then it is the ask, not a re-argument.
- **A finding with no line to anchor to**: a requirement in the body or the linked issue the diff does not deliver, a change the diff owes in a file it never touches.
- **A question only the author can answer, where it decides the merge** — the verdict is then not mergeable and the question is the ask. One that does not decide it goes where the mode's file says.
- **What stopped the pass.**
- **Which gap the verdict rests on** (`~/.kk-flavor/standards/human-writing.md` → **Budget**) — an untrusted PR's unverified gates, a drive that was needed and did not run, a fix that went out undriven. **A red or missing check is one of these** — `gh pr checks <N>`, named because *mergeable* is partly a claim about it. A green board is not restated, and a mergeable verdict still carries the gap.
- Where you pushed, one line per commit saying what it addressed.

**Nothing else goes in it.** No stage list.

**Where the block goes is the mode's**, and `refine-description` produces none.
