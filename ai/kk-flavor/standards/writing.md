# Writing Guidelines

## Readability floor

The reader understands the text on the first read, without backtracking — in **everything you produce**: artifacts, your replies, and the reasoning you show. Nothing in the flavor overrides the floor, the Density rules below included.

* **One term per thing, every time**, hardest on terms we coined.
* **Define a term at first use, or don't use it.** Never cite an identifier or rule ID that resolves in no file.
* **One idea per sentence, under about 25 words.**
* **Name the actor.**
* **No noun stack over three words.**
* **Plain words, direct verbs, whole sentences.** Contractions are fine.
* **The point before the caveat, and a warning before the step it guards.**

## Density

* Scope. Stay at the artifact's own altitude and one abstraction level; link to other layers rather than restating them.
* Lead with the "why", not the implementation trace — the diff is source of truth for that.
* Group by purpose, not by file.
* Each line carries a fact unreachable from its surrounding context (code, types, siblings, the diff, …). Cut or link otherwise.
* **One line per item: `<what was wrong> — <what changed>`.** No nesting, and no preamble above the items. Holds wherever you report several things, a reply included.
* One home per fact ([ecosystem.md](ecosystem.md) → One home). Two statements that can't both hold get reconciled to one, never left to coexist.
* Open your enumerations — mark an open set ("e.g.", "and similar"); enumerate plainly only when the set is genuinely fixed.
* No backstory, hedging, or justification — describe what is true, not what we tried.
* **Outward text** additionally follows [human-writing.md](human-writing.md), which wins over this section on conflict.
* Before publishing finished prose, apply the retrofit lens whose description claims it — `kk-tighten` or `kk-humanize`. Prose an `idsd-qualify` pass already covered needs no separate one.

## Replying to a human

Your own reply in the session, not a message you compose for someone else ([human-writing.md](human-writing.md)).

* Carry only what they must know, decide, or do. Cut file lists, step-by-step narration, recaps of what you did, and preambles about what you will do.
* Substance with a durable home (a report, a ticket, a commit) is pointed at, never restated.
* **Close with one line: `Next: <the one immediate action>`.** Nothing follows it; when nothing is next, `Next: nothing — <the state this leaves>`. Spawned with no human, close with your verdict instead ([skill-protocol.md](skill-protocol.md)).
