# Human Writing (outward text)

Rules for text a person reads as communication: PR and ticket descriptions and comments, review replies, commit messages, chat/Slack, email, announcements, README-grade docs. Internal context-window artifacts (skills, standards, prompts, CLAUDE.md) stay on [writing.md](writing.md) house style alone. **Code comments** get their own form (**Code comments**, below). For outward text — and, per its comment form, for code comments — this file wins on conflict.

Two failure modes to defend against: text that reads machine-written, and text that says everything it knows. Apply these rules **while writing**; the `humanize` role (config.yaml) retrofits text that already exists.

## Budget — the reader-action test

The one rule every form in this file instantiates: **name this artifact's reader and the action they'll take; a sentence earns its place only by changing that action.** The sections below write it out for the classes that recur (change descriptions, code comments); any other class — a commit message, a ticket, an announcement — gets its bar from those two questions directly. Write a new form only when a class recurs with a repeated failure mode.

- True-but-inert detail gets cut — deliberately **lossy**, unlike `/tighten`. Substance that matters but not to this reader goes to a durable home (ticket, commit body, doc) with a link, or gets dropped.
- Lead with the answer or outcome; add evidence only where the reader will verify or push back.
- One concern per message. A reply addresses what was raised, not everything adjacent you know.
- Match the asker's altitude: a one-line question earns a short answer, not a briefing.

## Voice

- Write like you'd say it to a colleague. Contractions are fine; informal is fine.
- Vary sentence length and paragraph size — monotone rhythm reads as generated.
- Plain verbs and words: use, read, fix, check — not leverage, utilize, harness, streamline.
- Specific over abstract: name the file, the number, the person.

## AI tells

A watch-list, not a complete set — the patterns drift as models change. Their common thread: manufactured polish and symmetry a person writing quickly wouldn't produce.

**Typography & structure**
- Em dashes stitching clauses — in outward text use commas, periods, or parentheses instead.
- Bold-prefixed parallel bullets ("**Speed:** …"), emoji section headers, Title Case Headings.
- A bullet list where two sentences would do; headings over a handful of short paragraphs.

**Templates**
- The negative reframe: "not just X, it's Y", "It's not about X — it's about Y".
- The rule of three: triple adjectives, triple bullets, escalating triads ("faster, smarter, more resilient").
- Formulaic frames: an intro that previews the text, a conclusion that recaps it ("In conclusion", "Overall", "In summary").
- "In today's fast-paced / ever-evolving …" openers; "from X to Y" false ranges; "Whether you're X or Y"; rhetorical questions as transitions ("So what does this mean?").

**Vocabulary** (unnaturally frequent): delve, tapestry, underscore, harness, testament, leverage, utilize, robust, seamless, comprehensive, pivotal, crucial, foster, bolster, streamline, elevate, empower, unlock, game-changer, cutting-edge, landscape, realm, journey, navigate, myriad, plethora, holistic, synergy; "dive into" / "unpack" a topic; "stands as a testament", "plays a vital role", "rich tapestry".

**Conversational**
- Sycophancy and throat-clearing: "Great question!", "I hope this email finds you well", "I wanted to reach out", "I just wanted to follow up".
- Hedge frames: "It's important to note", "It's worth noting", "That said"; sentence-initial "Moreover / Furthermore / Additionally".
- Boilerplate closers: "Let me know if you have any questions", "Please don't hesitate", or a recap of what you just said.

## Change descriptions (PRs)

The reader is a reviewer deciding approve-or-not: a paragraph earns its place only if it could change the verdict or where they look. The shape, in order:

- **What changes and why** — a few sentences, leading with the outcome.
- **Review focus** — the risky or surprising parts, and each deliberate trade-off a reviewer might push back on.
- **Verification** — what ran and what it showed: links and counts, not narration.

Findings, measurements, and field observations are run *results*, not review input — their home is the ticket, a doc, or a PR comment, with one link line in the description. A repo's own PR template sections stay, filled per these rules.

## Code comments

Every comment taxes every future reader — it interrupts the skim whether or not it deserved to — so only essential ones stay. The whole-file test: someone reading just the code gets interrupted exactly where the code alone would mislead them, and nowhere else. All three groups above apply, in comment form:

- **First ask whether the comment should exist.** Delete it whole when it narrates what the code says — a doc block's `@param`/`@returns` restating the signature included — when a rename or extraction would carry it (note the rename for the `refactor` role), when it justifies a decision no reader would question, or when a sibling comment or doc block already covers it. `/** Key system families, valued as a player DRM configuration names them */` on `enum DrmSystem` fails twice — the first half narrates the declaration, the second doesn't clear the bar below: delete.
- **The bar is misread-or-misedit.** A comment earns its lines only where the code would be misread or wrongly edited without it — it looks wrong, arbitrary, or tempting to "fix". A why nobody would question is noise; a warning someone would trip over is the point. This license is lossy where the `tighten` role's is not: true, unique content goes too — anecdotes, provenance, secondary rationale, alternatives considered (a commit body or doc is their home when they matter).
- **Judge the file, not each comment alone.** In a subtle domain every decision has a defensible why; admit each and the code drowns even though every block passes individually. When comment load rivals the code it annotates, go back through with the deletion question asked harder.
- Default to the shortest comment that still carries the constraint — a couple of plain lines; needing a paragraph means the home is elsewhere.
- **Phrasing text as a constraint exempts nothing from the existence question.** But once a comment stays, shortening never drops its stated constraint, invariant, or warning — those are what comments exist for.
