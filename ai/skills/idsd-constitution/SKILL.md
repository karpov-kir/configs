---
name: idsd-constitution
description: Write or edit .idsd/constitution.md — an IDSD project's principles, baseline NFRs, and gate commands. Use for "seed the project", "define our standards". The developer-owned technical how, not idsd-charter's what & why.
---

Write `.idsd/constitution.md` — the stable, shared layer `idsd-build` reads as Context: project principles, the baseline NFRs every intent inherits, and the commands that resolve gates. Reference `CLAUDE.md` / `PROJECT_CODE_STYLE.md` and the code style, never restate them — a section that just echoes another file becomes a link to it.

## Phase 1 — Inventory what exists

Read the repo first: `CLAUDE.md` and `PROJECT_CODE_STYLE.md` (link to them; if one is missing and the project needs it, point the user to create it), then the tooling that holds the real gate commands — manifest scripts, lint and test config, CI workflow. On a greenfield repo with no tooling, name the intended toolchain and the commands the build will make real, rather than discovering them.

## Phase 2 — Grill the gaps only

Invoke `kk-grill` over the sections of `templates/constitution-template.md`, which says what belongs in each. Its legwork here is Phase 1's inventory — the tooling, CI, and existing config — so every gate command is confirmed from there, never invented. Cover only what isn't already written down.

## Phase 3 — Emit

Run `idsd-qualify`'s `scripts/report.sh check-ignore` first (`~/.claude/skills/idsd-qualify/SKILL.md` → **Report**), then confirm the path once and write `.idsd/constitution.md` from that template.

## Rules

Gate commands must be real, runnable, and **able to fail** — each must exercise the thing its NFR or constraint names and exit non-zero when the threshold is breached. A command that runs but can't fail (wrong target, no assertion, no server started) is worthless, not a gate.
