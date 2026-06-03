# Git

## Branches

Name branches `<type>/<TICKET>-<slug>` — type is `feature`, `fix`, `refactor`, `chore`, `docs`, `test`, or `style`; drop the ticket when there's none.

Good: `fix/TA-2826-bad-git-ref`, `refactor/TA-2847-extract-execasync`, `chore/bump-eslint`
Bad: `ta-2847-exec-timeouts`, `my-fix`

## Commits

Short, imperative, one-line subject (~50 chars); a body only when the *why* isn't obvious from the diff. Frame for the repo's consumer — app user, library dev, or operator — the user-visible effect, not the internal mechanism.

Good: `fix race in token refresh`
Bad (verbose): `Updated the auth middleware to fix a bug where tokens were sometimes refreshed twice`
Bad (technical): `loosen regex from \d{4} to \d+`
Good (user-facing): `support any version number in device names`

Match the recent style on the branch (`git log` first). Use semantic prefixes (`feat:`, `fix:`, …) only when the branch already does and commits land directly; PR branches default to plain, since the squash subject is what ships.

## Pull requests

- Open as drafts; follow the repo's PR template if it has one.
- No "Test plan" section.
- Follow CLAUDE.md's Writing Guidelines.
- Resolve a review comment by replying on its thread with `Done <link to commit>`.
