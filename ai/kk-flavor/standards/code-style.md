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

A **summary** sits on a declaration and says what it does in one sentence, starting with a verb, the way this repository already writes them: "Lists …", "Returns …", "Checks whether …", "Throws when …". Every exported symbol has one. A private symbol has one when its name does not say what it does. A summary may say in words what the signature says in types. It does not say why.

A **note** says something the code cannot say: a fact about the outside world the code relies on, or an edit that looks right and breaks something. It states the fact first, in a sentence with a subject, and the consequence second. It is at most two sentences. A note that needs more is one of three other things: a test whose name states it, a line in the PR body, or a shape the refactor lane changes.

Inside a block the summary comes first, then the note. A block is at most four prose lines. A file header is at most eight. A line carrying only a doc tag — `@param`, `@returns`, `@throws`, `@example` — is the signature written out, and counts as neither.

Use the identifier's name or the domain's own word. Where a specification names a thing and the code names it something else, a comment takes one of those two names and coins no third. A word the reader would need to have been in the room for does not go in a comment.

No markdown in a comment: no bold, no bullets, no headings.

The edit lane's bar measures a change set's comment share and its share of long blocks, and both stay at or under the host repository's. Compare against the host repository and against no other set. Over the bar, delete whole notes, weakest first. A summary does not pay the bar. Shorten no comment to pay the bar. A note that would have to be compressed to fit is a note that goes.

Delete a note when a rename would carry it (flag the rename), when it justifies a decision no reader would question, when another note in the change set already says it, or when it is history a reader can get from `git log`.

A published surface states in its summary block what the types do not carry: call order, lifecycle, error modes, units, ranges, caller invariants.

One pair, so the shape is not in doubt:

```ts
// Before
/**
 * The ledger allows `currency` on the book or on its entries, so both are consulted. Read the book's own attribute
 * alone and a book shaped the other way slips past totalling whole, rounding included.
 */
// After
/** Checks whether a book is priced. The ledger allows `currency` on the book or on each entry, so both are read. */
```

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
