---
name: idsd-ship
description: "Ship one ICE intent end-to-end — author it if missing, build, qualify, gate, merge — or review standalone changes. Subcommands: done, qualify, continue, promote. The orchestrator above idsd-build and idsd-qualify."
argument-hint: "<arg> | done | qualify [fast|full] | continue | promote"
---

Drive one intent from ICE to merge-ready — or review standalone changes — accumulating a `.idsd/ship-report.md` digest of what needs the human's attention. You **orchestrate** existing skills. Build, qualify, and the final gate always run; a stage that hard-fails stops the pipeline.

**Interactive first.** You run under the orchestrator rule in `~/.kk-flavor/standards/skill-protocol.md`. Ship's own seam: the sub-skills' clarify gates (e.g. `~/.claude/skills/idsd-build/SKILL.md` → **Phase 1**) still fire — never suppress one by recording instead.

**Offer cadence.** Both periodic offers below go through `scripts/cadence.sh <topic> due` (this skill's dir): exit 0 → make the offer; exit 1 → say nothing; exit 2 → say the cadence could not be read and offer nothing, never read it as not-due. Then `cadence.sh <topic> asked` once the answer is settled — on no immediately, on yes only after the stage returns, so a run that dies mid-stage is not suppressed for a week.

## Subcommands

| Command | What it does |
|---|---|
| `idsd-ship <arg>` | Build + **full** qualify + gate message. `<arg>` is an existing intent slug, or a **ticket / new-feature ref**. |
| `idsd-ship done` | Merge, gated on review freshness and the stage record. |
| `idsd-ship qualify [fast\|full]` | `idsd-qualify` over the working tree — **fast** unless `full` is passed; no build, no merge. |
| `idsd-ship continue` | Run the next step for wherever the change set stands. |
| `idsd-ship promote` | Turn a throwaway `.idsd/` into a durable idsd project. |

With no `<arg>` and no subcommand, list the not-yet-built intents and ask which.

## Report & .idsd lifecycle

The report contract — path, structure, carry rules, what goes in, and the **committed vs throwaway** repo modes — plus `scripts/report.sh` belong to `~/.claude/skills/idsd-qualify/SKILL.md` → **Report**. Ship adds the lifecycle:

- **Promote** — `report.sh promote` keeps a single-shot `.idsd/` and stages it; the human commits. A standalone qualify with no intents has nothing durable to promote — say so rather than promoting an empty `.idsd/`. Promotion makes the repo committed, so the `CLAUDE.md` pointer at `.idsd/` becomes owed here (`~/.claude/skills/idsd-build/SKILL.md` → **Phase 2 — Assemble Context**, which owns it and fires for a project committed from birth too).
- **Discard** — the counterpart, run by `done`: `report.sh discard` clears this ship's local scratch (throwaway mode only; the script refuses elsewhere). It keeps `.idsd/` whenever another intent or an authored `charter.md` / `constitution.md` / `language.md` / `playbook.md` is still there — those are the human's, not this ship's scratch, and in throwaway mode no other copy of them exists.

## Build, then qualify

1. **Open the report** — `report.sh check-ignore`, then, on the first pass over this change set, `report.sh init "<intent>"`, both per `~/.claude/skills/idsd-qualify/SKILL.md` → **Running a pass**. Ship's delta is only that they happen here, before Build, so Build's decisions have a home the moment they surface. Two things silently mis-key the whole pass:
   - **The report belongs in the worktree Build works in.** If `idsd-build` places itself in a new one, run `check-ignore`/`init` from there, or place Build in this one — a report at the original root fingerprints a tree holding none of the build's changes.
   - **`init` on a ticket ref is provisional.** Once `idsd-intent` emits `NNN-<slug>`, re-stamp the report's intent line to that slug before Build records anything, or the report indexes a slug no file has.
2. **Build** — **author the intent first if it's missing.** If no intent file matches `<arg>` (not in `.idsd/intents/` or the archive), run `idsd-intent` to author one, seeded from the ticket when `<arg>` is a ticket ref and a connector is available, else from `<arg>` as the feature description. Then run `idsd-build` for that intent in its **pipeline mode**. Both run **inline**, never in a subagent — each couples to the human continuously (intent grills; build's Phase 1 clarifies), and only this thread reaches the human. Before recording anything in the report, confirm idsd-build routed its follow-ups and constitution proposals as its own rules require; an unrouted follow-up is a build defect, not something the report absorbs.
   - Record as **Decide** items: deferrals to confirm, constraints that need human judgment, and decisions to ratify — each pointing to the durable home idsd-build already wrote. An ambiguity resolved with no open decision is not recorded.
3. **Offer the retro** — here, *before* Qualify, never after: asking afterwards stamps `retro:skipped` for a stage that then runs. `cadence.sh retro due`; on a yes, pass the answer into the qualify invocation so the retro runs as its last stage.
4. **Qualify** — invoke `idsd-qualify` **inline** in **full** mode over the build's changes. It owns the stages, the spawn protocol, the report writing, and the stamp; blocking findings still reach the human live, in this thread.

## `continue` — resume from current state

Read where the change set stands with `report.sh state` (never hand-parse the report); it prints one token:

| Token | `continue` does |
|---|---|
| `no-report` | Nothing in progress. With an `<intent>` arg, start `ship <intent>`; otherwise list the not-yet-built intents and recommend one. |
| `resume` | Run the full `ship <intent>` flow for the report's intent — build idempotently resumes to green, then the full qualify stamps. |
| `re-qualify` | Run `qualify` (fast by default); carry-forward keeps the open items. |
| `decide` | Present the gate message with the Decide list; the human clears each, then runs `done`. |
| `finalize` | Run `qualify full` — the pass trimmed stages for turnaround, and the merge gate demands a full one. |
| `ready` | Present the gate message (review the diff + report, then `done`). Never merge on its own — `done` owns that. |
| `done` | Say the intent is built and archived; recommend the next unbuilt one. |

`continue` only dispatches; it never relaxes a gate. A non-zero exit prints no token — say the state could not be read and stop, rather than guessing one.

## `done` — merge

Reads the intent from the report's frontmatter; error with no report, or on one a standalone qualify produced (no merge target).

1. **Gate.** Run `report.sh gate`. It names each block that fired and whether the human may override it; the human clears an open TODO first, by resolving it or routing it out of the report (to the ICE `## Follow-ups`, a backlog, a constitution proposal). Beyond the gate: if the target branch advanced past this branch's base since `reviewed-tree` was stamped, the review is stale against the new base — integrate the target and re-run `qualify full` (which re-stamps) before landing.
2. On a clean gate — or freshness/stages overridden with no open `- [ ]` — hand to `~/.claude/skills/idsd-build/SKILL.md` → **Phase 5**: `status: built`, archive, roadmap, commit (approval-gated — `~/.kk-flavor/standards/git.md` → Commits).
3. **Throwaway cleanup.** In throwaway repo mode (`report.sh repo-mode`), the local `.idsd/` — the report plus the intent Phase 5 just archived — outlives the ship and breaks the mode's zero-traces contract. So **after** the commit succeeds (only then — never lose the intent while the work is unlanded), **ask** whether to clear it (default yes): on yes run `report.sh discard`. Committed mode keeps `.idsd/` as its durable record; keeping a throwaway one instead is what `promote` (before `done`) is for.
4. **Offer an audit** — committed repo mode only: a throwaway `.idsd/` has no durable set to sweep, and step 3 may have just cleared it. This merge changed the set — an intent archived, the roadmap regenerated — which is what `idsd-audit` checks. `cadence.sh audit due`; invoke `idsd-audit` on a yes. Its record lives in this repo's `.git/`, so this cadence is per project where the retro's follows you across repos.
