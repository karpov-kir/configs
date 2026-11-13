**Layer:** craft

# How We Structure Code

## Repository: apps and packages

A repository with more than one part is a monorepo. Apps go under `apps/<name>/` and libraries under `packages/<name>/`.

- **An app is what gets built and released, and it has entrypoints.** One product on several runtimes is one app. A browser extension and a desktop app's windows are one such case. That app holds one set of entrypoints per runtime and one adapter per platform (`infra/chrome`, `infra/tauri`). A desktop app's native shell is an app of its own.
- **A package is a library.** It publishes one surface, plus a separate testing entry where it ships a test kit. Inside, it has no app layers. Its imports form no cycle.
- **Imports run from apps to packages, and between packages.** A package imports no app. An import into another app's source is forbidden. Code two apps share becomes a package, because only a published surface is one its owner keeps stable.

## App layers

Every app takes the same layers, whatever it is and whatever language it is written in: a backend, a frontend, a CLI, a browser extension.

```
entrypoints/    one folder per way into the app: its host and composition root
orchestration/  one action built from 2+ features (a screen, a scenario, a command)
features/       the app's capabilities, one module each
entities/       business types and pure logic that several features share
shared/         code with no business meaning (logger, pub-sub, utilities)
infra/          generic adapters to the outside (database, HTTP API, storage, messaging)
```

A **module** is one folder directly under a layer, or one file directly under `shared`. Only `orchestration`, `features` and `entities` modules must be imported through their surface. The layers import one way only, so a change to an inner layer is the only one that reaches outward:

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
- **Only entrypoints import `infra`.** An infra adapter knows no feature, so it stays generic: a key-value store, a media store, a message transport. The app's storage adapter (`infra/db`) holds the schema migrations and the row models. No ORM or driver type appears in its operations, transactions and joined read queries included. A feature declares its own port as the structural subset of the adapter it needs, and builds its repository over that port. Infra cannot see the feature, so the adapter cannot name the port it implements. The composition root passes the adapter in, and the compiler checks there that it fits.
- **No `pages` or `widgets` layer.** A block without logic or a business type goes to `shared`. One that renders an entity lives in the feature that shows it. A block with its own logic is a feature, and a block that assembles features is an orchestration module.
- **A guard holds the direction in every language**, and CI fails on a cycle. **By language** names both.

## By language

The rules above and below name no language's layout. This table is where each language spells it:

| | TypeScript | Go |
|---|---|---|
| Layers | under `src/` | packages at the module root, `entrypoints/` included |
| Entrypoints | `src/entrypoints/<name>/` | `entrypoints/<name>/`, an importable package, a cumulative one included. Each has a `cmd/<name>/`, the `package main` the Go tool builds, whose `func main` is one line calling it. `cmd/` stands outside the layers and imports only `entrypoints/`. |
| Module surface | `exports.ts` | the package's exported names |
| Surface per runtime | `exports.<entrypoint>.ts` | none: each runtime is its own binary |
| Internal | what `exports.ts` leaves out | unexported names, and `internal/` for packages only their parent tree imports |
| Library surface | `exports` in `package.json`, entry `"."` | the module's exported packages |
| Test kit | `"./testing"` in the same `exports` | a `<package>test` package, as `httptest` is |
| Composition root | `compositionRoot.ts`, `compose(overrides)` | `compose.go`, `Compose(overrides)`, in `entrypoints/<name>/` |
| Tests and their tooling | unit tests beside the code, the rest in `tests/` beside `src/` ([testing.md](../testing.md)) | `_test.go` beside the code |
| Direction guard | `layers.json`, which the [.oxlintrc.json](../../templates/.oxlintrc.json) template turns on | the `layers` analyzer of go-lint, KK Forge's golangci-lint build, which also holds `cmd/` to `entrypoints/`, each `func main` to one call, and an HTTP server to `entrypoints/` |
| Implicit names | `index.*`, forbidden by a rule in `typescript.json`, libraries included. A WXT extension extends `wxt.json`, where each entrypoint is a folder `entrypoints/<name>/` and its `index.ts` is the launcher. The `launcher` rule holds that file to one call. A script directly in `entrypoints/` fails. The linter reads no HTML, so review holds a WXT `index.html` to markup and the script tag that loads `./index.ts`, and rejects a flat `entrypoints/<name>.html`. | none. The launcher is `func main` in `cmd/<name>/`. |
| Cycles | `import/no-cycle` in `typescript.json`, which the same template extends | the compiler |

## Horizontal decoupling

Business logic must not depend on infrastructure — a database, a broker, an HTTP API, a UI framework, browser storage, the clock, the filesystem, … Dependencies point inward, never out.

**Functional core, imperative shell** — the layer that drives the ports hands values inward; the core computes and returns rather than awaiting I/O mid-decision.

### Ports and adapters

A port is an interface at a boundary to external technology, declared by the domain in its own terms. It can also be a **minimal structural subset** of a large third-party type — declare only the methods you use.

An adapter implements a port against one concrete technology. The core imports only the port; frameworks, SDKs, and platform APIs live in entrypoints and adapters, never in the core.

### Composition root

Each entrypoint has one composition root: an assembly function that takes partial overrides and returns the handle. The overrides default to the real adapters — the seam a test or another runtime swaps one dependency through. It constructs the adapters and injects them into the features. Business and domain code never construct an infrastructure type. For a plain function, the equivalent seam is a dependency option defaulting to the real implementation.

- **Assembly is an explicit call to that function.** A side-effect import or a global service locator hides what was built from a test and a reader, so use neither.
- **Three ambient dependencies skip injection**, because a test has no reason to swap them. Code reaches them directly, and they are the only globals:
  - the logger (**Logging & events**);
  - analytics events — `shared` holds a generic track function, `entities/analytics` holds the event catalogue that types it, and the composition root binds the function to its sender;
  - `shared/config`, read-only (the platform, the build's flags).

**Hand out domain handles — never a raw framework or vendor type.** The entrypoint calls the composition root and returns the handle. Its shape follows the app: `start()` / `stop()` plus `handle(request)` for a service, `mount()` / `unmount()` for a UI mount, `run(args)` for a CLI or batch job.

### Entrypoints

Ports and adapters are the **outbound** edge — tech the domain drives. **Entrypoints** are the **inbound** edge — what drives the domain, one entrypoint per way into the app (**By language** → Entrypoints). An entrypoint owns its **host** (lifecycle, plus the transport or mount it binds to), its composition root, and app-wide cross-cutting, then hands each inbound interaction inward. Only the logic behind it stays transport-agnostic.

- **An entrypoint file is named for what it starts** — `start` or `mountPopup`, not `main`, which names only the program's beginning. A launcher, below, keeps the name its language or framework requires.
- **No file takes a name the language or a tool resolves implicitly when its folder is imported or served.** An implicit name hides which file the caller is reading.
- **Where a language or framework requires such a name, the guard's configuration allows that file.** A preset for that framework allows it where one exists. Where none does, the project lists the file by path, with a reason for each path. Without a reason, the guard stays on.
- **A launcher is the file or function a language or framework requires by name.** It holds only that required definition and one call into its entrypoint.
- **No composition root composes composition roots.** To run several entrypoints in one process, a **cumulative entrypoint** named for the deployment (`startAll`, `web`, `worker`) starts their handles together. It has no composition root of its own.

## Module depth

A module's **interface** is everything a caller must learn to use it — its exports, their types, and the constraints only prose carries. Its **depth** is the functionality it hides divided by that interface. Deep is the goal, and depth is not size — **splitting a file is free, exporting the halves is not.**

- **Publish one surface per module.** A module declares its boundary in one place: the handle, port, or facade a caller holds. Everything else is internal and nothing outside imports it. Never give it a name the tooling resolves implicitly (**Entrypoints**).
- **A module that runs in several runtimes adds one surface per runtime.** Each takes the name of an entrypoint folder. The entrypoint of that name imports it instead of the general surface, so one runtime's bundle never pulls in another's UI or framework. An orchestration module serving one runtime takes that runtime's surface too, which the direction guard does not check.
- **A `shared` module is one file, or a folder of several files with its own surface.** A single file is its own surface.
- **Capability added inside lands behind the surface already published.**
- **No pass-throughs.** A method that mostly forwards to a same-named method one layer down costs interface and hides nothing.
- **No required sequences.** An interface the caller must drive in order — construct, then configure, then start — is shallow. Hide the order inside, or model the states so the wrong order can't be expressed.
- **The surface must stand alone.** Someone reading only the surface can use the module correctly, with no other file open; what the types cannot carry, contract prose does.

**Where the shape of a surface is genuinely open, draft two or three before you settle on one.** Comparing them costs far less than migrating callers off the wrong one. Make them deliberately different: the smallest surface, the one built for the commonest caller, the one built around a port. Then choose on depth, on where change concentrates, and on where the seam falls. This is for a surface other code will hold, never for a shape one caller sees.

## Per-side specifics

An inbound interaction flows inward through the layers. Add a layer inside a feature only when the path earns it.

```
any app         : entrypoint → orchestration (2+ features) → feature (→ its ports) → entities
backend         : entrypoint's HTTP controller → orchestration (2+ features) → feature's use case (where earned) → feature logic and entities
```

**Backend.** The HTTP controller is the entrypoint's inbound adapter (`entrypoints/<name>/http`). It translates a request into a call on a feature, or on an orchestration module where the action takes 2+ features. The HTTP framework stays out of both. Default to **CQRS** — reads and writes take separate paths. Tests reach the service through `handle(request)`, which delegates to the in-process dispatch a real request takes (`fastify.inject` / `hono.fetch`). A use case earns its place at 2+ boundaries or when it spans a unit of work (one transaction across several repositories or aggregates), and skip it below that. Put a read behind a port only where a fake adds value.

**Frontend.** Server cache — data mirrored from the API — goes to a data-fetching library (TanStack Query, a Solid resource) behind a port, never hand-rolled in components; UI state (open/closed, selection, draft) stays in component-local signals. A server semantic rejection (409/422) maps onto the offending field, not a generic toast.

## Validation

The inbound boundary checks **structure** — types, ranges, required fields — and rejects malformed input before deeper logic runs. A message from another runtime of the same app crosses an inbound boundary too. Inner layers check **semantics** — state-dependent rules no schema expresses (sufficient balance, the record exists).

Validate structure with a **declarative schema** — zod / valibot (TS), pydantic (Python) — never an unchecked cast or a hand-rolled `typeof`/`in` chain. One schema is the single authority: parse once into a typed value or a rejection, derive the static type from it, and delegate every format check to it.

## Logging & events

**Logging is ambient.** A logger is reached directly, not threaded through constructors: nothing branches on it and no test asserts it. One root logger, scoped per module so each line carries its source, and the logger filters by level. Config sets that level once. Backend: a logging library (pino / winston — TS; structlog — Python). Frontend: the ready-to-go scoped logger in [Logger.ts](Logger.ts). The project vendors it as `src/shared/Logger.ts`, and `typescript.json` lets only that file call the console.

**Domain events — injected like any port.** Notification goes through a typed pub-sub, fire-and-forget.

- **Dynamic 1:N fan-out** — one source emitting to a subscriber set that varies at runtime — *is* pub-sub: use one even inside a single module and even at the first site, rather than hand-rolling a listener `Set` plus notify loop. Without that fan-out, a fixed 1:1 call inside a module is simpler.
- **The owner hides it.** Hold it as a private `pubSub` field and expose one domain verb pair per channel: bound `onStateChanged`/`offStateChanged` fields typed `ChannelSubscriber<Payload>`. Consumers never import the pub-sub.
- A ready-to-go typed `PubSub<ChannelMap>` ships in [PubSub.ts](PubSub.ts); **vendor the part you need** into the project's `shared`, adapt the `Logger` seam, and head it with a provenance line — source repo plus commit. Cross-process delivery, durability, retries, or asynchronous handlers need a real broker.
