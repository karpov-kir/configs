# Human Writing (outward text)

Anything a person reads as communication — a PR body, a review comment or reply, a commit message, a ticket, chat, a doc. **Code comments** take the form at the end; everything above binds them too.

**A send you cannot recall goes to the human first** ([live-systems.md](live-systems.md) → **Arrange the undo before the act**).

## Budget — the keep test

**Name this artifact's reader and the one action they'll take.** Then, sentence by sentence, **name the edit it causes or the answer only this reader can give** — in those words, before it stays. "It's true", "they might want it" and "it shows the work was done" are not consequences. A sentence whose consequence you cannot name is already cut, and **unsure counts as unnamed**.

Deliberately **lossy** — detail that matters, but not to this reader, goes to a durable home (ticket, commit body, doc) with a link, or is dropped. **The link then stands in for that detail rather than introducing it.**

**Then score what survived** ([writing.md](writing.md) → **Score what survives**) on its `outward-text` lane — one score per point, never per sentence; what it cuts takes those same two exits.

**Your method is never the content** — not the machine you drove on, not what you substituted for something unavailable, not the concerns you checked and found clean. A verification *result* can earn a line; the route you took to it never does. Name the gap a verdict rests on, never the search that found it.

**Length is the one tell you can measure, and it has a single cause: explaining why you are right.** The surplus is always the case restated, the design justified, the alternative pre-refuted. **Cut to what they act on**; if it will not compress, you have not decided what you are asking them. Under a screen, and one thing said once.

## AI tells

A watch-list, not a complete set. What they share is manufactured polish and symmetry.

**Typography & structure** — em dashes stitching clauses (use commas or periods); bold-prefixed parallel bullets ("**Speed:** …"); emoji headers; Title Case Headings; bullets where two sentences would do.

**Templates** — negative reframes ("not just X, it's Y"); the rule of three (triple adjectives, triple bullets, escalating triads); an intro previewing the text, or any recap of what you already said ("In conclusion", "Overall"); "In today's fast-paced …"; "from X to Y" false ranges; "Whether you're X or Y"; rhetorical questions as transitions.

**Vocabulary** (unnaturally frequent) — delve, tapestry, underscore, harness, testament, leverage, utilize, robust, seamless, comprehensive, pivotal, crucial, foster, bolster, streamline, elevate, empower, unlock, game-changer, cutting-edge, landscape, realm, journey, navigate, myriad, plethora, holistic, synergy; "dive into" / "unpack" a topic; "plays a vital role".

**Conversational** — sycophancy and throat-clearing ("Great question!", "I wanted to reach out"); hedge frames ("It's important to note", "That said"); sentence-initial "Moreover / Furthermore / Additionally"; boilerplate closers ("Let me know if you have any questions").

**Register** — writing to a peer as though you were their assistant. Grading their reasoning ("your instinct holds", "good catch", "you're right"); asking permission you do not need ("your call", "say the word", "happy to"); a closing line handing back a decision they already hold. State the finding and what follows from it. Where a choice really is theirs, name it once, in the same voice as everything else — the deference is the tell, not the fact.

**Self-rating tails** — a clause appended to grade your own claim or effort, almost always in an "X rather than Y" frame: "inferred rather than observed", "checked rather than assumed". State the thing, or drop it. Where a review lens demands the status of a claim, that label rides the finding it belongs to, never a clause loose in prose.

## Change descriptions (PRs)

The author's side, for a reviewer deciding approve-or-not, in order. **A linked ticket owns the incident, the evidence and the timeline**, so the description carries the change and its review and nothing about how the problem was found.

- **What changes and why** — a few sentences, leading with the outcome.
- **Review focus** — the risky parts and each deliberate trade-off, led by the **surface delta**: which exports arrived, went, or changed shape.
- **Verification** — what ran and what it showed: links and counts, not narration. Only what CI and the diff don't already show.

**Not why the code is right** — deciding that is the reviewer's job, and taking it from them is what makes a body long.

## Review comments

The reviewer's side, for an author deciding what to change. **Each note goes on the line it concerns; the body carries the verdict, any mismatch with what was asked, and nothing else a line could have held.**

Drop a note that fails **Budget**'s keep test rather than marking it optional. **No coverage accounting** — the reviewer writing about the review, which **Budget** already bars.

**A reply is a review comment**, and opens on what changes rather than on agreeing — the thread already holds the case, and the change shows the agreement.

## Code comments

The existence bar and the published-surface exception to it are [code-style.md](code-style.md) → **Comments**. What that bar admits:

- Delete a comment whole when it narrates what the code says (`@param`/`@returns` restating the signature included), when a rename would carry it (flag the rename for the refactor lane), when it justifies a decision no reader would question, or when a sibling covers it. True, unique content goes too — anecdotes, provenance, alternatives considered. Once a comment stays, shortening it never drops its constraint, invariant, or warning.
- A published surface's contract prose carries what the types can't — call order, lifecycle, error modes, units, ranges, caller invariants.
