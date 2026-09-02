---
name: kk-humanize
description: Rewrite outward text — PR/ticket text, commit messages, chat, email, README-grade docs — and code comments so they read as a person wrote them. Use for "humanize", "de-AI this", "make the comments readable". The outward counterpart to kk-tighten; comment placement and truth belong to other lanes.
argument-hint: "file, git scope (\"the changes\", \"staged\"), or the text itself; plus an optional score threshold"
---

Rewrite the text resolved from `$ARGUMENTS`. **Lossy by license** — you may drop true, unique facts that don't serve this reader.

**Scope.** Outward text — the set `~/.kk-flavor/standards/human-writing.md` defines — and **code comments, which are for humans first**. Never code logic, and never the content of quoted text, code blocks, command output, …

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

**Where the scope holds code, start at the comment-heavy files**: run `~/.claude/skills/kk-humanize/scripts/comment-density.sh` — bare for the uncommitted changes, or the git revisions to scan — unless your caller passed you its output already, and work its outliers first.

## The lens

Check every artifact against all three. `~/.kk-flavor/standards/writing.md` → **Readability floor** comes first; no lens below trades against it.

1. **Tells** — scrub every pattern `~/.kk-flavor/standards/human-writing.md` → **AI tells** names, and anything else that reads manufactured: uniform rhythm, mirrored clause structure.
2. **Budget** — run `~/.kk-flavor/standards/human-writing.md` → **Budget**: its keep test over every sentence, then its score over what survives. Substance that matters but not to this reader goes to a `parked:` list for the caller to place. A class with a written form runs under it — change descriptions and review comments in that file, code comments in `~/.kk-flavor/standards/code-style.md` → **Comments**. Write a new form only when a class recurs.
3. **Voice** — like speech to a colleague: contractions, varied sentence and paragraph length, plain verbs, specific nouns, first person where natural.

## Guardrails on the lossy license

- Drop facts, never distort them, and never invent what the source doesn't carry. A number, name, commitment or severity is never dropped where its absence changes what the text commits to.

## Loop deltas

- **Literal text** (pasted or quoted in chat) replaces the loop: return the rewrite in chat, no files touched, then a `parked:` list for substance that needs a home (omit when empty).
