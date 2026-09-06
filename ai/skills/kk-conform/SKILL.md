---
name: kk-conform
description: Check a change set against the ask it was given — every requirement delivered, nothing beyond them, and no contradiction inside it. Use for "does this match the requirements". Judged by reading, never by running (kk-drive); needs the ask in hand, where kk-code-review needs none.
argument-hint: "the requirement set, plus the change set to judge against it"
---

You run under `~/.kk-flavor/standards/skill-protocol.md`, with these deltas. **The unit is a requirement**, not a file: the queue is the checklist you derive below. **You change no code**, so nothing here is a fix and a finding is resolved by returning it. The protocol's retry and its final sweep do not apply — neither converges anything you have no license to fix.

The gate you fill, and where each finding goes, is `~/.kk-flavor/standards/quality-pipeline.md` → **Conform it before you review it**.

## Derive the checklist, and show it first

A requirement set arrives as prose — a ticket, an issue, a PR body, an intent — and **prose cannot be checked off**. Enumerate it into requirements and **return that list before any verdict**, so your caller sees what you thought you were checking.

**The enumeration is what detects the no-ask case**: a set yielding no requirement is that case, and the section above owns what happens then. **Never derive the ask from the change itself** — a change judged against its own description always conforms, and that reads as a green gate.

## Against the ask

- **Every requirement delivered.** One partly delivered, or quietly descoped, is a finding with the gap named — never a note.
- **Nothing delivered beyond them.** An unrelated file, an opportunistic refactor, a second feature riding along — a finding whatever its quality.

## Internal coherence

- **One requirement satisfied two different ways in two places.**
- **A requirement met at one call site and missed at the parallel one.**
- **Code contradicting the contract prose, types or docs beside it.** A citation resolving to something other than what it claims belongs here too.
- **A term used in a sense the project's own vocabulary does not carry.**

## Your boundaries

- **Nothing you return is a runtime claim.** One you produce goes to the drive gate (`~/.kk-flavor/standards/quality-pipeline.md` → **Drive it before you review it**), labelled an unverified inference.
- **`kk-code-review` finds what is wrong with no ask in hand; you find only what needs the ask to see.** A defect visible without the requirement set is its finding, not yours.
- **`kk-refactor` owns duplication as a quality defect. You own two implementations of one requirement as a contradiction.**
- **One change set against one ask.** Consistency across a whole set of them is a different lane, never yours.

## Return

The derived checklist first, then `Requirement N/M <name> | OK` or `| WARN`; each finding line names its requirement.

**Each finding carries its kind — `undelivered`, `beyond the ask`, or `contradiction`** — the caller routes on it and cannot recover it from the wording.
