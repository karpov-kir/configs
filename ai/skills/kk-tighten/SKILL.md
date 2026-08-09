---
name: kk-tighten
description: Tighten internal prose — docs, skills, standards, prompts — for the context window, cutting redundancy and inferable filler. Lossy on agent-facing rules, lossless elsewhere. Triggers on "tighten", "shrink", "de-duplicate". Outward text and code comments are kk-humanize's.
argument-hint: "file, directory, or natural-language scope (e.g. \"the changes\", \"staged\")"
---

Tighten the prose in every artifact resolved from `$ARGUMENTS`: cut what costs context-window tokens without earning them.

**Scope.** Standalone prose only. Never code logic or behaviour (that's `kk-refactor`'s), and never code comments or docstrings — those are `kk-humanize`'s whole lane, read by people first.

## Two licenses, by what the artifact carries

- **Rules an agent reads** — any doc that instructs an agent (skills, standards, prompts, templates, `CLAUDE.md`): **lossy**, whole rules included. `~/.kk-flavor/standards/ecosystem.md` is the bar, and holds the deletion tests — read it before the pass.
- **Everything else** — a design doc, an investigation, an ICE, a report: **lossless**. Cut only what the surrounding context recovers — adjacent text, the code it documents, sibling artifacts, the diff. Unsure a cut loses meaning → keep it.

**Outward text — the set `~/.kk-flavor/standards/human-writing.md` defines — belongs to `kk-humanize`**: hand off after your pass, per `~/.kk-flavor/standards/skill-protocol.md` → **Finish in the lanes your edits opened**. Your handoff carries every outward-text artifact in your queue, edited or not — the lane is barred to you, so leaving one out strands it.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

## Setup (once)

- A directory globs prose documents recursively; **whole project** is every prose doc under the root.
- **Prose the run itself wrote or edited belongs in the queue too, even when it sits outside the code scope** — an intent, a design doc, a ticket body.

## The lens

Check every artifact against all seven:

1. **Redundancy** — the same fact or rule stated twice, within the artifact or across siblings. Keep one home; cross-reference from the rest. A deliberate repeat — a rule restated at its point of use, a safety-critical warning — is signal, not filler.
2. **Inferable filler** — text recoverable from context, plus backstory, hedging, and justification ("we tried…", "importantly"). Cut.
3. **Dead weight** — true, unique, and it changes nothing the reader would otherwise do. Delete under the lossy license; keep under the lossless one.
4. **Closed taxonomy** — an enumerated list implying completeness where the domain is open. Open it, or keep only if the set is genuinely fixed.
5. **Contradiction** — two statements that can't both hold. Reconcile to one.
6. **Density** — every rule in `~/.kk-flavor/standards/writing.md` → **Density**.
7. **Readability floor** — every rule in `~/.kk-flavor/standards/writing.md` → **Readability floor**; it outranks the other six.

## Loop deltas

- Apply cuts directly — tightening is routine. The one exception is a cut you can't settle against the license: flag it for your caller.
- Editing a sibling to absorb a duplication is in scope, under `skill-protocol.md` → Queue.
