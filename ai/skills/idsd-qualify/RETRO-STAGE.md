# Retro stage

The pipeline's last stage, and the whole of it — `SKILL.md` → **Running a pass** carries only *when* it runs.

Write a terse run-log — `idsd-retro`'s **Input** defines its content — then spawn `idsd-retro` **fresh**, building its prompt from `templates/retro-spawn-prompt.md` (this skill's dir) and filling only its slots. What the log omits becomes a blind spot. Writing it:

- **The stage numbering in `~/.kk-flavor/standards/quality-pipeline.md` is not the execution order.** Stage 3's comment pass runs after stage 4: write the log once that pass has returned, and name it among the stages.
- **State a defect the run hit and how it surfaced, as an event** — never grade the run or frame what a stage "should" have caught.
- **For anything that appears in no diff** — server state above all — record the command and what it printed, not the conclusion you drew.
- **Name each stage's *applied* changes, or point at them.**
- **Name each *returned* finding with its disposition**, never a count.
- **Derive the touched-files list from `git diff --stat`** against the base, never from memory.

**On its return, split by kind — only a defect in *this change* gates**, recorded as a **Decide** like any other stage's. An improvement *for next time* routes to one of the homes the spawn prompt named, never reported as filed unless that home contains it; a skill/standard/tooling one is proposed in the gate message and applied per `~/.kk-flavor/standards/ecosystem.md`. A run-improvement is never a gating `- [ ]`; its one line goes to the decision log.
