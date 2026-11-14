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
entities/       business types and pure logic that 2+ features, orchestration or entities modules share
shared/         code with no business meaning (logger, pub-sub, utilities)
infra/          generic adapters to the outside: protocols, and the clients and data 2+ features share (database, HTTP transport, storage, messaging, clock, random)
```

A **module** is one folder directly under a layer, or one file directly under `shared` or `infra`. A module of `orchestration`, `features` or `entities`, or a `shared` folder, is imported only through its surface (**Module depth**). The layers import one way only, so a change to an inner layer is the only one that reaches outward:

| Layer | Imports |
|---|---|
| `entrypoints` | every layer |
| `orchestration` | `features`, `entities`, `shared` |
| `features` | `entities`, `shared`, and from a feature's `infra/` the root `infra` |
| `entities` | other `entities` without cycles, `shared` |
| `shared` | `shared` |
| `infra` | other `infra`, `shared` |

- **A feature is whole.** Its port, repository, types, helpers and own adapters live in its folder. Its port and the feature's own types the port names sit in one place, which the feature's code and its `infra/` both import (**By language** → Feature port). Test placement is [testing.md](../testing.md) → **Commands and file names**.
- **A type and its logic live in the module that uses them.** They move to `entities` only once two or more `features`, `orchestration` or `entities` modules import them. An entity left with one such module moves into it. When that module is an orchestration module, which holds no logic, the entity moves into a feature it uses. An import from `entrypoints` or a test does not count, and one from a feature's `infra/` counts as that feature. An entrypoint takes the type from the feature's surface. The direction guard checks this count.
- **A feature imports no other feature, and an orchestration module imports no other orchestration module.** Two features link through an orchestration module or an entrypoint. It passes one feature's callback to the other.
- **An orchestration module holds no business logic of its own.** Review checks that, because the direction guard reads only imports. An orchestration module exists only where an action takes two or more features, and the direction guard checks that count. A single feature is mounted or started by its entrypoint directly.
- **An entity depends on no UI or render framework.** Entities link to each other mostly by type and id. A rule that spans several entities belongs to a feature.
- **Outside the root `infra`, only entrypoints and a feature's `infra/` import it.** A root adapter knows only its protocol. Examples are a fetch wrapper with timeouts, retries and the authorization header, a database connection, a store, a message transport and the clock. A protocol adapter stays in the root even when one feature alone uses it. An ORM's entity class, such as TypeORM's `@Entity`, is a row model. It lives in the root `infra` or a feature's `infra/`, and never in the `entities` layer. The app's storage adapter (`infra/db`) holds the connection and the database's migrations, one ordered sequence per database, which `db:migrate` applies ([project.md](../project.md) → **Migrations**). No ORM or driver type appears in its operations, transactions and joined read queries included. A feature declares its own port, as the structural subset of a root adapter or in its own terms for an adapter in its `infra/`. The composition root passes the adapter in, and the compiler checks there that it fits.
- **An adapter that knows one outside API or one feature's data lives in that feature's `infra/`.** A feature has an `infra/` only when it has such an adapter. A Semrush client is one. So is a client of the Chrome Web Store catalogue or a model provider. Such a client builds on the root HTTP transport. The feature's tables, row models and SQL are another, while their migrations join the sequence in `infra/db`. The feature's `infra/` is its second surface. Only the composition root imports it. Of its feature, it imports only the port and the entities that port names. Beyond those, it imports the root `infra` and `shared`. Once a second feature needs such a client or such data, it moves to the root `infra/`. There it keeps no feature's types.
- **No `pages` or `widgets` layer.** A block without logic or a business type goes to `shared`. One that renders an entity lives in the feature that shows it. A block with its own logic is a feature, and a block that assembles features is an orchestration module.
- **A guard holds the direction in every language**, and CI fails on a cycle. **By language** names both.
- **A guard checks exactly and never guesses.** It reads the parsed program: its imports, the targets of its calls, the symbols the types resolve and its syntax tree. It reads the repository's layout too: exact file and folder names and a module's path. It may use a regular expression that names a known set exactly, such as the file name `index.ts` or an entrypoint's file pattern. A guess reads meaning into the shape of text whose cases cannot all be listed in advance, as a rule on two capitals in a row would. The standard and review hold any rule that only a guess could check.

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
| Test kit | `"./testing"` in the same `exports` | a `<package>_testing` package, since Go keeps the `_test` suffix for external test packages |
| Composition root | `compose.ts` beside `start.ts` in `src/entrypoints/<name>/`, `compose(overrides)` | `compose.go` beside `start.go` in `entrypoints/<name>/`, `Compose(overrides)` |
| Tests and their tooling | unit tests beside the code, the rest in `tests/` beside `src/` ([testing.md](../testing.md)) | `_test.go` beside the code |
| Feature port | `features/<x>/port.ts`, or a `port/` folder. Planned: the direction guard holds a feature's `infra/` to its port. | the package `features/<x>/port`. Planned: the `layers` analyzer holds a feature's infra to its port. |
| Feature infra | `features/<x>/infra/` with its own `exports.ts`. Planned: the direction guard holds who imports it. | the package `features/<x>/infra` and the packages under it. Planned: the `layers` analyzer holds who imports it. |
| Direction guard | `layers.json`, which the [.oxlintrc.json](../../templates/.oxlintrc.json) template turns on | the `layers` analyzer of go-lint, KK Forge's golangci-lint build, which also holds `cmd/` to `entrypoints/`, each `func main` to one call, and an HTTP server to `entrypoints/` |
| Implicit names | `index.*`, forbidden by a rule in `typescript.json`, libraries included. A WXT extension extends `wxt.json`, where each entrypoint is a folder `entrypoints/<name>/` and its `index.ts` is the launcher. The `launcher` rule holds that file to one call. A script directly in `entrypoints/` fails. The linter reads no HTML, so review holds a WXT `index.html` to markup and the script tag that loads `./index.ts`, and rejects a flat `entrypoints/<name>.html`. | none. The launcher is `func main` in `cmd/<name>/`. |
| Package, folder and file names | as [code-style.md](../code-style.md) → **Naming** spells them | a name of several words joins them with `_`, as in `convert_request/` and `convert_request_test.go`, never `convertrequest`. A package takes its folder's name, except `main`, the package at the module root, and an external test package, which adds `_test`. A `main` package's folder joins its words with `_` too, as `cmd/go_lint/`, since the Go tool names the command after it. Review holds this, since no exact check can tell a joined name from a single word. |
| Environment guard | Planned: the `environment` rule of `layers.json` matches the global a reference resolves to and the `import.meta` expression, so a local name that only looks alike passes. Outside `infra`, it fails `Date.now()`, `new Date()` without a value, `Date()`, `performance.now()`, `Math.random()`, `crypto.randomUUID()`, `crypto.getRandomValues()`, `process.env`, `import.meta.env`, `setTimeout` and `setInterval`. Planned too: `no-console` in `typescript.json` allows the console only in `infra`. An entrypoint writes a CLI's result with `process.stdout.write` or `process.stderr.write`. A test is exempt. | Planned: the `layers` analyzer matches the symbol a call resolves to through the types, by its package's full path. Outside `infra`, it fails `time.Now`, `time.Since`, `time.Until`, `time.Sleep`, `time.After`, `time.Tick`, `time.NewTimer`, `time.NewTicker`, `time.AfterFunc`, `os.Getenv`, `os.LookupEnv`, `os.Environ`, `os.ExpandEnv`, and an import of `math/rand`, `math/rand/v2` or `crypto/rand`. It fails the `log` package's output there too. Outside `infra` and `entrypoints`, it fails `fmt.Print*`, `slog.New` and `slog`'s handlers. A feature may call `slog`'s package-level functions. A test and a test kit are exempt. |
| Cycles | `import/no-cycle` in `typescript.json`, which the same template extends | the compiler |

## Horizontal decoupling

Business logic must not depend on infrastructure — a database, a broker, an HTTP API, a UI framework, browser storage, the clock, the filesystem, … Dependencies point inward, never out.

**Functional core, imperative shell** — the layer that drives the ports hands values inward; the core computes and returns rather than awaiting I/O mid-decision.

### Ports and adapters

A port is an interface at a boundary to external technology, declared by the domain in its own terms. It can also be a **minimal structural subset** of a large third-party type — declare only the methods you use.

An adapter implements a port against one concrete technology. The core imports only the port. Frameworks, SDKs and platform APIs live in entrypoints and adapters. What the program does not control lives in adapters alone (**Composition root**).

### Composition root

Each entrypoint has one composition root, the function `compose(overrides)`, in its own file beside `start` (**By language** → Composition root). `compose` takes partial overrides and returns the handle. The overrides default to the real adapters — the seam a test or another runtime swaps one dependency through. `compose` constructs the adapters and injects them into the features. `start` only binds the handle to its host and transport. A test calls `compose` with its overrides and leaves the host out. Business and domain code never construct an infrastructure type. For a plain function, the equivalent seam is a dependency option defaulting to the real implementation.

- **A constructor or factory takes one object, `options: <Name>Options`**, its settings and its dependencies together. It keeps the dependencies in one field, `this.deps`, and each setting in a field of its own. The type of `deps` derives from the options type, and no line copies a dependency one by one. A factory function is `newX(options: XOptions)`. In Go the options are `type Options struct{…}`, holding the same settings and dependencies, and the constructor is `func New(o Options) *X`. `compose(overrides)` fills each dependency with the real adapter unless an override replaces it.

  ```ts
  type GenerationOptions = { retries: number; api: GenerationApi; clock: Clock };

  class Generation {
    private readonly retries: number;
    private readonly deps: Pick<GenerationOptions, 'api' | 'clock'>;

    constructor({ retries, ...deps }: GenerationOptions) {
      this.retries = retries;
      this.deps = deps;
    }
  }
  ```

- **Assembly is an explicit call to that function.** A side-effect import or a global service locator hides what was built from a test and a reader, so use neither.
- **Two ambient dependencies skip injection.** A test replaces only their output. Code reaches them directly, and they are the only globals:
  - the logger (**Logging & events**);
  - analytics events — `shared` holds a generic track function, and the composition root binds it to its sender. The event catalogue that types the function lives in the module that tracks, or in `entities/analytics` once two or more `features`, `orchestration` or `entities` modules track.

  An audit trail is behaviour, so it goes through a port the feature declares. Analytics is no such record.

**What the program does not control enters through infra**: I/O, the clock and its timers, random numbers and ids, the environment and log output. Only `infra` reads or writes them, a feature's `infra/` included, and entrypoints reach them through `infra` too. A CLI's result is the exception: an entrypoint writes it to standard output itself, as its outbound edge, with `process.stdout.write` in TypeScript and `fmt.Print*` in Go. Log lines still go through the logger, so the console and the `log` package stay in `infra`. The guard bans the calls and leaves the types alone, so an entity may hold a `time.Time` or a `Date`. Logic takes the value as a parameter, as in `isExpired(token, now)`. Browser storage and `fetch` are I/O, and `location` and `navigator` are the environment, so only `infra` calls them too. A test is exempt, since it drives the clock and the environment itself. The command line is no environment: an entrypoint reads its arguments as its inbound edge, as a server reads a request.

- **Of these primitives, `shared` holds only what they look like**: the `Clock` type, the `Random` or `IdGenerator` type, and the `Logger` class. The `Clock` has the timers too, such as `setTimeout` and `time.Sleep`. The `Logger` leaves the writing to `infra`. `infra` implements them, as in `systemClock`, `systemRandom` and the log sink in `infra/logger`. A port to an outside system — a database, an HTTP API, `chrome.*`, a vendor's API — is declared by the feature that needs it, as above.
- **The environment has no interface.** `infra/env` reads it. The composition root parses it into one config with a schema (**Validation**) and hands each feature the values it needs.

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

- **Publish one surface per module.** A module declares its boundary in one place: the handle, port, or facade a caller holds. The two exceptions are a surface per runtime, below, and a feature's `infra/`, which the composition root alone imports (**App layers**). Everything else is internal and nothing outside imports it. Never give it a name the tooling resolves implicitly (**Entrypoints**).
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

**Backend.** The HTTP controller is the entrypoint's inbound adapter (`entrypoints/<name>/http`). It translates a request into a call on a feature, or on an orchestration module where the action takes 2+ features. The HTTP framework stays out of both. Default to **CQRS** — reads and writes take separate paths. Tests reach the service through `handle(request)`, which delegates to the in-process dispatch a real request takes (`fastify.inject` / `hono.fetch`). A use case earns its place at 2+ boundaries or when it spans a unit of work (one transaction across several repositories or aggregates), and skip it below that. A read goes through the feature's port, as every access to infra does.

**Frontend.** Server cache — data mirrored from the API — goes to a data-fetching library (TanStack Query, a Solid resource) behind a port, never hand-rolled in components; UI state (open/closed, selection, draft) stays in component-local signals. A server semantic rejection (409/422) maps onto the offending field, not a generic toast.

## Validation

The inbound boundary checks **structure** — types, ranges, required fields — and rejects malformed input before deeper logic runs. A message from another runtime of the same app crosses an inbound boundary too. Inner layers check **semantics** — state-dependent rules no schema expresses (sufficient balance, the record exists).

Validate structure with a **declarative schema** — zod / valibot (TS), pydantic (Python) — never an unchecked cast or a hand-rolled `typeof`/`in` chain. One schema is the single authority: parse once into a typed value or a rejection, derive the static type from it, and delegate every format check to it. In Go, `compose` parses the config from the environment with `github.com/caarlos0/env/v11`. Struct tags mark a variable required or give its default. A custom type brings its own parser, and each error is wrapped with the variable's name.

## Logging & events

**Logging is ambient, and its output is infra.** A module reaches the logger directly, as `Logger.root.scope(...)`. One root logger, scoped per module so each line carries its source, filters by level. The logger hands each record to a sink from `infra`. The sink is a `compose` override, so a test passes a silent one. `infra/logger` holds one sink per `LOG_FORMAT`, `pretty` and `json`. `compose` installs the pretty sink at the default level on its first line, parses the config, and installs again the sink `LOG_FORMAT` names at the config's level. A config that fails to parse is logged through the first sink. In TypeScript a record written before the first install is lost. So no module writes a record at import time, and a test of the composition root checks that it installs the sink. This sequence is the exception to **No required sequences** (**Module depth**).

- **TypeScript**: the project vendors [Logger.ts](Logger.ts) as `src/shared/Logger.ts` and the sink [consoleSink.ts](consoleSink.ts) as `src/infra/logger/consoleSink.ts`. A backend's `json` sink writes through pino.

  ```ts
  // any module
  const logger = Logger.root.scope('ApiClient');
  logger.warn('Token refresh failed', error);

  // compose: first line, then after the config parses
  Logger.install(overrides.logSink ?? newConsoleSink({ clock: systemClock }), LogLevel.Info);
  const config = parseConfig(readEnv());
  Logger.install(overrides.logSink ?? logSinks[config.logFormat], config.logLevel);
  ```

- **Go**: `log/slog`, with no facade of the project's own. A feature calls `slog`. `Compose` sets the default logger over a handler from `infra` before it builds anything, and sets it again once the config parses, as above. An override replaces the handler both times. Until then `slog` writes through the `log` package to standard error.

  ```go
  // entrypoints/server/compose.go: first line, then after the config parses
  slog.SetDefault(slog.New(logger.NewHandler(logger.Pretty, slog.LevelInfo)))
  slog.SetDefault(slog.New(logger.NewHandler(config.LogFormat, config.LogLevel)))

  // features/history, inside the factory, never in a package-level variable
  logger := slog.With("feature", "history")
  ```

- **A package in `packages/` imports no app's logger.** It takes a logger as a parameter.

**Domain events stay inside the module that owns them.** The module notifies through a typed pub-sub, fire-and-forget, that it holds as a private field. The composition root leaves the pub-sub to its owner. Between features, an orchestration module or the composition root passes one feature's callback to the other.

- **Dynamic 1:N fan-out** — one source emitting to a subscriber set that varies at runtime — *is* pub-sub: use one even inside a single module and even at the first site, rather than hand-rolling a listener `Set` plus notify loop. Without that fan-out, a fixed 1:1 call inside a module is simpler.
- **The owner hides it.** Hold it as a private `pubSub` field and expose one domain verb pair per channel: bound `onStateChanged`/`offStateChanged` fields typed `ChannelSubscriber<Payload>`. Consumers never import the pub-sub.
- A ready-to-go typed `PubSub<ChannelMap>` ships in [PubSub.ts](PubSub.ts). **Vendor the part you need** into the project's `shared`, and head it with a provenance line: source repo plus commit. Point its `Logger` import at `shared/Logger.ts`, as the vendored `consoleSink.ts` does too. Cross-process delivery, durability, retries, or asynchronous handlers need a real broker.
