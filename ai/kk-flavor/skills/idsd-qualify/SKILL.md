---
name: idsd-qualify
description: Run the quality pipeline over the working tree with a merge stamp. Use for "qualify the changes" inside an IDSD project. The `.idsd` report layer over kk-qualify — the same pass without a report is that skill's.
---

**Runs:** inline — landing

Callers: standalone, or `idsd-ship`'s quality pass, which runs this skill **inline**. The current coordinator owns leaf dispatch under `~/.kk-flavor/standards/skill-protocol.md`; do not create another qualification coordinator.

**The pass is `~/.kk-flavor/skills/kk-qualify/SKILL.md`** — apply its pass **inline**, reusing the unchanged contract already held (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**). That skill ends at a closing reply; this one adds the report, the stamp and the lifecycle around it. **Every heading below that it also carries is the delta over its section of that name**, and the rest is this skill's own.

## Running a pass

1. **`~/.kk-flavor/skills/idsd-qualify/scripts/report.sh check-ignore`**, before anything else (**Report**).
2. **Set the base — the report this pass appends to.** With none for this intent, `report.sh init "<NNN-slug>"`, or `init "review: <description>"` for a standalone review. Over an existing one `init` refuses and prints the routing; follow it. `report.sh` resolves the repo from the shell's cwd, so confirm the path `init` prints is the change set's repo.
3. **`report.sh invalidate <intent>`**, once the report is set. Retain its JSON context for this attempt; pass it to the stage workers with their candidate scope.
4. For applicability-based skips, run `report.sh scope <base-ref> <intent>` with the review's explicit base. It records the exact candidate and worktree. Without a valid scope receipt, run every stage rather than inventing a skip. If repairs change the candidate, refresh this receipt against the same base and revalidate affected evidence before stamping.

**Dispatch `~/.kk-flavor/workers/idsd/qualify/reconcile.md` over each completed stage's return, then
accept it with `report.sh stage-result <result.json> <intent>`.** Hand that worker the return, the
report's JSON context and the stage's scope; it settles which findings become `items` and returns
them. **Reconciling in this session is what would hold it at the pass's own tier** — every
`report.sh` call here is deterministic and the stamp is a script, so the judgment is the only model
work, and it is bounded enough to leave
(`~/.kk-flavor/standards/model-policy.md` → **Cost is a design constraint**). The stamp stays here. [stage-results.md](stage-results.md) is the complete submission and recovery procedure. The tool records completion and renders its decision items. **Streamed, a patch is not a return** (`~/.kk-flavor/standards/streaming.md`): submit only after the worker's final verdict and your reconciliation of its findings.

**A stale gate is a Decide item** (`~/.kk-flavor/standards/quality-pipeline.md` → **Gates**), and gate verification precedes the stamp. Under `idsd-ship`, `idsd-build`'s Phase 2 already resolved them.

When all stages complete, stamp: `report.sh stamp "<stage entries>" <intent>` — **its usage string is the entry vocabulary's only home**, and a copy in this file would drift from the tool that validates it. A stage that ran is never stamped skipped, or vice versa.

**Historical marker-based passes need requalification.** Let the tool reject their missing typed evidence; do not construct receipts for work whose inputs cannot be verified.

**A human's "don't re-qualify" binds the tree it was said about, not the session** — once `report.sh state <intent>` prints `re-qualify`, the refusal has expired and you ask again rather than infer consent.

**Report wording settles before submission.** Apply `kk-edit` inline to authored items, preserving every decision, its evidence and stakes. Generated finding blocks preserve the accepted wording: resolve their checkboxes only on positive evidence, and put explanations outside the blocks. Surrounding authored prose remains editable. Do not spawn another worker or run an external judge for the structured report.

## Report

**`.idsd/` in this suite means the resolved scratch root, not a path in the repo.** `report.sh root` prints it, and it is the only way to learn it. In **committed** mode it is `<repo>/.idsd/`. In **throwaway** mode it is outside the working tree — shared by every branch and worktree of the clone — so a skill that joins `.idsd/` onto the repo root writes where no other worktree will look, and where the next `git add -A` can see it. Every `.idsd/<file>` below, and in every skill of this suite, is relative to what that subcommand printed.

**The layout is fixed at both scopes.** Human-facing project files are `.idsd/charter.md` and generated `.idsd/roadmap.md`; each active or archived intent exposes only `intent.md` beside `for-agents/`.

- Project records: `.idsd/for-agents/{language,decisions,playbook}.md`.
- Intent records and report: `.idsd/intents/<intent>/for-agents/{language,decisions,playbook,qualify-report}.md`; archiving preserves this structure under `.idsd/archive/<intent>/`.
- Arbitrary handoffs, proposals and evidence: `for-agents/supporting/` at the project or intent scope. Tool-owned qualification state stays in the tool's reserved paths.

Run `report.sh layout check` to validate the layout. For an old layout, explicitly run `report.sh layout migrate --dry-run`, review the planned moves, then `report.sh layout migrate --apply`. Migration refuses active reports: finish their pass or preserve and resolve their outstanding residue before closing them. Never bypass that refusal. Legacy `constraints.md` moves to `.idsd/for-agents/supporting/constraints.md` for manual curation through `idsd-charter`; migration never promotes its entries or invents a charter. Use only the new paths after migration.

**One report per intent**, at `.idsd/intents/<intent>/for-agents/qualify-report.md`, **persisting across runs** — the working digest. `~/.kk-flavor/skills/idsd-qualify/scripts/report.sh` owns the path and every deterministic operation on it, `idsd-ship`'s lifecycle ones included — never done by hand. **Every subcommand that reads a report takes the intent as its last argument**, omitted only while one report is open — so pass it whenever you know it.

**Two intents ship in parallel only in separate worktrees.** The freshness stamp fingerprints the **whole tree**, not the intent's files, so two ships in one worktree each stamp a tree holding the other's edits and invalidate each other's gate on every save.

**A landed ship's report is retired, not left standing** — `report.sh close <intent>`, which `idsd-ship done` runs. **A standalone review has no `done`** — `report.sh close review` retires it, and unsaid, `report.sh list` offers it as work in flight for good.

**Before the first write into `.idsd/` — any file, by any skill — run `report.sh check-ignore`**: it is what keeps the directory out of the human's `git add -A`, and nothing else runs it. Its exit 1 blocks the write however its message is worded.

**Two repo modes, decided by whether `.idsd/` is tracked in git** (`report.sh repo-mode` prints which): **committed** — `.idsd/` is part of the durable record; **throwaway** — the whole `.idsd/`, intents and report alike, leaves zero traces, and survives only if the human promotes it (`idsd-ship promote`). Either way **never commit a report — however that is authorised** (`~/.kk-flavor/standards/skill-protocol.md` → **Caller**): no route commits one, `promote` included, which keeps every ship's scratch ignored on its way to committed mode. A stamped report asserts a merge gate over a tree fingerprint, and committed it carries that assertion past the tree it was taken from.

### The decision log

`.idsd/for-agents/decisions.md`. **Written for the next agent, not the human** — nothing here is presented. It holds **decisions a stage settled without asking**, each with what determined it, and **standing observations** — monitor-only notes and pointers to follow-ups routed out of the report. The companion rule is `~/.kk-flavor/standards/skill-protocol.md` → **Orchestrators — interactive first**.

It is an appended record, so `~/.kk-flavor/standards/records.md` is the whole delta. Its two sections are `## Promotion candidates` and `## Decisions`, sharing the record's cap. Candidates are settled decisions worth proposing as project-wide invariants; open human questions remain in the report.

Classify an existing entry with `report.sh record --intent <NNN-slug> classify local-decisions "<selector>" candidate|decision` (for the project log, use `project-decisions` and omit `--intent <NNN-slug>`). Classification preserves the entry and its count; it neither edits the charter nor grants promotion approval.

**Write it only through `report.sh record --intent <NNN-slug> {append|bump|revise|evict|admit} local-decisions "<text>"`** — this ship's own log, which finalize merges upward. Two hand-run read-modify-writes leave the file holding whichever landed second, with nothing in any diff to say the other's entries went. In throwaway mode every worktree of the clone races for that one copy (**Report**).

Tracked in committed mode only; in throwaway mode `done` discards it, so route out anything that must outlive the ship. **Write it before `report.sh stamp`** — content added afterwards moves the tree out from under `reviewed-tree`, and the merge gate reads the pass as stale.

**Read it at pass start and re-evaluate every entry against the tree — the log is pruned here and nowhere else.** An entry whose subject is gone from the code, or which a later one supersedes, goes now: `record evict`, whatever its count. One still binding on what this pass examines takes `record bump`. **Then `report.sh decisions-reviewed <intent>`, after this pass's `invalidate`** — `stamp` refuses until you have, and `invalidate` clears it, so no pass inherits another's reading of the log.

### The residue

```markdown
# Decide

<optional: context several items share, stated once here, never per item>

- [ ] **<stable ID> · <Falsified | Fork | Pending evidence> —** <the action, one line>
  <the case: what it is, why it matters, the evidence>
  **Recommend:** <the answer>
```

**What earns an item and the order it lands in are `~/.kk-flavor/skills/kk-qualify/SKILL.md` → **The residue**.** This file adds only where that residue is written:

- **One group, `Decide`** — no per-stage sections, no summary, and no reading list. A monitor-only observation goes to the decision log.
- **Persisted items use stable IDs instead of positional numbers**, so a later stage can refer to an earlier decision without renumbering it. The tool renders submitted severity alongside each item.
- **On re-qualify every unresolved `- [ ]` carries forward verbatim** (`report.sh carry <intent>` lists them) — dropped only on positive evidence it's resolved (fixed in the tree, or the human acted on it), never because this pass didn't re-examine its area.

## After the pass

The closing message adds to `~/.kk-flavor/skills/kk-qualify/SKILL.md` → **After the pass**. The **repo mode** rides its status line. After an `idsd-ship` build, surface `idsd-build`'s checkpoint evidence too. After a standalone review, one line saying `report.sh close review` retires it (**Report**). In throwaway mode add one line — `.idsd/` is local scratch this run, `/idsd-ship promote` to keep it.

## Rules

- **Any verification that reads a ref runs against the stamped `reviewed-tree`**, not the remote.
- **Resuming after the history moved** is `report.sh invalidate <intent>`, then a fresh spawn against the new HEAD.
