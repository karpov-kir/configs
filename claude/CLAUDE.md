# Core Principles

1. Think before coding. State assumptions out loud. If ambiguous, ask; if a simpler approach exists, push back. When confused, name what is unclear — don't pick one interpretation and run.
2. Simplicity first. Minimum code that solves the problem. No speculative abstractions, no flexibility nobody asked for. Test: would a senior engineer call this overcomplicated.
3. Surgical changes. Touch only what the task requires. Don't improve neighboring code. Every changed line traces back to the request.
4. Goal-driven execution. Turn vague instructions into verifiable targets before writing a line. "Add validation" → "write tests for invalid inputs, then make them pass."

# Writing Guidelines

Applies to anything you write. Persistent artifacts (code comments, PR/commit descriptions, tickets, design/investigation docs, and similar) always follow these guidelines in full — lean, concise, fact-per-line — but in normal prose; caveman mode never applies to them, even when active. Chat responses still follow caveman style when on.

* Scope. Stay within the artifact's responsibility — reference only down to its own altitude; link to other layers rather than restating them. Function docs describe the function, not its callers; prose above the code (commit/PR descriptions, tickets, design docs, and similar) states the problem and outcome, not the files or functions implementing it — that trace lives in the diff.
* Lead with the "why", not the implementation trace.
* Describe conceptually — what happens and why, not call-by-call. One abstraction level.
* Keep code references minimal; the diff is source of truth.
* Group by purpose, not by file.
* Each line must carry a fact unreachable from surrounding context (code, types, siblings, the diff, etc.). Cut or link otherwise.
* No backstory, hedging, or justification — describe what is true, not what we tried.

# Architecture

Prefer **vertical slicing** (organize by feature) and **horizontal decoupling** (logic behind ports). Before designing a module layout or wiring dependencies, read [core.md](~/.claude/architecture/core.md) for the shared core, then [backend.md](~/.claude/architecture/backend.md) or [frontend.md](~/.claude/architecture/frontend.md) for the side you're on.

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

Follow [Writing Guidelines](#writing-guidelines). Additionally: prefer clear naming and small functions over explanatory comments (see Abstraction).

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

# Testing

No mocks; treat test code as production code. Before writing or reviewing tests, read [testing.md](~/.claude/testing.md).

# Defaults & Pins

Prefer defaults and latest stable — tools (linters, formatters, build, test runners, type checkers, etc.), libraries (ORMs, loggers, HTTP clients, etc.), runtimes, base images, and anything similar. Choose latest LTS when upstream offers one; otherwise latest stable — expressed as a concrete pinned range (e.g. `^1.4.0`), never a floating tag like npm `latest` (non-reproducible). Avoid pre-releases (alpha/beta/RC, etc.) unless the feature is required and not yet in stable.

Override a default or pin to an older version only when concrete breakage forces it — "might be nicer" is not enough. Leave a one-line comment with the reason; if it doesn't fit on one line, the option probably doesn't belong. Prune overrides and pins when the reason no longer holds. No unused dependencies — remove a package once nothing uses it.

# Project Setup

Environments, scripts, and local dev / Docker: [project.md](~/.claude/project.md).

# Git

Before any `git commit` or `git push`: print the full command and get explicit approval first. Branch, commit, and PR conventions: [git.md](~/.claude/git.md).

# Tooling

- Use TypeScript LSP for TS/JS work — diagnostics, types, go-to-def, refs.

# Caveman Mode

If caveman mode is active (startup hook sets it), display this banner as the first thing in the first message of the session:

```
🦴 CAVEMAN MODE ACTIVE 🦴
```

# Memory

Add new memory entries to this section. Do **not** create or write to per-project memory dirs (`~/.claude/projects/*/memory/`) — keep all memory here.

This is a staging area. Once enough memory accumulates, the user folds entries into the proper sections of this document. Do not reorganize on your own. Entries here are authoritative — apply them as if they were in a structured section.

- Never add NOSONAR or similar inline lint/Sonar suppression comments. Kirill resolves unfixable Sonar findings manually in the SonarCloud UI — if a finding can't be fixed in code, leave it and report it instead.
