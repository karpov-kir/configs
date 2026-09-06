---
name: kk-ecosystem
description: Refine what agents read — skills, standards, prompts, templates, CLAUDE.md — cutting what no longer earns its place and checking the wiring. Use for "refine the ecosystem", "de-bloat", or anything routed to the instruction lane. Runs the shape (kk-skillcraft) and prose (kk-tighten) stages itself.
argument-hint: "the ecosystem root, a subset of it, or the change to refine (default: the whole ecosystem)"
---

Refine the ecosystem. The product is a **smaller** set of instructions that steers an agent the same way or better; a pass that only rewords has failed.

**The bar is `~/.kk-flavor/standards/ecosystem.md`** — read it first; every judgment below is its.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md` as an orchestrator (→ **Orchestrators — interactive first**); the per-file queue and loop belong to the subagents you spawn.

**You are the instruction lane** (`~/.kk-flavor/standards/quality-pipeline.md` → **The stages**), **and you own it whole, in this order: rule economy (yours), then shape (`kk-skillcraft`), then prose (`kk-tighten`).** Reversing any pair wastes the earlier one: prose tightened before the cut that deletes it, or before the shape pass that moves it, was tightened for nothing.

## 1. Check the wiring

Run `~/.claude/skills/kk-ecosystem/scripts/check.sh` over the ecosystem root and fix what it finds before anything else. Changing `check.sh` itself carries duties its own header states.

## 2. Audit the always-loaded set

`check.sh` prints this set's size, not its members. The set is `CLAUDE.md`, `~/.kk-flavor/inject.md`, the standards that file marks as read on every task, every `@import` those carry, and every skill `description:` the harness holds for a skill without `disable-model-invocation`. Read them whole and hold each line to the top of the rising bar (`~/.kk-flavor/standards/ecosystem.md` → **Earn the place**); anything narrower moves down a tier. Run `~/.kk-flavor/scripts/bloat-judge.sh instruction <file>` over each file in it, and delete what it names. An import an installer owns outside this tree is audited here too. Being unable to change it makes the finding a report, not a skip.

## 3. Cut, or move

Work the three things a lens reading one file at a time will not reach:

- **Contradictions** — two claims that cannot both be followed. Reconcile to one home and delete the loser. **Hunt them two ways.** By inbound reference: for every file or script your scope names, grep the tree for what names it back and read those claims side by side. And **within each file the change set under review touched** — read the sentences either side of every one of its edits, not only your own. Widening to read is not touching and needs no confirmation.
- **Restatements** — one rule living in several files (`~/.kk-flavor/standards/ecosystem.md` → **One home**): keep the copy whose file owns that lane, cross-reference the rest. **Run `~/.claude/skills/kk-ecosystem/scripts/ruleecho.sh <root>` over the root and read the report it prints**, which labels every pair it found by kind. A grep by inbound reference finds only pairs where one file names the other, and two files that never mention each other are the pair that drifts. The scan reads bolded markdown only, so a rule in a heading, a restatement reworded below its shared-word threshold, and duplication in code are invisible — **a clean run proves no bolded markdown duplication, never no duplication.**
- **Under-reuse** — a rule with one home that a second file needs and cannot reach (`~/.kk-flavor/standards/ecosystem.md` → **One home**). A restatement leaves two copies to compare; this leaves a gap. **Run `~/.claude/skills/kk-ecosystem/scripts/cite-graph.sh <root>` over the root and read the report it prints**, which maps who reaches each file and describes its own figures. What it cannot see is a consumer that needs a rule and names nothing at all — for each rule your scope owns, name the files that act on it and check each can reach it.

Before calling a rule dead, read `~/.kk-flavor/standards/ecosystem.md` → **Move it before you cut it** and try **each** move it names. Hunt candidates deliberately: the largest file's rarely-reached sections, a procedure written out in two or more skills, prose restating what a script already prints.

## 4. Shape

Spawn `kk-skillcraft` over every skill directory in the resolved scope, **and over the standards, prompts, templates and `CLAUDE.md` in it**. Skip the stage only when the scope holds none of them, and say so when you do.

## 5. Prose

Spawn `kk-tighten` over the resolved artifact set, plus whatever the two stages above moved.

## 6. Hand off the scripts

A script this pass edited is code no stage of this lane reviews. Hand it off per `~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**.

## 7. Account for it

Re-run `~/.claude/skills/kk-ecosystem/scripts/check.sh` — the cuts themselves break references — then report, in this order:

- every rule **deleted**, and what still covers it — or plainly that nothing did;
- every rule **added**, and which one it replaced;
- every **move** step 3 judged and rejected;
- the always-loaded budget, before and after;
- every `description:` this lane's stages touched, and what the change bought;
- total lines, before and after;
- the handoff step 6 named, or plainly that this pass changed no script.

## Rules

Relocating a rule is in scope, **writing a new one is not** — an addition this pass was not asked for is a proposal to your caller, never an edit of your own.
