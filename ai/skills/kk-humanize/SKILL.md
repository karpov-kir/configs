---
name: kk-humanize
description: Rewrite outward text — PR/ticket text, commit messages, chat, email, README-grade docs — and code comments so they read as a person wrote them. Use for "humanize", "de-AI this", "make the comments readable". The outward counterpart to kk-tighten; a comment on the wrong construct is kk-refactor's, a false one kk-code-review's.
argument-hint: "file, git scope (\"the changes\", \"staged\"), or the text itself"
---

Rewrite the text resolved from `$ARGUMENTS`. **Lossy by license** — you may drop true, unique facts that don't serve this reader.

**Scope.** Outward text — the set `~/.kk-flavor/standards/human-writing.md` defines — and **code comments, which are for humans first** (Budget takes the comment form that standard defines). Internal agent-facing artifacts are `kk-tighten`'s. Never code logic, and never the content of quoted text, code blocks, or command output.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

**Where the scope holds code, start at the comment-heavy files**: run `~/.claude/skills/kk-humanize/scripts/comment-density.sh` unless your caller passed you its output already, and work its outliers first. Full path, for the reason in `~/.kk-flavor/standards/ecosystem.md` → **Conventions a new file joins**. It reads a change set, not a whole tree — bare, the uncommitted changes; otherwise the git revisions you name. Exit 2 means the scan did not run, so never read it as clean.

## The lens

Check every artifact against all three. `~/.kk-flavor/standards/writing.md` → **Readability floor** comes first; no lens below trades against it.

1. **Tells** — scrub every pattern the standard's watch-list names, and anything else that reads manufactured: uniform rhythm, mirrored clause structure.
2. **Budget** — run the standard's reader-action test over every sentence. Cut true-but-inert detail; lead with the answer; one concern per message; match the asker's altitude. Substance that matters but not to this reader goes to a `parked:` list for the caller to place. A class with a written form in the standard (change descriptions, code comments) runs under it; write a new form only when a class recurs.
3. **Voice** — like speech to a colleague: contractions, varied sentence and paragraph length, plain verbs, specific nouns, first person where natural. Vary length within the floor's sentence limit, never past it.

## Guardrails on the lossy license

- Drop facts, never distort them: whatever stays must mean exactly what it meant, and never invent what the source doesn't carry. Numbers, names, commitments, and severity are never altered or softened.
- A cut you're unsure the reader can spare → ask your caller.

## Loop deltas

- **Literal text** (pasted or quoted in chat) replaces the loop: return the rewrite in chat, no files touched, then a `parked:` list for substance that needs a home (omit when empty).
