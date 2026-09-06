# Git

## Branches

Name branches `<type>/<TICKET>-<slug>` — type is `feature`, `fix`, `refactor`, `chore`, `docs`, `test` or `style`, or the prefix a workflow that owns the branch's whole lifecycle defines for itself; drop the ticket when there's none.

## Commits

**Print the command and get approval before any commit or push.**

**A push to a shared or default branch cannot be taken back.** [live-systems.md](live-systems.md) → **Arrange the undo before the act** holds there even where the approval above was lifted.

Short, imperative, one-line subject (~50 chars); a body only when the *why* isn't obvious from the diff. Frame it for the repo's consumer — the user-visible effect, not the internal mechanism.

**Re-form the change set after any review pass**, and never re-present a commit command formed earlier in the run.

**Stage by explicit path, never `-A`, wherever anything else may be writing the tree** — another agent, another session, a watcher. `-A` commits whatever it finds, so a file someone else is mid-edit lands in your change set under your message, and neither of you sees it happen. **A stage left sitting is the same hazard from the other side**: `git add` can succeed and the commit then abort, and the next writer's `-A` sweeps what you staged. Confirm the commit landed, and unstage what it did not take. **And a file that reads as finished is not a finished one** — leave a file another writer is in out of your commit until they call it done. A clean working tree is not that declaration.

**The message is outward text** — [human-writing.md](human-writing.md) → **Budget** runs over it before you print the command.

Match the recent style on the branch (`git log` first). Use semantic prefixes (`feat:`, `fix:`, …) only when the branch already does and commits land directly; PR branches default to plain.

## Branch shape

**A reviewer opens the branch to read the change, not the route you took to it.** A correction to work that exists only on this branch amends or fixes up into the commit it corrects, rather than reaching the PR as a commit of its own. A squash on merge does not cover this — the branch is read before it is merged.

**A rework spanning several unlanded commits is one honest commit**: there is nothing to fix up into, and a rebase across the branch costs more than the shape buys. **The rule reaches only the commits no reviewer has read** — rewriting one they have read discards what they checked, so a correction to those lands openly and names what it corrects. **Never reshape pushed history to satisfy this rule.**

## Pull requests

Open as drafts; follow the repo's PR template if it has one.
