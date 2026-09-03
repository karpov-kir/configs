# Streaming a Multi-Stage Pass

The whole delta for a pass whose stages queue patches as they find them instead of returning proposals at the end. A stage streams **exactly where its spawn prompt names a patch queue and that stage's tier** in the apply order below ([skill-protocol.md](skill-protocol.md) → **Caller**). Every other rule in that file still binds.

**It pays only where stages are live at once over one change set**, trading their round trips for continuous verification. A chain that spawns its stages one after another already gets each tier boundary from the spawn it does anyway. **The caller is whoever spawns the round's stages** — not a router that spawns the whole pass as one skill. **Where the test passes, you stream.**

## The stage's half

- **Emit each proposed change as you find it**, to `<queue>/<tier>-<skill>-<seq>.patch`, **your tier being the token your spawn prompt named** and never one you coin, with a sibling `.md` carrying its case. Nothing waits for your return. **Write the `.md` first**: the caller applies on arrival, so a patch that lands before its case is one judged without it. The queue is the directory your spawn prompt named, in the scratch dir and **outside the repository** ([skill-protocol.md](skill-protocol.md) → **Queue**) — a patch file written inside it joins the change set every later stage then reviews.
- **`<seq>` never restarts.** A resumed stage carries on its own numbering, or it overwrites the patches it already queued.
- **A file, never a message.** A message's envelope names only its sender's subagent *type*, so two stages of one type are indistinguishable at the receiver. No tier order can rest on a sender that self-reports.
- **A proposal your own license gates never enters the queue** — it returns as a proposal. The caller applies on arrival, so queuing it lands it unasked, which is the thing the gate exists to stop ([skill-protocol.md](skill-protocol.md) → **Caller**).
- **Computed against the tree you read, not against the base commit.** A `git diff` over the change set emits the change set, which never applies to a tree that already holds it — and the caller bounces it to you forever. Diff your edited copy against the file as you read it.
- **A patch, not a description.** "Extract these ten sites, with these call-site rewrites" does not survive prose, and a caller re-deriving it is doing the work rather than applying it.
- **A change you did not queue is as likely your caller mid-apply as another lane.** Applying on arrival means the tree moves continuously, so a fix that appears without you is not evidence someone else owns it. Withdrawing your own patch on that reading retracts work that had already landed. Ask before you withdraw.
- **A resume is a re-read** ([skill-protocol.md](skill-protocol.md) → **Loop**). Checking the caller's rendering of your own request is the point, and your surviving context is what it gets checked against.
- **Your verdict names the patches you queued for that file.** A verdict whose patches all landed and verified restates no finding — the queue and the diff are its record.

## The caller's half

- **Apply each patch as it arrives**, never when a tier completes. A queue holding applicable work while you wait for a stage to return is the batching this whole path exists to remove — and a tier is only complete when its stage returns, so waiting on one costs you the entire stage.
- **A patch whose `.md` has not arrived waits**, however cleanly it would apply. Still caseless when its author returns, it goes back to them.
- **Apply with reduced context (`git apply -C1`) before calling a patch inapplicable.** Patches to one file drift against each other through hunk context alone, on regions that never overlap — and a strict apply then bounces a higher tier because a lower one landed first, inverting the order this whole path exists to enforce. Reduced context absorbs drift and still refuses a patch whose target is gone.
- **The tier order resolves conflicts; it does not schedule.** A higher tier wins the lines it touches: back the lower tier's patch out by its inverse, apply the higher one, and send the loser to its author. Nothing waits on a conflict that may never happen.
- **Gate and broadcast at the tier boundary, not per patch.** A suite run and a re-read for every patch costs more than the round trips it saves; a broadcast per tier is a handful.
- **A patch its author withdraws after you applied it comes back out**, by its inverse, and whatever its landing displaced goes back in. A withdrawal is not a conflict, so the tier order does not settle it — the author's retraction does, whichever tier they are.
- **A patch that still does not apply goes back to its author** to recompute or withdraw. Repairing one by hand re-derives the work and leaves you owning the result unreviewed.
- **Name a distinct queue for each pass you run** — the scratch dir is per session, and a second pass inheriting the first's withdrawn patches applies them unasked.
- **An applied patch leaves the queue for an applied set you keep**, so what is still queued is what is outstanding, nothing lands twice, and every inverse stays available. **Backing a tier out is the inverse of its patches, never a checkout** ([skill-protocol.md](skill-protocol.md) → **Queue**).
- **A gate's result is never recalled from a resumed stage**; re-run it yourself.
- **One objection round per tier.** Then the tier order decides, and a disagreement surviving that is a report item rather than another round.
- **Say in the pass's own output that it streamed**, and treat what breaks in the mechanism as a finding.

## A quality pass's tiers

**A pass that would otherwise run refactor after its round streams instead** — that stage, and the code-review of the fixes applied before it, are the round trips this replaces. Refactor runs over any changed code, so a pass over code streams; a change set holding none has neither round trip, and the test above fails there. **It fails too where the pass's product is a review rather than applied edits** — patches landing on arrival move the diff that review is written against.

**`security`, `code-review`, `refactor`, `comments`** — that order, not the stage numbering in [quality-pipeline.md](quality-pipeline.md) → **The stages**, and those four tokens verbatim in a patch's filename. **The resume is the re-review** that file owes for fixes applied between the round and refactor.

**Refactor joins as a tier.** Its tier boundary is the serialization a fresh spawn would otherwise give it ([quality-pipeline.md](quality-pipeline.md) → **The round**): nothing from the tiers above it is outstanding at that boundary. **Its compliance verdict is still a fresh spawn** — a surviving verdict is what verifies an applied patch and what corrupts a compliance judgment.

**A runtime-behaviour claim is re-driven at the tier boundary of its own tier**, there being no round return to bring it back to.
