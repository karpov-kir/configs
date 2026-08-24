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

* Stay at the artifact's own altitude and one abstraction level; link to other layers rather than restating them.
* Lead with the "why", not the implementation trace — the diff is source of truth for that.
* Group by purpose, not by file.
* Each line carries a fact unreachable from its surrounding context (code, types, siblings, the diff, …). Cut or link otherwise.
* **One line per item: `<what was wrong> — <what changed>`.** No nesting, and no preamble above the items. Holds wherever you report two or more things, a reply included — reading as explanation is not an exemption, it is the dodge.
* One home per fact ([ecosystem.md](ecosystem.md) → **One home**). Two statements that can't both hold get reconciled to one, never left to coexist.
* No backstory, hedging, or justification — describe what is true, not what we tried.
* **Outward text**: [human-writing.md](human-writing.md) wins over this section on conflict.

## Replying to a human

Your own reply in the session, not a message you compose for someone else ([human-writing.md](human-writing.md)).

* Carry only what they must know, decide, or do. Cut file lists, step-by-step narration, recaps of what you did, and preambles about what you will do.
* **No headings, and no bold lead-in restating its own line.**
* Substance with a durable home (a report, a ticket, a commit) is pointed at, never restated.
* **Close with one line: `Next: <the one immediate action>`.** Nothing follows it; when nothing is next, `Next: nothing — <the state this leaves>`. Spawned with no human, close with your verdict instead ([skill-protocol.md](skill-protocol.md) → **Verdict**).
