**Layer:** base

# Human Writing (outward text)

Anything a person reads as communication — a PR body, a review comment or reply, a commit message, a ticket, chat, a doc. **Code comments too**: everything here binds them. What a comment must first clear is [code-style.md](code-style.md) → **Comments**.

**Each prose paragraph is one line.** A field you type into — a PR or ticket body, a comment, a chat message — turns your newline into a line break, so the paragraph arrives ragged. Repository files and commit messages are not fields: wrap those as usual.

**A send you cannot recall goes to the human first** ([live-systems.md](live-systems.md) → **Arrange the undo before the act**), and takes any lane it is owed before the send, never after.

## Edit pass

Run the **edit lane** over every outward artifact in what you present for approval, delivery or publication — PR titles and bodies, commit messages, tickets, docs, messages to others and code comments — **whoever wrote it**. **Authorship sets the landing, never whether the lane runs**: your own text it edits, another's it proposes. **Voice is theirs, volume is not** — how they say it stays theirs, how much of it there is answers to the bar.

Apply it inline by default; a separate worker is not required. **The judge runs over anything this lane covers whose kind it names** — `bloat-judge.sh` lists them when asked for one it has not got — and where a measured bar also says the artifact is over it, how many go is the bar's rather than the author's. **Boilerplate is never a unit**: a repo template's own lines and any tool-generated block stay, whatever the judge makes of them. Recheck changed text after substantive revisions; an unchanged artifact already covered by the lane needs no repeat pass.

**The judge does not run over a reply, a worker's structured return or a report, though it names a kind for all three.** A reply and a return take no lane either: apply the writing rules directly. A report takes the lane inline, never a separate worker. Agent instructions take their wording pass inside the instruction lane, after semantics and structure settle.

## Budget — the keep test

**Name this artifact's reader and the one action they'll take** — for a comment, every later reader of the file, not this change's reviewer. Then, sentence by sentence, **name the edit it causes or the answer only this reader can give** — in those words, before it stays. "It's true", "they might want it" and "it shows the work was done" are not consequences. Cut a sentence with no consequence only when the meaning-preservation check below permits it. Keep uncertain cases for judgment.

Preserve the artifact's required meaning: facts, negation, exceptions, numbers, tense, conditionality, commitments, severity and open questions. When a sentence carries one of these, shorten its expression instead of deleting its substance. Keep quoted text unchanged.

Apply this check within the current editing pass. External voting is an explicit tool for a disputed durable deletion, not a required call for each reply, review return or artifact.

**Your method is never the content** — not the machine you drove on, not what you substituted for something unavailable, not the concerns you checked and found clean. A verification *result* can earn a line; the route you took to it never does. Name the gap a verdict rests on, never the search that found it.

**Length is the one tell you can measure, and it has two causes.** One is explaining why you are right: the case restated, the design justified, the alternative pre-refuted. The other is a slot that wants filling, a heading or a template field. You answer it from whatever is at hand, and that is what the reader can already see. **An empty slot is a finished one.** **Cut to what they act on**; if it will not compress, you have not decided what you are asking them. Under a screen, and one thing said once.

## AI tells

A watch-list, not a complete set.

**Typography & structure** — em dashes stitching clauses (use commas or periods); bold-prefixed parallel bullets ("**Speed:** …"); emoji headers; Title Case Headings; bullets where two sentences would do.

**Templates** — negative reframes ("not just X, it's Y"); the rule of three (triple adjectives, triple bullets, escalating triads); an intro previewing the text, or any recap of what you already said ("In conclusion", "Overall"); "In today's fast-paced …"; "from X to Y" false ranges; "Whether you're X or Y"; rhetorical questions as transitions.

**Vocabulary** (unnaturally frequent) — delve, tapestry, underscore, harness, testament, leverage, utilize, robust, seamless, comprehensive, pivotal, crucial, foster, bolster, streamline, elevate, empower, unlock, game-changer, cutting-edge, landscape, realm, journey, navigate, myriad, plethora, holistic, synergy; "dive into" / "unpack" a topic; "plays a vital role".

**Conversational** — sycophancy and throat-clearing ("Great question!", "I wanted to reach out"); hedge frames ("It's important to note", "That said"); sentence-initial "Moreover / Furthermore / Additionally"; boilerplate closers ("Let me know if you have any questions").

**Register** — writing to a peer as though you were their assistant. Grading their reasoning ("your instinct holds", "good catch", "you're right"); asking permission you do not need ("your call", "say the word", "happy to"); a closing line handing back a decision they already hold. State the finding and what follows from it. Where a choice really is theirs, name it once, in the same voice as everything else — the deference is the tell, not the fact.

**Self-rating tails** — a clause appended to grade your own claim or effort, almost always in an "X rather than Y" frame: "inferred rather than observed", "checked rather than assumed". State the thing, or drop it. Where a review lens demands the status of a claim, that label rides the finding it belongs to, never a clause loose in prose.

## Change descriptions (PRs)

The author's side, for a reviewer deciding approve-or-not. **Link the ticket wherever the branch carries one**: it owns the incident, the evidence and the timeline, so the description carries the change and nothing about how the problem was found.

**A PR based on another open PR's branch carries the stack map**, one line beside the ticket link: `Stack, base first: #12 ← #13 ← **#14 (this one)**`. The same line stands in every PR of the stack, so a reviewer landing on any one of them sees the whole order.

**Prose, never headings of your own**: a heading you add is a slot, and **Budget** rules what fills one. A repo template's headings stand.

Cover three things, in this order, and only while each has content. What changes and why, led by the outcome the consumer sees. A surface that arrived, went or changed shape belongs in that outcome, not in a note of its own. Then what you chose and what it cost, where the diff does not carry it — an alternative you rejected, a duplication kept on purpose, an invariant now split across two files. Then what this leaves someone to do, such as a release to cut or a pin to drop.

**Not why the code is right** — deciding that is the reviewer's job. **Not what ran** — CI reports itself. What it cannot produce, a manual drive or a migration against real data, falls to **Budget**.

## Review comments

The reviewer's side, for an author deciding what to change. **Each note goes on the line it concerns; the body carries the verdict, any mismatch with what was asked, and nothing else a line could have held.**

Drop a note that fails **Budget**'s keep test rather than marking it optional. **No coverage accounting** — the reviewer writing about the review.

**A reply is a review comment**, and opens on what changes rather than on agreeing — the thread already holds the case, and the change shows the agreement. Resolve a thread with `Done <link to the commit>` and nothing else.
