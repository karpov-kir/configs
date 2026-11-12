**Layer:** craft

# How We Structure Code

## Repository: apps and packages

A repository with more than one part is a monorepo. Apps go under `apps/<name>/` and libraries under `packages/<name>/`.

- **An app is what gets built and released, and it has entrypoints.** One product on several runtimes is one app. A browser extension and a desktop app's windows are one such case. That app holds one set of entrypoints per runtime and one adapter per platform (`infra/chrome`, `infra/tauri`). A desktop app's native shell is an app of its own.
- **A package is a library.** It publishes its API through `exports` in its `package.json`: `"."`, plus `"./testing"` where it ships a test kit. Inside, it has no app layers. Its imports form no cycle.
- **Imports run from apps to packages, and between packages.** A package imports no app. An alias or a relative path into another app's `src` is forbidden. Code two apps share becomes a package, because only an `exports` map is a surface its owner keeps stable.

## App layers

Every TypeScript app takes the same layers, whatever it is: a backend, a frontend, a CLI, a browser extension.

```
src/
  entrypoints/    one folder per way into the app: its host and composition root
  orchestration/  one action built from 2+ features (a screen, a scenario, a command)
  features/       the app's capabilities, one module each
  entities/       business types and pure logic that several features share
  shared/         code with no business meaning (logger, pub-sub, utilities)
  infra/          generic adapters to the outside (database, HTTP API, storage, messaging)
```

A **module** is one folder directly under a layer, or one file directly under `shared`. Only `orchestration`, `features` and `entities` modules must be imported through their `exports.ts`. The layers import one way only, so a change to an inner layer is the only one that reaches outward:

| Layer | Imports |
|---|---|
| `entrypoints` | every layer |
| `orchestration` | `features`, `entities`, `shared` |
| `features` | `entities`, `shared` |
| `entities` | other `entities` without cycles, `shared` |
| `shared` | `shared` |
| `infra` | other `infra`, `entities`, `shared` |

- **A feature is whole.** Its port, repository, types and helpers live in its folder. Test placement is [testing.md](../testing.md) → **Commands and file names**.
- **A feature imports no other feature, and an orchestration module imports no other orchestration module.** Two features link through an orchestration module or an entrypoint. It passes one feature's callback to the other.
- **An orchestration module holds no business logic of its own.** It exists only where an action takes two or more features. A single feature is mounted or started by its entrypoint directly.
- **An entity depends on no UI or render framework.** Entities link to each other mostly by type and id. A rule that spans several entities belongs to a feature.
- **Only entrypoints import `infra`.** An infra adapter stays generic, such as a key-value store, a media store or a message transport. A feature declares its own port as the structural subset of the adapter it needs, and builds its repository over that port. Infra cannot see the feature, so the adapter cannot name the port it implements. The composition root passes the adapter in, and TypeScript checks there that it fits.
- **No `pages` or `widgets` layer.** A block without logic or a business type goes to `shared`. One that renders an entity lives in the feature that shows it. A block with its own logic is a feature, and a block that assembles features is an orchestration module.
- **In TypeScript, the linter holds these boundaries.** Add `slices.json` to `extends`, from the oxlint config package that [.oxlintrc.json](../../templates/.oxlintrc.json) already extends. It checks the layers and surfaces, and the package's `base.json` forbids cycles.
- **A Go backend is an app too, and one Go module.** Its entrypoints live in `cmd/`. Each other folder is a Go package, which plays a TypeScript module's part, and its exported identifiers are its surface. `internal/` holds the code the app shares but no other module may import. The compiler enforces `internal/` and forbids cycles. A shared Go test built on `golang.org/x/tools/go/packages` holds the layer direction. Go code moves to `packages/` only when several apps take it.

## Horizontal decoupling

Business logic must not depend on infrastructure — a database, a broker, an HTTP API, a UI framework, browser storage, the clock, the filesystem, … Dependencies point inward, never out.

**Functional core, imperative shell** — the layer that drives the ports hands values inward; the core computes and returns rather than awaiting I/O mid-decision.

### Ports and adapters

A port is an interface at a boundary to external technology, declared by the domain in its own terms. It can also be a **minimal structural subset** of a large third-party type — declare only the methods you use.

An adapter implements a port against one concrete technology. The core imports only the port; frameworks, SDKs, and platform APIs live in entrypoints and adapters, never in the core.

### Composition root

Each entrypoint's composition root is `compositionRoot.ts`, exporting `compose(overrides)`. It constructs the adapters and injects them into the features. Business and domain code never call `new` on an infrastructure class. The partial overrides default to the real adapters — the seam a test or another runtime swaps one dependency through. For a plain function, the equivalent seam is a dependency option defaulting to the real implementation.

- **Assembly is an explicit `compose()` call.** A side-effect import or a global service locator hides what was built from a test and a reader, so use neither.
- **Three ambient dependencies skip injection**, because a test has no reason to swap them. Code reaches them directly, and they are the only globals:
  - the logger (**Logging & events**);
  - analytics events — `shared` holds a generic `track`, `entities/analytics` holds the event catalogue that types it, and `compose()` binds `track` to its sender;
  - `shared/config`, read-only (the platform, the build's flags).

**Hand out domain handles — never a raw framework or vendor type.** The entrypoint calls `compose()` and returns the handle. Its shape follows the app: `start()` / `stop()` plus `handle(request)` for a service, `mount()` / `unmount()` for a UI mount, `run(args)` for a CLI or batch job.

### Entrypoints

Ports and adapters are the **outbound** edge — tech the domain drives; **entrypoints** are the **inbound** edge — what drives the domain, one folder under `entrypoints/` per way into the app. An entrypoint owns its **host** (lifecycle, plus the transport or mount it binds to), its composition root, and app-wide cross-cutting, then hands each inbound interaction inward. Only the logic behind it stays transport-agnostic.

- **An entrypoint file is named for what it starts** — `start.ts` or `mountPopup.tsx`, not `main.ts`, which names only the program's beginning.
- **Avoid a file a framework requires by name** (a WXT `index.ts`) where the framework lets you. Where it does not, the file holds the framework's own setup or one line calling the named file's `start…()` / `mount…()`.
- **No composition root composes composition roots.** To run several entrypoints in one process, a **cumulative entrypoint** named for the deployment (`startAll.ts`, `web.ts`, `worker.ts`) starts their handles together.

## Module depth

A module's **interface** is everything a caller must learn to use it — its exports, their types, and the constraints only prose carries. Its **depth** is the functionality it hides divided by that interface. Deep is the goal, and depth is not size — **splitting a file is free, exporting the halves is not.**

- **Publish one surface per module.** A module declares its boundary in `exports.ts`: the handle, port, or facade a caller holds. Everything else is internal and nothing outside imports it. Never name it `index.ts`, whose implicit resolution hides which file the caller is actually reading. This file name, like `compositionRoot.ts`, is TypeScript's. Another language takes its own equivalent.
- **A module that runs in several runtimes adds one surface per runtime**, `exports.<entrypoint folder>.ts` (`exports.background.ts`). The entrypoint of that name must import it instead of `exports.ts`, so one runtime's bundle never pulls in another's UI or framework. An orchestration module serving one runtime takes that runtime's surface too, which the linter does not check.
- **A `shared` module is one file, or a folder of several files with its own `exports.ts`.** A single file is its own surface.
- **Capability added inside lands behind the surface already published.**
- **No pass-throughs.** A method that mostly forwards to a same-named method one layer down costs interface and hides nothing.
- **No required sequences.** An interface the caller must drive in order — construct, then configure, then start — is shallow. Hide the order inside, or model the states so the wrong order can't be expressed.
- **The surface must stand alone.** Someone reading only the surface file can use the module correctly, with no other file open; what the types cannot carry, contract prose does.

**Where the shape of a surface is genuinely open, draft two or three before you settle on one.** Comparing them costs far less than migrating callers off the wrong one. Make them deliberately different: the smallest surface, the one built for the commonest caller, the one built around a port. Then choose on depth, on where change concentrates, and on where the seam falls. This is for a surface other code will hold, never for a shape one caller sees.

## Per-side specifics

An inbound interaction flows inward through the layers. Add a layer inside a feature only when the path earns it.

```
any app         : entrypoint → orchestration (2+ features) → feature (→ its ports) → entities
backend feature : controller → use case → feature logic and entities
```

**Backend.** Default to **CQRS** — reads and writes take separate paths. Tests reach the service through `handle(request)`, which delegates to the in-process dispatch a real request takes (`fastify.inject` / `hono.fetch`). A use case earns its place at 2+ boundaries or when it spans a unit of work (one transaction across several repositories or aggregates), and skip it below that. Put a read behind a port only where a fake adds value.

**Frontend.** Server cache — data mirrored from the API — goes to a data-fetching library (TanStack Query, a Solid resource) behind a port, never hand-rolled in components; UI state (open/closed, selection, draft) stays in component-local signals. A server semantic rejection (409/422) maps onto the offending field, not a generic toast.

## Validation

The inbound boundary checks **structure** — types, ranges, required fields — and rejects malformed input before deeper logic runs. A message from another runtime of the same app crosses an inbound boundary too. Inner layers check **semantics** — state-dependent rules no schema expresses (sufficient balance, the record exists).

Validate structure with a **declarative schema** — zod / valibot (TS), pydantic (Python) — never an unchecked cast or a hand-rolled `typeof`/`in` chain. One schema is the single authority: parse once into a typed value or a rejection, derive the static type from it, and delegate every format check to it.

## Logging & events

**Logging is ambient.** A logger is reached directly, not threaded through constructors: nothing branches on it and no test asserts it. One root logger, scoped per module so each line carries its source, and the logger filters by level. Config sets that level once. Backend: a logging library (pino / winston — TS; structlog — Python). Frontend: the ready-to-go scoped logger in [Logger.ts](Logger.ts).

**Domain events — injected like any port.** Notification goes through a typed pub-sub, fire-and-forget.

- **Dynamic 1:N fan-out** — one source emitting to a subscriber set that varies at runtime — *is* pub-sub: use one even inside a single module and even at the first site, rather than hand-rolling a listener `Set` plus notify loop. Without that fan-out, a fixed 1:1 call inside a module is simpler.
- **The owner hides it.** Hold it as a private `pubSub` field and expose one domain verb pair per channel: bound `onStateChanged`/`offStateChanged` fields typed `ChannelSubscriber<Payload>`. Consumers never import the pub-sub.
- A ready-to-go typed `PubSub<ChannelMap>` ships in [PubSub.ts](PubSub.ts); **vendor the part you need** into the project's `shared`, adapt the `Logger` seam, and head it with a provenance line — source repo plus commit. Cross-process delivery, durability, retries, or asynchronous handlers need a real broker.
