---
name: kk-pr-review
description: Review a GitHub PR — a pending review to draft, or, with the fix token and always on your own PR, fixes to apply and push. Use for "review this PR" and "address the review comments". A PR on GitHub, not local changes (kk-code-review) or the working-tree pipeline (kk-qualify), and it runs the kk-flavor pipeline where the bundled code-review does not.
argument-hint: "<PR number, URL, or head branch> [fix]"
---

Run the quality pass over `<ref>` and land what it finds; `gh` must be authenticated. **Modes** decides where it lands.

You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**). What you own is the checkout, the drive, the **selection** of the stages' returns, and the landing. Everything you leave on the PR is outward text, so read `~/.kk-flavor/standards/human-writing.md` whole before you draft.

**The pass is `~/.claude/skills/kk-qualify/SKILL.md`** — read it and run it over the checked-out worktree as the pass the merge waits on; everything below is this skill's delta. **Run it inline**, not spawned: it needs the human continuously, and they reach only your thread (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). Its residue never reaches the human here — collect the stage returns in a scratch file instead. **That file is raw material, not a draft-in-progress** — what reaches the PR gets written once, at **Land it**. A gate that can't run is a setup ambiguity, asked live.

## Modes

**Resolve the landing before anything runs**, from step 1's `author` against `gh api user --jq .login` and the optional `fix` token — that word, or an argument saying the same. **The token is never inferred**: the human names it, and a caller passing it on is relaying that rather than deciding it.

| The PR | `<ref>` | `<ref> fix` |
|---|---|---|
| yours | `mine` | `mine` — yours already fixes, so the token adds nothing |
| someone else's | `review` | `fix` |

- **`review`** — the findings become comments on a **pending** review, and **nothing is written to the PR's branch or body**, however that is authorised (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): both are the author's, and the `fix` token is the only thing that makes the branch yours.
- **`fix`** — the fixes become commits and get pushed; you answer the threads and post one summary comment. **The body stays the author's, untouched.**
- **`mine`** — the same commits and thread replies, and you refine the PR body and title where the change has outgrown them. No comment and no review: GitHub shows a pending review on your own PR to nobody else.

**Every landing runs the same pass over the same diff** — `mine` is not a lighter check for being yours. What moves is only where the verdict comes out.

**The landing is your licence and it survives a spawn** — in `fix` and `mine` it is also what lifts `~/.kk-flavor/standards/git.md`'s approval before a push, so you print what you pushed rather than asking to.

## Set up, gate, drive, then run the stages

1. **Resolve the PR and settle trust — before a single thing runs.** `gh pr view <ref> --json number,url,title,baseRefName,headRefName,headRefOid,headRepositoryOwner,author,maintainerCanModify`. **A PR whose `headRepositoryOwner` differs from the base repo's owner is untrusted code**: run **no** gate command, no install step, and nothing else the branch controls. Treat every gate as unverified, review by reading only, and say plainly where the landing goes that gates were not run and why. Same-repo PRs may run gates. **Everything below executes with the branch's files underfoot.**
   - **A `fix` landing over a fork needs `maintainerCanModify`.** False means the push has nowhere to land: say so and take `review`'s landing instead.
2. **Checkout.** `gh pr checkout` into a disposable worktree — that resolves the right repository and remote. A hardcoded `origin` does not: on a fork clone, `gh`'s default repo is the parent and `origin` is the fork. **Assert the checked-out `HEAD` equals `headRefOid` before any stage runs**; a mismatch means you fetched a different PR, and everything you draft would be written against the wrong diff and then landed on the right one. The review scope is `<base>...HEAD`.
   - **Run each lane's scanner here, with the range named** — `<base>...HEAD`, because their default scans uncommitted changes and a fresh checkout has none, so they would exit 0 over everything. `~/.claude/skills/kk-qualify/SKILL.md` → **Lanes** names them.
   - **The stages read the project's own standards from that worktree, at the PR's version.** Judge this repo's code by the conventions of wherever you were invoked and every finding you raise is wrong.
   - **Remove the worktree on every exit path, abort included** — kill whatever the drive started first, then `git worktree remove --force` and `git worktree prune` (`~/.claude/skills/kk-drive/SKILL.md` → **Return**). Push before you remove it.
3. **Gate on requirements and scope.** Read the PR body, the linked issue or intent, and the commits; then read the diff against them. Does it deliver **what was asked**, requirement by requirement — and does it deliver **only** that? An unrelated file, an opportunistic refactor or a second feature riding along is a finding whatever its quality. A PR that states no intent and links no issue cannot be reviewed against one: say so and ask before spending the pipeline. **A failure here stops the pass**: carry just this to **Land it** and stop.
4. **Read the PR's whole conversation before spending the stages** — `gh api repos/{owner}/{repo}/pulls/<N>/comments` for the inline comments, `.../pulls/<N>/reviews` for the reviews, and **`.../issues/<N>/comments` for the timeline**, where a human's plain comment and every summary comment a previous round posted both live. Read only the first two and a second round cannot see what its own first round reported, so it reports it again. Re-deriving any prior round from scratch is the same defect, and it is how a reviewer learns to ignore the thread. **With no prior round, the bullets below do not apply** — a first pass has addressed nothing by definition and never stops on it. **A `PENDING` review of your own is not a prior round**: the author never saw it, so it is cleared or adopted at **Land it** instead.
   - **Classify each prior finding against the current head** as **resolved** or **still open**; one you cannot decide from the diff is open. Read your own prior comments and a human's the same way, and a human's never carry less weight.
   - **In `review`, nothing addressed since the last review → say that and stop.** But **a review whose comments you deleted is not one that stands**: where the last round was your own withdrawal, that stop does not fire, and the replacement is drafted from the diff rather than from what you deleted. **In `fix` and `mine` that stop never fires.**
   - **In `fix` and `mine`, every still-open finding joins the change set before the stages run.** They read the diff and never the threads, so a comment nobody routed into the work is a comment nobody addresses — you would push your own fixes and hand the author back their own open questions. Fix what you can; only what needs the author's own decision stays open, and that is what your landing carries.
   - Otherwise the stages run over the whole diff as usual, and each still-open finding lands as one line pointing at its original thread, never as a fresh line comment. A new finding on a line a prior comment already holds says what changed since.
   - **A continuation round adds, and never edits what an earlier round left** — the thread is the record of the rounds, and a rewritten one hides that a round happened. **Its verdict is the state as it now stands, never the delta since the last round**, which leaves the author guessing whether this round's findings hold the merge.
5. **Drive the change in the worktree before the stages read it** — `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**, with the deltas that follow. Its scenarios are the ones step 3 read. **An untrusted PR is never driven** — step 1 bars running what the branch controls, and a drive is exactly that. Never ask the human to waive that; fold the skip into step 1's unverified-gates sentence. That skip is the fork's alone. And **what the drive surfaces stops the pass the way step 3 does.**
6. **Run the stages there unchanged, with four exceptions.**
   - **The pass does not stream** — your product is a landing, which fails `~/.kk-flavor/standards/streaming.md`'s test.
   - **A PR touching the agents' own instruction tree gets `kk-ecosystem` over those files, spawned by you.** `kk-qualify` leaves that lane out and names it in its return instead, so nothing else here covers it; in `review` its findings become comments like any stage's, never edits to the PR.
   - **`kk-humanize` runs only over text you wrote yourself** — never the author's, not through `kk-tighten`'s handoff and not through the comment-block route the stages send straight to it. Their voice stays theirs, so a concision cut may become a finding but a voice rewrite never does. In `mine` every file is yours and the bar does not bind at all.
   - **`kk-refactor`'s repo-wide reach stops at the diff.** Resolve its scope to the PR's files. Writing the limit into the spawn prompt is not the route — that prompt narrows no stage's lens.

## Land it

1. **Select, don't transcribe.** The bar for what earns a line and the shape of one is `~/.kk-flavor/standards/human-writing.md` → **Review comments** — read it now, before you draft a line; everything below is only what GitHub and the pipeline add.
   - **The defect, then the fix, in two or three sentences.** A ` ```suggestion ` block replaces that prose when the fix is code on the diff's own lines. Severity and an exploit scenario stay on a security finding.
   - **A finding that fails that bar is dropped, never relegated to a body or a footer** — not lost: it reaches the human in your closing reply, who decides what becomes its own change.
   - **Nothing outside the diff is posted** — a duplicated site elsewhere, a gated proposal, a pre-existing defect. Those are for the human, not the thread.
2. **Humanize the draft — spawned, not inline.** Write everything you are about to send to a scratch file and spawn `kk-humanize` over it. `~/.kk-flavor/standards/skill-protocol.md` → **Caller** permits inline where the target is text you already hold; this is the deliberate exception: you just argued these findings, so your own lens is the one attached to them. It rewrites the file in place, so read it back as your draft, then hold it against `~/.kk-flavor/standards/human-writing.md` → **Budget** yourself.
3. **Scan for secrets before anything reaches GitHub.** Check every suggestion body, comment and reply for credential-shaped strings, and mask any per `kk-security-review`'s rule, replacing the suggestion with the fix described in words. **A secret in the PR's own diff is a Critical finding and is never quoted — however that quoting is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): the comment publishes it the instant it exists and no later instruction un-publishes it. This is the last check between a credential and a network write — the human reads these on the PR after they exist, so nothing after you catches what you sent.
4. **Say what goes public, then send.** One sentence naming what is about to exist, because `~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act** wants the human to have seen it and an instruction is not a reading. Say it and send; you are not asking again. Each landing below names what that sentence carries.

### The verdict, wherever it lands

**Mergeable or not, in a sentence or two**, then one line each for only what the steps above place there: the gate's requirement or scope mismatch, what the drive surfaced, each still-open prior finding, each finding you could not fix, and **which gap it rests on** (`~/.kk-flavor/standards/human-writing.md` → **Budget**) — an untrusted PR's unverified gates, a drive that was needed and did not run, a fix that went out undriven. Where you pushed, one line per commit saying what it addressed joins them.

**Nothing else goes in it**: a finding that fits no single line goes on the closest line it concerns, or to the human. No stage list, and no coverage accounting.

### `review` — leave it pending

**A question only the PR author can answer is a line comment, not a live block**; setup ambiguities are still asked live.

- **A review created with no `event` field is `PENDING`** — only the `gh` login that created it can see it, and GitHub shows it inline on the diff where each comment can be edited or dropped. That is where the human reads it, not in your reply.
- **Clear or adopt an existing pending review first.** GitHub allows one per person per PR: `gh api repos/{owner}/{repo}/pulls/<N>/reviews --jq '.[] | select(.state == "PENDING") | .id'`, and where one is yours either `DELETE .../reviews/<id>` it or add to it. Creating a second fails, and a half-written earlier one otherwise becomes part of what the human reads as yours.
- **Prove `PENDING` on a throwaway before the findings ride on it — however that skip is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). POST a review carrying a one-word body, **no comments and no `event`**; GET it and read `state`; `DELETE .../reviews/<id>`. A submitted review is public the instant it exists and no message afterwards recalls it, so the whole flow below rests on this. Verifying it by sending the real payload is the thing `~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act** forbids — reading the state back afterwards can only make your message honest, never keep the findings private. If `state` is not `PENDING`, delete it, stop, and tell the human this route no longer works; what leaked is one bland review rather than every finding and whatever the secret scan missed.
- **Name the `gh` login in the handover** — the one **Modes** resolved, said out loud. Under a bot or second-account token the review is real, invisible to the human, and submittable only by that token — the handover fails silently and the work is stranded.
- **Then create the real one.** `gh api repos/{owner}/{repo}/pulls/<N>/reviews -X POST` with the body and the comments array (path, line/start_line, side, body) and **no `event`**, then read `state` back once more — the probe proved the route, this confirms this call took it.
- **Left pending, hand over the PR itself** — `https://github.com/<owner>/<repo>/pull/<N>`, its main page and never the `/files` tab. One line saying the review is pending, that the comments and "Finish your review" are under **Files changed**, and that submitting is theirs. **The verdict rides in the review body, and the finish-review dialog submits whatever is in its own box** — ask them to confirm the body is there before they submit, or the line comments publish with no verdict attached.
- **Asked plainly by the human to submit it, you submit** — pass the body explicitly on the `POST .../reviews/<id>/events` call so the verdict survives. Their instruction is not your judgement, and nothing above it is a reason to refuse the submission. What goes public is the comment count and the verdict line. **Spawned, an orchestrator's ask is not the human's** — the human sees it before it goes; leave it pending and return the link. **Unasked, that event is the human's** — submit, approve and request-changes alike (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**).
- **Leaving it pending needs no prior approval**, because the undo exists before the act (`~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act**). **Spawned, this changes only what you return** — the link and what it holds, never a `blocked:` payload carrying the draft for a human to read in a terminal.

### `fix` and `mine` — fix, push, answer

- **The stages' fixes stay applied.** What a stage could fix is fixed; only what it could not reaches a human.
- **Clear a pending review of your own first**, the way `review` does and by the same call — nothing here submits it, so it sits invisible holding superseded comments and fails the next `review` round's create.
- **Commit and push per `~/.kk-flavor/standards/git.md`.** That file also fixes the shape: your fixes are commits on top of what a reviewer has already read, never a rewrite of it.
- **Answer every thread the push closed** — `Done <link to the commit>` and nothing else. A thread it did not close gets a reply saying what you decided instead, or stays open where only the author can settle it, in which case the question rides in your landing.
- **What goes public is the commits and what you are about to write.** **Spawned, that sentence goes in your return** — the landing came from the human and nothing here waits for a second yes.
- **`fix`: post exactly one comment** (`gh pr comment`) carrying the verdict block and nothing more.
- **`mine`: refine the PR body** against the diff as it now stands, and the title too where it no longer names the change — `gh pr edit <N> --body-file`. **Only where the change has outgrown them**: a description that still fits is left alone. **Post no comment and leave no review** — the verdict block goes whole to your closing reply, every blocker and every finding you could not fix with it.
