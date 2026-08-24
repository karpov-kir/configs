# Streaming a Multi-Stage Pass

The whole delta for a pass whose stages queue patches as they find them instead of returning proposals at the end. **Optional, and the caller's**: a stage streams only where its spawn prompt names a patch queue and that stage's tier in the apply order below ([skill-protocol.md](skill-protocol.md) → **Caller**). Every other rule in that file still binds.

**It pays only where stages are live at once over one change set**, trading their round trips for continuous verification. A chain that spawns its stages one after another already gets each tier boundary from the spawn it does anyway.

## The stage's half

- **Emit each proposed change as you find it**, to `<queue>/<tier>-<skill>-<seq>.patch`, with a sibling `.md` carrying its case. Nothing waits for your return. The queue sits in the scratch dir, **outside the repository** ([skill-protocol.md](skill-protocol.md) → **Queue**) — a patch file written inside it joins the change set every later stage then reviews.
- **`<seq>` never restarts.** A resumed stage carries on its own numbering, or it overwrites the patches it already queued.
- **A file, never a message.** A message's envelope names only its sender's subagent *type*, so two stages of one type are indistinguishable at the receiver. No tier order can rest on a sender that self-reports.
- **A patch, not a description.** "Extract these ten sites, with these call-site rewrites" does not survive prose, and a caller re-deriving it is doing the work rather than applying it.
- **A resume is a re-read** ([skill-protocol.md](skill-protocol.md) → **Loop**). Checking the caller's rendering of your own request is the point, and your surviving context is what it gets checked against.
- **Your verdict names the patches you queued for that file.** A verdict whose patches all landed and verified restates no finding — the queue and the diff are its record.

## The caller's half

- **Apply in tier order, gate at each boundary, then resume every live stage with what landed.**
- **A patch that no longer applies goes back to its author** to recompute or withdraw. Repairing one by hand re-derives the work and leaves you owning the result unreviewed.
- **An applied patch leaves the queue**, so what remains is what is outstanding and nothing lands twice. **Backing a tier out is the inverse of its patches, never a checkout** ([skill-protocol.md](skill-protocol.md) → **Queue**).
- **A gate's result is never recalled from a resumed stage**; re-run it yourself.
- **One objection round per tier.** Then the tier order decides, and a disagreement surviving that is a report item rather than another round.
- **Say in the pass's own output that it streamed**, and treat what breaks in the mechanism as a finding.

## A quality pass's tiers

**Full streams; fast does not** ([quality-pipeline.md](quality-pipeline.md) → **The stages**, for both modes). Full's refactor loop and the code-review between the round and refactor are the round trips this replaces; a fast pass has neither.

**Security, code-review, refactor, comments** — that order, not the stage numbering in [quality-pipeline.md](quality-pipeline.md) → **The stages**. **The resume is the re-review** that file owes for fixes applied between the round and refactor.

**Refactor joins as a tier.** Its tier boundary is the serialization a fresh spawn would otherwise give it ([quality-pipeline.md](quality-pipeline.md) → **The round**): nothing from the tiers above it is outstanding when its patches land. **Its compliance verdict is still a fresh spawn** — a surviving verdict is what verifies an applied patch and what corrupts a compliance judgment.
