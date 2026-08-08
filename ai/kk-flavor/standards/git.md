# Git

## Branches

Name branches `<type>/<TICKET>-<slug>` — type is `feature`, `fix`, `refactor`, `chore`, `docs`, `test`, or `style`; drop the ticket when there's none.

## Commits

**Print the command and get approval before any commit or push.**

Short, imperative, one-line subject (~50 chars); a body only when the *why* isn't obvious from the diff. Frame for the repo's consumer — app user, library dev, or operator — the user-visible effect, not the internal mechanism.

**Re-form the change set after any review pass.** Run `git add -A` after the final pass, and never re-present a commit command formed earlier in the run.

Match the recent style on the branch (`git log` first). Use semantic prefixes (`feat:`, `fix:`, …) only when the branch already does and commits land directly; PR branches default to plain, since the squash subject is what ships.

## Pull requests

- Open as drafts; follow the repo's PR template if it has one.
- Link the ticket in the description when the branch carries one.
- The description follows [human-writing.md](human-writing.md).
- Resolve a review comment by replying on its thread with `Done <link to commit>`.
