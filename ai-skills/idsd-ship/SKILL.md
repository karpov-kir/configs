---
name: idsd-ship
description: "Ship an ICE intent end-to-end or review standalone changes. Subcommands: `idsd-ship <intent>` (full pipeline), `idsd-ship done` (merge), `idsd-ship review` (quality stages only, no build or merge)."
argument-hint: "<intent> | done | review"
---

Drive one intent from ICE to merge-ready — or review standalone changes — through a fixed pipeline, accumulating a `idsd-ship-report.md` digest of what needs the human's attention. You **orchestrate** existing skills — invoke them, never reimplement; their rules still hold (gates absolute, follow-ups routed before archive, no-mocks, …).

**Interactive first.** Prefer asking the human live over deferring to the digest. A decision you can't settle from the intent, the code, or a sensible default — or an ambiguity that changes what gets built — is *blocking*: stop and ask now. The sub-skills' own clarify gates (e.g. `idsd-build`'s Phase 1) still fire — never suppress one by recording instead. `idsd-ship-report.md` is for what does *not* block — surfaced for the human at the final checkpoint, not lost in chat history.

Where this fits: `idsd-intent` → (`idsd-audit`) → **`idsd-ship`** (= `idsd-build` + `/review` + `/security-review` + `/refactor` + `/tighten` + `idsd-retro`, sequenced).

## Subcommands

| Command | What it does |
|---|---|
| `idsd-ship <intent>` | Full pipeline: build + quality stages + checkpoint. Ends with the gate message (see **After the quality stages**). |
| `idsd-ship done` | Proceeds to merge (idsd-build Phase 5), gated on review freshness (see **`done` — merge**). Reads the intent from the report's frontmatter. Error if no report exists or if the report was produced by `review` mode (no merge target). |
| `idsd-ship review` | Quality stages only (2–5), no build, no merge. For standalone changes with no intent, or for re-reviewing after post-pipeline refinements. |

If `<intent>` is unspecified (and the subcommand is not `done` or `review`), list the not-yet-built intents and ask which.

## Flow

```
ship <intent>           review (standalone)
  │                         │
  ▼                         │
Build                       │
  │                         │
  ▼                         ▼
Quality stages ◄──── review (re-review)
  │                         ▲
  ▼                         │
Gate message                │
  │                         │
  ├── user edits ──────────►│
  │                         │
  └── done                  │
        │                   │
        ├─ tree clean ──► Merge
        │
        └─ tree dirty ──► "run review first"
              │
              ├─ user approves skip ──► Merge
              │
              └─ user agrees ──► review (re-review)
```

## Report

`idsd-ship-report.md` lives at the repo root and **persists across runs** — the working digest, never committed (the ICE and git history are the durable record). Add it to `.gitignore` if it isn't already (`scripts/report.sh check-ignore` asserts this). The deterministic report operations — stamp, the `done` gate, carry-forward — run through `scripts/report.sh` (in the skill dir), never by hand.

### Structure

```markdown
---
intent: <NNN-slug or "review: <description>">
reviewed-tree: <git tree hash at time quality stages completed>
---

# Decide
- [ ] <a decision the human must make or ratify before merge>

# Watch
- <a monitor-only item; no action now>
```

`done` compares `reviewed-tree` against the current tree to catch unreviewed changes (see **`done` — merge**). The report is **only the residue that needs the human — not a record of the run** (*What goes in*, below). At most two groups, each present only when it has items — **Decide** (`- [ ]` actions) and **Watch** (monitor-only) — with no per-stage sections and no summary. On re-review **every unresolved `- [ ]` carries forward verbatim** — dropped only with positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area; new findings append as fresh `- [ ]`. Watch bullets are re-evaluated each pass — kept while relevant, dropped when moot.

### What goes in (and what never does)

The report holds the *non-blocking* attention set — decisions to ratify, deferrals, gated/declined fixes, monitor-only watches — never a blocking question (asked live, per Interactive first). A **Decide** item is one `- [ ]` line: the decision and what the human must do, at their altitude — no "what we tried", no dep lists, command strings, or code identifiers (those live in the diff, commit, or ICE — link, don't restate); tag its origin stage in parentheses only when that aids the human. A **Watch** item is monitor-only, no checkbox.

**The test: if the human takes no action, it is not in the report.** A resolved or applied fix, a passed / clean / not-applicable stage, "here's what changed", and any verification narration (what passed, what's byte-identical, invariants confirmed) are all **omitted** — they live in the diff, the commit, and the fact the pipeline ran. The report shrinks to nothing when nothing needs the human — that's the success case, not an omission. The one exception is a fix the human might want to *reverse*: record that as a Decide item (ratify or revert), not as an FYI.

## Quality stages

Start from the right base. **First review of a change set** (no report, or one recording a different intent): reset `idsd-ship-report.md` from `templates/ship-report-template.md`, recording in its frontmatter the intent (or change-set description) it covers. **Re-review** (the report already covers this change set): keep it and reconcile per the carry rule above — `scripts/report.sh carry` lists the prior open `- [ ]` so none are lost. Either way, run the stages in order, appending each item to **Decide** or **Watch** the moment it surfaces, not batched at stage end.

When all quality stages complete, stamp the tree fingerprint: run `scripts/report.sh stamp` — it computes `git write-tree` and writes the hash into `reviewed-tree`.

**How each stage runs.** Build runs **inline**, not for consistency's sake but because its human coupling is *continuous* — `idsd-build` restates, clarifies, and decides with the human throughout, a live dialogue the `blocked`→resume bridge (built for *occasional* pauses) would turn into constant ping-pong. The analysis stages are the opposite: mostly autonomous, returning findings as data with an occasional `blocked`, so each runs in a **dedicated subagent** (which also isolates its heavier context from the orchestrator) — code-review, security-review, refactor, tighten, and retro. The subagent executes the skill in full and returns structured findings; it never decides whether to run, and never fakes the pass with its own inline judgment.

**Spawn the skill, not your own review.** Each subagent prompt names exactly one skill and hands it the change scope — nothing more. The skill defines what to check; the spawn prompt must not pre-select, narrow, or invent which rules apply, must not borrow another stage's lane (correctness is `/review`'s, style/structure/architecture is `/refactor`'s, vulnerabilities are `/security-review`'s, prose/concision is `/tighten`'s), and must let the subagent run the skill's full decomposition itself. The only thing you may inject is emphasis the **user explicitly stated** this run (e.g. "ensure arch-doc compliance" → pass to refactor) — never a rule you inferred. A spawn prompt that lists specific CLAUDE.md rules to look for, or asks for findings outside the named skill's scope, is the defect this guards against. Only the main thread has the human, so every decision and all report-writing stay here: take the subagent's returned findings, ask the human live for any that block, record the rest. When a subagent hits something only the human can settle — a clarification, a gated choice — it pauses and returns `blocked: <what it needs>` rather than guessing. Answer it, then **resume that same subagent by its ID** so it continues with its context and progress intact; never start a fresh one — a new spawn loses the work it already did and the skill state it was holding.

**Reconcile contradictions.** When two stages give opposing verdicts on the same location — one clears what another flags — or a claim contradicts an observation (a subagent reports green while a tool shows otherwise) — adjudicate empirically before recording: re-run the check yourself and trust neither side's word over the result.

**Scale to the change; settle design once.** Match review weight to the delta — a small, low-risk change doesn't need every stage's full fan-out, a broad or risky one does; scope each subagent to the changed surface rather than re-running the world. And when a change reworks a **shared or cross-repo primitive** (a vendored arch primitive, a public type, a cross-cutting contract), settle its target shape with the human in one pass *before* iterating — converging a design through many one-finding-at-a-time review round-trips is the most expensive path.

1. **Build** (skip on `review`) — run `idsd-build` for the intent in its **pipeline mode**: it runs restate/confirm, context, and implementation until gates are green, then hands back — skipping its self-review, checkpoint, and merge, which the dedicated passes and the final approval below replace. As it builds, idsd-build *records and routes* every follow-up to the ICE's `## Follow-ups` and every durable standard to a constitution proposal — at build time, its own rule (resolving them stays merge-gated under `done`). Before recording, confirm it did: an unrouted follow-up is a build defect, not something the report absorbs.
   - Record as **Decide** items: deferrals to confirm, constraints that need human judgment (can't become a gate), and decisions to ratify — each pointing to its durable home (the ICE `## Follow-ups`, a constitution proposal) idsd-build already wrote. An ambiguity resolved with no open decision is not recorded. The report flags for the human; it never replaces the durable record.
2. **Code-review** — spawn a subagent to run `/review` on the build's changes: it applies every fix it can make correctly and returns the rest — findings needing a human decision (a trade-off, an ambiguous intent, a risky change), plus any behaviour-changing fix it made. On its return, ask the human live for blocking findings; record the others.
   - Record as **Decide** items: findings needing a human decision. A fix already applied is recorded only if the human might want to reverse it (ratify-or-revert); otherwise it's just the diff.
3. **Security-review** — *only if* the change touches a security surface (input handling, filesystem/network/exec, auth or session, secrets, deserialization, or any constitution security invariant); otherwise skip. Spawn a subagent to run `/security-review` on the build's changes: it applies trivial safe fixes (e.g. secret redaction) and returns the rest as findings with severity, exploit scenario, and fix. On its return, ask the human about anything blocking; record the others.
   - Record as **Decide** items: findings needing a human decision (severity + exploit + fix). Record nothing when the surface was clean or untouched.
4. **Refactor** (loop, max 3 iterations) — the pipeline's safety net for completeness; don't shortcut it. Each iteration spawns a **fresh** subagent (not a resume) to run `/refactor` on the changes: it applies routine fixes, returns gated architecture/testing/project-setup ones as proposals (not applied), and reports whether the change is now fully compliant. Spawning fresh re-reads the now-changed tree, catching fixes that only surface after earlier ones land. Stop the moment a subagent reports compliant; otherwise iterate to the cap. The blocked→resume rule still holds *within* an iteration — if that pass's subagent pauses, resume it by ID; only a new iteration spawns fresh. On each return, ask the human about gated proposals; record what's deferred or declined.
   - Record: gated fixes the human must rule on (**Decide**), duplication deferred under the extract threshold (**Watch**), and — if it hit the cap without reaching compliant — that, with what's open (**Decide**). Nothing when it reached compliant with no gated proposal.
5. **Tighten** (optional) — *only if* the build added or changed standalone prose (docs, markdown) or non-trivial comment blocks; otherwise skip. Spawn a subagent to run `/tighten` on those files: it applies the lossless cuts and returns only what it couldn't safely cut (a doubtful-substance cut, an unresolved `WARN`). On its return, record that remainder.
   - Record as a **Decide** item only what `/tighten` flagged but couldn't safely cut; nothing otherwise.
6. **Retro** — *runs as the last stage of a full `idsd-ship <intent>`; skip on a bare `review` re-run unless asked — except always run when the human course-corrected during the run, or the run modified its own tooling (a skill, this pipeline, a shared script).* Course-corrected is observable, not a vibe: the human answered a blocking question, redirected or re-scoped a stage, rejected or reworked an applied fix, or flagged a missed rule. Write a terse, factual run-log — what was asked, what each stage did, where the human corrected course, what was deferred (decisions only, no self-assessment) — then spawn `idsd-retro` as a **fresh** subagent (per **Spawn the skill**), handing it the run-log plus the diff and any skill/doc/prompt/script the run touched. It owns the adversarial method and lenses; it returns routed, evidence-backed findings.
   - Record each finding as a **Decide** item — the improvement, its target, and the routing home it names (a constitution proposal, an edit to a skill/arch/prompt file, a backlog entry, or a fix folded back into the change). On the human's ratification, route it exactly as `idsd-build` routes follow-ups — the report only flags; it never becomes the durable record. Record nothing only when the run genuinely surfaced no improvement (a course-corrected run never does).

## After the quality stages

Present the `idsd-ship-report.md` file itself — the Decide/Watch list as written — alongside `idsd-build`'s checkpoint evidence (gate + scenario + observed outcomes) when running from an intent; point the human at the file, don't re-author it as a parallel chat digest (per *Keep chat lean*). When the report is empty, say so plainly — the change needs nothing from the human beyond the gate evidence.

**Gate message.** Tell the human clearly:
- Review the diff and the report.
- If you make changes, run `/idsd-ship review` to re-run the quality stages.
- When you're ready to merge, run `/idsd-ship done`.

For `review` mode without an intent, the gate message omits the `done` option — there is no merge step. Instead: "Review the diff and the report. If you make changes, run `/idsd-ship review` again."

**Dogfooding that turns into a redesign.** The gate-message loop (`review` → edit → `review`) is for *refinements* that keep the intent's contract. When the human's hands-on use instead reshapes that contract — a different presentation, a reworked surface, a new sub-feature — it's a **re-scope, not an open edit session**: amend the ICE via `idsd-intent` first so the new shape is recorded, then commit the reviewed state as a checkpoint *before* the rework starts, so the redesign lands as its own distinct change set. Skip the checkpoint commit and the reviewed work and the rework fuse into one diff that can no longer be split.

## `done` — merge

On `/idsd-ship done`:

1. **Gate.** Run `scripts/report.sh gate` (in the skill dir). It exits non-zero on either a **stale tree** (current `git write-tree` ≠ `reviewed-tree`) or **any open `- [ ]`**, printing which block(s) fired. A freshness-only block the human may explicitly override (then proceed). An open-TODO block has **no override** — the human clears each first: resolve it (do it, then check or delete the box) or route it out of the report (to the ICE `## Follow-ups`, a backlog, a constitution proposal). Watch bullets don't gate.
2. On a clean gate — or freshness overridden with no open `- [ ]` — hand to `idsd-build`'s Phase 5: `status: built`, archive, roadmap, commit (which asks first). The pipeline never commits on its own.

## Rules

- Adds only sequencing and the digest; the sub-skills own the actual work.
- Keep chat lean — write attention items to `idsd-ship-report.md`, never into a chat summary, and don't echo the digest back in prose.
- A stage that hard-fails (red gate, build can't complete) stops the pipeline — never relax a sub-skill's gate to keep moving; a blocking decision is asked live, not recorded to dodge it (Interactive first).
