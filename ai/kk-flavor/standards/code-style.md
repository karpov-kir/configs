# Code Style

**A script under a skill or `kk-flavor/` is also something agents read**, so [ecosystem.md](ecosystem.md) → **Prefer the mechanism** binds it on top of the rules below.

## Naming

- Full, descriptive names; abbreviate only where the abbreviation is well established (`i` in a tight numeric loop is fine).
- Strict camelCase / TitleCase for multi-word names; acronyms case like normal words (`remoteUrl`, not `remoteURL`).
- Booleans take a predicate prefix: `is`/`has`/`can`/`should`/`was`/`will`, …
- A function returning a new instance takes a `new` prefix — `newApiClient`, not `create…`/`make…`.

## Parameters

- Named parameters (parameter object, keyword args, or equivalent) for 3+ params; positional style for single-param functions. Exception: a signature you don't own — an external interface you implement or fake, or a published package's public surface.
- A parameter the body branches on to pick between behaviours is a flag — split it into two named functions.

## Comments

Comment form is [human-writing.md](human-writing.md) → **Code comments**. **The default is no comment**: one earns existence only where the code would be misread or wrongly edited without it.

**A published surface is the exception, and it runs the other way** — state the contract the types don't carry. That text is interface, not commentary.

## Type Safety

- Never bypass the type system with escape hatches — `any`/`@ts-ignore` (TS), `unsafe` (Rust), unchecked or non-null assertions anywhere. Narrow the type first, or pass a guaranteed value explicitly.
- In TS/JS, absence is `undefined`, never `null` — one absence value per codebase.
- Prefer enums (or named-enumeration constructs) that expose symbolic members at call sites — renames ripple through the type checker. A literal type alias doesn't satisfy this.
- Inline single-use object/interface shapes; extract a named type only when referenced from 2+ places. The exception is a type crossing a module boundary, which earns its name at one call site ([architecture/core.md](architecture/core.md) → **Module depth**).

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
- Levels by the action needed — `error`, `warn`, `info`, `debug`. If nobody would act on it, it isn't `error`.
- No per-item logging at `info`+ inside loops — one aggregate line with counts, or drop to `debug`.
- Keep secrets and PII out at the call site rather than redacting downstream — pass only what's safe to print.

## Abstraction

- **One abstraction level per unit — don't mix.** Push down the low-level mechanics an intent-revealing operation is built from.
- **Name the operations.** Wrap raw mechanics in an intent-named operation so the unit reads top-down as prose. A comment narrating *what* a block does is a missing name.

## Classes vs functions

Reach for a class (or a `newX` factory over private state) when operations share state, configuration, or an injection boundary; plain functions for genuinely standalone logic. A class of only static methods is a module with extra syntax.

## Extraction & Size

- Functions do one thing. Extract when concerns split, abstraction blurs (above), or length exceeds ~100 lines.
- Tolerate duplication at 1–2 sites; extract a shared helper on the 3rd. Earlier abstraction risks the wrong shape.
- Don't extract tiny wrappers around self-evident code.
- Keep files focused on a single responsibility — split when a file grows beyond ~450 lines or contains unrelated concepts. A flat command dispatch over one responsibility counts by its longest arm, not its total.
- Avoid barrel/index files; import from the source module directly. A module's published surface file ([architecture/core.md](architecture/core.md) → **Module depth**) is not covered by this ban. Never re-export a symbol through a module that isn't its home — a symbol has one home; update importers if that home moves.
