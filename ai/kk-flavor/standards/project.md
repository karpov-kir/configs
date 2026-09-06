# How We Set Up a Project

Follow this on a new project or one that already does; otherwise match the project's existing setup.

Starter configs: [`templates/`](../templates/) — match them closely; diverge only on what no template covers.

## Environments

A service's own folders — `env/`, `docker/` — live beside its `src/`, inside the package in a monorepo (`packages/<service>/`), never the repo root. `env/` holds one file per environment (`.env.<env>`) — real, working values, never `.env.example` placeholders.

- **Select by environment.** A build tool with mode support (Vite, wxt) reads `.env.<env>` itself; otherwise a small loader reads `.env.$ENV`, defaulting to `development`. Config reads that same selector (`ENV`, or the build tool's mode), never a second variable like `NODE_ENV`. Loading is the app's job (build tool or loader), never the task runner's — a go-task `dotenv:` bypasses the encrypt-at-rest flow.

- **Commit real env; encrypt only secrets.** Per file, by whether it holds a secret:
  - **No secrets** → committed plain and baked into the image (**Local dev & Docker**, below).
  - **Has secrets** → committed only as `.env.<env>.encrypted` (plaintext gitignored), never in the image; decrypted and injected at deploy.

  Encrypt/decrypt **per environment** (`env:encrypt:<env>`/`env:decrypt:<env>`), each prompting for that environment's own password (never stored).

## Config

The app reads its config from one typed object, never raw `os.Getenv` / `import.meta.env`. Build it at startup as a singleton.

- Coerce to real types: numbers, booleans, the environment as an **enum** — not bare strings.
- Define defaults inline; env vars only override them.
- Log it with secrets masked.

## Logging

- **Format and level are explicit, never derived from the environment** — so any environment (production included) runs locally with readable logs. `LOG_FORMAT` is `pretty` or `json`; `LOG_LEVEL` is one of the levels in [code-style.md](code-style.md) → **Logging**.

## Scripts

Same names on every stack. Prefer the built-in runner (npm/bun scripts); reach for a **Taskfile** only with no runner (e.g. Go) or a task the runner can't express:

| Script | Does |
|---|---|
| `start` / `start:dev` | the built long-running service (fails if not built) / the dev loop — compile + watch |
| `exec` / `exec:dev` | the built one-shot tool (a CLI) / the same from source |
| `build` / `build:<env>` | one env-agnostic build; `build:<env>` only where the artifact can't take env at run time (a chrome extension) |
| `test` / `test:<level>` | the test suites — see [testing.md](testing.md) |
| `lint` / `lint:fix` | every linter / every linter's fix |
| `lint:<type>` / `lint:<type>:fix` | one linter — `lint:eslint`, … |

`<env>` is `development`, `staging`, or `production`. Drop any script that doesn't apply.

Group related operations under a shared first segment (`docker:build`/`docker:publish`). Where a project has both a runner and a Taskfile, the runner is the entry point — it wraps Taskfile tasks (`"start:dev": "task start:dev"`) and keeps short commands inline.

In a monorepo, the root re-exposes each package's commands as `<package>:<command>` (`backend:start:dev`, …); repo-wide aggregates (`test`, `lint`) stay bare. Each package owns its `Taskfile.yml` / `package.json`; the root keeps only the workspace config and these commands.

## Local dev & Docker

The service runs on the host; its dependencies and the HTTP edge run once in a single shared **dev-infra** stack (point `$DEV_INFRA` at it).

- **One Traefik** owns `:443` + TLS and fronts every service; each service contributes a route + cert.
- **One Postgres** — every service connects to its own database at `localhost:5432/<service>` (credentials in `$DEV_INFRA/docker-compose.yml`, the shared dev superuser); never a per-service container.

A service's `docker/` holds its own container setup — its Traefik route slice, its `dev-certs/`, its `Dockerfile`.

- **Add `127.0.0.1 api.<app>.dev` to `/etc/hosts`** so the local TLS domain resolves; the certs `install:dev-certs` mints are per-machine, gitignored.
- **Give the service its own distinct host port** — not the template default, and not one already listening on this machine.
- **Service-specific containers** go in the service's own `docker/docker-compose.yml` (brought up by its `start:dev`); common deps stay in the dev-infra.
- **Env in the image follows the secret rule above** — the non-secret files `COPY`-ed, staging and production, not dev. `ENV` itself is always provided at deploy, never baked.

## Migrations

Schema changes are versioned SQL files an idempotent tool applies — forward-only: never edit an applied migration, add a new one. A `db:migrate` command applies them as an explicit, gated step, not on app boot; in prod it must succeed before the new version rolls out. **Expand, then contract**, across separate deploys: every migration must leave the currently-running version working, so a rename is add-backfill-switch-drop, never one step.

## Repo files

Baseline in every project: `.editorconfig`, `.gitignore`, and `.dockerignore`.

## Dependencies

Latest LTS where upstream offers one, pinned as a concrete range — never a floating tag, never a pre-release. An override or an older pin carries a one-line reason and is pruned when it lapses.
