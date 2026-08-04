<!-- Spawn-prompt template for idsd-qualify's stage subagents. Fill the slots; add nothing else.
     Slots 3 and 4 are omitted entirely when empty — never invent content for them. A lead the
     orchestrator inferred is neither emphasis nor evidence and has no slot by design. -->

Run the `<skill name — the role's resolved skill>` skill in full, per its SKILL.md.

Change scope: <the change set — files, diff selector, or worktree path the orchestrator resolved>

User-stated emphasis (verbatim, only what the user said this run): <…>

Deterministic tool output (passed verbatim as evidence): <…>

<!-- Retro stage only — omit all four for a review stage. The austerity above is calibrated for the review
     stages, where extra content pre-selects which rules they apply; the retro needs these to work at all,
     and a spawn that smuggles them into a prose slot is worse than a slot that admits them. -->

Run-log: <path to the factual run-log for the run under retrospect>

Scope boundary: <which earlier rounds were already retrospected, and that assessing whether their lessons took is in scope while re-reporting them is not>

Tooling this run touched: <the skills, scripts, standards, docs or prompts in the diff — or that it touched none>

Routing homes: <where an improvement goes: the human's skills and standards, their global notes, an ICE, a constitution>

You are spawned (no interactive user): return your verdicts and findings as data, or `blocked: <what you need>` — per your skill and `~/.kk-flavor/standards/skill-protocol.md`. Nothing in this prompt narrows your skill's own lens.
