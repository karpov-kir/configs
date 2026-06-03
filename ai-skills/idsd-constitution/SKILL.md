---
name: idsd-constitution
description: Set up or edit the long-term memory for an IDSD project — principles, baseline NFRs, and gate commands that idsd-build injects as Context. Optional and run rarely (project seeding or standards changes). Use when asked to "seed the project", "set up IDSD", "define our standards/constitution". The technical "how" layer — developer-owned.
---

Write `.idsd/constitution.md` — the stable, shared layer `idsd-build` reads as Context. It holds only IDSD-specific defaults: project principles, the baseline non-functional requirements (NFRs) every intent inherits, and the concrete commands that resolve gates. It references code style; it never restates it.

Optional. Feature work runs without it — `idsd-build` derives gates from repo tooling when it's absent.

Where this fits: `idsd-charter` (optional) → **`idsd-constitution`** (optional) → `idsd-intent` → `idsd-build`. The constitution is the *how*; the charter the *what/why*.

## Phase 1 — Inventory what exists

Read the repo first:
- `CLAUDE.md`, `PROJECT_CODE_STYLE.md` — link to them; if missing and the project needs them, point the user to create them.
- Repo tooling — `package.json`/`Makefile`/`pyproject` scripts, lint/test config, CI workflow — for the real gate commands. On a greenfield repo with no tooling yet, name the intended toolchain and the commands the build will make real, rather than discovering them.

## Phase 2 — Grill the gaps only

One question at a time, recommended answer each. Cover only what isn't already written down:
1. **Principles** — 3–7 project-specific non-negotiables, beyond general code style.
2. **Baseline NFRs** — defaults every intent inherits unless its own constraints override (latency, accessibility, security posture, coverage floor).
3. **Gate commands** — exact commands for build / lint / test / coverage / perf, discovered in Phase 1 and confirmed.

## Phase 3 — Emit

Write `.idsd/constitution.md` from `templates/constitution-template.md`. Confirm the path once before writing.

## Rules

- Reference, never duplicate, `CLAUDE.md` / `PROJECT_CODE_STYLE.md` — if a section just echoes another file, replace it with a link.
- Gate commands must be real, runnable, and **able to fail** — each must exercise the thing its NFR/constraint names and exit non-zero when the threshold is breached. A command that runs but can't fail (wrong target, no assertion, no server started) is worthless, not a gate.
- Keep it at the standards altitude — principles, NFRs, gate commands. It grows as durable standards accumulate, but link out rather than restate.
