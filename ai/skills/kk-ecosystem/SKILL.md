---
name: kk-ecosystem
description: Refine what agents read — skills, standards, prompts, templates, CLAUDE.md — cutting what no longer earns its place and checking the wiring. Use for "refine the ecosystem", "de-bloat". Runs the shape (kk-skillcraft) and prose (kk-tighten) stages itself.
argument-hint: "the ecosystem root, a subset of it, or the change to refine (default: the whole ecosystem)"
---

Refine the ecosystem. The product is a **smaller** set of instructions that steers an agent the same way or better; a pass that only rewords has failed.

**The bar is `~/.kk-flavor/standards/ecosystem.md`** — read it first; every judgment below is its.

**Protocol.** You orchestrate under `~/.kk-flavor/standards/skill-protocol.md` → **Caller** and → **Orchestrators — interactive first**; the per-file queue and loop belong to the subagents you spawn.

**You own this whole lane, in this order: rule economy (yours), then shape (`kk-skillcraft`), then prose (`kk-tighten`).** Reversing any pair wastes the earlier one: prose tightened before the cut that deletes it, or before the shape pass that moves it, was tightened for nothing.

## 1. Check the wiring

Run `~/.claude/skills/kk-ecosystem/scripts/check.sh` over the ecosystem root and fix what it finds before anything else. Changing `check.sh` itself carries duties its own header states.

## 2. Audit the always-loaded set

`check.sh` prints this set's size, not its members. The set is `CLAUDE.md`, `~/.kk-flavor/inject.md`, the standards that file marks as read on every task, every `@import` those carry, and every skill `description:` the harness holds for a skill without `disable-model-invocation`. Read them whole and hold each line to the top of the rising bar (ecosystem.md → **Earn the place**); anything narrower moves down a tier. An import an installer owns outside this tree is audited here too. Being unable to change it makes the finding a report, not a skip.

## 3. Cut, or move

Work the two things no per-file pass can see:

- **Contradictions** — two files that cannot both be followed. Reconcile to one home and delete the loser. **Hunt them by inbound reference**: for every file or script your scope names, grep the tree for what names it back and read those claims side by side. Widening to read is not touching (`~/.kk-flavor/standards/skill-protocol.md` → **Queue**) and needs no confirmation.
- **Restatements** — one rule living in several files (ecosystem.md → **One home**): keep the copy whose file owns that lane, cross-reference the rest. A rule with two homes is a contradiction that has not happened yet.

Before calling a rule dead, read ecosystem.md → **Move it before you cut it** and try **each** move it names. Hunt candidates deliberately: the largest file's rarely-reached sections, a procedure written out in more than two skills, prose restating what a script already prints. Report a move you judged and rejected — that it was considered is the finding.

## 4. Shape

Spawn `kk-skillcraft` over every skill directory in the resolved scope — the shape lens, which nothing above applies. Skip it only when the scope holds no skill directory, and say so when you do.

## 5. Prose

Spawn `kk-tighten` over the resolved artifact set, plus whatever the two stages above moved.

## 6. Hand off the scripts

A script this pass edited is code no stage of this lane reviews. Hand it off per `~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**.

## 7. Account for it

Report, in this order:

- every rule **deleted**, and what still covers it — or plainly that nothing did;
- every rule **added**, and which one it replaced;
- the always-loaded budget, before and after;
- every `description:` this lane's stages touched, and what its addition bought — one lengthened for a trigger never went through step 3 (ecosystem.md → **Earn the place**);
- total lines, before and after;
- the handoff step 6 named, or plainly that this pass changed no script.

Then re-run `~/.claude/skills/kk-ecosystem/scripts/check.sh` — the cuts themselves break references.

## Rules

Relocating a rule is in scope, **writing a new one is not** — an addition this pass was not asked for is a proposal to your caller, never an edit of your own.
