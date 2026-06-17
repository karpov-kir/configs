# How We Structure Code

Examples below are language-agnostic pseudo-code. Patterns matter, not the tools.

This file holds the **shared core** — the principles backend and frontend both follow. Two companion files apply that core to each side; read this first, then the one you need:

- Backend / services → [architecture-backend.md](~/.claude/architecture-backend.md)
- Frontend / UI → [architecture-frontend.md](~/.claude/architecture-frontend.md)

**Applying these documents.** Follow them when a project already does, or when starting a new one; otherwise match the project's existing architecture — consistency within a project wins.

---

## Two axes

How a system is decomposed and wired, along two axes that compose: **vertical slicing** (per-feature cohesion) and **horizontal decoupling** (each slice swappable from its tech). The same ports and composition-root seam make it testable — the real graph with a fake at one edge, no mocks (see [testing.md](~/.claude/testing.md)).

### Vertical slicing

Organize the top level by feature, not by technical layer — a slice owns its boundary, business logic, and data access together (a feature change stays in one folder). Prefer this over a layer-first `controllers/`/`services/`/`repositories/` tree.

Shared folder vocabulary, used by both sides — each companion file shows its concrete tree:

| Folder | Role |
|---|---|
| `features/` | Vertical slices — one folder per feature, owning its boundary, logic, data, and adapters. |
| `shared/` | Cross-slice primitives — no feature logic. |
| `entrypoints/` | Inbound edges — one per way into the app; each owns its host, composition root, and app-wide cross-cutting. |
| `infra/` | Global / cross-slice outbound plumbing and adapters. |

- A slice may use `shared`, never another slice's internals; cross-slice needs go through a published port or an event.
- Keep a slice whole — its adapters, tests, types, and helpers live with it.

### Horizontal decoupling

Business logic must not depend on infrastructure (e.g. a database, a message broker, an HTTP API, a UI framework, browser storage, the clock, the filesystem) — dependencies point inward, never out.

#### Ports

A port is an interface at a boundary to external technology, declared by the domain in its own terms.

```
interface Store:                          // a port, declared by the domain
    load(key) -> Promise<Value | undefined>
    save(key, value) -> Promise<void>
```

A port can also be a **minimal structural subset** of a large third-party type — declare only the methods you use:

```
type MinimalFileHandle = pick(FileHandle, [read, write, close, truncate, stat])
```

#### Adapters

An adapter implements a port against one concrete technology. The core imports only the port; concrete tech sits at the edge that faces it — frameworks, SDKs, and platform APIs live in entrypoints and adapters, never in the core. The same port takes one adapter per technology — including one per side:

```
class SqlStore implements Store:          // backend adapter
    load(key): ...db query...
class LocalStorageStore implements Store: // frontend adapter
    load(key): ...read window.localStorage...
```

#### Composition root

A composition root constructs adapters and injects them into the domain; business and domain code never call `new` on an infrastructure class.

```
class CompositionRoot(overrides = {}):
    // partial overrides default to the real adapters — the seam a test
    // (or another runtime) swaps one dependency through
    store     = overrides.store ?? new RealStore()
    clock     = overrides.clock ?? new SystemClock()
    reminders = new ReminderService(store, clock)
    // the handle callers use:
    handle    = new WebServer(reminders)     // backend
    // handle = new ReminderApp(reminders)   // frontend
```

**Hand out domain handles — never a raw framework or vendor type.** A leaked framework instance or vendor client couples callers back to the technology the ports hide. The handle's shape follows the app — for example:

- service → `start()` / `stop()`, plus `handle(request)` for in-process tests
- UI mount (SPA or extension surface) → `mount()` / `unmount()`
- CLI / batch job → `run(args)`
- library → the use-case API (or a domain facade), handed out directly

#### Entrypoints

Ports/adapters are the **outbound** edge — tech the domain drives. **Entrypoints** are the **inbound** edge — what drives the domain. One per way into the app.

An entrypoint owns its **host** (lifecycle, plus the transport or mount it binds to), its **composition root**, and app-wide cross-cutting, then hands each inbound interaction inward to a slice's boundary; only the logic behind it stays transport-agnostic.

Match structure to need: an entrypoint starts as one file and earns a folder when it needs more than one. One entry sits in a singular `entrypoint/`; several go under `entrypoints/`, one named folder each. A split-out composition root takes the entrypoint's name (`WebServerCompositionRoot`, `PopupCompositionRoot`); a single entrypoint's is just `CompositionRoot`. No composition root composes composition roots — to run several in one process, a cumulative entrypoint starts their handles together (`await all(a.start(), b.start())`).

Each side's flavor: transport hosts in [architecture-backend.md](~/.claude/architecture-backend.md), mount surfaces in [architecture-frontend.md](~/.claude/architecture-frontend.md).

#### Injection seams

- **Constructor injection** — a class receives its dependencies, never constructs them (above).
- **Function option with a real default** — give a plain function a dependency option defaulting to the real implementation:

```
function append(path, data, options = {}):
    openFile = options.openFile ?? realOpenFile    // production default
    ...operate through the injected handle...
```

**Functional core, imperative shell** — a judgment call, worth more as the logic grows heavier or more critical, not for thin CRUD. Keep business logic a pure **core** (decisions from inputs — no I/O, clock, or mutation) wrapped by a thin **shell** that gathers inputs, calls the core, and enacts the result; a pure core tests trivially: data in, data out, nothing to fake.

### Depth matches complexity

An inbound interaction flows inward through a slice's layers; **add a layer only when the path earns it**. The roles are shared; each side names them and documents when each is earned:

```
backend  : entrypoint(host)  → controller → use case → core / domain  (→ ports)
frontend : entrypoint(mount) → page       → feature  → core / domain  (→ ports)
```

The heavier patterns each side can reach for — DDD and event-driven on services, state machines and event buses on the client — carry real cost; adopt them only when complexity justifies it.

## Validation

The inbound boundary checks **structure** — types, ranges, required fields — and rejects malformed input before deeper logic runs. Inner layers check **semantics** — state-dependent rules no schema expresses (sufficient balance, the record exists). The two failure modes stay distinct: malformed input is the caller's error, rejected at the boundary; a well-formed value naming nothing real is a not-found deeper in.

Validate structure with a **declarative schema** from a validation library — parse once into a typed value or a rejection. Never an unchecked cast, never a hand-rolled `typeof`/`in` chain where a schema fits. Define the schema **once** as the single authority and derive the static type from it; every surface checking the format delegates to it. At the edge, follow the robustness principle: liberal in what you accept (only the fields you use), conservative in what you emit.

## Decision guides

#### Where does this code go?

- Feature-specific behaviour → that feature's slice.
- Used by 2+ slices, no infrastructure → `shared`.
- Talks to external technology → a port (declared by the domain) + an adapter, in the slice that uses it.
- Global plumbing, or an adapter shared across slices → `infra`.
- Inbound host + composition root → `entrypoints`.

#### Tempted to couple to infrastructure? Do this instead

| Temptation | Instead |
|---|---|
| Domain imports the ORM / client / framework | Declare a port in the domain; implement it in an adapter |
| `new SomeAdapter()` inside the core | Inject via the constructor; the composition root constructs it |
| One slice imports another slice's internals | Depend on a published port, or emit an event |
| Global singleton / service locator | Pass the dependency explicitly from the composition root |
| Composition root returns a raw framework / vendor handle | Hand out a domain handle (a server's `start`/`stop`, a UI mount's `mount`/`unmount`) |
