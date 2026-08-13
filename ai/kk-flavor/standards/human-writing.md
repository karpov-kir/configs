# Human Writing (outward text)

The set this covers: anything a person reads as communication — PR/ticket descriptions and comments, review replies, commit messages, chat, email, announcements, README-grade docs, … **Code comments** take the form below; everything above it applies to them too.

**A send you cannot recall goes to the human first** ([live-systems.md](live-systems.md) → **Arrange the undo before the act**).

## Budget — the reader-action test

**Name this artifact's reader and the action they'll take; a sentence earns its place only by changing that action.** Deliberately **lossy** — detail that matters, but not to this reader, goes to a durable home (ticket, commit body, doc) with a link, or is dropped. **The link then stands in for that detail rather than introducing it.** Naming the home and retelling it anyway is the restatement banned everywhere else, and this reader pays for it twice: they read the summary, then they open the thing it summarised.

## AI tells

A watch-list, not a complete set. What these tells share is manufactured polish and symmetry.

**Typography & structure** — em dashes stitching clauses (use commas or periods); bold-prefixed parallel bullets ("**Speed:** …"); emoji headers; Title Case Headings; bullets where two sentences would do.

**Templates** — negative reframes ("not just X, it's Y"); the rule of three (triple adjectives, triple bullets, escalating triads); an intro previewing the text, or any recap of what you already said ("In conclusion", "Overall", "In summary"); "In today's fast-paced / ever-evolving …"; "from X to Y" false ranges; "Whether you're X or Y"; rhetorical questions as transitions.

**Vocabulary** (unnaturally frequent) — delve, tapestry, underscore, harness, testament, leverage, utilize, robust, seamless, comprehensive, pivotal, crucial, foster, bolster, streamline, elevate, empower, unlock, game-changer, cutting-edge, landscape, realm, journey, navigate, myriad, plethora, holistic, synergy; "dive into" / "unpack" a topic; "plays a vital role".

**Conversational** — sycophancy and throat-clearing ("Great question!", "I hope this email finds you well", "I wanted to reach out", "I just wanted to follow up"); hedge frames ("It's important to note", "It's worth noting", "That said"); sentence-initial "Moreover / Furthermore / Additionally"; boilerplate closers ("Let me know if you have any questions", "Please don't hesitate"); narrating your own care ("I left the open questions in rather than guessing", "checked X rather than assuming") — the reader wants the questions, not your restraint in not answering them.

## Change descriptions (PRs)

The author's side, for a reviewer deciding approve-or-not, in order. **A linked ticket owns the incident, the evidence and the timeline** (**Budget**), so the description carries the change and its review and nothing about how the problem was found.

- **What changes and why** — a few sentences, leading with the outcome.
- **Review focus** — the risky parts and each deliberate trade-off, led by the **surface delta**: which exports arrived, went, or changed shape.
- **Verification** — what ran and what it showed: links and counts, not narration. Only what CI and the diff don't already show.

## Review comments

The reviewer's side, for an author deciding what to change. **Each note goes on the line it concerns; the body carries the verdict, any mismatch with what was asked, and nothing else a line could have held.**

A note earns its place only by producing an edit, or an answer only the author can give. Drop the rest instead of marking it optional: a note they skip trains them to skim the ones that matter. What you ran is your business — no coverage accounting, and no reporting a concern you checked and cleared.

## Code comments

The existence bar and the published-surface exception to it are [code-style.md](code-style.md) → Comments. What that bar admits:

- Delete a comment whole when it narrates what the code says (`@param`/`@returns` restating the signature included), when a rename would carry it (flag the rename for `kk-refactor`), when it justifies a decision no reader would question, or when a sibling covers it. True, unique content goes too — anecdotes, provenance, alternatives considered. Once a comment stays, shortening it never drops its constraint, invariant, or warning.
- A published surface's contract prose carries what the types can't — call order, lifecycle, error modes, units, ranges, caller invariants.
