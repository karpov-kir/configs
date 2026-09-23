**Layer:** base

## Readability floor

Write so the reader understands the text on the first read, without backtracking, in **everything you produce**, down to the reasoning you show. The floor holds over every other rule in the flavor, **Density** included.

* Use one term per thing, every time. Hold hardest to this on a term we coined.
* Define a term at first use in prose, or leave it out. Never cite an identifier or rule ID that resolves in no file.
* In code and in comments, use the identifier's name or the domain's own word. A coined term belongs in neither.
* Put one idea in a sentence, and keep it under about 25 words.
* Two ideas joined by a semicolon are two sentences.
* Name the actor.
* Name the thing again where a pronoun would stand for something last named two clauses back.
* Stack no more than three words into a noun.
* Use plain words, direct verbs and whole sentences. Contractions are fine.
* Put the point alone in the first sentence and the caveat after it, and put a warning before the step it guards.

## Density

* Stay at the artifact's own altitude and one abstraction level.
* In a change description, a report or a reply, lead with the why, because the diff is the source of truth for the implementation trace. A code comment leads with what the symbol does; its form is [code-style.md](code-style.md) → **Comments**.
* Group by purpose, not by file.
* Give each line a fact the reader cannot reach from its surrounding context — the code, the types, the siblings, the diff. Cut or link the rest.
* Re-cut what an edit lands beside. Cut your new sentence together with the ones either side of it, because a clause appended next to one already carrying half of it leaves both. Rework the claim a correction corrects, and do not trail the correction after it.
* Report two or more things as one line per item: `<what was wrong> — <what changed>`. Use no nesting and no preamble above the items. This holds wherever you report several things, a reply included. Text that reads as explanation takes the same shape, and calling it explanation is the dodge. An action still open is the exception, and nests, because the item carries its case and its recommendation.
* Reconcile two statements that cannot both hold into one, and leave neither standing beside the other.
* Describe what is true. Leave out backstory, hedging, justification and what you tried.
* **Outward text**: [human-writing.md](human-writing.md) wins over this section on conflict.

## Replying to a human

Your own reply in the session. A message you compose for someone else is outward text and takes [human-writing.md](human-writing.md). [human-writing.md](human-writing.md) → **AI tells** binds here as well, because a tell marks manufactured writing whoever reads it.

* Carry only what they must know, decide or do. Cut file lists, step-by-step narration, recaps of what you did, and preambles about what you will do. Edit the reply directly.
* Use no headings, and no bold lead-in restating its own line. Render items as a `*` list.
* Order it so they can stop early: chronological where the content is a sequence, otherwise what they must decide before what they only need to know.
* Point at substance that has a durable home — a report, a ticket, a commit — and do not restate it.
* Close with one line: `Next: <the one immediate action>`. Write no line after it. Where no action is next, write `Next: nothing — <the state this leaves>`.
