---
name: idsd-finalize
description: Land an already qualified intent through a direct merge or a PR, and archive its records. Use for "finalize the intent" or an explicit final checkpoint. Waits for verified landing; the broader ship lifecycle and throwaway cleanup belong to idsd-ship.
disable-model-invocation: true
argument-hint: "<NNN-slug>"
---

An explicit request to finalize invokes this skill directly; the invocation marker disables automatic selection, not a human's natural-language request.

The last stage of a ship: what its own records learned goes up into the project's, and the ship moves to `archive/`. You orchestrate under `~/.kk-flavor/standards/skill-protocol.md`, and step 3 is `~/.kk-flavor/standards/records.md` applied rather than restated — read it whole first.

**Every `.idsd/` path here hangs off the resolved scratch root** (`~/.kk-flavor/skills/idsd-qualify/SKILL.md` → **Report**).

**Finalizing is serial across the whole clone**, because it writes records every ship shares. Use `report.sh merge-slot` for record mutation and landing. Settle human decisions outside the slot; release it before a new question or a PR wait. Step 2 covers requalification while holding it.

## 1. Clear what can still refuse or ask

On resume, reconcile the recorded branch or PR with the target before rerunning preparation. Follow [pr.md](pr.md) for a pending PR. If a direct merge already landed, finish only the remaining archive work. Missing qualification evidence requires a fresh pass; a prepared archive alone never supplies it.

Read the caller's landing instruction: direct merge or open a PR and wait, with the target branch and authorized actions. Reuse the reactor's user-approved instruction without asking again. Resolve tracked versus untracked intent storage through `report.sh repo-mode`.

**The ICE's `## Follow-ups` are closed in `idsd-build`, before `idsd-qualify` stamps — not here.** **Resolving one here invalidates the stamp the gate below reads**, which then blocks on freshness and leaves the ship asking for an override on its own edit. One that surfaces only here is resolved and then sent back through `idsd-qualify`.

**Then `~/.kk-flavor/skills/idsd-qualify/scripts/report.sh gate <NNN-slug>`, and let a non-zero exit stop you.** It is the whole of what stands between this ship and a merge nothing qualified: it asks whether an untrimmed `idsd-qualify` stamped this tree, in this worktree, with the ICE at `status: approved` and no `- [ ]` open in either it or the report. **On first entry, no report means no pass ran: run `idsd-qualify`.** A freshness, stages or unapproved-intent block is the human's to override and you ask them here; an open `- [ ]` is nobody's.

**Re-run the build's gates**, resolved the way the build resolved them (`~/.kk-flavor/standards/building.md` → **Before the loop**) — no record carries the commands. The qualify pass and the follow-up work both edited this tree since the build ran them, and a fix that broke one is invisible until they run again.

**Prepare the record merge before taking the slot.** Apply step 3's rules to the ship and project records, resolve questions and cap decisions now, and retain the exact inputs with the settled operations in private scratch outside the reviewed repository. Use the protected adjudication role from `~/.kk-flavor/standards/model-policy.md`.

**Review promotion candidates before taking the slot.** Identify settled decisions that qualify as high-level project invariants, from both the ship and project logs. Use `~/.kk-flavor/skills/idsd-charter/SKILL.md` → **Phase 3 — Emit** to extract and rewrite the obligation for a nontechnical reader, preserving useful source details. Check its **Rules** before presenting the exact charter change, including any replaced or removed bullets. Obtain an explicit user approval or rejection for each proposed promotion before continuing; an unanswered proposal pauses finalize before taking the slot, even unattended. Rejection leaves the charter and source decision intact. Apply only the approved changes; remove a fully promoted source decision only after the accepted constraint is written. After any approved promotion, refresh the prepared record inputs and operations. Every accepted charter constraint change requires requalification of affected work before continuing, in committed and throwaway modes alike; an unchanged git fingerprint does not preserve semantic qualification.

**Then check this intent's `links:`** by the rules `idsd-audit` applies set-wide. A bad link stops you; fix or route it first. Whole-set consistency stays that skill's job.

**At the end of preparation, settle the landing action.** Present the qualified diff, prepared record/archive changes, target branch and concrete commit, push and merge commands or PR draft. Without an authorized choice, ask the user whether to merge directly, open a PR and wait, or leave the prepared work. Obtain only missing authorization under `~/.kk-flavor/standards/git.md`; choosing a route waives no quality gate. A PR instruction authorizes opening and waiting, not merging it or sending review notifications. Carry any explicit gate override forward with its scope.

Before retiring the report, preserve the report, stage evidence and active intent records in private scratch outside the repository. Retain the qualified tree, target base, final diff and any PR URL in the task's resume record. The archive on a feature branch is prepared work; only the target branch proves landing.

## 2. Take the slot

`report.sh merge-slot take <NNN-slug>`. **Exit 4 means another ship holds it**, and the refusal names the holder's intent and worktree — wait for it. `--force` is for a holder you have established is gone, never for one you are impatient with: it breaks into a merge that may be half-written.

**Refresh the target and rerun `report.sh gate <NNN-slug>` before the record/archive mutation.** Integrate target changes and requalify affected work. Hold the slot through that pass and the authorized landing attempt; release it if the pass needs a human decision. The local slot cannot freeze remote merges: verify the target again before landing and honor required repository checks and reviews. A failed gate stops the attempt.

**Establishing that is yours, and the refusal cannot do it for you** — the tool started no process it could ask about. Look for a session working in the worktree it names; none, and the slot outlived its holder. **Release only a slot this run acquired.** A session that dies while holding it leaves one nobody else frees without checking the holder.

**Revalidate the record inputs after acquiring the slot**, before any record mutation. If they changed, release the slot and settle the new merge outside it. Revalidate on every acquisition; retain completed record operations so a retry never bumps or appends them twice.

## 3. Merge the three records upward

`decisions`, `playbook` and `language`, each of the ship's own entries against the project's. Step 1 settles these rules; after step 2 revalidates their inputs, apply only those settled operations at step 4's mode-specific boundary. Write through `report.sh record`, naming `project-*` for the destination.

- **It restates one already there** → `record bump` the project's entry. **That bump is the point of the whole split**: two ships independently needing one thing becomes a count, which `~/.kk-flavor/standards/records.md` → **Promotion is the exit upward** reads as a rule nobody has written down yet.
- **It says something new** → `record append`.
- **It contradicts one** — and that is never a write. Finalize has no authority to choose between a project truth and a ship's, so it goes to the human before taking the slot. The ship's entry stays unmerged until they settle it.
- **The project record is full** → `~/.kk-flavor/standards/records.md` → **Reaching the cap**, whose four moves you work in its order. This is the one place the cap is judged with the whole batch visible, which is why it is judged here and not where each entry was written.

**Language needs the distinction spelled out, because a term is not a command.** The same term in the same sense is a duplicate and bumps. The same term carrying a different meaning, or two terms for one thing, is a **contradiction** — the check `idsd-audit` runs set-wide, firing here per ship and on two candidates rather than on the whole set.

**Charter promotions are settled in step 1 through `idsd-charter`.** Preserve each decision’s candidate classification when merging it upward; resolve a classification disagreement with the other record questions before taking the slot.

## 4. Archive at the landing boundary

In **committed** mode, apply step 3 and archive on the feature branch before its final commit, so the archive, project records and roadmap land with the implementation. In **throwaway** mode, defer step 3 and archiving until the implementation's merge into the target is verified. Keep the active intent and report while it waits; include no `.idsd/` files in the commit or PR.

To archive, set `status: built`, then run `report.sh finalize <NNN-slug>`. It deletes the report and stage markers and moves the intent with its three records to `.idsd/archive/NNN-<slug>/`. Regenerate `.idsd/roadmap.md` if present, using `idsd-intent`'s format. Review the resulting record/archive delta against step 1's settled operations before committing.

**Direct merge:** commit the prepared change set under the settled authorization, then merge the branch into the agreed target. Perform any authorized target push and verify the resulting target ref. A feature-branch commit alone is not completion. In throwaway mode, apply step 3 and archive now, under the slot. Release the slot after verifying both landing and archiving.

**PR:** follow [pr.md](pr.md), the whole delta for opening, waiting and resuming this route.
