---
name: idsd-finalize
description: Merge one built intent's records into the project's, then archive it. Use for "finalize the intent". The pipeline's fourth stage, after idsd-qualify — idsd-ship's `done` invokes it.
disable-model-invocation: true
argument-hint: "<NNN-slug>"
---

The last stage of a ship: what its own records learned goes up into the project's, and the ship moves to `archive/`. You orchestrate under `~/.kk-flavor/standards/skill-protocol.md`, and step 3 is `~/.kk-flavor/standards/records.md` applied rather than restated — read it whole first.

**Every `.idsd/` path here hangs off the resolved scratch root** (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**).

**Finalizing is serial across the whole clone**, because it writes records every ship shares. `report.sh merge-slot` holds that, and **everything that can ask a human happens in step 1, before the slot is taken** — a question asked inside it stalls every other ship behind a thread nobody is watching. **Step 2's re-qualify is the one exception**, and says there why it has to be.

## 1. Clear what can still refuse or ask

**The ICE's `## Follow-ups` are closed in `idsd-build`, before `idsd-qualify` stamps — not here.** **Resolving one here invalidates the stamp the gate below reads**, which then blocks on freshness and leaves the ship asking for an override on its own edit. One that surfaces only here is resolved and then sent back through `idsd-qualify`.

**Then `~/.claude/skills/idsd-qualify/scripts/report.sh gate <NNN-slug>`, and let a non-zero exit stop you.** It is the whole of what stands between this ship and a merge nothing qualified: it asks whether an untrimmed `idsd-qualify` stamped this tree, in this worktree, with the ICE at `status: approved` and no `- [ ]` open in either it or the report. **No report means no pass ran, so the answer is to run `idsd-qualify`.** A freshness, stages or unapproved-intent block is the human's to override and you ask them here; an open `- [ ]` is nobody's.

**Re-run the build's gates**, resolved the way the build resolved them (`~/.kk-flavor/standards/building.md` → **Before the loop**) — no record carries the commands. The qualify pass and the follow-up work both edited this tree since the build ran them, and a fix that broke one is invisible until they run again.

**Then check this intent's `links:`** by the rules `idsd-audit` applies set-wide. A bad link stops you; fix or route it first. Whole-set consistency stays that skill's job.

## 2. Take the slot

`report.sh merge-slot take <NNN-slug>`. **Exit 4 means another ship holds it**, and the refusal names the holder's intent and worktree — wait for it. `--force` is for a holder you have established is gone, never for one you are impatient with: it breaks into a merge that may be half-written.

**A re-qualify forced by a moved `main` happens with the slot held.** The rebase invalidates the stamp step 1 checks, and a pass takes long enough that `main` moves again — so releasing the slot to re-run the stages is what loses the race, and the ship laps without ever landing. The slot serialises landing, not the tree; holding it across the pass is the only thing that keeps the tree still long enough to finish. Every other finalize waits on that whole pass, so say what you are holding it for.

**Establishing that is yours, and the refusal cannot do it for you** — the tool started no process it could ask about. Look for a session working in the worktree it names; none, and the slot outlived its holder. **A slot is held from here until step 4 finishes**, so a session that dies in between leaves one nobody else frees.

## 3. Merge the three records upward

`decisions`, `playbook` and `language`, each of the ship's own entries against the project's. Write only through `report.sh record`, naming `project-*` for the destination.

- **It restates one already there** → `record bump` the project's entry. **That bump is the point of the whole split**: two ships independently needing one thing becomes a count, which `~/.kk-flavor/standards/records.md` → **Promotion is the exit upward** reads as a rule nobody has written down yet.
- **It says something new** → `record append`.
- **It contradicts one** — and that is never a write. Finalize has no authority to choose between a project truth and a ship's, so it goes to the human as a proposal and the ship's entry stays unmerged until they settle it.
- **The project record is full** → `~/.kk-flavor/standards/records.md` → **Reaching the cap**, whose four moves you work in its order. This is the one place the cap is judged with the whole batch visible, which is why it is judged here and not where each entry was written.

**Language needs the distinction spelled out, because a term is not a command.** The same term in the same sense is a duplicate and bumps. The same term carrying a different meaning, or two terms for one thing, is a **contradiction** — the check `idsd-audit` runs set-wide, firing here per ship and on two candidates rather than on the whole set.

**`constraints.md` is not one of the three.** A human owns its wording, so a threshold this ship earned is a proposal through `idsd-charter` and never a write from here.

## 4. Archive

Set `status: built` in the intent **first**, so what lands in the archive says what it is.

**Then `report.sh finalize <NNN-slug>`.** It deletes the ship's report, drops the stage markers, and moves the folder to `.idsd/archive/NNN-<slug>/`. The report goes because any later pass reproduces it; the ship's own three records ride along, because step 3 is allowed to leave an entry unmerged and the archived folder is then the only place that entry exists — `language.md` being the one no later pass can rebuild.

**Then regenerate `.idsd/roadmap.md`** if it exists, to `idsd-intent`'s format, which owns it. After the move rather than before, or it still lists this ship as unbuilt.

**Then land everything in one approval-gated commit** (`~/.kk-flavor/standards/git.md` → **Commits**).

**Then `report.sh merge-slot release`.** Yours to drop, because you took it: finalize releases only a slot it took for itself. One released on your behalf would open the queue at the line above, with this ship's commit still unwritten and the next ship regenerating records against a tree that does not have it yet.
