**Layer:** craft

# Comments

Write a comment only where a reader of this line needs something the code cannot show.

Keep or add a comment only where its absence leaves the reader wrong or stuck: a fact from outside the code, or what the name and signature leave unsaid. When in doubt, write none: a reader who gets it from the next few lines of code needs no comment. Inside a body, a comment earns its place only where an outside fact decides that line.

Write in the style of ASD-STE100, about 80% of the way: short sentences, one idea each, active voice, plain words.

A comment is one of two kinds.
- A summary: one sentence on a declaration whose name and signature leave a case, a unit, an ordering, a side effect or a precondition unsaid. It opens on a verb: "Returns the posting, or undefined when the book is closed."
- A note: a fact from outside the code that the code relies on, then what this code does because of it and what that gives its caller. "The bank refuses a posting after 17:00, so this check moves it to the next business day."

Remove a comment that restates the name, the signature or the body. Remove one that names a test, walks through the code line by line, or tells the story of the change.

Write so the comment is understood from itself and its line alone:
- Use the words of the domain and the code. Coin no term.
- Name who acts in each sentence: the bank, the caller, this check. A value never acts.
- At most four lines.
- State the fact and the act. A fact alone leaves "and what?". An act alone leaves "why?".
- Say what the act prevents or gives, not how it is computed.
- Give each sentence a subject a reader can picture, never "whatever" or "anything". Write each sentence to be read once.
- Name an identifier only where the reader would look it up anyway.
- Where a comment leans on a mechanism elsewhere, such as a warm-up or a cache, say in a few words what it is for. The reader then follows the sentence without a second file.
- Write plain text: drop markdown, hedges and pointers to other comments. Say "this function" only where it is the clearest subject.

A comment sits on the declaration it is about. Siblings, such as enum members or table rows, take one form.
