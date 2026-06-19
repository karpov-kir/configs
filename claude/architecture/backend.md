# Backend Architecture

Backend / services flavor of [core.md](~/.claude/architecture/core.md) — read it first for the shared core. This file is the backend's flavor of each shared section; it restates no principle and never references its sibling doc.

## Folder layout

```
src/
  features/        # one vertical slice: api + logic + data + its adapters
    user/
    billing/
  shared/          # cross-slice primitives, no feature logic
  entrypoints/     # transport hosts + composition roots
  infra/           # global / cross-slice plumbing — db pool, unit of work
```

## Entrypoints — transport hosts

A backend entrypoint is a transport host: an HTTP server, a message consumer, a CLI, a cron job. It owns the framework instance, server lifecycle, route mounting, and app-wide cross-cutting (e.g. a global error→status handler), then hands each request to a slice's controller.

Every entry is runnable: it builds its graph through a composition root and exposes `start()`. No `main.ts`. To run several in one process, add a **cumulative entrypoint** named for the deployment (`startAll.ts`, `web.ts`, `worker.ts`).

```
entrypoints/
  syncTickers.ts            # single-file entrypoint — builds (newSyncTickers) and starts
  webServer/                # folder entrypoint — scoped logic lives inside
    server.ts               #   the entry: builds via the composition root, then starts
    WebServerCompositionRoot
    routes/
  startAll.ts               # cumulative — several in one process
```

```
// entrypoints/startAll.ts — web + workers in one process, sharing infra
const db = newDbClient()

await all(
    new WebServerCompositionRoot({ db }).webServer.start(),
    newSyncTickers({ db }).start(),
)
```

Returning the raw app *only* so tests can call `app.request(...)` is the trap — `handle(request)` delegates to the same in-process dispatch (`fastify.inject` / `hono.fetch`), so callers never touch the `FastifyInstance` / `Hono` or `PrismaClient`.

## Layering

The inward chain is `controller → use case → domain`. Default to **CQRS** — reads and writes take separate paths.

```
controller   // boundary: structure in, result out — no business logic
  use case   // orchestrate ONE business operation
    domain   // business rules; reaches infra only through ports
```

Keep the boundary thin: protocol mechanics — serialization, streaming/SSE framing, error→status mapping — live in a transport helper or the entrypoint's cross-cutting, not each controller.

A use case earns its place when logic leaves the controller for a transport-agnostic, testable unit: 2+ boundaries call it (an HTTP controller and a CLI command share the *same* one), or it spans a unit of work (see State). Otherwise skip it.

Sample paths, shallow to deep:

- read → `controller → query` — no rules, even a near-raw query
- thin command → `controller → domain` — nothing to orchestrate
- command worth a use case → `controller → use case → domain`
- business-logic-heavy → DDD: `use case → aggregate → entities → domain events`
- highest complexity → event-driven: domain events on a bus, use cases as handlers

Put a read behind a port only where a fake adds value — faking a raw query just echoes a canned value; verify with an integration or e2e test instead.

## Validation

Malformed request → reject at the controller as a client error (HTTP 400). Well-formed but naming nothing real → the domain's not-found (404) or conflict (409). Schema library: zod / valibot (TS), pydantic (Python).

## External tech

Edges the domain drives — e.g. a database, a message broker, a cache, a vendor SDK, the filesystem, the clock — each behind a port, implemented by an adapter in the slice (or `infra` when shared). The domain names the port (`UserRepository`); the adapter binds the tech (`SqlUserRepository`).

## State

Handlers hold no state between requests — it lives in the store, reached only through ports. A write spanning several repositories or aggregates commits atomically in a **unit of work** (one transaction); the use case owns that boundary.

## Tempted to couple? Do this instead

| Temptation | Instead |
|---|---|
| ORM entity returned straight from the controller | Map to a response shape at the boundary |
| Business logic in the controller | Push to a use case / domain |
| Per-request data stashed in a global / singleton | Pass it down the call chain |
