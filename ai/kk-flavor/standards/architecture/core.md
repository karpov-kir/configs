# How We Structure Code

Follow this on a new project; otherwise match the project's existing architecture.

## Three axes

### Vertical slicing

Organize the top level by feature, not by technical layer. A **slice** is one feature's folder, owning its boundary, business logic, and data access together, so a feature change stays in one place. The top level: `features/` (the slices), `shared/` (cross-slice primitives, no feature logic), `entrypoints/` (inbound edges), `infra/` (cross-slice outbound plumbing and adapters).

- A slice may use `shared`, never another slice's internals; cross-slice needs go through a published port or an event.
- Keep a slice whole — its adapters, tests, types, and helpers live with it.

### Horizontal decoupling

Business logic must not depend on infrastructure — a database, a broker, an HTTP API, a UI framework, browser storage, the clock, the filesystem, … Dependencies point inward, never out.

**Functional core, imperative shell** — the layer that drives the ports hands values inward; the core computes and returns rather than awaiting I/O mid-decision.

#### Ports and adapters

A port is an interface at a boundary to external technology, declared by the domain in its own terms. It can also be a **minimal structural subset** of a large third-party type — declare only the methods you use.

An adapter implements a port against one concrete technology, and the same port takes one adapter per technology. The core imports only the port; frameworks, SDKs, and platform APIs live in entrypoints and adapters, never in the core.

#### Composition root

A composition root constructs adapters and injects them into the domain; business and domain code never call `new` on an infrastructure class. Its constructor takes partial overrides that default to the real adapters, and it exposes the handle callers hold. Those overrides are the seam a test, or another runtime, swaps one dependency through. For a plain function, the equivalent seam is a dependency option defaulting to the real implementation.

**Hand out domain handles — never a raw framework or vendor type**, which couples callers back to the technology the ports hide. The shape follows the app: `start()` / `stop()` plus `handle(request)` for a service, `mount()` / `unmount()` for a UI mount, `run(args)` for a CLI or batch job, the use-case API for a library.

#### Entrypoints

Ports and adapters are the **outbound** edge — tech the domain drives. **Entrypoints** are the **inbound** edge — what drives the domain, one per way into the app. An entrypoint owns its **host** (lifecycle, plus the transport or mount it binds to), its **composition root**, and app-wide cross-cutting, then hands each inbound interaction inward to a slice's boundary; only the logic behind it stays transport-agnostic.

One entrypoint sits in a singular `entrypoint/`; several go under `entrypoints/`, one named folder each. No composition root composes composition roots — to run several entrypoints in one process, a **cumulative entrypoint** named for the deployment (`startAll.ts`, `web.ts`, `worker.ts`) starts their handles together (`await all(a.start(), b.start())`).

### Module depth

A module's **interface** is everything a caller must learn to use it — its exports, their types, and the constraints only prose carries. Its **depth** is the functionality it hides divided by that interface. Deep is the goal, and depth is not size — **splitting a file is free, exporting the halves is not.**

- **Publish one surface per module.** A slice or module declares its boundary in one file: the handle, port, or facade a caller holds. Everything else is internal and nothing outside imports it. Name that file `exports.ts` (or the language's equivalent), never `index.ts` — an implicitly-resolved name hides which file the caller is actually reading, and it is what turns a declared surface back into the re-export barrel [code-style.md](../code-style.md) → Extraction & Size bans.
- **Adding capability inside must not widen the surface.**
- **No pass-throughs.** A method that mostly forwards to a same-named method one layer down costs interface and hides nothing.
- **No required sequences.** An interface the caller must drive in order — construct, then configure, then start — is shallow. Hide the order inside, or model the states so the wrong order can't be expressed.
- **The surface must stand alone.** Someone reading only the surface file can use the module correctly, with no other file open. The types carry what they can; the contract prose beside them carries the rest ([code-style.md](../code-style.md) → Comments exempts that prose from the no-comment default).

## Per-side specifics

An inbound interaction flows inward through a slice's layers; add a layer only when the path earns it.

```
backend  : entrypoint(host)  → controller → use case → core / domain  (→ ports)
frontend : entrypoint(mount) → page       → feature  → core / domain  (→ ports)
```

**Backend.** Default to **CQRS** — reads and writes take separate paths. No `main.ts`. Tests reach the service through `handle(request)`, which delegates to the in-process dispatch a real request takes (`fastify.inject` / `hono.fetch`). A use case earns its place at 2+ boundaries or when it spans a unit of work (one transaction across several repositories or aggregates); otherwise skip it. Put a read behind a port only where a fake adds value. Schema library: zod / valibot (TS), pydantic (Python).

**Frontend.** Server cache — data mirrored from the API — goes to a data-fetching library (TanStack Query, a Solid resource) behind a port, never hand-rolled in components; UI state (open/closed, selection, draft) stays in component-local signals. A server semantic rejection (409/422) maps onto the offending field, not a generic toast.

## Validation

The inbound boundary checks **structure** — types, ranges, required fields — and rejects malformed input before deeper logic runs. Inner layers check **semantics** — state-dependent rules no schema expresses (sufficient balance, the record exists).

Validate structure with a **declarative schema**, parsed once into a typed value or a rejection — never an unchecked cast or a hand-rolled `typeof`/`in` chain. Define the schema once as the single authority, derive the static type from it, and delegate every format check to it.

## Logging & events

**Logging — the one ambient exception to injection.** A logger is reached directly, not threaded through constructors: nothing branches on it and no test asserts it. One root logger, scoped per slice/feature so each line carries its source, and the logger filters by level; level set once from config ([project.md](../project.md) → Logging). Backend: a logging library (pino / winston — TS; structlog — Python). Frontend: the ready-to-go scoped logger in [Logger.ts](Logger.ts).

**Events — injected like any port.** Notification goes through a typed pub-sub, fire-and-forget, injected by the composition root.

- **Dynamic 1:N fan-out** — one source emitting to a subscriber set that varies at runtime — *is* pub-sub: use one even inside a single slice and even at the first site, rather than hand-rolling a listener `Set` plus notify loop. Without that fan-out, a fixed 1:1 call inside a slice is simpler.
- **The owner hides it.** Hold it as a private `pubSub` field and expose one domain verb pair per channel: bound `onStateChanged`/`offStateChanged` fields typed `ChannelSubscriber<Payload>`. Consumers call `owner.onStateChanged(handler)` and never import the pub-sub.
- A ready-to-go typed `PubSub<ChannelMap>` ships in [PubSub.ts](PubSub.ts); **vendor the slice you need** into the project's `shared`, adapt the `Logger` seam, and head it with a provenance line — source repo plus commit. Cross-process delivery, durability, retries, or asynchronous handlers need a real broker.
