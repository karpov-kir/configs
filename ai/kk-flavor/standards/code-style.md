# Code Style

Baseline rules below. If the project root has a `PROJECT_CODE_STYLE.md`, merge its rules on top (project overrides win).

## Naming

- Use full, descriptive variable names: `event` not `e`/`evt`, `error` not `err`/`e`, `element` not `el`, `index` not `i` (unless in a tight numeric loop), `accumulator` not `acc`, `value` not `val`, `screenshotDataUrl` not `dataUrl`.
  - Exception: well-established abbreviations are fine.
- Strict camelCase / TitleCase for multi-word names. Acronyms are treated like normal words: `remoteUrl` not `remoteURL`, `HttpClient` not `HTTPClient`.
- Booleans take a predicate prefix — `is`/`has`/`can`/`should`/`was`/`will`, … (e.g. `isLoading` not `loading`).
- A function that returns a new instance takes a `new` prefix: `newWriter`, `newApiClient` (not `create…`/`make…`).
- For recurring concepts, use consistent terminology across the codebase — always `options` or always `params`, not both.
- Qualify a name by the case it belongs to. When a value applies only under a specific mode, branch, or variant — or sits beside siblings it could be confused with — put that qualifier in the name: `FINE_MOVE_STEP` not `MOVE_STEP`, `retryDelayMs` not `delayMs` (when other delays exist), `adminEmails` not `emails`. Test: in the wrong context the unqualified name reads as plausibly correct, the qualified one as obviously wrong.

## Parameters

- Use named parameters (parameter object, keyword arguments, or equivalent) for 3+ params; positional style for single-param functions. Exception: match the signature of an external interface you implement or fake.
- A parameter the body branches on to pick between behaviours is a flag — split it into two named functions.

## Comments

Follow [writing.md](writing.md). Additionally: prefer clear naming and small functions over explanatory comments (see Abstraction).

## Type Safety

- Never bypass the type system with escape hatches — `any`/`@ts-ignore` (TS), `unsafe` (Rust), unchecked assertions (Go), equivalents elsewhere.
- Don't silence possible missing-value errors with assertion syntax. Narrow the type first or pass a guaranteed value explicitly.
- Prefer precise types; a too-wide type hides bugs the same way a missing type does.
- Prefer enums (or named-enumeration constructs) that expose symbolic members at call sites — renames ripple through the type checker. A literal type alias doesn't satisfy this; call sites still embed the raw value.
- No unused imports or variables — remove rather than keep dead code.
- Inline single-use object/interface shapes. Extract a named type only when referenced from 2+ places.

## Control Flow

- Prefer early returns to reduce nesting.
- Limit nesting depth to ~3 levels; deeper that early returns can't fix → extract inner logic into a named function.
- Prefer multi-line `if` statements with braces over single-line guard clauses like `if (!value) return`.
- Use the language's single idiomatic absence value. In TS/JS prefer `undefined` over `null`; pick one and stick to it.
- No special case bolted onto an unrelated flow — move it behind its own abstraction, or into the slice that owns it.

## Logging

Call-site rules — where log lines belong and what they say; the logger itself (construction, format, levels) is [project.md](project.md)'s. Review for *absence* as much as quality: a boundary or handled failure with no log line violates these rules the same way a bad message does.

- Log at boundaries: inbound work accepted (request, job, message), outbound calls to other systems, process lifecycle (startup with resolved config, shutdown). Pure interior logic returns values instead of logging — its callers log.
- Every failure path that doesn't propagate must log: a caught-and-handled error, a retry, a fallback, a degraded mode. Silent recovery is invisible behavior — the most common logging defect.
- Log an error where it's *handled*, once — never at every layer it passes through, and never log-and-rethrow at the same layer.
- A message names the operation, its key identifiers, and the outcome with its cause — "failed to \<operation\> for \<entity\>: \<error\>", never a bare "error occurred". Static message text plus structured fields (ids, counts, durations) beats interpolated prose: grep finds the one site, and json format keeps fields queryable.
- Carry enough correlating fields (request id, entity id, attempt) to follow one flow across lines.
- Levels by action needed: `error` — someone must act or an invariant broke; `warn` — handled but degraded, worth attention if recurring; `info` — the operational narrative an incident timeline needs; `debug` — development diagnostics. If nobody would act on it, it isn't `error`.
- No per-item logging at `info`+ inside loops — one aggregate line with counts, or drop to `debug`.
- Keep secrets and PII out at the call site rather than redacting downstream — pass only what's safe to print.

## Abstraction

Two demands — a unit can hold one and fail the other:

- **One level — don't mix.** Keep a unit at one abstraction level; don't interleave an intent-revealing operation with the low-level mechanics it's built from — push those down. Callers depend on `apiClient.signUp()`, never `httpClient.post(...)`.
- **Name the operations.** Wrap raw mechanics in an intent-named operation so the unit reads top-down as prose — `emitToken(delta)`, not an inline `stream.writeSSE({ … })`. A comment narrating *what* a block does is a missing name.

## Classes vs functions

Reach for a class (or a `newX` factory returning an object with private state) when operations share state or configuration, form one cohesive named unit, or sit at a boundary that benefits from injection — an API client, an adapter, a service, a stateful session. A module of free functions threading the same implicit dependencies — a shared base URL, a `fetch`, private helpers everyone calls — is that class turned inside-out; group it into one. Keep as plain functions only what is genuinely standalone: stateless pure logic (the functional core — data in, data out) and one-off helpers. Don't invert it either — a class with only static methods is a module with extra syntax, and wrapping a single function in a class is noise.

## Extraction & Size

- Prefer the reframe that deletes a branch or concept over cleanup that keeps it — model state so invalid cases can't be represented, and the conditional disappears instead of getting tidied.
- Functions do one thing. Extract when concerns split, abstraction blurs (above), or length exceeds ~100 lines.
- Tolerate duplication at 1–2 sites; extract a shared helper on the 3rd. Earlier abstraction risks the wrong shape.
- Don't extract tiny wrappers around self-evident code — indirection without payoff.
- Keep files focused on a single responsibility — split when a file grows beyond ~450 lines or contains unrelated concepts.
- Avoid barrel/index files. Import from the source module directly — they hide locations, hurt tree-shaking, and invite circular imports. Likewise, never re-export a symbol through a module that isn't where it's defined just to keep an import path stable; a symbol has one home, so import it from there and update importers if that home moves.

## Dependencies

Prefer defaults and latest stable — tools (linters, formatters, build, test runners, type checkers, etc.), libraries (ORMs, loggers, HTTP clients, etc.), runtimes, base images, and anything similar. Choose latest LTS when upstream offers one; otherwise latest stable — expressed as a concrete pinned range (e.g. `^1.4.0`), never a floating tag like npm `latest` (non-reproducible). Avoid pre-releases (alpha/beta/RC, etc.) unless the feature is required and not yet in stable.

Override a default or pin to an older version only when concrete breakage forces it — "might be nicer" is not enough. Leave a one-line comment with the reason; if it doesn't fit on one line, the option probably doesn't belong. Prune overrides and pins when the reason no longer holds. No unused dependencies — remove a package once nothing uses it.

## Tooling

- Use TypeScript LSP for TS/JS work — diagnostics, types, go-to-def, refs.
