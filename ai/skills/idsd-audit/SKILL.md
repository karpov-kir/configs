---
name: idsd-audit
description: Audit the .idsd/ intent set for cross-intent consistency — links, build order, milestones, charter scope, inherited constraints. Use for "audit the intents", "do these intents still hang together". Read-only; each fix routes to its owning skill. The whole set — one intent's own ambiguities are idsd-intent's clarify pass.
---

## Phase 1 — Load the set

**The set you are auditing lives under the resolved scratch root, which is not always in the repo** — `~/.claude/skills/idsd-qualify/SKILL.md` → **Report** names what prints the directory.

Read the intent set under `.idsd/`: active intents (`intents/`), built ones (`archive/`), `charter.md`, `constraints.md`, `language.md`, `roadmap.md`, `decisions.md`. No `.idsd/` → say so and stop.

## Phase 2 — Check the invariants

Skip a check only when its inputs are absent.

- **Links & build order** — every `links` entry uses a known relation (`extends`/`depends-on`/`blocks`, nothing else) and resolves to a real intent; the `depends-on` graph is acyclic; directions follow the Links rule in `~/.claude/skills/idsd-intent/SKILL.md` → **Rules**.
- **Build batches (parallel schedule)** — group the unbuilt intents into batches whose members share no unbuilt dependency — one worktree per intent.
- **Milestone coherence** — no `mvp` intent depends on a `vnext`/unscheduled or still-`draft` one.
- **Inherited constraints** — flag an entry in `constraints.md` no `mvp` intent satisfies, and any intent constraint contradicting one.
- **Charter scope** — flag intents outside the charter's Scope, in-scope areas no intent covers, and intents orphaned by a past scope cut. Scope's **Not yet specified** bucket is none of those: it is in scope and deliberately unsharp.
- **Language** — the set's vocabulary against `language.md`: a domain term the intents lean on with no entry; **two terms in use for one thing**; two entries that cannot both hold; an entry no artifact uses any more (`~/.kk-flavor/standards/records.md` → **Promotion is the exit upward**).
- **Duplication** — overlapping goals/constraints across intents; a constraint recurring in ≥3 intents, or a choice `decisions.md` has re-settled that often → propose promoting it to `.idsd/constraints.md`.
- **Well-formedness** — each intent has a goal with a real *why*, 3–7 constraints, and ≥1 success + ≥1 failure scenario; a goal joined by "and" is either mis-named (rename) or two intents (split); scenario coverage shouldn't be thin for the intent's surface. Flag the gap; don't re-grill here.
- **Status hygiene** — `built` intents live in `archive/`, not `intents/`; numbers are unique and contiguous; `roadmap.md` matches current frontmatter.
- **Follow-up hygiene** — flag any archived intent still carrying an unchecked `- [ ]`, and an active intent's open `- [ ]` item that names an intent which doesn't exist.

## Phase 3 — Report

One report, grouped by severity — **Blocker** (breaks a build or ships a violation), **Fix** (drift to reconcile), **Nit** — plus the informational **Build batches** list when any intent is unbuilt.

Each finding names the file(s), the **owning skill** to fix it through — `idsd-intent` (intents, links, scope sync, `language.md`), `idsd-charter` (vision, scope, `constraints.md`) — and the smallest reconciling move, never a redesign.
