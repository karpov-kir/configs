# Code Style

## Naming

- Full, descriptive names; abbreviate only where the abbreviation is well established (`i` in a tight numeric loop is fine).
- Strict camelCase / TitleCase for multi-word names; acronyms case like normal words (`remoteUrl`, not `remoteURL`).
- Booleans take a predicate prefix: `is`/`has`/`can`/`should`/`was`/`will`, …
- A function returning a new instance takes a `new` prefix — `newApiClient`, not `create…`/`make…`.
- One term per thing, identifiers included ([writing.md](writing.md)).

## Parameters

- Named parameters (parameter object, keyword args, or equivalent) for 3+ params; positional style for single-param functions. Exception: a signature you don't own — an external interface you implement or fake, or a published package's public surface.
- A parameter the body branches on to pick between behaviours is a flag — split it into two named functions.

## Comments

Follow [writing.md](writing.md) and [human-writing.md](human-writing.md)'s comment form. **The default is no comment**: one earns existence only where the code would be misread or wrongly edited without it.

**A published surface is the exception, and it runs the other way** — state the contract the types don't carry. That text is interface, not commentary.

## Type Safety

- Never bypass the type system with escape hatches — `any`/`@ts-ignore` (TS), `unsafe` (Rust), unchecked or non-null assertions anywhere. Narrow the type first, or pass a guaranteed value explicitly.
- Prefer precise types over wide ones.
- In TS/JS, absence is `undefined`, never `null` — one absence value per codebase, so nothing has to test for both.
- Prefer enums (or named-enumeration constructs) that expose symbolic members at call sites — renames ripple through the type checker. A literal type alias doesn't satisfy this.
- No unused imports or variables — remove rather than keep dead code.
- Inline single-use object/interface shapes; extract a named type only when referenced from 2+ places. The exception is a type crossing a module boundary, which earns its name at one call site ([architecture/core.md](architecture/core.md) → Module depth).

## Control Flow

- Prefer early returns to reduce nesting.
- Limit nesting depth to ~3 levels; where early returns can't flatten deeper nesting, extract the inner logic into a named function.
- Prefer multi-line `if` statements with braces over single-line guard clauses like `if (!value) return`.
- No special case bolted onto an unrelated flow — move it behind its own abstraction, or into the slice that owns it.

## Logging

Call-site rules — where log lines belong and what they say; the logger itself belongs to [architecture/core.md](architecture/core.md) → Logging & events (construction) and [project.md](project.md) → Logging (format and level). Review for *absence*: a boundary or handled failure with no log line violates these rules the same way a bad message does.

- Log at boundaries: inbound work accepted (request, job, message), outbound calls to other systems, process lifecycle (startup with resolved config, shutdown).
- Every failure path that doesn't propagate must log: a caught-and-handled error, a retry, a fallback, a degraded mode.
- Log an error where it's *handled*, once — never at every layer it passes through, and never log-and-rethrow at the same layer.
- A message names the operation, its key identifiers, and the outcome with its cause — "failed to \<operation\> for \<entity\>: \<error\>", never a bare "error occurred". Prefer static text plus structured fields (ids, counts, durations) over interpolated prose.
- Carry enough correlating fields (request id, entity id, attempt) to follow one flow across lines.
- Levels by the action needed — `error`, `warn`, `info`, `debug`. If nobody would act on it, it isn't `error`.
- No per-item logging at `info`+ inside loops — one aggregate line with counts, or drop to `debug`.
- Keep secrets and PII out at the call site rather than redacting downstream — pass only what's safe to print.

## Abstraction

- **One level — don't mix.** Keep a unit at one abstraction level; push down the low-level mechanics an intent-revealing operation is built from.
- **Name the operations.** Wrap raw mechanics in an intent-named operation so the unit reads top-down as prose. A comment narrating *what* a block does is a missing name.

## Classes vs functions

Reach for a class (or a `newX` factory over private state) when operations share state, configuration, or an injection boundary — free functions threading the same implicit dependencies are that class turned inside-out. Plain functions for genuinely standalone logic; a class of only static methods is a module with extra syntax.

## Extraction & Size

- Functions do one thing. Extract when concerns split, abstraction blurs (above), or length exceeds ~100 lines.
- Tolerate duplication at 1–2 sites; extract a shared helper on the 3rd. Earlier abstraction risks the wrong shape.
- Don't extract tiny wrappers around self-evident code — indirection without payoff.
- Keep files focused on a single responsibility — split when a file grows beyond ~450 lines or contains unrelated concepts. A flat command dispatch over one responsibility counts by its longest arm, not its total: splitting it puts one concern in two files, which is what the rule exists to prevent.
- Avoid barrel/index files; import from the source module directly. A module's published surface file ([architecture/core.md](architecture/core.md) → Module depth) is not covered by this ban: it *declares* what it hands out rather than re-exporting symbols that live elsewhere. Never re-export a symbol through a module that isn't its home just to keep an import path stable — a symbol has one home; update importers if that home moves.
