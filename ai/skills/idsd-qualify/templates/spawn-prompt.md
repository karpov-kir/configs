<!-- Spawn-prompt template for idsd-qualify's review-stage subagents. Fill the slots; add nothing
     else — extra content pre-selects which rules the stage applies. Slots 3 and 4 are omitted
     entirely when empty — never invent content for them. A lead the orchestrator inferred is
     neither emphasis nor evidence and has no slot by design. The retro stage has its own richer
     template: retro-spawn-prompt.md. -->

Run the `<skill name — the role's resolved skill>` skill in full, per its SKILL.md.

Change scope: <the change set — files, diff selector, or worktree path the orchestrator resolved>

User-stated emphasis (verbatim, only what the user said this run): <…>

Deterministic tool output (passed verbatim as evidence): <…>

You are spawned (no interactive user): return your verdicts and findings as data, or `blocked: <what you need>` — per your skill and `~/.kk-flavor/standards/skill-protocol.md`. Nothing in this prompt narrows your skill's own lens.
