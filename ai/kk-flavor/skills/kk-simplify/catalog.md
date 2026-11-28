# What kk-simplify looks for

The scout checks each class below against every file and passage it reads. Each entry names what to look for, the default verdict, the simpler form, and an example. The verdicts are `~/.kk-flavor/workers/simplify/scout.md` → **The bar**. A shape not listed here still gets that bar.

## Docs, README and instruction files

- **Look for:**
  - a long doc, a walkthrough of code, and backstory;
  - a section two docs carry;
  - a repeat of what the code, the types or a command show;
  - a rule a linter, the types or a check enforces.
- **Default:** `delete`. A doc with a reader is `shorten`.
- **Becomes:** the essence in three to five lines, plus a link to the code or command that shows the rest. A README says what the project is, how to run it, and where to look. Instruction files follow `~/.kk-flavor/standards/project.md` → **Repo files**. A shared section gets one home, and the other doc links to it.
- **Example:** a 40-line setup guide becomes "Run `<task> setup`. The task file lists the steps."

## Comments

- **Look for:** a comment that restates or narrates the code, tells the story of a change, or points to a test as its evidence. Commented-out code.
- **Default:** `delete`. A note with an outside fact is `shorten`.
- **Becomes:** one sentence on the act and the outside fact behind it, by `~/.kk-flavor/standards/comments.md`.
- **Example:** "// loop over the users and check each one" goes. A five-line note on a vendor's retry quirk becomes "Retries twice, because the vendor drops the first request after a deploy."

## Code

- **Look for:**
  - code without a caller, and an unreachable error path;
  - a branch on a flag that is on everywhere, or on an environment that does not exist;
  - defensive code the types or the callers already rule out: a check for the impossible, a try/catch that only rethrows, a fallback that never runs;
  - an interface or layer with one implementation, and a pass-through function;
  - a function or component with one caller, and a parameter no caller passes;
  - a compatibility path for an old shape;
  - two ways to do one thing in one repository: two helpers, two patterns, two styles;
  - deep nesting, a long branch chain, and a home-grown utility the platform or a dependency in use already gives.
- **Default:** `delete` for uncalled code, dead branches and defensive code. `simplify` for the rest.
- **Becomes:** a direct call, the inlined body, the single value every caller uses, early returns, a lookup table, or the platform's own function. Every caller moves to the new form, and no shim stays. The bar for depth is `~/.kk-flavor/standards/architecture/core.md` → **Module depth**.
- **Example:** a five-line helper module with one caller is inlined into that caller, and the module goes.

## Architecture

- **Look for:** a generic framework, a plugin point or a dependency container serving one case. Layers or packages that only forward to each other. A home-grown build step, generator or tool. Several runtime profiles with the same behaviour.
- **Default:** `simplify`.
- **Becomes:** plain code for that case, one merged package, the standard tool, or one profile.
- **Example:** a plugin registry with one plugin becomes a direct call to that plugin.

## Config

- **Look for:**
  - a value equal to its default;
  - a setting repeated across files;
  - per-environment files that differ in one value;
  - an unused variable or script, and the config of an absent tool;
  - a lint suppression without a reason.
- **Default:** `delete`, then `generalize`.
- **Becomes:** one source that the others read, or one file and the value that differs. Fix the code a removed suppression hid.
- **Example:** a suppression of "no explicit any" goes, and the parameter becomes `unknown`.

## Dependencies

- **Look for:** an unused dependency, two libraries for one job, and a heavy dependency used for one function the platform gives.
- **Default:** `delete`, then `simplify`.
- **Becomes:** no dependency, or a single library, or the platform's function.
- **Example:** a date library used for one ISO format call becomes the platform's date formatting.

## CI and tasks

- **Look for:**
  - a job or step two workflows repeat;
  - a job whose result branch protection ignores and a person never reads;
  - an unused task-runner task or script;
  - a workflow permission that no step uses;
  - a test suite near its timeout.
- **Default:** `delete`. A slow suite is `simplify`.
- **Becomes:** one job, the task that is used, the narrowest permissions. A slow suite gets faster or splits, and its timeout stays as it is.
- **Example:** a lint job that runs in two workflows stays in one.

## Tests

- **Look for:** a test that cannot fail, restates the behaviour it asserts, or repeats a cheaper test's proof. A test of the implementation. A mock. A test on a slower level than it needs. Long setup. Many cases of one behaviour. A flaky or slow test. A huge snapshot. A skipped or focused test.
- **Default:** `delete`, then `simplify`.
- **Becomes:** one table-driven case, one scenario through the published surface, a fake or the real thing, the cheapest level, a builder, or a small exact assertion. A skipped test runs again with its cause fixed, or goes. The bar is `~/.kk-flavor/standards/testing.md`.
- **Example:** six cases that each check one invalid option become one table of inputs and expected errors.

## Guards

A guard is a guard test, a CI check or a hook.

- **Look for:**
  - a guard that guesses from the shape of text;
  - a guard for a case the types can rule out;
  - a guard for a single past incident;
  - two overlapping guards, and a hook that repeats a CI check.
- **Default:** `delete`, then `simplify`.
- **Becomes:** a check on the parsed structure by `~/.kk-flavor/standards/architecture/core.md` → **App layers**, the type that rules the case out, or one guard. A rule that a tool already wired in can check on structure moves into that tool. A rule that no such tool can check stays with the standard and review.
- **Example:** a test that greps source files for a forbidden import becomes the linter's import rule.

## Rules and principles

- **Look for:**
  - a specific rule, a decision, a dated exception, and a list of past exceptions;
  - a rule a linter or the types enforce;
  - a principle that overlaps another, or whose every exclusion another principle already makes.
- **Default:** `delete`, then `generalize`.
- **Becomes:** the broad principle that covers the specific cases. A specific rule stays only at the bar in `~/.kk-flavor/standards/project.md` → **Repo files**.
- **Example:** three dated exceptions about the changelog become "Change the changelog only with the human's approval."

## Facts and links

- **Look for:** a path, command, name, number or described behaviour in a doc or a comment that the code does not match. A broken link, and a link to a deleted file, PR or ticket.
- **Default:** `simplify` for a wrong fact, `delete` for a dead link.
- **Becomes:** the fact the code shows. A dead link goes.
- **Example:** "the fake writes only strings" becomes "the fake writes strings and bytes", which its code does.

## API surface and schemas

- **Look for:** a route, a response field, a parameter or an enum value without a consumer.
- **Default:** `delete`. It needs the human when a user of the product sees it.
- **Becomes:** removed, together with its schema description in the same change.
- **Example:** a response field no client reads goes from the handler and from the schema.

## Data and storage

- **Look for:** an unused table, column or storage key.
- **Default:** `delete` by a new migration. A migration that drops anything holding rows needs the human.
- **Becomes:** a migration that drops it, and the code and schema that named it go too.
- **Example:** a column the code stopped writing a year ago goes by a new migration, after the human approves.

## Observability

- **Look for:**
  - a log line, metric or analytics event that has no reader;
  - a noisy log line that leads to no action.
- **Default:** `delete`.
- **Becomes:** removed, or one log line at the level its reader acts on.
- **Example:** a debug log on every request goes.

## Names

- **Look for:** a name that misleads after the code around it changed.
- **Default:** `simplify`.
- **Becomes:** the name of what the code does now, with every reference renamed.
- **Example:** a module named after analytics that now only reports health is renamed after health.

## Permissions, secrets and environments

- **Look for:**
  - an extension permission or host permission that the code does not use;
  - an unused secret or environment variable;
  - an env file for an environment that does not exist.
- **Default:** `delete`.
- **Becomes:** removed from the manifest, the workflow or the env file. A secret's value stays with the human.
- **Example:** a tabs permission no code calls goes from the extension manifest.

## History, generated and binary files

- **Look for:**
  - a changelog that repeats git;
  - a superseded plan or decision record, a handoff, a report, a brief;
  - retired migrations, stale data and fixtures;
  - generated or vendored code tracked without need, and committed build output;
  - an unused or unoptimised image or font.
- **Default:** `delete`. An image in use is `simplify`.
- **Becomes:** removed, with generated output added to the ignore file. An image in use gets optimised.
- **Example:** a handoff note for a merged PR goes.

## Agent files

- **Look for:** a skill, agent file or prompt in the repository that is stale or repeats a shared standard.
- **Default:** `delete`. One that adds a repository fact is `shorten`.
- **Becomes:** removed, or cut down to the repository fact it adds.
- **Example:** a repository skill that restates the shared testing standard goes.

## Duplicates across repositories

- **Look for:** a rule, doc, script, config or workflow that another repository or a shared package also carries. Read the shared sources this repository already uses and the sibling clones beside it, read only.
- **Default:** `generalize`.
- **Becomes:** one source, such as the shared standard, the shared workflow or the config package. The other copies link to it or go.
- **Example:** a copied release workflow becomes a call to the shared one.

## Product features

- **Look for:** a feature with no sign of use.
- **Default:** a proposal for the human only. Give the evidence you found, and leave the code as it is.

## Tails

The `tails` scout lists these for the human, and changes none of them:

- untracked and ignored files other than build output, stashes, worktrees, and every branch other than the default one;
- repository settings: secrets, environments, webhooks, labels, stale issues and PRs;
- local container images and volumes the project left behind.
