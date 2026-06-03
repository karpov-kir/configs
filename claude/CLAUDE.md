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

# Code Style

Baseline rules below. If the project root has a `PROJECT_CODE_STYLE.md`, merge its rules on top (project overrides win).

## Naming

- Use full, descriptive variable names: `event` not `e`/`evt`, `error` not `err`/`e`, `element` not `el`, `index` not `i` (unless in a tight numeric loop), `accumulator` not `acc`, `value` not `val`, `screenshotDataUrl` not `dataUrl`.
  - Exception: well-established abbreviations are fine.
- Strict camelCase / TitleCase for multi-word names. Acronyms are treated like normal words: `remoteUrl` not `remoteURL`, `HttpClient` not `HTTPClient`.
- Booleans take a predicate prefix — `is`/`has`/`can`/`should`/`was`/`will` (e.g. `isLoading` not `loading`).
- For recurring concepts, use consistent terminology across the codebase — always `options` or always `params`, not both.

## Parameters

- Use named parameters (parameter object, keyword arguments, or equivalent) for 3+ params; positional style for single-param functions.

## Comments

Follow [Writing Guidelines](#writing-guidelines). Additionally: prefer clear naming and small, declaratively named functions over explanatory comments.

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

## Extraction & Size

- Functions do one thing at one abstraction level. Extract when concerns split, abstraction levels mix, or length exceeds ~100 lines.
- Tolerate duplication at 1–2 sites; extract a shared helper on the 3rd. Earlier abstraction risks the wrong shape.
- Don't extract tiny wrappers around self-evident code — indirection without payoff.
- Keep files focused on a single responsibility — split when a file grows beyond ~400 lines or contains unrelated concepts.
- Avoid barrel/index files. Import from the source module directly — they hide locations, hurt tree-shaking, and invite circular imports. Likewise, never re-export a symbol through a module that isn't where it's defined just to keep an import path stable; a symbol has one home, so import it from there and update importers if that home moves.

# Defaults & Pins

Prefer defaults and latest stable — tools (linters, formatters, build, test runners, type checkers, etc.), libraries (ORMs, loggers, HTTP clients, etc.), runtimes, base images, and anything similar. Choose latest LTS when upstream offers one; otherwise latest stable. Avoid pre-releases (alpha/beta/RC, etc.) unless the feature is required and not yet in stable.

Override a default or pin to an older version only when concrete breakage forces it — "might be nicer" is not enough. Leave a one-line comment with the reason; if it doesn't fit on one line, the option probably doesn't belong. Prune overrides and pins when the reason no longer holds.

# Git

## Branches

Name branches `<type>/<TICKET>-<slug>` — type is `feature`, `fix`, `refactor`, `chore`, `docs`, `test`, or `style`. Drop ticket when none exists.

Good: `fix/TA-2826-bad-git-ref`, `refactor/TA-2847-extract-execasync`, `chore/bump-eslint`
Bad: `ta-2847-exec-timeouts`, `my-fix`

## Commits & Push

Commit messages: short, imperative, one-line subject (~50 chars). Body only when *why* isn't obvious from the diff.

Frame for the repo's end user — app users for apps, downstream devs for libraries, operators for infra, and so on. Describe the user-visible effect, not the internal mechanism.

Good: `fix race in token refresh`
Bad (verbose): `Updated the auth middleware to fix a bug where tokens were sometimes refreshed twice`
Bad (technical): `loosen regex from \d{4} to \d+`
Good (user-facing): `support any version number in device names`

Match the recent commit style on the branch — `git log` first. Use semantic prefixes (`feat:`, `fix:`, …) only when the branch already does AND commits land directly. PR branches default to plain since squash subjects are what ship.

Before executing any `git commit` or `git push` command, always:
1. Print the full command you intend to run
2. Ask for explicit approval before executing

## Pull Requests

* Always follow [Writing Guidelines](#writing-guidelines).
* If the repo has a PR template, follow it too.
* Do not add a "Test plan" section.
* When addressing a PR comment, reply on the comment thread with `Done <link to commit>` pointing at the commit that resolves it.

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
