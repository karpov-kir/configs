**Layer:** craft

# How We Structure Code

## Repository: apps and packages

A repository with more than one part is a monorepo. Apps go under `apps/<name>/` and libraries under `packages/<name>/`.

- **An app is what gets built and released, and it has entrypoints.** One product on several runtimes is one app. A browser extension and a desktop app's windows are one such case. That app holds one set of entrypoints per runtime and one platform adapter per platform (**Platform adapters**). A desktop app's native shell is an app of its own.
- **A package is a library.** It publishes one surface per layer it holds code of, and each surface takes its layer's name, as in `@repo/logger/infra`. The bare name, `@repo/logger`, is its `shared`. A Redis pub-sub package that holds only `infra` publishes `@repo/redis-pubsub/infra`. A test kit gets a testing entry of its own (**By language** → Test kit), and a platform adapter gets a surface of its own (**Platform adapters**). Inside, it takes the app layers its code needs, `infra` included, and has no `entrypoints`. Its imports form no cycle.
- **A technical engine with a domain API is a package of its own, even when one app alone uses it.** A canvas annotation engine is one. Its `package.json` marks it with `"layers": { "kind": "engine" }`, which the direction guard reads (**By language** → Engine mark). It stays out of an app's layers. Its domain meaning keeps it out of `shared`, and an entity or a feature holds one app's business. Like any package, it publishes a surface per layer. Its model and editing logic is its `shared`, as `@repo/annotator`. The code that touches the DOM or a canvas is its `infra`, as `@repo/annotator/infra`.
- **A large package splits a module of its layer into inner modules.** An **inner module** is a folder inside it with its own surface. Code outside that folder imports it only through that surface. Inner modules import each other without a cycle.
- **An app imports each surface of a package as it would its own layer.** Entrypoints and a feature's `infra/` import a package's `infra`, as they do the root `infra`. Any layer imports its `shared`.
- **Imports run from apps to packages, and between packages.** A package imports no app. By default an app imports no other app. A project that needs one declares a one-way pair in the direction guard's option (**By language** → Direction guard). The pair names an app and the app it may import. The first app then imports the second only by name, through that app's `exports` `"./*": "./src/*"`, and under the layer rules. A path into another app's source, the reverse direction and any undeclared pair stay forbidden.
- **Code that two or more apps of the repository use lives once in a package.** It goes in `packages/<name>`, named `@<repo>/<name>`, and each app imports it from there. Code of any layer but `entrypoints` goes there, `infra` included, and vendored code too. Why: one copy is one place to fix, and copies kept per app drift apart. Code that a declared pair shares stays in the imported app. Code one app uses stays in that app, as a repository with one app keeps the vendored `Logger.ts` in its `src/shared` and `consoleSink.ts` in its `src/infra/logger`. A package that one app alone still uses, with no other package importing it, moves back into that app, unless it is an engine with a domain API. This is the rule for `entities` one level up. The rules of a game that both its web and server apps import live in `@<repo>/game`. A Redis pub-sub that two backends share is a package too.
- **A package runs in every runtime of the apps that use it, and each app assembles it.** It uses only what each of those runtimes provides. It holds a platform's SDK and global APIs only in its own platform adapter (**Platform adapters**). It reads no environment: each app's composition root builds it and passes in its configuration and dependencies as parameters. The logger and the other ambient services are the exception (**Logging & events**). A package reaches one ambiently where the repository has a package for it, and otherwise takes it as a parameter.

## App layers

Every app takes the same layers, whatever it is and whatever language it is written in: a backend, a frontend, a CLI, a browser extension.

```
entrypoints/    one folder per way into the app: its host and composition root
orchestration/  one action built from 2+ features (a screen, a scenario, a command)
features/       the app's capabilities, one module each
entities/       business types and pure logic that 2+ features, orchestration or entities modules share
shared/         code with no business meaning (logger, pub-sub, utilities)
infra/          generic adapters to the outside: protocols, one adapter per platform, and the clients and data 2+ features share (database, HTTP transport, storage, messaging, clock, random)
```

A **module** is one folder directly under a layer, or one file directly under `shared` or `infra`. A module of `orchestration`, `features` or `entities`, or a `shared` folder, is imported only through its surface (**Module depth**). The layers import one way only, so a change to an inner layer is the only one that reaches outward:

| Layer | Imports |
|---|---|
| `entrypoints` | every layer |
| `orchestration` | `features`, `entities`, `shared` |
| `features` | `entities`, `shared`, and from a feature's `infra/` the root `infra` but a platform adapter |
| `entities` | other `entities` without cycles, `shared` |
| `shared` | `shared` |
| `infra` | other `infra` but a platform adapter, `shared` |

- **A feature is whole.** Its port, repository, types, helpers and own adapters live in its folder. It holds no inner module but its `infra/` and its port. A feature too large for one module splits into several features. Its port and the feature's own types the port names sit in one place, which the feature's code and its `infra/` both import (**By language** → Feature port). Test placement is [testing.md](../testing.md) → **Commands and file names**.
- **A feature's surface gives out its components, its `<x>UseCases` and its errors, plus types.** A component shows, and takes its data as props. `<x>UseCases` is the feature's behaviour: one type per feature, with each scenario a method. Its constructor takes its options as **Composition root** shows. The surface exports no free function that acts. A helper sits in a file of its own named for what it does, and the surface leaves it out. In an app with components, a feature that only shows gives out its component and skips the use cases file. A UI that builds its DOM in TypeScript in place of JSX holds no components. There, showing is the feature's behaviour, so `<x>UseCases` holds it in a method such as `mount(root)`, and its options carry what it shows. An error is part of the feature's contract, which a caller matches: an error class in TypeScript, and a sentinel error in Go. An enum counts as a type. With a surface per runtime, each surface gives out its components, its types and a use cases class whose name carries its runtime. For the `exports.serviceWorker.ts` of `convert`, the class `ConvertServiceWorkerUseCases` and its factory `newConvertServiceWorkerUseCases` in `convertUseCases.serviceWorker.ts`. The direction guard reads the surface's exports by their declarations and takes the name `<x>UseCases` from the feature's folder name exactly (**By language** → Feature surface). Review keeps logic out of a component. A component of the feature may take the feature's own use cases as a prop or from a context and call their methods. It takes no other feature's use cases, since an orchestration module links two features. The direction guard already fails an import from another feature, and review holds a prop typed to look like one.

  ```ts
  // features/texts/textsUseCases.ts
  export type TextsUseCasesOptions = { api: TextsApi; clock: Clock };

  export class TextsUseCases {
    private readonly deps: TextsUseCasesOptions;

    constructor({ ...deps }: TextsUseCasesOptions) {
      this.deps = deps;
    }

    async save(text: Text): Promise<void> {
      await this.deps.api.save({ ...text, savedAt: this.deps.clock.now() });
    }
  }

  export function newTextsUseCases(options: TextsUseCasesOptions): TextsUseCases {
    return new TextsUseCases(options);
  }

  // features/texts/exports.ts
  export { TextsPanel } from './TextsPanel';
  export { newTextsUseCases, TextsUseCases, type TextsUseCasesOptions } from './textsUseCases';
  ```

- **A type and its logic live in the module that uses them.** They move to `entities` only once two or more `features`, `orchestration` or `entities` modules import them. An entity left with one such module moves into it. When that module is an orchestration module, which holds no logic, the entity moves into a feature it uses. An import from `entrypoints` or a test does not count, and one from a feature's `infra/` counts as that feature. A module of an app that imports this one under a declared pair counts too. An entrypoint takes the type from the feature's surface. The direction guard checks this count.
- **A feature imports no other feature, and an orchestration module imports no other orchestration module.** Two features link through an orchestration module or an entrypoint. It passes one feature's callback to the other.
- **An orchestration module holds no business logic of its own.** Review checks that, because the direction guard reads only imports. An orchestration module exists only where an action takes two or more features, and the direction guard checks that count. A single feature is mounted or started by its entrypoint directly.
- **An entity depends on no UI or render framework.** Entities link to each other mostly by type and id. A rule that spans several entities belongs to a feature.
- **Outside the root `infra`, only entrypoints and a feature's `infra/` import it.** A platform adapter takes fewer importers, which **Platform adapters** names. A root adapter knows only its protocol. Examples are a fetch wrapper with timeouts, retries and the authorization header, a database connection, a store, a message transport and the clock. A protocol adapter stays in the root even when one feature alone uses it. An ORM's entity class, such as TypeORM's `@Entity`, is a row model. It lives in the root `infra` or a feature's `infra/`, and never in the `entities` layer. The app's storage adapter (`infra/db`) holds the connection and the database's migrations, one ordered sequence per database, which `db:migrate` applies ([project.md](../project.md) → **Migrations**). No ORM or driver type appears in its operations, transactions and joined read queries included. A feature declares its own port, as the structural subset of a root adapter or in its own terms for an adapter in its `infra/`. The composition root passes the adapter in, and the compiler checks there that it fits.
- **An adapter that knows one outside API or one feature's data lives in that feature's `infra/`.** A feature has an `infra/` only when it has such an adapter. A Semrush client is one. So is a client of the Chrome Web Store catalogue or a model provider. Such a client builds on the root HTTP transport. The feature's tables, row models and SQL are another, while their migrations join the sequence in `infra/db`. The feature's `infra/` is its second surface. Only the composition root imports it. Of its feature, it imports only the port and the entities that port names. Beyond those, it imports `shared` and the root `infra` except a platform adapter. It reaches a platform through a port (**Platform adapters**). Once a second feature needs such a client or such data, it moves to the root `infra/`. There it keeps no feature's types.
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
| Library surface | `exports` in `package.json`: entry `"."` for its `shared`, and `"./<layer>"` for each other layer the package holds code of | the module's exported packages |
| Engine mark | `"layers": { "kind": "engine" }` in the package's `package.json` | none: the `layers` analyzer counts no package's apps |
| Test kit | `"./testing"` in the same `exports`, naming `src/testing/exports.ts`. The kit is that folder even for one helper. | a `<package>_testing` package, since Go keeps the `_test` suffix for external test packages |
| Composition root | `compose.ts` beside `start.ts` in `src/entrypoints/<name>/`, `compose(overrides)` | `compose.go` beside `start.go` in `entrypoints/<name>/`, `Compose(overrides)` |
| Tests and their tooling | unit tests beside the code, the rest in `tests/` beside `src/` ([testing.md](../testing.md)) | `_test.go` beside the code |
| Feature port | `features/<x>/port.ts`, or a `port/` folder. Planned: the direction guard holds a feature's `infra/` to its port. | the package `features/<x>/port`. Planned: the `layers` analyzer holds a feature's infra to its port. |
| Feature surface | `exports.ts` gives out components, each declaring its return type `JSX.Element`, and from `features/<x>/<x>UseCases.ts` the class `<X>UseCases`, its factory `new<X>UseCases(options: <X>UseCasesOptions)` and the options type. `exports.<runtime>.ts` gives out instead, from `<x>UseCases.<runtime>.ts`, the class `<X><Runtime>UseCases`, where each word of the runtime's name, split at `.` and `-`, starts upper-case. An error class extends `Error`, or another error class of the same feature, which the guard follows through its imports. An `enum` counts as a type. Planned: the direction guard fails any other value it exports. | the package `features/<x>` gives out, from `<x>_use_cases.go`, `type UseCases`, `type Options` and `func New(o Options) *UseCases`. It may also give out a sentinel error, which a caller matches with `errors.Is`: a variable of type `error` set by `errors.New`, or by `fmt.Errorf` without `%w`. Planned: the `layers` analyzer fails any other exported function or variable. |
| Feature infra | `features/<x>/infra/` with its own `exports.ts`. Planned: the direction guard holds who imports it. | the package `features/<x>/infra` and the packages under it. Planned: the `layers` analyzer holds who imports it. |
| Direction guard | `layers.json`, which the [.oxlintrc.json](../../templates/.oxlintrc.json) template turns on. A project declares its app pairs in the option `apps` of its rule `layers`, as `{ "apps": { "apps/a": ["apps/b"] } }`. Planned: it holds a platform's SDK to its platform adapter by a fixed list per platform, such as `wxt/browser` for `infra/chrome` and `@tauri-apps/*` for `infra/tauri`. It holds each platform adapter to the entrypoints on its platform, and an entrypoint's import from a platform's framework to its entry API (**Platform adapters**). | the `layers` analyzer of go-lint, a golangci-lint build, which also holds `cmd/` to `entrypoints/`, each `func main` to one call, and an HTTP server to `entrypoints/` |
| Implicit names | `index.*`, forbidden by a rule in `typescript.json`, libraries included. A WXT extension extends `wxt.json`, where each entrypoint is a folder `entrypoints/<name>/` and its `index.ts` is the launcher. The `launcher` rule holds that file to one call. A script directly in `entrypoints/` fails. The linter reads no HTML, so review holds a WXT `index.html` to markup and the script tag that loads `./index.ts`, and rejects a flat `entrypoints/<name>.html`. | none. The launcher is `func main` in `cmd/<name>/`. |
| Package, folder and file names | as [code-style.md](../code-style.md) → **Naming** spells them | a name of several words joins them with `_`, as in `convert_request/` and `convert_request_test.go`, never `convertrequest`. A package takes its folder's name, except `main`, the package at the module root, and an external test package, which adds `_test`. A `main` package's folder joins its words with `_` too, as `cmd/go_lint/`, since the Go tool names the command after it. Review holds this, since no exact check can tell a joined name from a single word. |
| Environment guard | Planned: the `environment` rule of `layers.json` matches the global a reference resolves to and the `import.meta` expression, so a local name that only looks alike passes. It matches a global reached through `globalThis`, `window` or `self` as the same global. A package's `infra` fails `process.env` and `import.meta.env` too. Outside `infra`, it fails `Date.now()`, `new Date()` without a value, `Date()`, `performance.now()`, `Math.random()`, `crypto.randomUUID()`, `crypto.getRandomValues()`, `process.env`, `import.meta.env`, `setTimeout`, `setInterval`, `fetch`, `navigator`, `location`, `localStorage`, `sessionStorage` and `indexedDB`. It fails `chrome` and `browser` outside `infra/chrome`, and `__TAURI__` and `__TAURI_INTERNALS__` outside `infra/tauri` (**Platform adapters**). Planned too: `no-console` in `typescript.json` allows the console only in `infra`, an app's or a package's. An entrypoint writes a CLI's result with `process.stdout.write` or `process.stderr.write`. Test code and test kits (`src/**/testing`) are exempt. | Planned: the `layers` analyzer matches the symbol a call resolves to through the types, by its package's full path. Outside `infra`, it fails `time.Now`, `time.Since`, `time.Until`, `time.Sleep`, `time.After`, `time.Tick`, `time.NewTimer`, `time.NewTicker`, `time.AfterFunc`, `os.Getenv`, `os.LookupEnv`, `os.Environ`, `os.ExpandEnv`, and an import of `math/rand`, `math/rand/v2` or `crypto/rand`. It fails the `log` package's output there too. A package's `infra` fails `os.Getenv`, `os.LookupEnv`, `os.Environ` and `os.ExpandEnv` too. Outside `infra` and `entrypoints`, it fails `fmt.Print*`, `slog.New` and `slog`'s handlers. A feature may call `slog`'s package-level functions. A test and a test kit are exempt. |
| Cycles | `import/no-cycle` in `typescript.json`, which the same template extends | the compiler |

## Horizontal decoupling

Business logic must not depend on infrastructure — a database, a broker, an HTTP API, a UI framework, browser storage, the clock, the filesystem, … Dependencies point inward, never out.

**Functional core, imperative shell** — the layer that drives the ports hands values inward; the core computes and returns rather than awaiting I/O mid-decision.

### Ports and adapters

A port is an interface at a boundary to external technology, which the domain declares in its own terms. It can also be a **minimal structural subset** of a large third-party type — declare only the methods you use.

An adapter implements a port against one concrete technology. The core imports only the port. An app framework, such as Solid or Hono, lives in entrypoints, adapters and a feature's components. A platform's SDK and global APIs live in its platform adapter (**Platform adapters**). What the program does not control lives in adapters alone (**Composition root**).

### Platform adapters

A **platform** is a host that gives an app its own SDK and global APIs, as a browser does for an extension and Tauri does for a desktop app's windows. A **platform adapter** is the module of the root `infra`, or of a package's `infra`, that holds one platform, named `infra/<platform>`. A platform's **entry API** is the names its framework requires an entrypoint to call to define or mount itself.

- **A platform's SDK and global APIs live only in its platform adapter, except its entry API.** A browser extension's `infra/chrome` holds `wxt/browser` and the `chrome` and `browser` globals. A desktop app's `infra/tauri` holds `@tauri-apps/*` and `window.__TAURI__`.
- **Only an entrypoint that runs on a platform imports that platform's adapter.** The rest of the root `infra`, a feature's `infra/` and every other layer never import it. An entrypoint on another platform never imports it either.
- **An entrypoint imports from a platform's framework only its entry API.** WXT's entry API is `defineBackground`, `defineContentScript`, `defineUnlistedScript`, `createShadowRootUi`, `createIntegratedUi`, `createIframeUi` and the `ContentScriptContext` type. Each `create*Ui` brings the types of the UI it returns and of its options. The entrypoint reaches the rest of the platform through the platform adapter. A platform such as Tauri's JS API has no entry API, so its entrypoint reaches the whole platform through the adapter.
- **A feature whose `infra/` needs a platform declares a port for it.** The platform adapter implements the port, and the composition root passes it in. For example, a feature's `infra/` that would read `chrome.storage` declares a storage port. `infra/chrome` implements that port, and `compose` injects it.
- **A package's `infra/<platform>` is a platform adapter under the same rules as an app's.** The package publishes it as a surface of its own, `@<repo>/<name>/infra/<platform>`. Only an app's entrypoint that runs on that platform imports it. No other layer of the package or of an app imports it. The rest of the package holds no platform. Package code that needs a platform declares a port, which a platform adapter implements and the app's composition root passes in. For example, two browser extensions in one repository share `@<repo>/<name>/infra/chrome`, and the entrypoints of each extension import it.
- **The guards check these rules exactly** (**By language** → Direction guard, Environment guard). The adapter names are fixed as `infra/chrome` and `infra/tauri`, and each platform has a fixed SDK list. The guard matches a platform global by the global it resolves to. A type that names one, such as `chrome.tabs.Tab`, counts too, because a port declares its own types. An entrypoint runs on one platform, and the guard reads which one from what the entrypoint's files import. Test code and test kits (`src/**/testing`) are exempt. Planned: oxlint-config 11.0.0 checks a package's platform adapter, following its `infra/<platform>` surface and a deep path into a package's layer. Version 10.0.0 checks an app's platform adapters alone.

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
- **Three ambient dependencies skip injection.** A test replaces only the sink or the source behind them. Code reaches them directly, and they are the only globals:
  - the logger (**Logging & events**);
  - analytics events — `shared` holds a generic track function, and the composition root binds it to its sender. The event catalogue that types the function lives in the module that tracks, or in `entities/analytics` once two or more `features`, `orchestration` or `entities` modules track.
  - the interface strings — `shared` holds a facade that calls no platform, and `infra` holds its source, such as `chrome.i18n` or the translation files. The composition root installs the source on its first line, and a test installs a stub source.

  An audit trail is behaviour, so it goes through a port the feature declares. Analytics is no such record.

**What the program does not control enters through infra**: I/O, the clock and its timers, random numbers and ids, the environment and log output. Only `infra` reads or writes them, a feature's `infra/` and a package's `infra` included, and entrypoints reach them through `infra` too. A package's `infra` still reads no environment, since its values come from the composition root. A CLI's result is the exception: an entrypoint writes it to standard output itself, as its outbound edge, with `process.stdout.write` in TypeScript and `fmt.Print*` in Go. Log lines still go through the logger, so the console and the `log` package stay in `infra`. The guard bans the calls and leaves the types alone, so an entity may hold a `time.Time` or a `Date`. Logic takes the value as a parameter, as in `isExpired(token, now)`. Browser storage and `fetch` are I/O, and `location` and `navigator` are the environment, so only `infra` calls them too. A platform's own APIs, such as `chrome.storage`, stay in its platform adapter (**Platform adapters**). A test is exempt, since it drives the clock and the environment itself. The command line is no environment: an entrypoint reads its arguments as its inbound edge, as a server reads a request.

- **Of these primitives, `shared` holds only what they look like**: the `Clock` type, the `Random` or `IdGenerator` type, and the `Logger` class. Where two or more apps use one of them, the package that holds it does instead. The `Clock` has the timers too, such as `setTimeout` and `time.Sleep`. The `Logger` leaves the writing to `infra`. `infra` implements them, as in `systemClock`, `systemRandom` and the log sink in `infra/logger`, the app's or the logger package's. A port to an outside system — a database, an HTTP API, `chrome.*`, a vendor's API — is declared by the feature that needs it, as above.
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

- **Publish one surface per module.** A module declares its boundary in one place: the handle, port, or facade a caller holds. The exceptions are a surface per runtime, below, a feature's `infra/`, which the composition root alone imports (**App layers**), and a package's surface per layer (**Repository: apps and packages**). A package's test kit is one more (**By language** → Test kit), and so is a package's platform adapter (**Platform adapters**). Everything else is internal and nothing outside imports it. Never give it a name the tooling resolves implicitly (**Entrypoints**).
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
backend         : entrypoint's HTTP controller → orchestration (2+ features) → feature's `<x>UseCases` → feature logic and entities
```

**Backend.** The HTTP controller is the entrypoint's inbound adapter (`entrypoints/<name>/http`). It translates a request into a call on a feature, or on an orchestration module where the action takes 2+ features. The HTTP framework stays out of both. Default to **CQRS** — reads and writes take separate paths. Tests reach the service through `handle(request)`, which delegates to the in-process dispatch a real request takes (`fastify.inject` / `hono.fetch`). The controller calls a method of the feature's `<x>UseCases`, and a method that spans a unit of work runs it in one transaction. A read goes through the feature's port, as every access to infra does.

**Frontend.** Server cache — data mirrored from the API — goes to a data-fetching library (TanStack Query, a Solid resource) behind a port, never hand-rolled in components; UI state (open/closed, selection, draft) stays in component-local signals. A server semantic rejection (409/422) maps onto the offending field, not a generic toast.

## Validation

The inbound boundary checks **structure** — types, ranges, required fields — and rejects malformed input before deeper logic runs. A message from another runtime of the same app crosses an inbound boundary too. Inner layers check **semantics** — state-dependent rules no schema expresses (sufficient balance, the record exists).

Validate structure with a **declarative schema** — zod / valibot (TS), pydantic (Python) — never an unchecked cast or a hand-rolled `typeof`/`in` chain. One schema is the single authority: parse once into a typed value or a rejection, derive the static type from it, and delegate every format check to it. In Go, `Compose` parses the config with `github.com/caarlos0/env/v11`, from the variables `infra/env` read, which it passes as `env.Options{Environment: vars}`. Struct tags mark a variable required or give its default. A default other code reads too is a constant instead, which the code that parses sets the field to before the parse (**Logging & events**). A custom type brings its own parser, and each error is wrapped with the variable's name.

## Logging & events

**Logging is ambient, and its output is infra.** A module reaches the logger directly, as `Logger.root.scope(...)`. One root logger, scoped per module so each line carries its source, filters by level. The logger hands each record to a sink from `infra`. The sink is a `compose` override, so a test passes a silent one. The app's `infra/logger` holds one sink per `LOG_FORMAT`, `pretty` and `json`, except a sink a package holds, and `newLogSink(format, options)` builds the sink a format names. `compose` reads the environment once, through `infra/env`. `infra/env` writes no record. It returns what it found, such as the file it read, and `compose` logs that after the first install. `compose` first installs the sink `LOG_FORMAT` names, at the default level, parsing that one value with the config's own schema field. An unknown value installs the pretty sink, and the config's parse then rejects it. Two reads parse a value with a schema field before the config does. One is this read of `LOG_FORMAT`. The other is the read of `ENV` by which `infra/env` picks the env file it reads ([project.md](../project.md) → **Environments**). An unknown `ENV` reads no file. Each read leaves the rejection to the config's parse. `compose` then parses the whole config and installs the sink again at the config's level. A config that fails to parse is logged through the first sink, so a `json` log stays `json`, its config error included. In TypeScript a record written before the first install is lost. So no module writes a record at import time, and a test of the composition root checks that it installs the sink. Some apps have one sink and no `LOG_FORMAT`, such as a web app or a browser extension. Such an app installs its sink at the default level first in `compose`, together with the interface strings' source, before it reads the environment. It installs the sink again only where its config sets the level. This sequence is the exception to **No required sequences** (**Module depth**).

- **TypeScript**: the project vendors [Logger.ts](Logger.ts) as `src/shared/Logger.ts` and the sink [consoleSink.ts](consoleSink.ts) as `src/infra/logger/consoleSink.ts`. Where two or more apps of a repository log, both go once into `packages/logger`, `Logger.ts` in its `shared` and `consoleSink.ts` in its `infra/logger` (**Repository: apps and packages**). Each app's composition root installs the sink. A backend's `json` sink writes through pino, so it stays in the backend's `infra/logger`, or in a package only backends import. The app writes `newLogSink` there too, choosing between `newConsoleSink` and the `json` sink.

  ```ts
  // any module
  const logger = Logger.root.scope('ApiClient');
  logger.warn('Token refresh failed', error);

  // compose: the sink LOG_FORMAT names first, then again after the config parses
  const { env, found } = readEnv();
  Logger.install(overrides.logSink ?? newLogSink(logFormatOf(env), { clock: systemClock }), LogLevel.Info);
  Logger.root.scope('compose').info('Read the environment', found);
  const config = parseConfig(env);
  Logger.install(overrides.logSink ?? newLogSink(config.logFormat, { clock: systemClock }), config.logLevel);

  // config.ts: the schema field the config parses LOG_FORMAT with, read alone
  export const logFormatOf = (env: Env): LogFormat => logFormatField.safeParse(env.LOG_FORMAT).data ?? LogFormat.Pretty;
  ```

- **Go**: `log/slog`, with no facade of the project's own. A feature calls `slog`. `Compose` sets the default logger over a handler from `infra` before it builds anything, and sets it again once the config parses, as above. The first handler takes its format from `logFormatOf`. It runs the `LogFormat` type's parser over `LOG_FORMAT`, and gives `Pretty` where that parser fails. The default is one constant, because a struct tag holds only a literal. The code that parses the config sets the `Config` field to it before the parse, in place of an `envDefault` tag. That is `Compose` itself, or a `config.Parse` it calls. `logFormatOf` falls back to it for an unset or empty variable, which the parse leaves as it was. An override replaces the handler both times. Until then `slog` writes through the `log` package to standard error.

  ```go
  // entrypoints/server/compose.go: the handler LOG_FORMAT names first, then again after the config parses
  vars, found := environment.Read() // infra/env, under another name than caarlos0's env
  slog.SetDefault(slog.New(logger.NewHandler(logFormatOf(vars), slog.LevelInfo)))
  slog.Info("read the environment", "found", found)
  config := Config{LogFormat: defaultLogFormat} // the default, in place of an envDefault tag
  if err := env.ParseWithOptions(&config, env.Options{Environment: vars}); err != nil {
    return nil, fmt.Errorf("parse the config: %w", err)
  }
  slog.SetDefault(slog.New(logger.NewHandler(config.LogFormat, config.LogLevel)))

  // features/history, inside the factory, never in a package-level variable
  logger := slog.With("feature", "history")
  ```

- **A package in `packages/` imports no app's logger.** It logs through the repository's logger package where one exists, and otherwise it takes a logger as a parameter.

**Domain events stay inside the module that owns them.** The module notifies through a typed pub-sub, fire-and-forget, that it holds as a private field. The composition root leaves the pub-sub to its owner. Between features, an orchestration module or the composition root passes one feature's callback to the other.

- **Dynamic 1:N fan-out** — one source emitting to a subscriber set that varies at runtime — *is* pub-sub: use one even inside a single module and even at the first site, rather than hand-rolling a listener `Set` plus notify loop. Without that fan-out, a fixed 1:1 call inside a module is simpler.
- **The owner hides it.** Hold it as a private `pubSub` field and expose one domain verb pair per channel: bound `onStateChanged`/`offStateChanged` fields typed `ChannelSubscriber<Payload>`. Consumers never import the pub-sub.
- A ready-to-go typed `PubSub<ChannelMap>` ships in [PubSub.ts](PubSub.ts). **Vendor the part you need** into the app's `shared`, or into `packages/<name>` once two or more apps use it, and head it with a provenance line: source repo plus commit. Point its `Logger` import at the app's `shared/Logger.ts`, or at the logger package, as the vendored `consoleSink.ts` does too. A packaged PubSub in a repository with no logger package takes a logger as a parameter in place of `Logger.root`. Cross-process delivery, durability, retries, or asynchronous handlers need a real broker.
