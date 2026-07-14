---
name: idsd-ship
description: "Ship an ICE intent end-to-end or review standalone changes. Subcommands: `idsd-ship <arg>` (full pipeline; authors the intent if missing), `idsd-ship done` (merge), `idsd-ship qualify` (quality stages only, no build or merge), `idsd-ship continue` (resume from current state), `idsd-ship promote` (keep a throwaway .idsd/)."
argument-hint: "<arg> | done | qualify | continue | promote"
---

Drive one intent from ICE to merge-ready — or review standalone changes — through a fixed pipeline, accumulating a `.idsd/ship-report.md` digest of what needs the human's attention. You **orchestrate** existing skills — invoke them, never reimplement; their rules still hold (gates absolute, follow-ups routed before archive, no-mocks, …).

**Interactive first.** Prefer asking the human live over deferring to the digest. A decision you can't settle from the intent, the code, or a sensible default — or an ambiguity that changes what gets built — is *blocking*: stop and ask now. Every such question carries a **recommended answer you earned by legwork first** (the intent, code, charter, constitution, precedent) — recommend from evidence, not a guess; when it stays open after checking, say what you checked. This holds for a subagent's `blocked` too: on relay it names its recommendation. The sub-skills' own clarify gates (e.g. `idsd-build`'s Phase 1) still fire — never suppress one by recording instead. `.idsd/ship-report.md` is for what does *not* block — surfaced for the human at the final checkpoint, not lost in chat history.

Where this fits: `idsd-intent` → (`idsd-audit`) → **`idsd-ship`** (= `idsd-build` + `/code-review` + `/security-review` + `/refactor` + `/tighten` + `idsd-retro`, sequenced).

**kk-flavor wiring.** If the flavor isn't injected, read `~/.kk-flavor/inject.md` first. Resolve every cross-referenced skill and toggle through the bucket at `~/.kk-flavor`:
- The skill names below (`/code-review`, `/security-review`, `/refactor`, `/tighten`, and the idsd-* names) are **roles** — spawn the skill mapped in `~/.kk-flavor/config.yaml` → `roles` (unlisted names resolve to themselves; the quality roles default to the `kk-` variants).
- Run a stage only when its `~/.kk-flavor/config.yaml` → `pipeline` flag is true. A stage turned off is **skipped silently** — don't spawn it and don't record its absence as a gap. `build`, `code-review`, and the final gate are structural and always run.

## Subcommands

| Command | What it does |
|---|---|
| `idsd-ship <arg>` | Full pipeline: build + quality stages + checkpoint. `<arg>` is an existing intent slug, or a **ticket / new-feature ref** — if no intent file matches, Build authors one first (see **Build**). Ends with the gate message (see **After the quality stages**). |
| `idsd-ship done` | Proceeds to merge (idsd-build Phase 5), gated on review freshness (see **`done` — merge**). Reads the intent from the report's frontmatter. Error if no report exists or if the report was produced by `qualify` mode (no merge target). |
| `idsd-ship qualify` | Quality stages only (2–5), no build, no merge. For standalone changes with no intent, or for re-qualifying after post-pipeline refinements. |
| `idsd-ship continue` | Detect where the change set stands and run the next step (or report it's done). Reads state from the report — needs no `<arg>`. See **`continue`**. |
| `idsd-ship promote` | Turn a throwaway `.idsd/` into a durable idsd project via `scripts/report.sh promote`; never commits on its own (mechanism in **Report**). |

If `<arg>` is unspecified (and the subcommand is not `done`, `qualify`, `continue`, or `promote`), list the not-yet-built intents and ask which.

## Flow

```
ship <intent>           qualify (standalone)
  │                         │
  ▼                         │
Build                       │
  │                         │
  ▼                         ▼
Quality stages ◄──── qualify (re-qualify)
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
        └─ tree dirty ──► "run qualify first"
              │
              ├─ user approves skip ──► Merge
              │
              └─ user agrees ──► qualify (re-qualify)
```

## Report

`ship-report.md` **always** lives at `.idsd/ship-report.md` and **persists across runs** — the working digest, never committed (the ICE and git history are the durable record). `scripts/report.sh` owns the path and scaffolds it (`report.sh init "<intent>"` creates `.idsd/` + the report from the template); never hardcode a location. The deterministic report operations — init, stamp, the `done` gate, carry-forward, the `continue` state token, and `promote` — run through `scripts/report.sh` (in the skill dir), never by hand.

**Two modes — committed vs throwaway — decided by whether `.idsd/` is tracked in git** (`report.sh mode` prints which). This load-bearing distinction drives how the report stays out of git and whether `.idsd/` is durable:
- **Committed** — the repo has *committed* `.idsd/` content, so it durably uses idsd. The report is ignored via a **shared, tracked `.gitignore` entry** (`.idsd/ship-report.md`), and `.idsd/` changes (intents authored this run) are part of the durable record — committed at `done`.
- **Throwaway** — no `.idsd/`, or an untracked one a single-shot run created. The **whole `.idsd/`** (intents *and* report) is excluded locally via `.git/info/exclude`, so it leaves **zero traces** across any number of tickets and **never touches `.gitignore`**. `.idsd/` is never committed unless the human promotes it.

**Never commit the report or a `.gitignore` diff for it.** Always run `scripts/report.sh check-ignore` before any stage: it applies the mode's mechanism so the fingerprinting `git add -A` (stamp/gate) can never stage the report — committed mode asserts the tracked entry and WARNs if missing (add it), throwaway mode ensures the local `.idsd/` exclusion. Either way the report is local scratch, never staged, never committed.

**Promoting a throwaway run.** When the human wants to keep a single-shot `.idsd/`, `report.sh promote` converts it: drop the local exclusion, add the shared `.gitignore` entry, stage `.idsd/` (report excluded) — then the human commits. It never commits on its own. A pure `qualify` with no intents has nothing durable to promote (only the report, which is never committed) — say so rather than promoting an empty `.idsd/`.

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

`done` compares `reviewed-tree` against the current tree to catch unreviewed changes (see **`done` — merge**). The report is **only the residue that needs the human — not a record of the run** (*What goes in*, below). At most two groups, each present only when it has items — **Decide** (`- [ ]` actions) and **Watch** (monitor-only) — with no per-stage sections and no summary. On re-qualify **every unresolved `- [ ]` carries forward verbatim** — dropped only with positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area; new findings append as fresh `- [ ]`. Watch bullets are re-evaluated each pass — kept while relevant, dropped when moot.

### What goes in (and what never does)

The report holds the *non-blocking* attention set — decisions to ratify, deferrals, gated/declined fixes, monitor-only watches — never a blocking question (asked live, per Interactive first).

**Every item must stand on its own.** Someone reading only the report — who never saw the run or this chat — must understand *what it is* and *why it matters*, in plain language, and be able to act. Cut run-narration and jargon: no "what we tried", no dependency lists, no command strings, no bare code identifiers or private tags (`retro F5a`, "at altitude") — state the thing in words. But never cut the stakes — a one-liner that only makes sense if you were there fails the bar as surely as bloat does, so spend the sentence or two it takes. If an item can't be self-contained without pasting run detail, that detail belongs in a durable home (backlog, ICE, commit) and the report links to it.

A **Decide** item is the decision, what the human must do, and **your recommended resolution** (earned by legwork, per Interactive first — never a bare choice); tag its origin stage in plain words only when that aids the human. A **Watch** item is monitor-only, no checkbox — a thing to keep an eye on, or a one-line pointer to follow-ups routed out of the report (e.g. retro improvements filed to a backlog). Watch never gates `done`.

**The test: if the human takes no action, it is not in the report.** A resolved or applied fix, a passed / clean / not-applicable stage, "here's what changed", and any verification narration (what passed, what's byte-identical, invariants confirmed) are all **omitted** — they live in the diff, the commit, and the fact the pipeline ran. The report shrinks to nothing when nothing needs the human — that's the success case, not an omission. The one exception is a fix the human might want to *reverse*: record that as a Decide item (ratify or revert), not as an FYI.

## Quality stages

Start from the right base. **First pass on a change set** (no report, or one recording a different intent): scaffold the report with `scripts/report.sh init "<intent-or-description>"` — it creates `.idsd/` and stamps the frontmatter intent it covers. **Re-qualify** (the report already covers this change set): keep it and reconcile per the carry rule above — `scripts/report.sh carry` lists the prior open `- [ ]` so none are lost. Either way, run the stages in order, appending each item to **Decide** or **Watch** the moment it surfaces, not batched at stage end.

When all quality stages complete, stamp the tree fingerprint: run `scripts/report.sh stamp` — it computes `git write-tree` and writes the hash into `reviewed-tree`.

**How each stage runs.** Build runs **inline**, not for consistency's sake but because its human coupling is *continuous* — `idsd-build` restates, clarifies, and decides with the human throughout, a live dialogue the `blocked`→resume bridge (built for *occasional* pauses) would turn into constant ping-pong. Across parallel ships the human still runs this dialogue inline, just one build at a time (see *Parallel execution*). The analysis stages are the opposite: mostly autonomous, returning findings as data with an occasional `blocked`, so each runs in a **dedicated subagent** (which also isolates its heavier context from the orchestrator) — code-review, security-review, refactor, tighten, and retro. The subagent executes the skill in full and returns structured findings; it never decides whether to run, and never fakes the pass with its own inline judgment.

**Spawn the skill, not your own review.** Each subagent prompt names exactly one skill and hands it the change scope — nothing more. The skill defines what to check; the spawn prompt must not pre-select, narrow, or invent which rules apply, must not borrow another stage's lane (correctness is `/code-review`'s, style/structure/architecture is `/refactor`'s, vulnerabilities are `/security-review`'s, prose/concision is `/tighten`'s), and must let the subagent run the skill's full decomposition itself. The only thing you may inject is emphasis the **user explicitly stated** this run (e.g. "ensure arch-doc compliance" → pass to refactor) — never a rule you inferred. A spawn prompt that lists specific CLAUDE.md rules to look for, or asks for findings outside the named skill's scope, is the defect this guards against. Only the main thread has the human, so every decision and all report-writing stay here: take the subagent's returned findings, ask the human live for any that block, record the rest. When a subagent hits something only the human can settle — a clarification, a gated choice — it pauses and returns `blocked: <what it needs>` rather than guessing. Answer it, then **resume that same subagent by its ID** so it continues with its context and progress intact; never start a fresh one — a new spawn loses the work it already did and the skill state it was holding.

**Reconcile contradictions.** When two stages give opposing verdicts on the same location — one clears what another flags — or a claim contradicts an observation (a subagent reports green while a tool shows otherwise) — adjudicate empirically before recording: re-run the check yourself and trust neither side's word over the result.

**Scale to the change; settle design once.** Match review weight to the delta — a small, low-risk change doesn't need every stage's full fan-out, a broad or risky one does; scope each subagent to the changed surface rather than re-running the world. And when a change reworks a **shared or cross-repo primitive** (a vendored arch primitive, a public type, a cross-cutting contract), settle its target shape with the human in one pass *before* iterating — converging a design through many one-finding-at-a-time review round-trips is the most expensive path.

1. **Build** (skip on `qualify`) — **author the intent first if it's missing.** If no intent file matches `<arg>` (not in `.idsd/intents/` or the archive), spawn `idsd-intent` to author one before building — seeding it from the ticket when `<arg>` looks like a ticket ref (fetch via the `atlassian` skill), else treating `<arg>` as the feature description. Then run `idsd-build` for the intent in its **pipeline mode**: it runs restate/confirm, context, and implementation until gates are green, then hands back — skipping its self-review, checkpoint, and merge, which the dedicated passes and the final approval below replace. As it builds, idsd-build *records and routes* every follow-up to the ICE's `## Follow-ups` and every durable standard to a constitution proposal — at build time, its own rule (resolving them stays merge-gated under `done`). Before recording, confirm it did: an unrouted follow-up is a build defect, not something the report absorbs.
   - Record as **Decide** items: deferrals to confirm, constraints that need human judgment (can't become a gate), and decisions to ratify — each pointing to its durable home (the ICE `## Follow-ups`, a constitution proposal) idsd-build already wrote. An ambiguity resolved with no open decision is not recorded. The report flags for the human; it never replaces the durable record.
2. **Code-review** — spawn a subagent to run `/code-review` on the build's changes: it applies every fix it can make correctly and returns the rest — findings needing a human decision (a trade-off, an ambiguous intent, a risky change), plus any behaviour-changing fix it made. On its return, ask the human live for blocking findings; record the others.
   - Record as **Decide** items: findings needing a human decision. A fix already applied is recorded only if the human might want to reverse it (ratify-or-revert); otherwise it's just the diff.
3. **Security-review** — *only if* the change touches a security surface (input handling, filesystem/network/exec, auth or session, secrets, deserialization, or any constitution security invariant); otherwise skip. Spawn a subagent to run `/security-review` on the build's changes: it applies trivial safe fixes (e.g. secret redaction) and returns the rest as findings with severity, exploit scenario, and fix. On its return, ask the human about anything blocking; record the others.
   - Record as **Decide** items: findings needing a human decision (severity + exploit + fix). Record nothing when the surface was clean or untouched.
4. **Refactor** (loop, max 3 iterations) — the pipeline's safety net for completeness; don't shortcut it. Each iteration spawns a **fresh** subagent (not a resume) to run `/refactor` on the changes: it applies routine fixes, returns gated architecture/testing/project-setup ones as proposals (not applied), and reports whether the change is now fully compliant. Spawning fresh re-reads the now-changed tree, catching fixes that only surface after earlier ones land. Stop the moment a subagent reports compliant; otherwise iterate to the cap. The blocked→resume rule still holds *within* an iteration — if that pass's subagent pauses, resume it by ID; only a new iteration spawns fresh. On each return, ask the human about gated proposals; record what's deferred or declined.
   - Record: gated fixes the human must rule on (**Decide**), duplication deferred under the extract threshold (**Watch**), and — if it hit the cap without reaching compliant — that, with what's open (**Decide**). Nothing when it reached compliant with no gated proposal.
5. **Tighten** (optional) — *only if* the build added or changed standalone prose (docs, markdown) or non-trivial comment blocks; otherwise skip. Spawn a subagent to run `/tighten` on those files: it applies the lossless cuts and returns only what it couldn't safely cut (a doubtful-substance cut, an unresolved `WARN`). On its return, record that remainder.
   - Record as a **Decide** item only what `/tighten` flagged but couldn't safely cut; nothing otherwise.
6. **Retro** — *runs as the last stage of a full `idsd-ship <intent>`; skip on a bare `qualify` re-run unless asked — except always run when the human course-corrected during the run, or the run modified its own tooling (a skill, this pipeline, a shared script).* Course-corrected is observable, not a vibe: the human answered a blocking question, redirected or re-scoped a stage, rejected or reworked an applied fix, or flagged a missed rule. Write a terse, factual run-log — what was asked, what each stage did, where the human corrected course, what was deferred (decisions only, no self-assessment) — then spawn `idsd-retro` as a **fresh** subagent (per **Spawn the skill**), handing it the run-log plus the diff and any skill/doc/prompt/script the run touched. Derive the touched-files list from `git diff --stat` against the base, not from memory — the diff is authoritative, so never tell the retro a file is "unchanged" when it shows in the diff (if it was reviewed in an earlier round this session, say *that*, don't hide it). It owns the adversarial method and lenses; it returns routed, evidence-backed findings.
   - **Split retro's findings by kind — only a defect in *this change* gates.** A finding that's a defect the run introduced (a bug or risk, or a fix to fold back in) → record as a gating **Decide**, like any other stage. A finding that's an improvement *for next time* (a better rule, a tooling tweak, a standard worth adding) → file it straight to the durable home it names — a backlog entry, a constitution proposal, or a note against the skill/doc — non-gating, exactly as `idsd-build` files its follow-ups at build time. Leave the report just **one non-gating line under Watch**, e.g. `- 5 retro follow-ups filed → <home>`. A run-improvement is a follow-up, never a gating `- [ ]`: it must not block the change's merge or crowd the Decide list. Record nothing only when the run surfaced no improvement (a course-corrected run never does).

## After the quality stages

**The report is the single source of substance; the chat message is short.** Everything the human must read, decide, or act on lives in `.idsd/ship-report.md`, self-contained — so they read *one* place, never combining chat and file. The chat message after the passes is a short pointer, not a second digest. It contains only:
- **one status line** — the mode (throwaway/committed) and the item count (e.g. "2 to decide, 1 to watch"), or "report is empty — nothing needs you" when it is;
- **the next-step actions** (below);
- **at most one live blocking question** (per *Interactive first*).

It must **not** contain: per-stage results or verdicts ("code-review clean", "tighten cut two lines"), a retro narrative, a restatement or preview of the report's items, or "here's what changed". Those are verification narration — omitted from the report *and* from chat (per *if the human takes no action, it is not in the report*). If something is worth the human's attention, it goes **in the report** as a Decide or Watch item, not into a chat paragraph. When running from an intent, the one exception is `idsd-build`'s checkpoint evidence (gate + scenario + observed outcomes) — surface that alongside the pointer.

**Gate message (the next-step actions).**
- Review the diff and the report.
- If you make changes, run `/idsd-ship qualify` to re-run the quality stages.
- When you're ready to merge, run `/idsd-ship done`.

For `qualify` mode without an intent, omit the `done` option — there is no merge step. Instead: "Review the diff and the report. If you make changes, run `/idsd-ship qualify` again."

**Throwaway notice.** In throwaway mode (`report.sh mode` → `throwaway`), add **one line** to the status: `.idsd/` is local scratch this run (excluded, nothing committed, zero traces) — `/idsd-ship promote` to keep it, else it stays local-only. Never promote or commit `.idsd/` unless the human asks. (The full mechanism lives in *Report*, not the chat.)

**Dogfooding that turns into a redesign.** The gate-message loop (`qualify` → edit → `qualify`) is for *refinements* that keep the intent's contract. When the human's hands-on use instead reshapes that contract — a different presentation, a reworked surface, a new sub-feature — it's a **re-scope, not an open edit session**: amend the ICE via `idsd-intent` first so the new shape is recorded, then commit the reviewed state as a checkpoint *before* the rework starts, so the redesign lands as its own distinct change set. Skip the checkpoint commit and the reviewed work and the rework fuse into one diff that can no longer be split.

## `continue` — resume from current state

`idsd-ship continue` recovers where the change set stands and runs the next step. Read the state deterministically with `scripts/report.sh state` (never hand-parse the report); it prints one token, and each routes to an existing behaviour whose rules hold unchanged:

| Token | State | `continue` does |
|---|---|---|
| `no-report` | nothing in progress | Say so. With an `<intent>` arg, start `ship <intent>`; otherwise list the not-yet-built intents and recommend one (per *Interactive first*). |
| `resume` | quality never completed (`reviewed-tree` unstamped) | Run the full `ship <intent>` flow for the report's intent — build restates and idempotently resumes to green (a no-op if already there), then the quality stages run and stamp. |
| `re-qualify` | reviewed once, tree moved since | Run `qualify` (quality stages only) — the build already shipped; carry-forward keeps open items. |
| `decide` | quality done, tree fresh, open `- [ ]` remain | Present the gate message with the Decide list; the human clears each, then runs `done`. |
| `ready` | quality done, tree fresh, nothing open | Present the gate message (review the diff + report, then `done`). Never merge on its own — `done` owns that. |
| `done` | the intent is built and archived | Report everything is done; recommend the next unbuilt intent if any. |

`continue` only dispatches; it never relaxes a gate.

## `done` — merge

On `/idsd-ship done`:

1. **Gate.** Run `scripts/report.sh gate` (in the skill dir). It exits non-zero on either a **stale tree** (current `git write-tree` ≠ `reviewed-tree`) or **any open `- [ ]`**, printing which block(s) fired. A freshness-only block the human may explicitly override (then proceed). An open-TODO block has **no override** — the human clears each first: resolve it (do it, then check or delete the box) or route it out of the report (to the ICE `## Follow-ups`, a backlog, a constitution proposal). Watch bullets don't gate.
2. On a clean gate — or freshness overridden with no open `- [ ]` — hand to `idsd-build`'s Phase 5: `status: built`, archive, roadmap, commit (which asks first). The pipeline never commits on its own.

## Parallel execution

Ship many intents concurrently by isolating each; each ship stays single-intent. `idsd-build`'s **Parallel execution** rule is canonical — this only adds the orchestration seams.

- **A worktree per intent.** `idsd-ship <intent>` runs in a dedicated worktree + branch `idsd/NNN-<slug>`, created unless the caller (an external orchestrator, launching one ship per intent from `idsd-audit`'s build batches) already placed you in one. Because the report lives under each worktree (in its `.idsd/`), each ship's `.idsd/ship-report.md` is isolated by construction — no cross-run clobber, no frontmatter thrash from the "different intent" reset. `check-ignore` still runs per worktree.
- **One human, serialized.** Build's coupling is still continuous, but across parallel ships the human is a single attention queue: attend each build's live moments in turn while the others' autonomous stretches and quality-stage subagents run in the background. The subagents already return-or-`block`; the build pauses via blocked→resume. Don't demand simultaneous live sessions.
- **`done` merges serially against an up-to-date target.** Beyond the stale-tree / open-TODO gate: if the target branch advanced past this branch's base since the quality stages stamped `reviewed-tree`, the review is stale against the new base — integrate the target and re-run `qualify` (which re-stamps) before landing. Merges queue: one `done` at a time.

## Rules

- Adds only sequencing and the digest; the sub-skills own the actual work.
- Keep chat lean — write attention items to `.idsd/ship-report.md`, never into a chat summary, and don't echo the digest back in prose. But *lean* is not *cryptic*: the "stand on its own" bar applies to every human-facing message too (the gate message, any live explanation), not just report lines — say the thing in plain words the human can act on, never so terse it only makes sense if they watched the run. And a pointer must point at real substance: don't claim something is "filed"/"noted"/"routed" unless it lives in a named durable home that actually contains it — a pointer to nothing is worse than saying it plainly.
- A stage that hard-fails (red gate, build can't complete) stops the pipeline — never relax a sub-skill's gate to keep moving; a blocking decision is asked live, not recorded to dodge it (Interactive first).
