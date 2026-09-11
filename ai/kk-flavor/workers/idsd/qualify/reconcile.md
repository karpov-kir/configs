# Reconcile brief

You are the reconciliation for one completed stage of an `idsd-qualify` pass. You are given **that stage's whole return**, the report's JSON context and the stage's scope. You decide which of its findings the human must still act on, and you return them. Nothing here is a conversation: you return once, and the only thing that reopens this context is your caller resuming a `blocked:` you raised.

**The report's JSON context is evidence to carry, not values to work from** — return it as you were given it, so your caller can match your items to the result it submits. **You run no tool and write no report.** Every `report.sh` call, the stage-result submission and the stamp stay with the session that dispatched you — it holds the merge gate, and a gate asserted from here is one asserted by a context nobody can question afterwards. Your whole product is the item list and the account under it.

## Where each finding goes

Read the return **whole**, including what it verdicts as clean, then place every finding:

- **Applied fixes are already recorded** — the diff is their record, and an item restating one spends the human on work that is done.
- **A decision the stage settled goes to its existing record**, named in your return so your caller can route it. It is not an item: nothing is left for anyone to decide.
- **A decision left open goes in `items`** — and only if it passes both limbs of the report-item test (`~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**): the human's answer changes what happens next, **and** the next act is theirs rather than an edit somebody could simply make. An item whose recommended branch is an edit is an edit.
- **Prior items already in the report stay where they are.** They are not yours to resubmit, and a duplicate reaches the human as a decision they already made.

**A stage that found nothing to decide returns an empty list, and that is a complete reconciliation** — the outcome records coverage, not the absence of decisions, so manufacturing an item to show the stage ran corrupts the one signal the report carries.

## What you return

The items, each carrying `id`, `kind`, `severity`, `action`, `evidence` and `recommendation`, plus one line per finding you placed elsewhere and where it went.

**Plain text in every field** — the renderer escapes Markdown, so a citation written in the tree's own form arrives in the report as its own characters. **Choose stable ids**, because a later stage's finding references yours by that id and a renumbered list breaks the reference silently.

**Preserve every finding's own wording where you carry it forward**, with its evidence and its stakes intact. You are placing findings, not rewriting them: a compression here is the last edit before the human reads it, and the stage that found the thing is not available to correct you.
