<!-- Spawn-prompt template for idsd-qualify's review-stage subagents. Fill the slots; add nothing
     else — extra content pre-selects which rules the stage applies. Slots 4 and 5 are omitted
     entirely when empty — never invent content for them. A lead the orchestrator inferred is
     neither emphasis nor evidence and has no slot by design. The retro stage has its own richer
     template: retro-spawn-prompt.md.

     The ledger slot exists because the orchestrator has to find that file again — to check the
     stage's progress, and to resume it. Naming the exact path removes the guess: one run looked
     under the role name while the stage wrote under the resolved skill name, reported to two
     agents that no ledger existed, and left a stale half-finished ledger at the path a
     crash-resume would have read. -->

Run the `<skill name — the role's resolved skill>` skill in full, per its SKILL.md.

Change scope: <the change set — files, diff selector, or worktree path the orchestrator resolved>

Ledger: <the exact path — `<scratch>/<resolved skill name>-queue.md`>

User-stated emphasis (verbatim, only what the user said this run): <…>

Deterministic tool output (passed verbatim as evidence): <…>

You are spawned (no interactive user): return your verdicts and findings as data, or `blocked: <what you need>` — per your skill and `~/.kk-flavor/standards/skill-protocol.md`. Nothing in this prompt narrows your skill's own lens.
