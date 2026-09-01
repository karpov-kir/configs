# Git

## Branches

Name branches `<type>/<TICKET>-<slug>` — type is `feature`, `fix`, `refactor`, `chore`, `docs`, `test` or `style`, or the prefix a workflow that owns the branch's whole lifecycle defines for itself; drop the ticket when there's none.

## Commits

**Print the command and get approval before any commit or push.**

Short, imperative, one-line subject (~50 chars); a body only when the *why* isn't obvious from the diff. Frame it for the repo's consumer — the user-visible effect, not the internal mechanism.

**Re-form the change set after any review pass**, and never re-present a commit command formed earlier in the run.

**Stage by explicit path, never `-A`, wherever anything else may be writing the tree** — another agent, another session, a watcher. `-A` commits whatever it finds, so a file someone else is mid-edit lands in your change set under your message, and neither of you sees it happen. **A stage left sitting is the same hazard from the other side**: `git add` can succeed and the commit then abort, and the next writer's `-A` sweeps what you staged. Confirm the commit landed, and unstage what it did not take. **And a file that reads as finished is not a finished one** — committing work another writer has not called done publishes a decision they had not made, under your name, and a clean working tree is not that declaration.

Match the recent style on the branch (`git log` first). Use semantic prefixes (`feat:`, `fix:`, …) only when the branch already does and commits land directly; PR branches default to plain.

## Pull requests

- Open as drafts; follow the repo's PR template if it has one.
- Link the ticket in the description when the branch carries one.
- The description follows [human-writing.md](human-writing.md).
- Resolve a review comment by replying on its thread with `Done <link to commit>`.
