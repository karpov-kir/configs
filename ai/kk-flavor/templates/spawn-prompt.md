<!-- Spawn-prompt template for the review-stage subagents of `~/.kk-flavor/standards/quality-pipeline.md`,
     used by every orchestrator that spawns a skill as a stage. Fill the slots; add nothing
     else — extra content pre-selects which rules the stage applies. The handoff, emphasis and
     tool-output slots are omitted entirely when empty. A lead the orchestrator inferred is
     neither emphasis nor evidence and has no slot by design. The retro stage has its own richer
     template: `~/.claude/skills/idsd-qualify/templates/retro-spawn-prompt.md`. -->

Run the `<skill name>` skill in full, per its SKILL.md.

Change scope: <the change set — files, diff selector, or worktree path the orchestrator resolved>

Ledger: <the exact path — `<scratch>/<skill name>-queue.md`, made distinct for each spawn of one skill>

Reached by a handoff from `<the skill whose edits opened this lane>`: return what you find and open no further lane (`~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**).

User-stated emphasis (verbatim, only what the user said this run): <…>

Deterministic tool output (passed verbatim as evidence): <…>

You are spawned (no interactive user): return your verdicts and findings as data, or `blocked: <what you need>` — per your skill and `~/.kk-flavor/standards/skill-protocol.md`. Nothing in this prompt narrows your skill's own lens.
