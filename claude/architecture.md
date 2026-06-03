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
    user/            # one vertical slice: api + logic + data for "user"
    billing/         # another slice
  shared/            # cross-slice primitives, no feature logic
  infra/             # process startup, server, db connection, composition root
```

- A slice may use `shared`, never another slice's internals; cross-slice needs go through a published port or an event.
- Keep a slice whole — its tests, types, and helpers live with it.

### Horizontal decoupling

Business logic must not depend on infrastructure (database, web framework, broker, external services, clock, filesystem) — dependencies point inward, never out.

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

An adapter implements a port against one concrete technology. The core imports only the port; database clients, web frameworks, and vendor SDKs are imported solely inside adapters and `infra`.

```
class SqlUserRepository implements UserRepository:    // production adapter
    save(user): ...orm insert...
```

#### Composition root

The single place that constructs every adapter and injects it into the domain; nothing else calls `new` on an infrastructure class.

```
class CompositionRoot(overrides = {}):
    userRepository = overrides.userRepository ?? new SqlUserRepository()
    emailService   = overrides.emailService   ?? new SmtpEmailService()
    webServer      = wire(new UserService(userRepository, emailService), ...)
```

The constructor takes **partial overrides defaulting to the real adapters** — the seam a test (or another runtime) uses to swap one dependency.

Keep its public surface at the domain level — expose `start()`/`stop()` or an app handle, never a raw framework type; a leaked `FastifyInstance` (or `PrismaClient`) couples callers back to the technology the ports hide.

#### Injection seams

- **Constructor injection** — a class receives its dependencies, never constructs them (above).
- **Function option with a real default** — give a plain function a dependency option defaulting to the real implementation:

```
function append(path, data, options = {}):
    openFile = options.openFile ?? realOpenFile    // production default
    ...operate through the injected handle...
```

**Functional core, imperative shell** — a context call, worth more as the logic grows heavier or more critical, not for thin CRUD. Decoupling taken further: keep business logic a pure **core** (decisions from inputs — no I/O, clock, or mutation) wrapped by a thin **shell** that gathers inputs, calls the core, and enacts the result; a pure core tests trivially: data in, data out, nothing to fake.

### Decision guides

#### Where does this code go?

- Feature-specific behaviour → that feature's slice.
- Used by 2+ slices, no infrastructure → `shared`.
- Talks to external technology → a port (declared by the domain) + an adapter.
- Wiring / startup → composition root in `infra`.

#### Tempted to couple to infrastructure? Do this instead

| Temptation | Instead |
|---|---|
| Domain imports the ORM / client / framework | Declare a port in the domain; implement it in an adapter |
| `new SomeAdapter()` inside a use case | Inject via the constructor; construct only in the composition root |
| One slice imports another slice's internals | Depend on a published port, or emit an event |
| Global singleton / service locator | Pass the dependency explicitly from the composition root |

---

## Code

Code-level design within a unit (general rules — naming, control flow, types, size — live in CLAUDE.md's Code Style).

### Abstraction layers

Keep each unit at one abstraction level — don't interleave a high-level, intent-revealing operation with the low-level mechanics it's built from; push those down a layer. Holds for any pair: policy over mechanism, orchestration over steps, domain over transport. A domain client over a generic transport is one case:

```
class HttpClient:                    // transport — protocol verbs only
    get(path), post(path, body), put(path, body)

class ApiClient (over HttpClient):   // domain — intent-named operations
    signUp(request):  post("signUp", request)
    signIn(request):  post("signIn", request)
```

Callers depend on `ApiClient`, not `httpClient.post(...)` — reaching down leaks a lower level up.
