---
name: kk-humanize
description: Rewrite outward text (PR/ticket descriptions and comments, commit messages, chat, email, announcements, README-grade docs) so it reads as a person wrote it and carries only what the reader needs — scrub AI tells, enforce the reader-action budget, fix voice and rhythm. Lossy counterpart to kk-tighten. Code comments too — easy to follow for the next engineer, constraints never dropped. Use when asked to "humanize", "make it sound human", "de-AI this", "make the comments readable", or when kk-tighten hands off outward text or comments.
argument-hint: "file, git scope (\"the changes\", \"staged\"), or the text itself"
---

Rewrite the outward text resolved from `$ARGUMENTS` so a colleague reads it as written by a person, and reads only what they need. **Lossy by license** — the counterpart to `/tighten`'s lossless contract: you may drop true, unique facts that don't serve this reader.

**Why.** Outward text in the house style — em-dash-stitched, fact-per-line, maximally dense — reads as AI-written, and lossless density still overwhelms a reader who needed one answer. Both cost trust and attention.

**Scope.** Outward text — the set `~/.kk-flavor/standards/human-writing.md` defines (PR/ticket text and comments, chat, email, README-grade docs, …) — gets all three lenses. **Code comments are in scope, for humans first**: all three lenses, with Budget in the comment form that standard defines. Other internal context-window artifacts (skills, standards, prompts, CLAUDE.md) are `/tighten`'s lane and keep house style — don't touch them uninvited. Never code logic, and never the content of quoted text, code blocks, or command output inside an artifact.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

## Setup (once)

- Inject the kk-flavor if needed (read `~/.kk-flavor/inject.md` when its routing isn't already in context), then read the skill protocol (above) and `standards/human-writing.md` — what you rewrite against.
- Resolve the target from `$ARGUMENTS`: a path or directory (outward prose files, plus source files for their comments), a git scope (per the protocol) filtered the same way, or **literal text** (pasted or quoted in chat) → rewrite and return it in chat, no files touched.
- If the text is long or visibly duplicated, run the `tighten` role first (dedup is its job); humanize assumes substance is already deduped.

## The lens

Check every artifact against all three:

1. **Tells** — scrub every pattern the standard's watch-list names (typography, templates, vocabulary, conversational), and anything else that reads manufactured: uniform rhythm, mirrored clause structure, polish a person typing quickly wouldn't produce.
2. **Budget** — the standard's generator: name this artifact's reader and the action they'll take; a sentence stays only if it changes that action. Cut true-but-inert detail; lead with the answer; one concern per message; match the asker's altitude. Substance that matters but not to this reader → park it in a durable home (ticket, commit body, doc) and link, or list it as parked for the caller to place. A class with a written form in the standard (change descriptions, code comments) runs under that form — its bar, its order, its guardrail.
3. **Voice** — like speech to a colleague: contractions, varied sentence and paragraph length, plain verbs, specific nouns, first person where natural.

## Guardrails on the lossy license

- Drop facts, never distort them: whatever stays must mean exactly what it meant — and never invent facts or add content the source doesn't carry. Numbers, names, commitments, and severity are never altered or softened.
- A cut you're unsure the reader can spare → ask your caller, don't guess.
- Don't reintroduce what the lens removes: no hedges, no boilerplate courtesy, no summary paragraph re-added for "politeness".

## Loop deltas

- Literal-text mode replaces the loop: return the rewritten text, then a `parked:` list for any substance you cut that needs a home (omit when empty).

## Verdict

- Pass: `Artifact N/M <path> | <lines>L | OK`
- Fail: `Artifact N/M <path> | <lines>L | WARN` + one line per unresolved issue.
