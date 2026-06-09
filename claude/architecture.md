# How We Structure Code

Examples below are language-agnostic pseudo-code. Patterns matter, not the tools.

**Applying this document.** Follow these when a project already does, or when starting a new one; otherwise match the project's existing architecture — consistency within a project wins.

---

## Enterprise

How the system is decomposed and wired, along two axes that compose: **vertical slicing** (per-feature cohesion) and **horizontal decoupling** (each slice swappable from its tech). The same ports and composition-root seam make it testable — the real graph with a fake at one edge, no mocks (see [testing.md](~/.claude/testing.md)).

### Vertical slicing

Organize the top level by feature, not by technical layer — a slice owns its API, business logic, and data access together (a feature change stays in one folder). Prefer this over a layer-first `controllers/`/`services/`/`repositories/` tree.

```
src/
  modules/
    user/            # one vertical slice: api + logic + data + its adapters for "user"
    billing/         # another slice
  shared/            # cross-slice primitives, no feature logic
  entrypoints/       # transport host + composition roots
  infra/             # global / cross-slice infra only — db pool, unit of work
```

- A slice may use `shared`, never another slice's internals; cross-slice needs go through a published port or an event.
- Keep a slice whole — its adapters, tests, types, and helpers live with it.

### Horizontal decoupling

Business logic must not depend on infrastructure (e.g. database, web framework, broker, third-party API, clock, filesystem) — dependencies point inward, never out.

#### Ports

A port is an interface at a boundary to external technology, declared by the domain in its own terms.

```
interface UserRepository:                 // declared by the domain
    save(user) -> Promise<User>
    findByEmail(email) -> Promise<User | undefined>
```

A port can also be a **minimal structural subset** of a large third-party type — declare only the methods you use:

```
type MinimalFileHandle = pick(FileHandle, [read, write, close, truncate, stat])
```

#### Adapters

An adapter implements a port against one concrete technology. The core (use case + domain) imports only the port; concrete tech sits at the edge that faces it — web frameworks in controllers and entrypoints, database clients and vendor SDKs in adapters and `infra`.

```
class SqlUserRepository implements UserRepository:    // production adapter
    save(user): ...orm insert...
```

#### Composition root

A composition root constructs adapters and injects them into the domain; business and domain code never call `new` on an infrastructure class.

```
class CompositionRoot(overrides = {}):
    // partial overrides default to the real adapters — the seam a test
    // (or another runtime) swaps one dependency through
    userRepository = overrides.userRepository ?? new SqlUserRepository()
    emailService   = overrides.emailService   ?? new SmtpEmailService()
    userService    = new UserService(userRepository, emailService)
    webServer      = new WebServer(userService)    // the handle callers use
```

**Hand out domain handles — never a raw framework or vendor type.** The handle's shape follows the app — for example:

- web service → a handle with `start()` / `stop()`, plus `handle(request)` for in-process tests
- CLI / batch job → a handle with `run(args)`
- library → the use-case API (or a domain facade), handed out directly

A leaked `FastifyInstance`/`Hono` (or `PrismaClient`) couples callers back to the technology the ports hide. Returning the raw app *only* so tests can call `app.request(...)` is the trap — `handle(request)` delegates to the same in-process dispatch (`fastify.inject` / `hono.fetch`) without exposing the framework type.

#### Entrypoints

Ports/adapters are the **outbound** edge — tech the domain drives. **Entrypoints** are the **inbound** edge — what drives the domain: an HTTP server, a message consumer, a CLI, a cron job. One per way into the app.

An entrypoint owns the transport host — framework instance, server lifecycle, mounting the slices' routes, app-wide cross-cutting (e.g. a global error→status handler) — then hands each request to a slice's controller, its HTTP boundary (see Layering); only the use case behind it is transport-agnostic.

Match structure to need: an entrypoint starts as one file — host plus inline wiring — and earns a folder when it needs more than one. A split-out composition root takes the entrypoint's name (`WebServerCompositionRoot`); a single entrypoint's is just `CompositionRoot`.

No composition root composes composition roots. To run several in one process, a thin startup file imports the entrypoints and starts them.

```
entrypoints/
  main.ts              # startup
  syncTickers.ts       # simple entrypoint
  webServer/           # grown entrypoint
    WebServer
    WebServerCompositionRoot
```

```
// main.ts
const db = newDbClient()    // shared infra

const webServer   = new WebServerCompositionRoot({ db }).webServer
const syncTickers = newSyncTickers({ db })

await all(webServer.start(), syncTickers.start())
```

#### Injection seams

- **Constructor injection** — a class receives its dependencies, never constructs them (above).
- **Function option with a real default** — give a plain function a dependency option defaulting to the real implementation:

```
function append(path, data, options = {}):
    openFile = options.openFile ?? realOpenFile    // production default
    ...operate through the injected handle...
```

**Functional core, imperative shell** — a judgment call, worth more as the logic grows heavier or more critical, not for thin CRUD. Decoupling taken further: keep business logic a pure **core** (decisions from inputs — no I/O, clock, or mutation) wrapped by a thin **shell** that gathers inputs, calls the core, and enacts the result; a pure core tests trivially: data in, data out, nothing to fake.

### Layering & request flow

Within a slice, a request flows inward through layers; **match depth to complexity** — add a layer only when the path earns it. Default to **CQRS** — reads and writes take separate paths.

```
composition root wires the graph; a request enters at one boundary
  controller   // boundary ↔ use case: validate input, map result out — no business logic
    use case   // application service: orchestrate ONE business operation
      domain   // business rules and decisions; reaches infra only through ports
```

Keep the boundary thin: reusable protocol mechanics — serialization, streaming/SSE framing, error→status mapping — live in a transport helper or the entrypoint's cross-cutting, not piled into each controller; interleaving request flow with low-level writes mixes abstraction levels (CLAUDE.md → Abstraction).

**Validation splits across the layers.** The boundary checks *structure* — types, ranges, required fields — and rejects malformed input before business logic runs. The domain checks *semantics* — state-dependent rules no schema can express (sufficient balance, the SKU exists). At the edge, follow the robustness principle: liberal in what you accept (validate only the fields you use), conservative in what you emit.

A use case earns its place when logic is worth pulling out of the controller into a transport-agnostic, testable unit — for instance more than one boundary calls it (an HTTP controller and a CLI command share the *same* one), or it wraps writes across several repositories/aggregates in a **unit of work** (one atomic commit). Otherwise, skip it.

Sample paths, shallow to deep (not fixed tiers):

- query / read → `controller → query` — no rules to apply, even a near-raw query
- thin command → `controller → domain` — nothing to orchestrate
- command worth a use case → `controller → use case → domain`
- business-logic-heavy → a DDD domain: `use case → aggregate → entities → domain events`
- highest complexity → event-driven: domain events on a message bus, use cases as command/event handlers

Put a read behind a port only where a fake adds value — faking a raw store query just echoes a canned value, so verify it with an integration or e2e test instead.

DDD and event-driven patterns carry real cost — adopt them **only when the complexity justifies it**.

### Decision guides

#### Where does this code go?

- Feature-specific behaviour → that feature's slice.
- Used by 2+ slices, no infrastructure → `shared`.
- Talks to external technology → a port (declared by the domain) + an adapter, in the slice that uses it.
- Global plumbing, or an adapter shared across slices → `infra`.
- Inbound transport host + composition roots → `entrypoints`.

#### Tempted to couple to infrastructure? Do this instead

| Temptation | Instead |
|---|---|
| Domain imports the ORM / client / framework | Declare a port in the domain; implement it in an adapter |
| `new SomeAdapter()` inside a use case | Inject via the constructor; the composition root constructs it |
| One slice imports another slice's internals | Depend on a published port, or emit an event |
| Global singleton / service locator | Pass the dependency explicitly from the composition root |
| Composition root returns a `FastifyInstance` / `Hono` / `PrismaClient` | Hand out a domain handle (e.g. a `WebServer` with `start`/`stop`) |

