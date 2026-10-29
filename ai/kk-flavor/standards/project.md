**Layer:** craft

# How We Set Up a Project

Starter configs: [`templates/`](../templates/) — match them closely; diverge only on what no template covers.

## Environments

A service's own folders — `env/`, `docker/` — live beside its `src/`, inside the package in a monorepo (`packages/<service>/`), never the repo root. `env/` holds one file per environment (`.env.<env>`) — real, working values, never `.env.example` placeholders.

- **Select by environment.** A build tool with mode support (Vite, wxt) reads `.env.<env>` itself; otherwise a small loader reads `.env.$ENV`, defaulting to `development`. Config reads that same selector (`ENV`, or the build tool's mode), never a second variable like `NODE_ENV`. Loading is the app's job (build tool or loader), never the task runner's — a mise `[env] _.file` bypasses the encrypt-at-rest flow.

- **Commit real env; encrypt only secrets.** Per file, by whether it holds a secret:
  - **No secrets** → committed plain and baked into the image (**Local dev & Docker**, below).
  - **Has secrets** → committed only as `.env.<env>.encrypted` (plaintext gitignored), never in the image; decrypted and injected at deploy.

  Encrypt/decrypt **per environment** (`env:encrypt:<env>`/`env:decrypt:<env>`), each asking for that environment's own password (never stored).

- **Ask every password in a macOS dialog with hidden input** — env encryption, sudo on a server, and any other password a task needs. The dialog text names the password and what happens once it is given. Pipe the answer into the consuming command's stdin (`openssl -pass stdin`, `sudo -S`). Keep a password out of terminal prompts, command arguments and exported variables. Copy the `ask` helper from [`templates/mise.toml`](../templates/mise.toml) for the dialog. A script that runs on a server reads its passwords from stdin, and the workstation task that starts it asks for them and pipes them over ssh.

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

`<env>` is `development`, `staging`, or `production`. Drop any script that doesn't apply.

Group related operations under a shared first segment (`docker:build`/`docker:publish`). A package runner's scripts (npm/bun) stay in its `package.json`, and a mise task that only runs one names it (`run = "npm run build"`, `dir = "web"`).

In a monorepo, the root re-exposes each package's commands as `<package>:<command>` (`backend:start:dev`, …); repo-wide aggregates (`test`, `lint`) stay bare. The repo keeps one `mise.toml`, at the root, and each package keeps its own `package.json`.

## Local dev & Docker

The service runs on the host; its dependencies and the HTTP edge run once in a single shared **dev-infra** stack (point `$DEV_INFRA` at it).

- **One Traefik** owns `:443` + TLS and fronts every service; each service contributes a route + cert.
- **One Postgres** — every service connects to its own database at `localhost:5432/<service>` (credentials in `$DEV_INFRA/docker-compose.yml`, the shared dev superuser); never a per-service container.

A service's `docker/` holds its own container setup — its Traefik route slice, its `dev-certs/`, its `Dockerfile`.

- **Add `127.0.0.1 api.<app>.dev` to `/etc/hosts`** so the local TLS domain resolves; the certs `install:dev-certs` mints are per-machine, gitignored.
- **Give the service its own distinct host port** — not the template default, and not one already listening on this machine.
- **Service-specific containers** go in the service's own `docker/docker-compose.yml` (brought up by its `start:dev`); common deps stay in the dev-infra.
- **Only the non-secret env files are `COPY`-ed into the image** — staging and production, not dev. `ENV` itself is always provided at deploy, never baked.
- **Run a Go or Node build stage on `--platform=$BUILDPLATFORM`** and have it build for `$TARGETPLATFORM` ([`templates/docker/Dockerfile`](../templates/docker/Dockerfile)).

## CI, images & deploys

- **Pick the CI runner by repository visibility.** Run a public repository's CI on hosted runners only. A pull request from a fork runs its author's code on the runner. Keep self-hosted runners for private repositories.
- **CI builds and pushes every app image after each commit to the default branch that passes its checks**, through the shared release-image workflow. Its Release records each image's digest reference, `<registry>/<image>@sha256:<digest>`.
- **Keep `docker:build` / `docker:publish` for trial builds.** `docker:publish` prints the pushed digest reference as its last line, taken from that push's own output: a later read of the tag could return another push's digest.
- **Deploy only through the signed deploy a human approves**, passing the digest references from the Release. Pin every service to a digest reference. No CI job and no app-repo task rolls a service.

## Migrations

Schema changes are versioned SQL files an idempotent tool applies — forward-only: never edit an applied migration, add a new one. A `db:migrate` command applies them as an explicit, gated step, not on app boot; in prod it must succeed before the new version rolls out. **Expand, then contract**, across separate deploys: every migration must leave the currently-running version working, so a rename is add-backfill-switch-drop, never one step.

## Repo files

Baseline in every project: `.editorconfig`, `.gitignore`, and `.dockerignore`.

Shared project instructions live in `AGENTS.md`. `CLAUDE.md` imports it with `@AGENTS.md` and holds Claude-specific additions; keep it a separate file, not a symlink.

## Dependencies

Latest LTS where upstream offers one, pinned as a concrete range — never a floating tag, never a pre-release. **An override or an older pin is a decision you bring to the human**, never one you take; once taken it carries a one-line reason and is pruned when it lapses.
