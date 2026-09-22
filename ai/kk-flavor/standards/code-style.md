**Layer:** craft

# Code Style

## Naming

- Full, descriptive names; abbreviate only where the abbreviation is well established (`i` in a tight numeric loop is fine).
- Strict camelCase / TitleCase for multi-word names; acronyms case like normal words (`remoteUrl`, not `remoteURL`).
- Booleans take a predicate prefix: `is`/`has`/`can`/`should`/`was`/`will`, …
- A function returning a new instance takes a `new` prefix — `newApiClient`, not `create…`/`make…`.

## Parameters

- Named parameters (parameter object, keyword args, or equivalent) for 3+ params; positional style for single-param functions. Exception: a signature you don't own — an external interface you implement or fake, or a published package's public surface.
- A parameter the body branches on to pick between behaviours is a flag — split it into two named functions.

## Comments

A comment is one of two kinds, and each kind has one shape.

A **summary** sits on a declaration and says what it does in one sentence. It starts with a verb, the way this repository already writes them: "Lists …", "Returns …", "Checks whether …", "Throws when …". A symbol has one where its name and signature leave something to say. A type's fields and members are part of its signature. What is left to say is a return case, a unit, an ordering, a side effect or a precondition. A summary that only restates the identifier in words is deleted, exported or not. Where the body is five lines or fewer the reader reads the body. A summary there says a fact from outside the function: what a caller expects, what a format or a platform does, why a bound was chosen. The parameter names and the return type are part of what the identifier says. A field whose meaning is not in its name and type gets a sentence on that field. A summary may say in words what the signature says in types when that is the thing left to say. It says what the symbol decides or returns in the words its caller uses. It does not define the symbol against another symbol. It does not say why.

A **note** says something the code cannot say: a fact about the outside world the code relies on, or an edit that looks right and breaks something. It states the fact first, in a sentence with a subject, and the consequence second. The fact sentence keeps the fact's own subject, the outside thing it is about: a type, a format, a specification, a library, or a party the domain names. A claim opening on something the reader cannot name reads out of the blue, and it is written again from the named thing it turns on. A fact that holds sometimes is written as the case it holds in, in the `when <case>` shape the summary pattern uses. The site enters in the consequence, which names the site's declaration or a caller's act on the result. A caller a grep finds is a named element of the system, and a caller no grep finds is the hypothetical actor. Where the consequence would name a caller no grep reaches, the fact stays and the doubt goes to review. A note earns its place by the consequence. A fact whose consequence is missing belongs to the summary or to the PR body. A claim that this code and something outside it agree is never stale: where they have parted, that claim is what makes the parting findable. Such a note keeps the obligation in its wording, as must match or stays in step with. A note saying one copies the other states what is, where the claim states what is owed, and a reader changing one side is owed the second. A claim the change cannot check is never stale either. It stays, and the doubt goes to review. It is at most two sentences, in the words the domain uses. A coined phrase is a rename finding in prose. A note that needs more is one of five other things. It is a lint rule whose message states it, a test whose name states it, a line in the PR body, or a shape the refactor lane changes. The fifth is a named field on the data entry it describes. A fact about a row of data belongs to the row. It goes in a field on the entry, where the next reader of that row finds it. An ordering, a pairing or an invariant earns a sentence only where a caller depends on it. Where a test holds it, the test is its home. An invariant that more than one file states is enforced by a lint rule or a test, which then carries the message, and the notes go.

A summary fills one pattern: `<Verb> <what>` and, where there is a case, `, or <value> when <case>`. A note fills one pattern: `<One fact>, so <consequence>.` It is one sentence, and a second fact is a second sentence in the same pattern. The edit lane writes toward these patterns. A sentence that resists one is a finding, and never an automatic deletion.

A comment sits on the declaration it is about. A claim about a function goes on that function and names it, wherever the claim was found. A file header keeps what spans the file, and a header saying what the file is for is deleted where the file's name says it.

Inside a block the summary comes first, then the note. A block is at most four prose lines. A file header is at most eight. A line carrying only a doc tag — `@param`, `@returns`, `@throws`, `@example` — is the signature written out, and counts as neither.

Use the identifier's name or the domain's own word. Where a specification names a thing and the code names it something else, a comment takes one of those two names and coins no third. A word the reader would need to have been in the room for does not go in a comment.

No markdown in a comment: no bold, no bullets, no headings. A comment cites a file and not a section within it, because a section reference needs the delimiters this line forbids.

The edit lane reports a change set's comment share beside the host repository's. The figure gates no edit.

Before a note stays, ask whether a rename, a moved line or a line of code would carry it. Where the symbol is in the change set, that rename or move is the edit and the note goes; where it is not, flag the rename. A note saying which of two branches handles which case is carried by a named function per branch. A fact about the world this code relies on stays at the site even where a test pins the behaviour it explains. A test is enforcement, and a reader understanding the code should not have to go and read one. A numeric constant's name says what the number bounds, for whom, and in what unit. Write the arithmetic between constants as code and leave it out of prose. Delete a note that justifies a decision no reader would question. Delete one that another note in the change set already says. Delete one that is history a reader can get from `git log`.

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

A comment is prose to a reader and input to a check at the same time. A usage line, a test
declaration and a shared-region marker live in a comment. That is where their reader looks. An edit
made for the prose changes what a tool reads, so leave those lines alone.

Keep the block that opens a file whole. A header scan ends at the first line of code. A blank line
put inside that block takes the lines under it away from their reader, and the loss is silent.

Comment form is also [human-writing.md](human-writing.md), which binds every outward text.

## Type Safety

- Never bypass the type system with escape hatches — `any`/`@ts-ignore` (TS), `unsafe` (Rust), unchecked or non-null assertions anywhere. Narrow the type first, or pass a guaranteed value explicitly.
- In TS/JS, absence is `undefined`, never `null` — one absence value per codebase.
- Prefer enums (or named-enumeration constructs) that expose symbolic members at call sites — renames ripple through the type checker. A literal type alias doesn't satisfy this.
- Inline single-use object/interface shapes; extract a named type only when referenced from 2+ places. The exception is a type crossing a module boundary, which earns its name at one call site.

## Control Flow

- Limit nesting to ~3 levels: prefer early returns, and where they can't flatten it, extract the inner logic into a named function.
- Prefer multi-line `if` statements with braces over single-line guard clauses like `if (!value) return`.
- No special case bolted onto an unrelated flow — move it behind its own abstraction, or into the slice that owns it.

## Logging

Where log lines belong and what they say; how you obtain a logger is [architecture/core.md](architecture/core.md) → **Logging & events**. Review for *absence*: a boundary or handled failure with no log line breaks these rules.

- Log at boundaries: inbound work accepted (request, job, message), outbound calls to other systems, process lifecycle (startup with resolved config, shutdown).
- Every failure path that doesn't propagate must log: a caught-and-handled error, a retry, a fallback, a degraded mode.
- Log an error where it's *handled*, once — never at every layer it passes through, and never log-and-rethrow at the same layer.
- A message names the operation, its key identifiers, and the outcome with its cause — "failed to \<operation\> for \<entity\>: \<error\>". Prefer static text plus structured fields (ids, counts, durations) over interpolated prose. Carry enough correlating fields (request id, entity id, attempt) to follow one flow across lines.
- Choose the level by the action needed: `error`, `warn`, `info`, `debug`. A line the reader would not act on is not `error`.
- No per-item logging at `info`+ inside loops — one aggregate line with counts, or drop to `debug`.
- Keep secrets and PII out at the call site instead of redacting them downstream. Pass only what is safe to print.

## Abstraction

- **One abstraction level per unit.** Push down the low-level mechanics an intent-revealing operation is built from.
- **Name the operations.** Wrap raw mechanics in an intent-named operation so the unit reads top-down as prose. A comment narrating *what* a block does is a missing name.

## Classes vs functions

Reach for a class (or a `newX` factory over private state) when operations share state, configuration, or an injection boundary; plain functions for genuinely standalone logic. A class of only static methods is a module with extra syntax.

Share behaviour by **composition**: a collaborator each type holds and delegates to. Do not share it by inheritance. Subclass only where a framework or an external interface demands it. An abstract base with a single subclass is that collaborator written longer.

## Extraction & Size

- Functions do one thing. Extract when concerns split, abstraction blurs (above), or length exceeds ~100 lines.
- Tolerate duplication at 1–2 sites; extract a shared helper on the 3rd. Earlier abstraction risks the wrong shape.
- Leave self-evident code inline, because a wrapper around it hides no detail.
- Keep files focused on a single responsibility, and split a file that grows beyond ~450 lines or carries unrelated concepts. Count a flat command dispatch over one responsibility by its longest arm and never by its total.
- Avoid barrel/index files; import from the source module directly. A module's published surface file ([architecture/core.md](architecture/core.md) → **Module depth**) is not covered by this ban. Never re-export a symbol through a module that isn't its home — a symbol has one home; update importers if that home moves.
