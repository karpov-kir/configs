---
name: kk-ecosystem
description: Refine what agents read — skills, standards, prompts, templates, CLAUDE.md — against the bar in ecosystem.md, checking the wiring and cutting what no longer earns its place. Use for "refine the ecosystem", "de-bloat". Runs that whole lane — it spawns kk-skillcraft and kk-tighten itself, so queue it alone.
argument-hint: "the ecosystem root, a subset of it, or the change to refine (default: the whole ecosystem)"
---

Refine the ecosystem. The product is a **smaller** set of instructions that steers an agent the same way or better; a pass that only rewords has failed.

**The bar is `~/.kk-flavor/standards/ecosystem.md`** — read it first; every judgment below is its.

**Protocol.** You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` — its Caller and orchestrator rules bind you; the per-file queue and loop belong to the subagents you spawn.

**You own this whole lane, in this order: rule economy (yours), then shape (`kk-skillcraft`), then prose (`kk-tighten`).** Reversing any pair wastes the earlier one — tightened prose you then delete was tightened for nothing, and a skill re-shaped after its prose pass moves text the pass already judged. A caller queues **only you**, never a lane stage beside you. Build each spawn prompt from `~/.kk-flavor/templates/spawn-prompt.md`.

## 1. Check the wiring

Run `~/.claude/skills/kk-ecosystem/scripts/check.sh` over the ecosystem root and fix what it finds before anything else.

## 2. Audit the always-loaded set

`check.sh` prints this set's size, not its members: it is `CLAUDE.md`, `~/.kk-flavor/inject.md`, and the standards that file marks as read on every task. Read them whole and hold each line to the top of the rising bar (ecosystem.md → **Earn the place**); anything narrower moves down a tier.

## 3. Cut, or move

Work the two things no per-file pass can see:

- **Contradictions** — two files that cannot both be followed. Reconcile to one home and delete the loser.
- **Restatements** — one rule living in several files (ecosystem.md → **One home**): keep the copy whose file owns that lane, cross-reference the rest. A rule with two homes is a contradiction that has not happened yet.

Before calling a rule dead, read ecosystem.md → **Move it before you cut it** and try **each** move it names. Hunt candidates deliberately: the largest file's rarely-reached sections, a procedure written out in more than two skills, prose restating what a script already prints. Report a move you judged and rejected — that it was considered is the finding.

## 4. Shape

Spawn `kk-skillcraft` over every skill directory in the resolved scope — the lens that asks whether a skill is *shaped* so an agent reaches it and complies, which nothing above asks. Skip it only when the scope holds no skill directory, and say so when you do.

## 5. Prose

Spawn `kk-tighten` over the resolved artifact set, plus whatever the two stages above moved.

## 6. Hand off the scripts

A script this pass edited is code no stage of this lane reviews. Hand it off per `~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**, carrying exactly the scripts this pass changed.

## 7. Account for it

Report, in this order:

- every rule **deleted**, and what still covers it — or plainly that nothing did;
- every rule **added**, and which one it replaced;
- the always-loaded budget, before and after;
- total lines, before and after;
- the handoff step 6 named, or plainly that this pass changed no script.

Then re-run `~/.claude/skills/kk-ecosystem/scripts/check.sh` — the cuts themselves break references.

The size-ledger row is your **caller's**, not yours: `kk-foreman` appends one after the chain it ran, and a row written here as well double-counts every chained pass. Invoked directly, report the budget and let the human place it.

## Rules

Relocating a rule is in scope, **writing a new one is not**. A rule you want to add that this pass was not asked for is a proposal to your caller, not an edit of your own.
