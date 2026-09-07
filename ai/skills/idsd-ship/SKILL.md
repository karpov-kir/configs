---
name: idsd-ship
description: "Ship one ICE intent end-to-end — author it if missing, build, qualify, gate, merge — or review standalone changes. Use for \"ship it\", \"merge this\", \"continue the ship\". The orchestrator above idsd-build, idsd-qualify and idsd-finalize; several intents at once is idsd-reactor's."
argument-hint: "<arg> | done [<intent>] | qualify | continue [<intent>] | promote"
---

You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**). You run `~/.claude/skills/idsd-build/SKILL.md`, `~/.claude/skills/idsd-qualify/SKILL.md` and `~/.claude/skills/idsd-finalize/SKILL.md`, so read all three whole rather than entering them a section at a time. Ship's own seam: the sub-skills' gap rounds (e.g. `~/.claude/skills/idsd-build/SKILL.md` → **Phase 1**) still fire — never suppress one by recording instead.

## Subcommands

| Command | What it does |
|---|---|
| `idsd-ship <arg>` | Build + qualify + gate message. `<arg>` is an existing intent slug, or a **ticket / new-feature ref**. |
| `idsd-ship done [<intent>]` | Merge, gated on review freshness and the stage record. **Names the intent whenever more than one ship is open** — `report.sh gate` refuses to guess between them. |
| `idsd-ship qualify` | `idsd-qualify` over the working tree — trimmed for turnaround unless your caller says the merge is waiting; no build, no merge. |
| `idsd-ship continue` | Run the next step for wherever the change set stands. |
| `idsd-ship promote` | Turn a throwaway `.idsd/` into a durable idsd project. |

With no `<arg>` and no subcommand, list the not-yet-built intents and ask which. Where more than one has every `depends-on` already shipped, offer `idsd-reactor` instead.

## Report & .idsd lifecycle

The report contract — the **committed vs throwaway** repo modes included — plus `~/.claude/skills/idsd-qualify/scripts/report.sh` belong to `~/.claude/skills/idsd-qualify/SKILL.md` → **Report**. Ship adds **promote** and its counterpart **discard**, the latter owned by `done` below.

**Promote** — `report.sh promote` stages `.idsd/`; the human commits. A standalone qualify with no intents has nothing durable to promote — say so rather than promoting an empty `.idsd/`. Promotion makes the repo committed, so add the `CLAUDE.md` pointer at `.idsd/` per `~/.claude/skills/idsd-build/SKILL.md` → **Phase 2 — Assemble Context**.

## Build, then qualify

1. **`report.sh check-ignore`, then open the report the moment you hold the `NNN-<slug>`.** `report.sh init "<intent>"` on the first pass over this change set, per `~/.claude/skills/idsd-qualify/SKILL.md` → **Running a pass**. The slug names the report *file*, so one opened under a ticket ref keeps that name whatever its frontmatter later says. Either way it precedes Build, so Build's decisions have a home the moment they surface.
   - **The report belongs in the worktree Build works in.** If `idsd-build` places itself in a new one, run `check-ignore`/`init` from there, or place Build in this one. A report at the original root fingerprints a tree holding none of the build's changes.
2. **Build** — **author the intent first if it's missing.** If no intent file matches `<arg>` — not in `.idsd/intents/` or the archive — run `idsd-intent` to author one. Seed it from the ticket when `<arg>` is a ticket ref and a connector is available, else from `<arg>` as the feature description. **That `NNN-<slug>` is step 1's trigger — open the report now.** Then run `idsd-build` for that intent in its **pipeline mode**. Both run **inline**, never spawned — each couples to the human continuously (intent grills; build runs its gap rounds), and only this thread reaches the human. Before recording anything in the report, confirm idsd-build routed its follow-ups and proposals as its own rules require. An unrouted follow-up is a build defect, not something the report absorbs.
   - Record as **Decide** items, each pointing to the durable home idsd-build already wrote: deferrals to confirm, constraints that need human judgment, and any choice Build settled that the human may still overturn — a **fork** whose default is what Build did. **What Build's conformance gate hands back is one of these** — it fixed and re-ran what was undelivered, so what reaches you is work beyond the ask, and contradictions. An ambiguity resolved with no open decision is not recorded.
3. **Qualify** — invoke `idsd-qualify` **inline** over the build's changes, as the pass the merge waits on. Blocking findings still reach the human live, in this thread.
4. **Present the gate message** — this is where `ship <arg>` ends. It is the closing status line of `~/.claude/skills/idsd-qualify/SKILL.md` → **After the pass**, plus the two things only ship owns: every open `- [ ]` for the human to clear, and the next act — review the diff and the report, then run `idsd-ship done`. **Never merge here**; `done` owns that.

## `continue` — resume from current state

Read where the change set stands with `report.sh state <intent>` (never hand-parse the report); it prints one token. With several ships open, take the intent from `<arg>`; `report.sh list` names them with their states:

| Token | `continue` does |
|---|---|
| `no-report` | Nothing in progress for that intent. With an `<intent>` arg, start `ship <intent>`; otherwise list the not-yet-built intents and recommend one. |
| `resume` | Run the full `ship <intent>` flow for the report's intent — build idempotently resumes to green, then the qualify stamps. An intent short of `status: approved` routes here too: build's Phase 1 is what closes it. |
| `re-qualify` | Run `qualify` as the pass the merge waits on; carry-forward keeps the open items. |
| `decide` | Present the gate message; its open items are what the human clears before `done`. |
| `finalize` | The stamp is a **trimmed** pass, which no merge accepts — re-run `qualify` untrimmed, as the pass the merge waits on. Not a route to `done`. |
| `ready` | Present the gate message — nothing is open, so the diff and the report are the whole of it. |
| `done` | Say the intent is built and archived; recommend the next unbuilt one. |

`continue` only dispatches; it never relaxes a gate. A non-zero exit prints no token — say the state could not be read and stop, rather than guessing one. The one exception is the several-reports refusal, which names them: ask which.

## `done` — merge

Reads the intent from the report's frontmatter; error with no report, or on one a standalone qualify produced (no merge target).

1. **Gate.** Run `report.sh gate <intent>`; the human clears an open `- [ ]` first, by resolving it or routing it out of the report — a backlog, an `idsd-charter` or `CLAUDE.md` proposal. **Routing it into the ICE `## Follow-ups` clears nothing**: the gate reads both, so the item stays open, and the edit lands after the stamp and adds a freshness block on top of it. Beyond the gate: the review is stale if the target branch advanced past this branch's base since `reviewed-tree` was stamped. Integrate the target and re-run `qualify` as the pass the merge waits on (which re-stamps) before landing.
2. On a clean gate — or an overridable block the human waived, with no open `- [ ]` — invoke `~/.claude/skills/idsd-finalize/SKILL.md` **inline**, since its step 1 asks the human and only this thread reaches them; it runs unchanged through its approval-gated commit. It gates again before archiving, so carry the human's override with you rather than asking for it twice. Its own `report.sh finalize` retires the report, so nothing here closes one.
3. **Throwaway cleanup.** In throwaway repo mode (`report.sh repo-mode`) the local `.idsd/` outlives the ship and breaks the mode's zero-traces contract. **After** the commit succeeds — never before, or the intent is lost while the work is unlanded — **ask** whether to clear it (default yes). On yes, `report.sh discard <intent>`. Keeping a throwaway `.idsd/` instead is what `promote` (before `done`) is for.
4. **Offer an audit** — committed repo mode only, when `~/.claude/skills/idsd-ship/scripts/cadence.sh audit due` says one is due; its usage line carries the exit codes. On exit 2 offer anyway and say the cadence could not be read. Invoke `idsd-audit` on a yes. Then `cadence.sh audit asked` once the answer is settled — on no immediately, on yes only after `idsd-audit` returns.
