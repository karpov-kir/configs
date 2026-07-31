# Standards (kk-flavor)

Read `~/.kk-flavor/inject.md` now and follow it — applies to all work, skill-invoked or ad-hoc. The first time you load it in a session, open that message with this banner so I can see it's active:

```
🍦 kk-flavor loaded 🍦
```

# Caveman Mode

If caveman mode is active (startup hook sets it), display this banner as the first thing in the first message of the session:

```
🦴 caveman mode active 🦴
```

# Memory

Add new memory entries to this section. Do **not** create or write to per-project memory dirs (`~/.claude/projects/*/memory/`) — keep all memory here.

This is a staging area. Once enough memory accumulates, the user folds entries into the proper sections of this document. Do not reorganize on your own. Entries here are authoritative — apply them as if they were in a structured section.

- Never add NOSONAR or similar inline lint/Sonar suppression comments. Kirill resolves unfixable Sonar findings manually in the SonarCloud UI — if a finding can't be fixed in code, leave it and report it instead.
- player-testautomation dev env: the repo pins node 22 (mise.toml / .nvmrc), but the default shell node is 24, which fails native builds (node-libcurl, sqlite3). Run every npm/jest/tsc via `mise exec node@22 -- …` from the repo root. Deps may be uninstalled with an out-of-sync lockfile (`npm ci` fails "Missing … from lock file") — use `mise exec node@22 -- npm install` (updates package-lock.json; don't commit that unless intended). Run tests with `node_modules/.bin/jest <pattern>` from root (jest projects config; `.test`/`.integ.test` suffixes). ts-jest maps `@bitmovin-internal/*/dist/*` imports to src, so jest type-checks changes without a build; a standalone `tsc -p tsconfig.build.json` instead fails with "Cannot find module …/dist/…" unless sibling packages are built first (`lerna run build`). `gh` is installed + authed (account karpov-kir); Jira via the atlassian skill needs an interactive `acli auth login --web` that can't run headless.
