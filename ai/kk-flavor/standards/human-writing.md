# Human Writing (outward text)

Anything a person reads as communication — a PR body, a review comment or reply, a commit message, a ticket, chat, a doc. **Code comments too**: everything here binds them. What a comment must first clear is [code-style.md](code-style.md) → **Comments**.

**A send you cannot recall goes to the human first** ([live-systems.md](live-systems.md) → **Arrange the undo before the act**).

## Budget — the keep test

**Name this artifact's reader and the one action they'll take** — for a comment, every later reader of the file, not this change's reviewer. Then, sentence by sentence, **name the edit it causes or the answer only this reader can give** — in those words, before it stays. "It's true", "they might want it" and "it shows the work was done" are not consequences. A sentence whose consequence you cannot name is already cut, and **unsure counts as unnamed**.

**Then run `~/.kk-flavor/scripts/bloat-judge.sh <kind>` over what survived, and delete what it names.** The kind is the artifact's class: `--changed comment <file>` for a source file (`--changed=<revisions>` where the scope is git revisions); `pr-body`, `review`, `commit`, `ticket` or `slack` on stdin. A class with no kind — a doc, an email — ends at the keep test above.

**Deleting is the only exit from either cut, and it needs no home and no permission.**

**Your method is never the content** — not the machine you drove on, not what you substituted for something unavailable, not the concerns you checked and found clean. A verification *result* can earn a line; the route you took to it never does. Name the gap a verdict rests on, never the search that found it.

**Length is the one tell you can measure, and it has two causes.** One is explaining why you are right: the case restated, the design justified, the alternative pre-refuted. The other is a slot that wants filling, a heading or a template field. You answer it from whatever is at hand, and that is what the reader can already see. **An empty slot is a finished one.** **Cut to what they act on**; if it will not compress, you have not decided what you are asking them. Under a screen, and one thing said once.

## AI tells

A watch-list, not a complete set. What they share is manufactured polish and symmetry.

**Typography & structure** — em dashes stitching clauses (use commas or periods); bold-prefixed parallel bullets ("**Speed:** …"); emoji headers; Title Case Headings; bullets where two sentences would do.

**Templates** — negative reframes ("not just X, it's Y"); the rule of three (triple adjectives, triple bullets, escalating triads); an intro previewing the text, or any recap of what you already said ("In conclusion", "Overall"); "In today's fast-paced …"; "from X to Y" false ranges; "Whether you're X or Y"; rhetorical questions as transitions.

**Vocabulary** (unnaturally frequent) — delve, tapestry, underscore, harness, testament, leverage, utilize, robust, seamless, comprehensive, pivotal, crucial, foster, bolster, streamline, elevate, empower, unlock, game-changer, cutting-edge, landscape, realm, journey, navigate, myriad, plethora, holistic, synergy; "dive into" / "unpack" a topic; "plays a vital role".

**Conversational** — sycophancy and throat-clearing ("Great question!", "I wanted to reach out"); hedge frames ("It's important to note", "That said"); sentence-initial "Moreover / Furthermore / Additionally"; boilerplate closers ("Let me know if you have any questions").

**Register** — writing to a peer as though you were their assistant. Grading their reasoning ("your instinct holds", "good catch", "you're right"); asking permission you do not need ("your call", "say the word", "happy to"); a closing line handing back a decision they already hold. State the finding and what follows from it. Where a choice really is theirs, name it once, in the same voice as everything else — the deference is the tell, not the fact.

**Self-rating tails** — a clause appended to grade your own claim or effort, almost always in an "X rather than Y" frame: "inferred rather than observed", "checked rather than assumed". State the thing, or drop it. Where a review lens demands the status of a claim, that label rides the finding it belongs to, never a clause loose in prose.

## Change descriptions (PRs)

The author's side, for a reviewer deciding approve-or-not. **Link the ticket wherever the branch carries one**: it owns the incident, the evidence and the timeline, so the description carries the change and nothing about how the problem was found.

**Prose, never headings of your own**: a heading you add is a slot, and **Budget** rules what fills one. A repo template's headings stand.

Cover three things, in this order, and only while each has content. What changes and why, led by the outcome the consumer sees. A surface that arrived, went or changed shape belongs in that outcome, not in a note of its own. Then what you chose and what it cost, where the diff does not carry it — an alternative you rejected, a duplication kept on purpose, an invariant now split across two files. Then what this leaves someone to do, such as a release to cut or a pin to drop.

**Not why the code is right** — deciding that is the reviewer's job. **Not what ran** — CI reports itself. What it cannot produce, a manual drive or a migration against real data, falls to **Budget**.

## Review comments

The reviewer's side, for an author deciding what to change. **Each note goes on the line it concerns; the body carries the verdict, any mismatch with what was asked, and nothing else a line could have held.**

Drop a note that fails **Budget**'s keep test rather than marking it optional. **No coverage accounting** — the reviewer writing about the review, which **Budget** already bars.

**A reply is a review comment**, and opens on what changes rather than on agreeing — the thread already holds the case, and the change shows the agreement. Resolve a thread with `Done <link to the commit>` and nothing else.
