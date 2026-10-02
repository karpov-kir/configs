**Layer:** craft

# Comments

A comment is one of two kinds, and each kind has one shape.

**The tooling decides every comment itself, without asking the human.** A reviewer's suggestion is one more fact.

A **summary** sits on a declaration and says what it does in one sentence. It starts with a verb, as this repository writes them: "Lists …", "Returns …", "Checks whether …", "Throws when …". A symbol has one where its name and signature leave something to say. A type's fields and members are part of its signature. What is left to say is a return case, a unit, an ordering, a side effect or a precondition. A summary that only restates the identifier in words is deleted, exported or not. Where the body is five lines or fewer the reader reads the body. A summary there says a fact from outside the function: what a caller expects, what a format or a platform does, why a bound was chosen. A field whose meaning is not in its name and type gets a sentence on that field. It says what the symbol decides or returns in its caller's words, and may put the signature's types in words. It does not define the symbol against another symbol, or say why, and holds one relative clause at most.

A **note** says something the code cannot say: a fact about the outside world the code relies on, or an edit that looks right and breaks something. It states the fact, and then what one identifier at the site does that the fact explains. On a declaration with a body the act follows from the fact, once, and says what the declaration establishes for its caller. It names the thing in the fact it satisfies, and a connector by itself is no link. Where the fact rules out an alternative, the note names what the alternative breaks. A copy says what it keeps that the original loses, unless its name does. The means follows with `by` wherever the outcome needs it to be understood, whatever the body shows. The connectors are `so`, `therefore`, `which is why`, `that is why`, `for that reason`, and `because` after the act. One stands only where the act follows. A warning about an edit gives the fact, then what the edit breaks, with no act or connector. The fact leads, on a thing a reader can picture, and a platform fact is the event in plain steps. Where it is about a state the act prevents or a thing the act produces, the act leads, verb-first.

The note never names the declaration it sits on. A declaration holding values states the fact alone or opens on its verb, as "Names …" or "Marks …". Siblings take one form: the members of an enum, the fields of a type, the rows of a table. On a declaration with a body the act's subject is the domain thing, a role noun such as this check, or this function. A value never keeps, drops or rejects. It is plain words. An identifier appears only where a reader would look it up anyway: a platform's or a library's interface, a constant, or another declaration the reader must edit. A local, a parameter or a private helper is said in words, and an interface's name stands beside the words and never in their place. That identifier is declared at the site or inside its body, and never somewhere else. The value beneath a constant, a field or an enum member is what its fact bears on.

A fact explaining an act goes to the declaration performing the act. Where the act is a declaration consulting something at all, that declaration performs it. A caller a grep finds is a named element of the system. One no grep finds is hypothetical: the fact stays and the doubt goes to review. Where a caller's behaviour is why this code does what it does, the behaviour is part of the fact and the note stays here. A note giving its reason as a pointer at another note leaves the reason at neither.

A claim that this code and something outside it agree is never stale: where they have parted, that claim is what makes the parting findable. Such a note keeps the obligation in its wording, as must match or stays in step with. A claim the change cannot check is never stale either. It stays, and the doubt goes to review.

A note is at most 40 words, in the words the domain uses. Each sentence states one fact or one act, in at most 25 words, and one connector may join a fact to the act that relies on it. Its sentences follow the descriptive rules of ASD-STE100, without its word list, its ban on -ing forms or its rules for procedures and paragraphs. A coined phrase is a rename finding in prose. A term outside the code and ordinary English is written as what it is, in the words of the code's own condition. A pronoun stands only for the subject of the sentence before it, and any other back-reference repeats the noun. A note that needs more is a lint rule's message or a shape the refactor lane changes.

A fact about code with a declaration never goes to the PR body. A fact about a row of data belongs to the row, as a note on it. An ordering, a pairing or an invariant earns a sentence where a caller depends on it, whatever test pins it. A carrier is what the reader sees at the site: a name, a type, a compiler or lint message. A test is never one. An invariant more than one file states is enforced by a lint rule, whose message carries it, and the notes go.

A summary fills one pattern: `<Verb> <what>` and, where there is a case, `, or <value> when <case>`. A note fills no pattern. The edit lane reads a note for what it carries, and one carrying a fact this code relies on stands however its sentence is built.

A comment sits on the declaration it is about. A claim about a function goes on that function, wherever the claim was found. A file header keeps what spans the file, and a header saying what the file is for is deleted where the file's name says it.

Inside a block the summary comes first, then the note. A block is at most four prose lines. A file header is at most eight. A line carrying only a doc tag — `@param`, `@returns`, `@throws`, `@example` — is the signature written out, and counts as neither.

Use the identifier's name or the domain's own word. Where a specification names a thing and the code names it something else, a comment takes one of those two names and coins no third. A word the reader would need to have been in the room for does not go in a comment.

No markdown in a comment: no bold, no bullets, no headings. A comment cites a file and not a section within it.

Before a note stays, ask whether a rename, a moved line or a line of code would carry it. Where the symbol is in the change set, that rename or move is the edit and the note goes; where it is not, flag the rename. A note saying which of two branches handles which case is carried by a named function per branch. A note is understood from itself and its line alone. Every term and every step from fact to act is in it, and the body and the tests stay unread. A numeric constant's name says what the number bounds, for whom, and in what unit. Write the arithmetic between constants as code and leave it out of prose. Delete a note that justifies a decision no reader would question. Delete one that another note in the change set already says. Delete one that is history a reader can get from `git log`.

A published surface states in its summary block what the types do not carry: call order, lifecycle, error modes, units, ranges, caller invariants.

One pair, so the shape is not in doubt:

```ts
// Before
/**
 * Books exported by older versions of the ledger have no `currency` field at all — it was only added in v3 —
 * so a book read out of one of those files carries nothing on the book and nothing on its entries either,
 * which means a call like this one answers false for every single one of them.
 */
// After
/** Exports written before v3 set no currency on the book or its entries, so those books read as unpriced. */
```

A usage line, a test declaration and a shared-region marker live in a comment, where their reader
looks, and a tool reads them. An edit made for the prose leaves them alone.

Keep the block that opens a file whole: a header scan ends at the first line of code, and the lines
under a blank inside it lose their reader silently.

Comment form is also [human-writing.md](human-writing.md), which binds every outward text.
