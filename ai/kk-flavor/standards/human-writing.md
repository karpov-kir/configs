# Human Writing (outward text)

Rules for text a person reads as communication: PR and ticket descriptions and comments, review replies, commit messages, chat/Slack, email, announcements, README-grade docs. Internal context-window artifacts (skills, standards, prompts, CLAUDE.md) stay on [writing.md](writing.md) house style alone. **Code comments are for humans** — a colleague skimming the code — so all three lens groups apply, with the Budget in **comment form**: a sentence earns its place by changing how the next person reads or edits this code — a constraint, a why, a warning, a non-obvious consequence. **This license is lossy where the `tighten` role's is not**: true, unique content also goes when it fails that bar — incident anecdotes, provenance, secondary rationale, alternatives considered (a commit body or doc is their home when they matter). Default to the shortest comment that still carries the constraint. **Never drop a stated constraint, invariant, or warning** — those are what comments exist for. For outward text — and, in its comment form, for code comments — this file wins on conflict.

Two failure modes to defend against: text that reads machine-written, and text that says everything it knows. Apply these rules **while writing**; the `humanize` role (config.yaml) retrofits text that already exists.

## Budget — the reader-action test

- A sentence earns its place only if it changes what the reader knows *and* will do or decide. True-but-inert detail gets cut — deliberately **lossy**, unlike `/tighten`. Substance that matters but not to this reader goes to a durable home (ticket, commit body, doc) with a link, or gets dropped.
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
