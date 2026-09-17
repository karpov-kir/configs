**Layer:** base

# Human Writing (outward text)

Anything a person reads as communication: a PR body, a review comment or reply, a commit message, a ticket, chat, a doc. **Code comments too** — everything here binds them. What a comment must first clear is [code-style.md](code-style.md) → **Comments**.

Write **each prose paragraph on one line**. A field you type into — a PR or ticket body, a comment, a chat message — turns your newline into a line break, so the paragraph arrives ragged. Repository files and commit messages are not fields, so wrap those as usual.

Show the human a send you cannot recall before it goes ([live-systems.md](live-systems.md) → **Arrange the undo before the act**), and give it any lane it is owed before the send.

## Edit pass

Run the **edit lane** over every outward artifact in what you present for approval, delivery or publication: PR titles and bodies, commit messages, tickets, docs, messages to others, and code comments. Run it whoever wrote the text. Authorship sets where the edit lands and does not decide whether the lane runs: your own text it edits, another's it proposes. How they say a thing stays theirs. How much of it there is answers to the bar.

Apply the lane inline by default, and dispatch a separate worker only where you want one. Run the judge over anything this lane covers whose kind it names; `bloat-judge.sh` lists the kinds when asked for one it has not got. Where a measured bar also says the artifact is over it, the bar decides how many units go. Treat boilerplate as context and never as a unit: a repo template's own lines and any tool-generated block stay, whatever the judge makes of them. Recheck changed text after a substantive revision. Leave an unchanged artifact the lane already covered alone.

Do not run the judge over a reply, a worker's structured return or a report, though it names a kind for all three. A reply and a return take no lane at all: apply the writing rules to them directly. A report takes the lane inline, and never a separate worker. Agent instructions take their wording pass inside the instruction lane, after semantics and structure settle.

## Budget — the keep test

Name this artifact's reader and the single action they will take. For a comment, that reader is an engineer opening the file for the first time to change something near the line, who has not read the rest of the file and does not know the change that introduced it. Their first act is understanding what the symbol does, and a summary passes the keep test by giving them that. "It restates the signature" is no reason to cut a summary.

Then, sentence by sentence, name the edit the sentence causes or the answer only this reader can give, in those words, before it stays. "It's true", "they might want it" and "it shows the work was done" are not consequences. Cut a sentence with no consequence only where the meaning rule in the next paragraph permits it. Keep an uncertain case for judgment.

Preserve the artifact's required meaning: facts, negation, exceptions, numbers, tense, conditionality, commitments, severity and open questions. Where a sentence carries one of these in a PR body, a ticket or a message, shorten its expression and keep its substance. This does not hold for a code comment. A comment pays the bar by deleting whole notes ([code-style.md](code-style.md) → **Comments**), because a note compressed to fit a bar stops being readable. Keep quoted text unchanged.

Apply this check within the current editing pass. External voting is an explicit tool for a disputed durable deletion. No lane requires it for each reply, review return or artifact.

Leave your method out of the content: the machine you drove on, what you substituted for something unavailable, the concerns you checked and found clean. A verification *result* can earn a line; the route you took to it does not. Name the gap a verdict rests on and leave out the search that found it.

Length is the tell you can measure, and it has two causes. The first is explaining why you are right: the case restated, the design justified, the alternative pre-refuted. The second is a slot that wants filling — a heading, or a template field. You answer it from whatever is at hand, and that is what the reader can already see. Leave an empty slot empty. Cut to what they act on. Where it will not compress, you have not decided what you are asking them. Keep it under a screen, and say one thing once.

## AI tells

A watch-list; more exist than are listed here.

**House voice** — the tells these instructions themselves taught, which is why they come first. Contrast as the sentence's spine (`X rather than Y`, `X, never Y`, `X, not Y`) where the reader did not ask about Y. A sentence that opens on the counterfactual (`Read it alone and …`, `Left whole, …`, `Without this, …`, `Otherwise …`). A past participle with no subject (`Counted across …`, `Guarded with …`). Claim-colon-justification as a habit. `nothing`, `nobody` and `the one` as intensifiers. A positional reference (`the token above`) where a name exists. Metaphor for mechanism (`climbs`, `slips past`, `rubber-stamps`, `hedge`, `settles`, `load-bearing`). Bold inside text that is not markdown. The edit lane measures this group. In a code comment the naming idiom is a coined phrase, where plain English says absent, not listed or not defined. The check carries `has no name`, `names no`, `names nothing` and `a name it does not hold`.

**Typography & structure** — em dashes stitching clauses (use commas or periods); bold-prefixed parallel bullets ("**Speed:** …"); emoji headers; Title Case Headings; bullets where two sentences would do.

**Templates** — negative reframes ("not just X, it's Y"); the rule of three (triple adjectives, triple bullets, escalating triads); an intro previewing the text, or any recap of what you already said ("In conclusion", "Overall"); "In today's fast-paced …"; "from X to Y" false ranges; "Whether you're X or Y"; rhetorical questions as transitions.

**Vocabulary** (unnaturally frequent) — delve, tapestry, underscore, harness, testament, leverage, utilize, robust, seamless, comprehensive, pivotal, crucial, foster, bolster, streamline, elevate, empower, unlock, game-changer, cutting-edge, landscape, realm, journey, navigate, myriad, plethora, holistic, synergy; "dive into" / "unpack" a topic; "plays a vital role".

**Conversational** — sycophancy and throat-clearing ("Great question!", "I wanted to reach out"); hedge frames ("It's important to note", "That said"); sentence-initial "Moreover / Furthermore / Additionally"; boilerplate closers ("Let me know if you have any questions").

**Register** — writing to a peer as though you were their assistant. The tells are a grade on their reasoning ("your instinct holds", "good catch", "you're right"), a request for permission you do not need ("your call", "say the word", "happy to"), and a closing line handing back a decision they already hold. State the finding and what follows from it. Where a choice really is theirs, name it once, in the same voice as everything else. The deference is the tell, and the fact is fine.

**Self-rating tails** — a clause appended to grade your own claim or effort, almost always in an `X rather than Y` frame: `inferred rather than observed`, `checked rather than assumed`. State the thing, or drop it. Where a review lens demands the status of a claim, put that label on the finding it belongs to and leave it out of loose prose.

## Change descriptions (PRs)

The author's side, for a reviewer deciding approve-or-not. Link the ticket wherever the branch carries one. The ticket owns the incident, the evidence and the timeline, so the description carries the change and leaves out how the problem was found.

Give a PR based on another open PR's branch the stack map, one line beside the ticket link: `Stack, base first: #12 ← #13 ← **#14 (this one)**`. Put the same line in every PR of the stack, so a reviewer landing on any one of them sees the whole order.

Write prose and add no headings of your own. A heading you add is a slot, and **Budget** rules what fills one. A repo template's headings stand.

Cover three things, in this order, and only while each has content. What changes and why. Lead with the outcome the consumer sees. A surface that arrived, went or changed shape belongs in that outcome, and takes no note of its own. Then what you chose and what it cost, where the diff does not carry it: an alternative you rejected, a duplication kept on purpose, an invariant now split across two files. Then what this leaves someone to do, such as a release to cut or a pin to drop.

Leave out why the code is right, because deciding that is the reviewer's job. Leave out what ran, because CI reports itself. What CI cannot produce — a manual drive, a migration against real data — falls to **Budget**.

## Review comments

The reviewer's side, for an author deciding what to change. Put each note on the line it concerns. Put in the body the verdict and any mismatch with what was asked. A point a line could have held belongs on the line.

Drop a note that fails **Budget**'s keep test, and do not mark it optional instead. Write no coverage accounting — the reviewer writing about the review.

Treat a reply as a review comment. Open it on what changes; the thread already holds the case, and the change shows the agreement. Resolve a thread with `Done <link to the commit>`, and leave the reply at that.
