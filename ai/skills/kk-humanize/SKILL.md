---
name: kk-humanize
description: Rewrite outward text — PR/ticket text, commit messages, chat, email, README-grade docs — and code comments so they read as a person wrote them. Use for "humanize", "de-AI this", "make the comments readable". The outward counterpart to kk-tighten; comment placement and truth belong to other lanes.
argument-hint: "file, git scope (\"the changes\", \"staged\"), or the text itself"
---

Rewrite the text resolved from `$ARGUMENTS`.

**Scope.** The outward text `~/.kk-flavor/standards/human-writing.md` defines, code comments included. Never code logic, and never anything reproduced verbatim — quoted text, code blocks, command output.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Artifact`; deltas below.

**Where the scope holds code, run `~/.claude/skills/kk-humanize/scripts/comment-density.sh --bar`** — with the git revisions to scan, or no argument at all for the uncommitted changes — and read the files it names first. A thermometer, never the pass's check — `bloat-judge` (**Budget**, below) decides what goes.

## The lens

Check every artifact against all three, `~/.kk-flavor/standards/writing.md` → **Readability floor** first.

1. **Tells** — scrub every pattern `~/.kk-flavor/standards/human-writing.md` → **AI tells** names, and anything else that reads manufactured: uniform rhythm, mirrored clause structure.
2. **Budget** — run `~/.kk-flavor/standards/human-writing.md` → **Budget**. A class with a written form runs under it — change descriptions and review comments in that file, code comments in `~/.kk-flavor/standards/code-style.md` → **Comments**. Write a new form only when a class recurs.
3. **Voice** — like speech to a colleague: contractions, varied sentence and paragraph length, plain verbs, specific nouns, first person where natural.

**Drop facts, never distort them, and never invent what the source doesn't carry.** A number, name, commitment or severity is never dropped where its absence changes what the text commits to.

## Loop deltas

- **Literal text** (pasted or quoted in chat) replaces the loop: return the rewrite in chat, no files touched.
