# <app>

<short description>

## Development

- Ensure Docker is running [if it has Docker dependencies]
- Install dependencies: `go mod tidy` / `npm ci` / `bun install`
- `task install:tools` [if it vendors CLI tools]
- For a local TLS domain [if applicable]:
  - Install [mkcert](https://github.com/FiloSottile/mkcert), then `task install:dev-certs`
  - Add `127.0.0.1 api.<app>.dev` to `/etc/hosts`
- Set `$DEV_INFRA` to your shared dev-infra checkout — one Traefik (`:443`) and one Postgres for every local service [if applicable]
- `task start:dev` (or `npm run start:dev`) — runs the service with hot reload, bringing up the shared dev-infra first [if it has Docker dependencies]
