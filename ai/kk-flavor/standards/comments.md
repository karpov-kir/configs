**Layer:** craft

# Comments

Keep or add a comment only where its absence leaves the reader wrong or stuck. The reader has only this file open. The reader knows the domain this code works in: the standards, protocols, formats and platform APIs it builds on. A fact any such reader knows is no reason for a comment. A reader who gets it from the next few lines of code needs no comment either. When in doubt, write none.

A comment is one of two kinds.
- A summary: one sentence on a declaration whose name and signature leave a case, a unit, an ordering, a side effect or a precondition unsaid. It opens on a verb: "Returns the posting, or undefined when the book is closed."
- A note: what the declaration does and what that gives the reader, then the fact from outside the code behind it. The fact is one the reader would not know, such as how a device, a vendor or a server actually behaves, or what a past failure showed. A caller's reason for calling is part of the fact, and a callers section, where one is given, shows it. "This check moves a posting made after 17:00 to the next business day. The bank refuses a posting after 17:00."

Inside a body, a comment earns its place only where an outside fact decides that line. Remove a comment that restates the name, the signature or the body. Remove one that points to a test as its evidence, walks through the code line by line, or tells the story of the change. Keep a comment only when it already follows every rule on this page, and rewrite every other one.

Write each comment so it is understood from itself and its line alone:
- Write in the style of ASD-STE100, about 80% of the way: short sentences, one idea each, active voice, plain words. Write each sentence to be read once.
- Use the words of the domain and the code. Coin no term.
- Name who acts in each sentence, as a subject a reader can picture: the bank, the caller, this check. Never write "whatever" or "anything" as the subject. A value never acts. Where "this code", "the code" or "this logic" would be the subject, name the actor by its role: the test, the check, the caller or the function. Say "this function" only where it is the clearest subject.
- State the act first and the fact behind it after. An act alone leaves "why?". A fact alone leaves "and what?".
- Say what the act prevents or gives, not how it is computed. Follow a chain of reasons past each mechanism to what the reader gains or avoids: a test that runs to its end, a stale file left unread.
- Name an identifier only where the reader would look it up anyway. Where the comment leans on a name another file defines, such as a test, a helper or a mode, say in the same sentence what it is for. A reference to another file names its path.
- Write plain text: drop markdown, hedges and pointers to other comments. Put identifiers and paths in backticks, the mark plain text keeps.
- Keep a comment to at most four lines, and a file header to at most eight.

A summary of a declaration, what it returns, takes or means, goes in a doc block (`/** */`), where editors and generated docs show it. An enum member's meaning is such a summary. A note on the context around the code, such as why one value is paired with another, is a line comment (`//`, `#`), even on an exported declaration.

A comment sits on the declaration it is about. Siblings, such as enum members or table rows, take one form.
