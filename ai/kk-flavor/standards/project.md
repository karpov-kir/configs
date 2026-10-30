**Layer:** craft

# How We Set Up a Project

Starter configs: [`templates/`](../templates/) — match them closely; diverge only on what no template covers.

## Environments

A service's own folders — `env/`, `docker/` — live beside its `src/`, inside the package in a monorepo (`packages/<service>/`), never the repo root. `env/` holds one file per environment (`.env.<env>`) — real, working values, never `.env.example` placeholders.

- **Select by environment.** A build tool with mode support (Vite, wxt) reads `.env.<env>` itself; otherwise a small loader reads `.env.$ENV`, defaulting to `development`. Config reads that same selector (`ENV`, or the build tool's mode), never a second variable like `NODE_ENV`. Loading is the app's job (build tool or loader), never the task runner's — a mise `[env] _.file` bypasses the encrypt-at-rest flow.

- **Commit real env; encrypt only secrets.** Per file, by whether it holds a secret:
  - **No secrets** → committed plain and baked into the image (**Local dev & Docker**, below).
  - **Has secrets** → committed only as `.env.<env>.encrypted` (plaintext gitignored), never in the image; decrypted and injected at deploy.

  Encrypt and decrypt **per environment**, with `env:encrypt:<env>` and `env:decrypt:<env>`. Let the infrastructure choose the encryption tool and its keys.

- **Ask every password in a macOS dialog with hidden input** — an encryption tool's password, a deploy's sudo on a server, and any other password a task needs. The dialog text names the password and what happens once it is given. Pipe the answer into the consuming command's stdin (`openssl -pass stdin`, `sudo -S`). Keep a password out of terminal prompts, command arguments and exported variables. Copy the `ask` helper from [`templates/mise.toml`](../templates/mise.toml) for the dialog. A script that runs on a server reads its passwords from stdin, and the workstation task that starts it asks for them and pipes them over ssh.

- **Name a CI secret `<SERVICE>_<KIND>` in upper snake case** (`SONAR_TOKEN`, `DOCKER_REGISTRY_PASSWORD`, `NPM_PUBLISH_TOKEN`, `DEPLOY_SSH_KEY`). Give a secret the same name in every repo and environment that holds it. Give each repo its own value, so a leaked one is revoked alone. Name a GitHub environment after the deploy track it guards (`staging`, `production`).

## Config

The app reads its config from one typed object, never raw `os.Getenv` / `import.meta.env`. Build it at startup as a singleton.

- Coerce to real types: numbers, booleans, the environment as an **enum** — not bare strings.
- Define defaults inline; env vars only override them. **Provider selection is required when multiple providers are supported.** Reject missing, invalid or unavailable selections with a clear error before acting; never infer a provider or fall back to another.
- For the same role across model providers, choose comparable capability, latency and cost tiers, and document the pairing.
- Log it with secrets masked.

## Logging

- **Format and level are explicit, never derived from the environment** — so any environment (production included) runs locally with readable logs. `LOG_FORMAT` is `pretty` or `json`; `LOG_LEVEL` is one of the levels in [code-style.md](code-style.md) → **Logging**.

## Scripts

**mise** runs the tasks (`mise.toml`, `mise run <task>`) and pins the tools, in place of any Taskfile. Use the same names on every stack:

| Script | Does |
|---|---|
| `start` / `start:dev` | the built long-running service (fails if not built) / the dev loop — compile + watch |
| `exec` / `exec:dev` | the built one-shot tool (a CLI) / the same from source |
| `build` / `build:<env>` | one env-agnostic build; `build:<env>` only where the artifact can't take env at run time (a chrome extension) |
| `test` / `test:<level>` | the test suites — see [testing.md](testing.md) |
| `lint` / `lint:fix` | every linter / every linter's fix |
| `lint:<type>` / `lint:<type>:fix` | one linter — `lint:eslint`, … |
| `setup` | installs every dependency of the repo, and is the only way to install them |
| `check` / `check:ci` | `lint`, `build` and `test` / the same with `test:ci`, which CI runs |
| `version` | the released part's version, `X.Y.Z` alone on one line (**Versions**, below) |

`<env>` is `development`, `staging`, or `production`. Drop any script that doesn't apply. Keep `setup` in every repo, empty where the repo installs none.

Make `check` and `check:ci` run only their dependencies, `lint`, `build` and `test` or `test:ci`, so a step added to one of those reaches both. Count a type check or a format check as a linter (`lint:tsc`, `lint:gofmt`, `lint:govet`). Where a repo has only `build:<env>`, `check` and `check:ci` depend on `build:production`.

Group related operations under a shared first segment (`docker:build`/`docker:publish`). A package runner's scripts (npm/bun) stay in its `package.json`, and a mise task that only runs one names it (`run = "npm run build"`, `dir = "web"`).

In a monorepo, re-expose each package's commands at the root as `<package>:<command>` (`backend:start:dev`, …). Keep repo-wide aggregates (`test`, `lint`) bare. Name a package's folder for its role: `backend` for the server, `web` for anything a browser runs (a browser extension or an admin UI included), `desktop` for a desktop app. Keep one `mise.toml` at the repo root and its own `package.json` in each package.

## Versions

A **released part** is what ships and deploys as one piece, with its own release.

- **Keep exactly one version per released part, in the native version field of its language or tool.** Raise it by hand in a PR. A web app, browser extension, desktop app or npm library keeps it in `package.json` `version`, and an extension's build writes its manifest's `version` from there.
- **Keep a `VERSION` file at the part's root only where no native field exists**, as in Go. Write one line `X.Y.Z` in it, and set no version through a constant or `-ldflags`. In Go, embed the file from a package at the part's root with `//go:embed`, trim its trailing newline, and pass the value down.
- **Give a package inside a released part no version of its own.** Leave `version` out of the `package.json` of a web UI embedded in the server's image.
- **Return the version from a service's `GET /health`**, as the JSON field `version`.
- **Release in CI from that version and never compute a bump.** Tag `v<version>` when the repo has one released part, and `<part>-v<version>` when it has several.
- **Skip the release when the tag already exists.** On the same commit, stop. On another commit, warn to raise the version and leave the job green.

| Repo | Version file | Tag |
|---|---|---|
| Go only | `VERSION` | `v<version>` |
| Go and web, released separately | `backend/VERSION`, `web/package.json` | `backend-v<version>`, `web-v<version>` |
| Go and web, released together | `backend/VERSION`, and `web/package.json` without `version` | `v<version>` |

## Local dev & Docker

Run the service on the host and open it at `http://localhost:<port>`, with no TLS-terminating proxy in front. `localhost` is a secure context, and Google OAuth accepts `http://localhost` redirects. Run its dependencies in the service's own `docker/docker-compose.yml`, under its own `name:`. Make its `start:dev` bring them up and stop them when the dev loop ends.

- **Publish each dependency on a random host port**, as in `127.0.0.1::5432`. Have every task that reaches a dependency, `start:dev` and `db:migrate` included, read its port with `docker compose port`. Pass it on through the app's own variables, such as `DATABASE_HOST` and `DATABASE_PORT`. Keep those variables out of `.env.development`.
- **Commit the Postgres password as `env/database-password.development`.** Have the compose mount it as a secret (`POSTGRES_PASSWORD_FILE`) and the app read it through `DATABASE_PASSWORD_FILE`.
- **Fix each app's own port in its `.env.development`**, and make it differ from every other service's port. Keep no registry of ports. A clash shows at once as a failed bind.
- **Wire the web to the API the way production does.** With the UI and the API on different production origins, let the API's CORS allow the web dev origin. Read the allowed origins through the app's config, set locally in `.env.development`. With both on one production origin, have the web dev server proxy the API path.
- **Start every cookie name with the app's name**, as in `<app>_session`, after any `__Host-` or `__Secure-` prefix. Browsers scope a cookie by host and ignore the port, so different services side by side on `localhost` would overwrite each other's cookies.

Keep the service's container setup in its own `docker/`, which holds its compose and its `Dockerfile`.

- **Only the non-secret env files are `COPY`-ed into the image** — staging and production, not dev. `ENV` itself is always provided at deploy, never baked.
- **Run a Go or Node build stage on `--platform=$BUILDPLATFORM`** and have it build for `$TARGETPLATFORM` ([`templates/docker/Dockerfile`](../templates/docker/Dockerfile)).

## CI, images & deploys

- **Pick the CI runner by repository visibility.** Run a public repository's CI on hosted runners only. A pull request from a fork runs its author's code on the runner. Keep self-hosted runners for private repositories.
- **CI builds and pushes every app image after each commit to the default branch that passes its checks.** The release it publishes records each image's digest reference, `<registry>/<image>@sha256:<digest>`.
- **Keep `docker:build` / `docker:publish` for trial builds.** `docker:publish` prints the pushed digest reference as its last line. It reads the digest from that push's own output, because a later read of the tag could return another push's digest.
- **Deploy the digest reference a release recorded** and pin every service to it. A tag can move under a running service. A rollback deploys the previous reference.

## Migrations

Schema changes are versioned SQL files an idempotent tool applies — forward-only: never edit an applied migration, add a new one. A `db:migrate` command applies them as an explicit, gated step, not on app boot; in prod it must succeed before the new version rolls out. **Expand, then contract**, across separate deploys: every migration must leave the currently-running version working, so a rename is add-backfill-switch-drop, never one step.

## Repo files

Baseline in every project: `.editorconfig`, `.gitignore`, and `.dockerignore`.

Shared project instructions live in `AGENTS.md`. `CLAUDE.md` imports it with `@AGENTS.md` and holds Claude-specific additions; keep it a separate file, not a symlink.

## Dependencies

Latest LTS where upstream offers one, pinned as a concrete range — never a floating tag, never a pre-release. **An override or an older pin is a decision you bring to the human**, never one you take; once taken it carries a one-line reason and is pruned when it lapses.
