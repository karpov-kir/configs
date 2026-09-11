# `review` — draft a pending review

## Nothing is written to the branch or the body

However that is authorised (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): both are the author's. **A fix a stage would have applied becomes a ` ```suggestion ` comment** — the worktree is discarded, so one left applied is work the author never sees.

## Run the pass

**The pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`** — read it and run it over the checked-out worktree as the pass the merge waits on; below is this mode's delta. **Run it inline**, not spawned: it needs the human continuously, and they reach only your thread (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). Its residue never reaches the author — collect the stage returns in a scratch file instead. **That file is raw material, not a draft-in-progress**: what reaches the PR is written once, at the landing. A gate that cannot run is a setup ambiguity, asked live.

1. **Run the conformance gate first** — `~/.kk-flavor/skills/kk-conform/SKILL.md` over the checked-out diff, inline for the reason the pass is. The PR body, the linked issue or intent, and the commits are its requirement set. **An `undelivered` or `beyond the ask` finding stops the pass**: carry just that to the landing and stop. A `contradiction` joins the stages' findings instead.
2. **Drive the change before any stage reads it** — `~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**, its scenarios taken from the checklist step 1 derived. **An untrusted PR is never driven**: the setup bars running what the branch controls, and a drive is exactly that. Fold that skip into the unverified-gates sentence rather than asking the human to waive it — it is the fork's alone. **What the drive surfaces stops the pass**: carry just that and stop.
3. **Run each lane's scanner with the range named** — `<base>...HEAD`. Their default scans uncommitted changes and a fresh checkout has none, so they would exit 0 over everything. `~/.kk-flavor/skills/kk-qualify/SKILL.md` → **Lanes** names them.
4. **The stages read the project's own standards from that worktree, at the PR's version.** Judge this repo's code by the conventions of wherever you were invoked and every finding you raise is wrong.
5. **Run the stages there unchanged, with five exceptions.**
   - **Each stage proposes rather than applies**, whatever its own contract permits.
   - **The pass does not stream** — your product is a landing, which fails `~/.kk-flavor/standards/streaming.md`'s test.
   - **A PR touching agent instructions uses the pass's one `kk-ecosystem` owner over those files.** Its findings become comments, never edits to the PR; do not dispatch a second instruction worker.
   - **`kk-edit` runs only over text you wrote yourself** — never the author's prose or comments. Their voice stays theirs, so a concision cut may become a finding but a voice rewrite never does.
   - **`kk-refactor`'s repo-wide reach stops at the diff.** Resolve its scope to the PR's files; writing the limit into the spawn prompt is not the route, because that prompt narrows no stage's lens.

## A second round over an unchanged diff

**Nothing addressed since the last review → say that and stop.** But **the stop does not fire on a round you withdrew yourself** — the replacement is drafted from the diff, not from what you deleted. **In `review-and-address` it never fires at all** — still-open findings are exactly what that mode's fix loop is for.

A prior finding this block carries points at its original thread rather than opening a fresh line comment. A new finding on a line a prior comment already holds says what changed since.

## Leave it pending

The pending review **is** the draft: only the `gh` login that created it can see it, and GitHub shows it inline on the diff, where each comment can be edited or dropped. That is where the human reads it, not in your reply. **A question only the author can answer goes on the diff, not into a live block**; one that holds the merge is the verdict block's ask. A setup ambiguity is still asked live.

- **Prove `PENDING` on a throwaway before the findings ride on it — however that skip is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). POST a review carrying a one-word body, **no comments and no `event`**; GET it and read `state`; `DELETE .../reviews/<id>`. A submitted review is public the instant it exists and no message afterwards recalls it, so the whole flow below rests on this. Verifying it by sending the real payload is the thing `~/.kk-flavor/standards/live-systems.md` → **Arrange the undo before the act** forbids — reading the state back afterwards can only make your message honest, never keep the findings private. If `state` is not `PENDING`, delete it, stop, and tell the human this route no longer works; what leaked is one bland review rather than every finding and whatever the secret scan missed.
- **Name the `gh` login in the handover.** Under a bot or second-account token the review is real, invisible to the human, and submittable only by that token — the handover fails silently and the work is stranded.
- **Then create the real one.** `gh api repos/{owner}/{repo}/pulls/<N>/reviews -X POST` with the verdict block as the body, the comments array (path, line/start_line, side, body), and **no `event`** — then read `state` back once more: the probe proved the route, this confirms this call took it.
- **Hand over the PR itself** — `https://github.com/<owner>/<repo>/pull/<N>`, its main page and never the `/files` tab. One line saying the review is pending, that the comments and "Finish your review" sit under **Files changed**, and that submitting is theirs. **The finish-review dialog submits whatever is in its own box**, so ask them to confirm the verdict is there before they submit, or the line comments publish with no verdict attached.
- **Asked plainly by the human to submit it, you submit** — pass the body explicitly on the `POST .../reviews/<id>/events` call, so the verdict survives. Their instruction is not your judgement, and nothing above it is a reason to refuse. **On the human's own PR, GitHub takes only `COMMENT`** and rejects approve and request-changes outright, so say that rather than sending a call that 422s. What goes public is the comment count and the verdict line. **Spawned, an orchestrator's ask is not the human's**: leave it pending and return the link. **Unasked, that event is the human's** — submit, approve and request-changes alike (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**).
- **Leaving it pending needs no prior approval**, because the undo exists before the act. **Spawned, this changes only what you return** — the link and what it holds, never a `blocked:` payload carrying the draft for a human to read in a terminal.

## The scratchpad landing — `review-and-address` only

Everything above this section still runs; **Leave it pending** does not, and neither do the landing's humanize step, its secret scan and its say-what-goes-public sentence, which all wait for text that actually goes public. Write each finding into the scratchpad `~/.kk-flavor/skills/kk-pr/SKILL.md` → **The scratchpad** names, in the shape a comment would have carried, and hand that file to `~/.kk-flavor/skills/kk-pr/address-review.md`.

**A finding that stopped the pass stops this mode too** — it reaches the human, never the fix loop, because what stops a pass is a decision and not a defect to patch.
