# Retro stage

Write a terse run-log; `~/.claude/skills/idsd-retro/SKILL.md` → **Input** defines its content. Then spawn `idsd-retro` **fresh**, with a prompt built from the slots of `~/.kk-flavor/templates/spawn-prompt.md` plus the extra ones in `templates/retro-spawn-prompt.md` (this skill's dir), and nothing else. Writing the log:

- **The run is everything that produced this change, not this pass.** Under `idsd-ship` it starts at the human's ask and takes in `idsd-intent`'s grilling and `idsd-build`'s clarify gate; a pass reached any other way starts wherever its own work did. `idsd-retro`'s friction lens judges round-trips and communication across the whole run, so a log that opens at the first quality stage hides the costliest ones and leaves that lens nothing to find. Draw it from the report's `Decide` items, the decision log and the session — never from memory alone.
- **The stage numbering in `~/.kk-flavor/standards/quality-pipeline.md` is not the execution order.** Stage 3's comment pass runs after stage 4: write the log once that pass has returned, and name it among the stages.
- **Never grade the run**, and never frame what a stage "should" have caught.
- **For anything that appears in no diff** — server state above all — record the command and what it printed, not the conclusion you drew.
- **Name each stage's *applied* changes, or point at them.**
- **Name each *returned* finding with its disposition**, never a count.
- **Derive the touched-files list from `git diff --stat`** against the base, never from memory.

**On its return, split by kind — only a defect in *this change* gates**, recorded as a **Decide** like any other stage's. An improvement *for next time* routes to one of the homes the spawn prompt named; never report it as filed unless that home contains it. It never gates: no `- [ ]`, and its one line goes to the decision log. A skill, standard or tooling improvement is proposed in **the pass's own closing message** — `~/.claude/skills/idsd-qualify/SKILL.md` → **After the pass** standalone, and among ship's gate-message items under a ship — as one line, and applied per `~/.kk-flavor/standards/ecosystem.md`.
