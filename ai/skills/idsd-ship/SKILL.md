---
name: idsd-ship
description: "Ship an ICE intent end-to-end or review standalone changes. Subcommands: `idsd-ship <arg>` (full pipeline; authors the intent if missing), `idsd-ship done` (merge), `idsd-ship qualify [fast|full]` (quality pipeline via idsd-qualify — no build or merge), `idsd-ship continue` (resume from current state), `idsd-ship promote` (keep a throwaway .idsd/)."
argument-hint: "<arg> | done | qualify [fast|full] | continue | promote"
---

Drive one intent from ICE to merge-ready — or review standalone changes — through a fixed pipeline, accumulating a `.idsd/ship-report.md` digest of what needs the human's attention. You **orchestrate** existing skills — invoke them, never reimplement; their rules still hold (gates absolute, follow-ups routed before archive, no-mocks, …).

**Interactive first.** The orchestrator rule in `~/.kk-flavor/standards/skill-protocol.md` applies: blocking decisions are asked live, each carrying a recommended answer earned by legwork; a subagent's `blocked` relays with its recommendation and resumes by ID. Ship's own seams: the sub-skills' clarify gates (e.g. `idsd-build`'s Phase 1) still fire — never suppress one by recording instead — and `.idsd/ship-report.md` is for what does *not* block, surfaced at the final checkpoint, not lost in chat history.

Where this fits: `idsd-intent` → (`idsd-audit`) → **`idsd-ship`** (= `idsd-build` + `idsd-qualify` + merge, sequenced; `idsd-qualify` in turn runs `/code-review` + `/security-review` + `/refactor` + `/tighten` + `idsd-retro`).

**kk-flavor wiring.** If the flavor isn't injected, read `~/.kk-flavor/inject.md` first, plus `~/.kk-flavor/standards/skill-protocol.md` (the orchestrator rule above and the contract spawned skills run under). The quality pass is `idsd-qualify` — a suite sibling, invoked directly by name (the `roles` map wires only the swappable kk- stage skills). The stage roles and per-stage `pipeline` flags are idsd-qualify's concern. Build, qualify, and the final gate are structural and always run.

## Subcommands

| Command | What it does |
|---|---|
| `idsd-ship <arg>` | Full pipeline: build + **full** qualify + gate message. `<arg>` is an existing intent slug, or a **ticket / new-feature ref** — if no intent file matches, Build authors one first (see **Build, then qualify**). |
| `idsd-ship done` | Proceeds to merge (idsd-build Phase 5), gated on review freshness and the stage record (see **`done` — merge**). Reads the intent from the report's frontmatter. Error if no report exists or if the report was produced by a standalone qualify (no merge target). |
| `idsd-ship qualify [fast|full]` | Invoke `idsd-qualify` on the working tree — **fast** unless `full` is passed — no build, no merge. For standalone changes with no intent, the edit→re-qualify loop, or the final full pass before `done`. |
| `idsd-ship continue` | Detect where the change set stands and run the next step (or report it's done). Reads state from the report — needs no `<arg>`. See **`continue`**. |
| `idsd-ship promote` | Turn a throwaway `.idsd/` into a durable idsd project via `report.sh promote`; never commits on its own (see **Report & .idsd lifecycle**). |

If `<arg>` is unspecified (and the subcommand is not `done`, `qualify`, `continue`, or `promote`), list the not-yet-built intents and ask which.

## Flow

```
ship <intent>:  build → qualify full → gate message → (edit → qualify fast)* → done (gate; edits demand a fresh qualify full) → merge
standalone:     qualify [fast|full] → gate message → …
```

`done`'s gate (fresh tree, full pass, no open `- [ ]`) routes back to `qualify full` or to the human — see **`done` — merge**.

## Report & .idsd lifecycle

The report contract — the path (`.idsd/ship-report.md`), its structure and carry rules, what goes in, the **committed vs throwaway** repo modes, and `scripts/report.sh` — is owned by `idsd-qualify` (see its **Report** section); the script lives in that skill's dir, and every deterministic operation (init, repo-mode, invalidate, stamp, gate, carry, check-ignore, state, promote, discard) runs through it, never by hand. Ship adds the lifecycle around the report:

**Promoting a throwaway run.** When the human wants to keep a single-shot `.idsd/`: `report.sh promote` converts it and stages the result (mechanics in the script) — then the human commits; it never commits on its own. A standalone qualify with no intents has nothing durable to promote (only the report, which is never committed) — say so rather than promoting an empty `.idsd/`.

**Discarding a throwaway run.** The counterpart to promote, run by `done` (see **`done` — merge**): `report.sh discard` removes this ship's local scratch, and the whole `.idsd/` plus its exclusion when nothing else remains (mechanics in the script). Throwaway-only (refuses in committed repos, where `.idsd/` is the durable record); slug-scoped, so a `.idsd/` holding other intents keeps them.

## Build, then qualify

Before Build, prepare the report: run `report.sh check-ignore`, then — first pass on this change set — `report.sh init "<intent>"`, so Build's decisions have a home the moment they surface.

1. **Build** — **author the intent first if it's missing.** If no intent file matches `<arg>` (not in `.idsd/intents/` or the archive), spawn `idsd-intent` to author one before building: seed it from the ticket when `<arg>` looks like a ticket ref (fetch via the `atlassian` skill), otherwise treat `<arg>` as the feature description. Then run `idsd-build` for the intent in its **pipeline mode**: it runs restate/confirm, context, and implementation until gates are green, then hands back, skipping its self-review, checkpoint, and merge — the qualify pass and the final approval below replace those. Build runs **inline**, not in a subagent. Its human coupling is *continuous* — restating, clarifying, deciding with the human throughout — and the `blocked`→resume bridge is built for *occasional* pauses; forcing a live dialogue through it means constant ping-pong. As it builds, idsd-build *records and routes* every follow-up to the ICE's `## Follow-ups` and every durable standard to a constitution proposal — that's its own rule, and resolving them stays merge-gated under `done`. Before recording anything in the report, confirm it did: an unrouted follow-up is a build defect, not something the report absorbs.
   - Record as **Decide** items: deferrals to confirm, constraints that need human judgment (can't become a gate), and decisions to ratify — each pointing to its durable home (the ICE `## Follow-ups`, a constitution proposal) idsd-build already wrote. An ambiguity resolved with no open decision is not recorded. The report flags for the human; it never replaces the durable record.
2. **Qualify** — invoke `idsd-qualify` **inline** in **full** mode over the build's changes. It owns the stages (the parallel round, refactor, retro), the spawn protocol, the report writing, and the stamp; blocking findings still reach the human live, in this thread. Don't duplicate its decisions here — its SKILL.md is the contract.

**After the pass**, the qualify skill's post-pass message contract applies (status line + next steps + at most one blocking question; substance in the report, not chat) — surfacing `idsd-build`'s checkpoint evidence (gate + scenario + observed outcomes) alongside the pointer.

**Dogfooding that turns into a redesign.** The gate-message loop (`qualify` → edit → `qualify`) is for *refinements* that keep the intent's contract. When the human's hands-on use instead reshapes that contract — a different presentation, a reworked surface, a new sub-feature — it's a **re-scope, not an open edit session**: amend the ICE via `idsd-intent` first so the new shape is recorded, then commit the reviewed state as a checkpoint *before* the rework starts, so the redesign lands as its own distinct change set. Skip the checkpoint commit and the reviewed work and the rework fuse into one diff that can no longer be split.

## `continue` — resume from current state

`idsd-ship continue` recovers where the change set stands and runs the next step. Read the state deterministically with `report.sh state` (never hand-parse the report); it prints one token, and each routes to an existing behaviour whose rules hold unchanged:

| Token | State | `continue` does |
|---|---|---|
| `no-report` | nothing in progress | Say so. With an `<intent>` arg, start `ship <intent>`; otherwise list the not-yet-built intents and recommend one (per *Interactive first*). |
| `resume` | quality never completed (`reviewed-tree` unstamped) | Run the full `ship <intent>` flow for the report's intent — build restates and idempotently resumes to green (a no-op if already there), then the full qualify runs and stamps. |
| `re-qualify` | reviewed once, tree moved since | Run `qualify` (fast by default) — the build already shipped; carry-forward keeps open items. |
| `decide` | quality done, tree fresh, open `- [ ]` remain | Present the gate message with the Decide list; the human clears each, then runs `done`. |
| `finalize` | tree fresh, nothing open, but the pass trimmed stages for turnaround (or predates the stage record) | Run `qualify full` — the merge gate demands a full pass. |
| `ready` | full-reviewed, tree fresh, nothing open | Present the gate message (review the diff + report, then `done`). Never merge on its own — `done` owns that. |
| `done` | the intent is built and archived | Report everything is done; recommend the next unbuilt intent if any. |

`continue` only dispatches; it never relaxes a gate.

## `done` — merge

On `/idsd-ship done`:

1. **Gate.** Run `report.sh gate`. It exits non-zero on a **stale tree** (current `git write-tree` ≠ `reviewed-tree`), a **turnaround-trimmed pass** (any `reviewed-stages` entry marked `(fast)`, or no stage record), or **any open `- [ ]`**, printing which block(s) fired. The freshness and stages blocks the human may explicitly override (then proceed). An open-TODO block has **no override** — the human clears each first: resolve it (do it, then check or delete the box) or route it out of the report (to the ICE `## Follow-ups`, a backlog, a constitution proposal). Watch bullets don't gate.
2. On a clean gate — or freshness/stages overridden with no open `- [ ]` — hand to `idsd-build`'s Phase 5: `status: built`, archive, roadmap, commit (which asks first). The pipeline never commits on its own.
3. **Throwaway cleanup.** In throwaway repo mode (`report.sh repo-mode` → `throwaway`), the local `.idsd/` scratch — the report, plus the intent Phase 5 just archived — is the one thing that outlives the ship, breaking the mode's zero-traces contract. So **after** the commit succeeds (only then — never lose the intent while the work is unlanded), **ask** the human whether to clear it (default yes): on yes run `report.sh discard`, on no leave it. Committed mode keeps `.idsd/` as its durable record — no cleanup; keeping a throwaway `.idsd/` instead is what `promote` (before `done`) is for.

## Parallel execution

Ship many intents concurrently by isolating each; each ship stays single-intent. `idsd-build`'s **Parallel execution** rule is canonical — this only adds the orchestration seams.

- **A worktree per intent.** `idsd-ship <intent>` runs in a dedicated worktree + branch `idsd/NNN-<slug>`, created unless the caller (an external orchestrator, launching one ship per intent from `idsd-audit`'s build batches) already placed you in one. Because the report lives under each worktree (in its `.idsd/`), each ship's `.idsd/ship-report.md` is isolated by construction — no cross-run clobber, no frontmatter thrash from the "different intent" reset. `check-ignore` still runs per worktree; the exclusion itself lives in the *shared* `.git/info/exclude`, so `discard` keeps it while other worktrees exist (script-enforced).
- **One human, serialized.** Build's coupling is still continuous, but across parallel ships the human is a single attention queue: attend each build's live moments in turn while the others' autonomous stretches and qualify subagents run in the background. The subagents already return-or-`block`; the build pauses via blocked→resume. Don't demand simultaneous live sessions.
- **`done` merges serially against an up-to-date target.** Beyond the gate: if the target branch advanced past this branch's base since `reviewed-tree` was stamped, the review is stale against the new base — integrate the target and re-run `qualify full` (which re-stamps) before landing. Merges queue: one `done` at a time.

## Rules

- Adds only sequencing and the merge lifecycle; `idsd-build` and the qualify skill own the actual work.
- Keep chat lean — attention items go to `.idsd/ship-report.md`, never a chat summary. But *lean* is not *cryptic*: every human-facing message (the gate message, any live explanation) must stand on its own, in plain words the human can act on (the full bar lives in the qualify skill's Rules).
- A stage that hard-fails (red gate, build can't complete) stops the pipeline — never relax a sub-skill's gate to keep moving; a blocking decision is asked live, not recorded to dodge it (Interactive first).
