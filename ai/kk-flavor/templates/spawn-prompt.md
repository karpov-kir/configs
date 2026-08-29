<!-- Fill the slots; add nothing else — what you add yourself pre-selects which rules the stage
     applies. A lead the orchestrator inferred is exactly that, however useful it looks. The
     handoff, patch-queue, emphasis and tool-output slots are omitted entirely when empty. -->

Run the `<skill name>` skill in full, per its SKILL.md.

Change scope: <the change set the orchestrator resolved — files, diff selector, worktree path, …>

Ledger: <the exact path — `<scratch>/<skill name>-queue.md`, made distinct for each spawn of one skill>

Patch queue: <the directory, and this stage's tier — naming it is what puts the stage in streaming mode (`~/.kk-flavor/standards/streaming.md`)>

Reached by a handoff from `<the skill whose edits opened this lane>`: return what you find and open no further lane (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**).

User-stated emphasis (the human's own words this run, verbatim, or the licence a standing instruction they invoked carries, quoted from the section that states it): <…>

Deterministic tool output (passed verbatim as evidence): <…>

You are spawned (no interactive user): return your verdicts and findings as data, or `blocked: <what you need>` — per your skill and `~/.kk-flavor/standards/skill-protocol.md`. Nothing in this prompt narrows your skill's own lens, and the emphasis slot above carries the human's authority, not your caller's inference.
