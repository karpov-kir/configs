# How We Set Up a Project

**Applying this document:** follow it on a new project or one that already does; otherwise match the project's existing setup.

Starter configs: [`templates/`](~/.claude/templates/) — match them closely: drop what a project doesn't need, don't add what a template deliberately omits (a Taskfile's `desc`/`deps`, etc.), and diverge only on a command no template covers. Their comments note file-specific intent, not these guidelines.

## Environments

A service's own folders — `env/`, `docker/` — live beside its `src/`, inside the package in a monorepo (`packages/<service>/`), never the repo root. `env/` holds one file per environment: `.env.development`, `.env.staging`, `.env.production` — real, working values, never `.env.example` placeholders.

- **Select by environment.** A build tool with mode support (Vite, wxt) reads `.env.<environment>` itself — point its env dir at `env/` and pass the env flag (Vite's `--mode <environment>`). Otherwise a small loader reads `.env.$ENV` — only when `ENV` is set and the file exists, no fallback: locally the dev runner sets `ENV`; a deploy has no such file, so the loader does nothing. Whatever names the environment — `ENV` for a loader, the build tool's mode otherwise — the app's config reads that same one, never a second variable like `NODE_ENV`. Loading is the app's job (build tool or loader), never the task runner's — a go-task `dotenv:` bypasses both the loader and the encrypt-at-rest flow.

- **Commit real env; encrypt secrets.** A file with no secrets is committed as-is; one with secrets is committed only as `.env.<env>.encrypted` (plaintext gitignored) — that encrypted file is the recoverable reference and must be committed. Encrypt/decrypt **per environment** (`env:encrypt:<env>`/`env:decrypt:<env>`), each prompting for that environment's own password (never stored) — so locally you hold and decrypt only `.env.development`. At deploy the orchestration provides the rest: an encrypted file decrypted and injected as a secret, a plain file's values set directly (e.g. the compose `environment:`) — never stored decrypted off the dev machine.

## Config

The app reads its config from one typed object, never raw `os.Getenv` / `import.meta.env`. Build it at startup as a singleton (a `Config` class in TS, a package struct in Go).

- Coerce to real types: numbers, booleans, the environment as an **enum** — not bare strings.
- Define defaults inline; env vars only override them.
- Log it with secrets masked.

## Scripts

Same names on every stack. Prefer the built-in runner (npm/bun scripts); reach for a **Taskfile** only with no runner (e.g. Go) or a task the runner can't express — a `dir:`-scoped or shared parameterized one (the per-env `env:encrypt:<env>` helper):

| Script | Does |
|---|---|
| `start` / `start:dev` | the built long-running service (fails if not built) / the dev loop — compile + watch |
| `exec` / `exec:dev` | the built one-shot tool, e.g. a CLI / the same from source via tsx |
| `build` / `build:<env>` | prefer one env-agnostic build; bake per environment with `build:<env>` only where the artifact can't take env at run time — e.g. a chrome extension |
| `test` / `test:<level>` | the test suites — see [testing.md](~/.claude/testing.md) |
| `lint` / `lint:fix` | every linter / every linter's fix |
| `lint:<type>` / `lint:<type>:fix` | one linter — `lint:eslint`, `lint:stylelint`, … |

`<env>` is `development`, `staging`, or `production`. Drop any script that doesn't apply (a library has no `start`); a script the table doesn't name — a `typecheck`, or a named one-off like a data backfill — is a fine project-documented extra.

Group related operations under a shared first segment: `env:encrypt`/`env:decrypt`, `docker:build`/`docker:publish`, `db:migrate`. Where a project has both a runner and a Taskfile, the runner is the entry point — it wraps Taskfile tasks (`"start:dev": "task start:dev"`) and keeps short commands inline.

In a monorepo, the root re-exposes each package's commands as `<package>:<command>` (`backend:start:dev`, `cli:exec`, `backend:db:migrate`, …); repo-wide aggregates (`test`, `lint`, an all-packages `build`) stay bare. Each package owns its `Taskfile.yml` / `package.json` (and its `env/`, `docker/`); the root keeps only the workspace config and these commands.

## Local dev & Docker

The service runs on the host; its dependencies and the HTTP edge run once in a single shared **dev-infra** stack (point `$DEV_INFRA` at it). A service with no backing store — an SPA — just starts its dev server against a backend already up.

- **One Traefik** owns `:443` + TLS and fronts every service — only one process can bind it; each service contributes a route + cert.
- **One Postgres** — every service connects to its own database at `localhost:5432/<service>` (credentials in `$DEV_INFRA/docker-compose.yml`, the shared dev superuser): table-level isolation, no per-service container, no port collision, and one DB address identical on every OS and runtime.

A service's `docker/` holds its container setup:

```
docker/
  traefik-routes.yml   # this service's slice of the shared Traefik
  dev-certs/           # mkcert certs
  Dockerfile           # production image (if containerised)
```

- **`start:dev`** (a Taskfile task) brings the shared dev-infra up idempotently with this service's route + cert registered, ensures its database exists and is migrated, then runs the service with watch.
- **The route slice** maps `https://api.<app>.dev` → `host.docker.internal:<port>`; prefix its router + middleware names with the app (`<app>-cors`) so they don't clash in the shared Traefik.
- **Certs** — `install:dev-certs` mints them into `dev-certs/`; local per-machine artifacts, gitignored.
- **Give the service its own distinct host port** — not the template default — so co-running host processes don't collide; ask the user which.
- **Service-specific containers** go in the service's own `docker/docker-compose.yml` (brought up by its `start:dev`); common deps stay in the dev-infra.
- **Env stays out of the image.** Never `COPY` env files in, and don't bake the `ENV` selector — one image runs any environment; env and `ENV` are provided at deploy. The `Dockerfile`'s `CMD` runs `start` (or the artifact).

## Migrations

Schema changes are versioned SQL files an idempotent tool applies — forward-only (to change something add a new migration; never edit an applied one). A `db:migrate` command applies them as an explicit, gated step, not on app boot: in prod it must succeed before the new version rolls out (so a bad migration stops the deploy, not every replica's startup); `start:dev` runs it locally. Keep each migration backward-compatible with the running version (expand → switch the code → contract in a later one) so rolling deploys stay zero-downtime.

## Repo files

Baseline in every project: `.editorconfig`, `.gitignore`, and `.dockerignore` — some have starters in [`templates/`](~/.claude/templates/).
