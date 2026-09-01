---
name: kk-tighten
description: Tighten internal prose — docs, skills, standards, prompts — for the context window, cutting redundancy and inferable filler. Triggers on "tighten", "shrink", "de-duplicate". Outward text and code comments are kk-humanize's.
argument-hint: "file, directory, or natural-language scope (e.g. \"the changes\", \"staged\"); plus an optional score threshold"
---

Tighten the prose in every artifact resolved from `$ARGUMENTS`: cut what costs context-window tokens without earning them.

**Scope.** Standalone prose only. Never code logic or behaviour — that's `kk-refactor`'s.

## Two licenses, by what the artifact carries

- **Rules an agent reads** — any doc that instructs an agent (skills, standards, prompts, templates, `CLAUDE.md`): **lossy**, whole rules included. `~/.kk-flavor/standards/ecosystem.md` is the bar, and holds the deletion tests — read it before the pass. **Score what survives them** (`~/.kk-flavor/standards/writing.md` → **Score what survives**) one score per rule, on lane `instruction` — or `always-loaded` for an artifact in the set `~/.claude/skills/kk-ecosystem/SKILL.md` → **2. Audit the always-loaded set** names, which then governs (`~/.kk-flavor/thresholds.conf`).
- **Everything else** — a design doc, an investigation, a spec, a report: **lossless**, and never scored. Cut only what the surrounding context recovers — adjacent text, the code it documents, sibling artifacts, the diff. Unsure a cut loses meaning → keep it.

**What `~/.kk-flavor/standards/human-writing.md` covers is `kk-humanize`'s — code comments and docstrings included — and barred to your lens.** Hand off every queued artifact carrying such text, edited or not, per `~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

## Setup (once)

- A directory globs prose documents recursively; **whole project** is every prose doc under the root.
- **Prose the run itself wrote or edited belongs in the queue too, even when it sits outside the code scope** — an intent, a design doc, a ticket body.
- **A change-set scope keeps its code files in the queue for the handoff above, never for your lens.**

## The lens

Check every artifact against all seven:

1. **Redundancy** — the same fact or rule stated twice, within the artifact or across siblings. Keep one home; cross-reference from the rest. A deliberate repeat — a rule restated at its point of use, a safety-critical warning — is signal, not filler.
2. **Inferable filler** — text recoverable from context. Cut.
3. **Dead weight** — true, unique, and it changes nothing the reader would otherwise do. Delete under the lossy license; keep under the lossless one.
4. **Closed taxonomy** — an enumerated list implying completeness where the domain is open. Open it, or keep only if the set is genuinely fixed.
5. **Contradiction** — two statements that can't both hold. Reconcile to one.
6. **Density** — every rule in `~/.kk-flavor/standards/writing.md` → **Density**.
7. **Readability floor** — every rule in `~/.kk-flavor/standards/writing.md` → **Readability floor**; it outranks the other six.

## Loop deltas

- Apply cuts directly — tightening is routine. The one exception is a cut you can't settle against the license: flag it for your caller.
- Editing a sibling to absorb a duplication is in scope, under `~/.kk-flavor/standards/skill-protocol.md` → **Queue**.
