# How We Write Tests

Examples below are language-agnostic pseudo-code. Patterns matter, not the tools.

**Applying this document.** Follow these when a project already does, or when starting a new one; otherwise match the project's existing test conventions — consistency within a project wins.

---

## 1. Core philosophy

1. **Test behaviour, not implementation.** Assert observable outcomes — returned values, stored state, messages sent outward — never which internal method was called. Then a refactor that preserves behaviour leaves the test green.
2. **Treat test code as production code** — same naming, type-safety, single-responsibility, and DRY rules; doubles and helpers included.
3. **No mocks.** Reach for fakes, drivers, and builders instead (see §3).
4. **Design for testability *is* design for production** — the ports and composition root that let a test inject a fake are what keep production code decoupled.
5. **Cheapest level first.** Push each behaviour down to the cheapest level that can prove it: edge cases and branches in fast unit tests; only wiring and real-infrastructure risk need the slow levels.

---

## 2. Test taxonomy

| Level | Verifies | Real collaborators / infra | Doubles |
|---|---|---|---|
| **Unit** (classical school) | One unit of *behaviour* | Real in-process collaborators; **no** real I/O | Fakes only for awkward/external edges |
| **Integration — normal** | More than one unit wired together | In-process; still no external tech | Fakes at the true edges |
| **Integration — infrastructure** | One real external technology adapter (DB, filesystem, server, broker) | The real technology | Real infra; fakes elsewhere |
| **Integration — acceptance** (gherkin) | A business scenario through the public boundary | Whole system, real entry point | Fakes only at true external edges (e.g. email) |
| **Integration — e2e** | The whole system through its real front door | Everything real | As few fakes as possible |
| **Performance** | A latency / throughput NFR (e.g. p99 < 50ms) under load | Whole system under load | As few as possible — measure the real path |

**Classical-school unit:** a unit of *behaviour*, not a single class — real in-process collaborators used directly, doubles only at external/awkward boundaries (clock, network, disk, email).

### Priority and adoption

Start with unit; add levels as the project needs them — the mix, and whether a level exists at all, depends on the project. Priority (importance and adoption order, not volume):

**unit → acceptance → integration → e2e → infra → perf**

Unit is usually the most numerous; acceptance comes next (it proves the system works for the user); performance last (NFRs, not correctness).

### Commands and file names

One script per level, matched by filename suffix; `test` aliases the unit suite:

| Script | Files it runs |
|---|---|
| `test` → `test:unit` | `*.test.<ext>` |
| `test:acceptance` | `*.accept.test.<ext>` (+ `.feature`) |
| `test:integration` | `*.integ.test.<ext>` |
| `test:e2e` | `*.e2e.test.<ext>` |
| `test:infra` | `*.infra.test.<ext>` |
| `test:perf` | `*.perf.test.<ext>` |

`test:unit` is fastest (no I/O) — the default, run on every save; the rest are separated for convenience (run one while working on it, all in CI) and don't all need real infrastructure (§4). On an existing project follow whatever suffixes it already uses (common variants: `.spec`, `.ispec`, `.steps`/`.e2espec`).

**Placement.** Unit tests sit beside the code they cover (`foo.test.ts` next to `foo.ts`); every other test and all test utilities (fakes, builders, object mothers, drivers) live in a `test/` folder beside `src/`, not per-feature `testing/` subfolders.

---

## 3. The anti-mock toolkit

Ports and the composition root are reused production patterns ([architecture.md](~/.claude/architecture.md)); fakes, drivers, builders, and object mothers are test constructs.

### 3.1 Ports

Tests substitute at **ports** — the domain's boundary interfaces. A fake implements the port; a minimal port (only the methods you use) keeps the fake small.

### 3.2 Fakes / in-memory implementations

A **working** in-memory implementation of a port — behaves like the real thing, not a stub returning canned values.

```
class FakeEmailService implements EmailService:
    sentEmails = []
    sendEmail(to, body): sentEmails.push({ to, body })

class InMemoryFileHandle implements MinimalFileHandle:
    content = initialContent ?? ""
    read(buffer, offset, length, position): ...slice from content...
    write(data, position):  content = splice(content, position, data); ...
    truncate(length):       content = content.slice(0, length)
```

Lets you assert on real resulting state, not on calls (no `when(...).thenReturn(...)` noise).

> A fake needs its own test only when its logic is non-trivial enough that a bug could pass silently or leave a failure ambiguous about which side broke — e.g. the in-memory file handle reimplements byte-offset read/write/truncate. Otherwise the tests using it already exercise it.

### 3.3 Drivers

A driver gives a test an intent-level handle on an interface with no domain abstraction of its own — a UI, a rendered page, a raw protocol — wrapping the low-level interaction (queries, clicks, waits) in domain-named methods.

```
// UI driver: the page/DOM has no domain abstraction, so the test builds one
class RegistrationDriver:
    constructor(screen)              // RTL render result, Puppeteer page, ...
    register(user):                  fill name / email / password; click "Sign up"
    shownProfileName():              return text of the profile header

driver.register(mother.defaultUser())
assert driver.shownProfileName() == "John Doe"
```

Reach for a driver only when no abstraction exists; when one does — say an `ApiClient` with `signUp`/`signIn` (see [architecture.md](~/.claude/architecture.md)) — use it directly.

### 3.4 Composition root

Tests reuse the composition root: build the real graph, overriding only the edge under test.

```
fake = new FakeEmailService()
system = new CompositionRoot({ emailService: fake })
```

For a function-level seam, pass the fake as the option:

```
await append("/irrelevant", { x: 1 }, { openFile: () => inMemoryFile })
```

### 3.5 Builders

Fluent test data with sane defaults, so each test states **only what it cares about** — named-constant defaults, deterministic values (fixed dates, monotonic ids).

```
class UserBuilder:
    DEFAULT_FIRST_NAME = "John"
    DEFAULT_EMAIL      = "john.doe@example.com"
    firstName = DEFAULT_FIRST_NAME; email = DEFAULT_EMAIL; ...
    withEmail(e): this.email = e; return this
    build(): return { id: nextId(), firstName, email, ... }
```

Two flavours: **in-memory** (`build()` returns a domain object — unit tests) and **persisting** (`build()` writes to a real store — infra/acceptance).

### 3.6 Object mothers

Canonical named instances of the common cases, plus transforms between shapes, built on top of builders. A mother gives *the* default case; a builder gives one that differs.

```
class UserMother:
    defaultUser():          return new UserBuilder().build()
    toSignUpRequest(user):  return { firstName: user.firstName, email: user.email, password: DEFAULT_PASSWORD }
```

### 3.7 Spies

**Spy** only to confirm an outward effect, on a fake or at a real boundary — never to stand in for the unit. Prefer asserting recorded state; spy only when no state reflects the effect.

```
// prefer: recorded state
assert fakeEmailService.sentEmails includes { to: user.email }

// spy only when no state shows it — e.g. batching (content identical; only the call count proves it)
assert inMemoryFile.write was called once
```

---

## 4. Setup strategy

| Test | Provisioning | Per-test isolation |
|---|---|---|
| On fakes / in-memory | none | none needed |
| Against real infrastructure (DB, filesystem, server) | shared global setup: load env, migrate schema, start the real infra | reset state between cases; tear down owned resources |

For tests against real infrastructure:
- **Reset shared state between cases** (truncate tables, delete temp files) so order never matters.
- **Run serially** when state is shared.
- **Clean up what the test created** (`afterAll`/`afterEach`): disconnect, stop the server, remove temp files.
- **Provision the real thing**, don't fake the boundary you're testing.

```
afterAll: remove all temp files this suite created

test "appends an entry to a real file":
    path = makeUniqueTempPath()
    await append(path, { newKey: "newValue" })          // real FS
    assert JSON.parse(read(path)) == [ { newKey: "newValue" } ]
```

Performance tests run the real system under load (autocannon / k6) and must **fail when the NFR threshold is breached** — drive the endpoint the NFR names, not a cheap proxy. A benchmark that only reports numbers, or hits the wrong target, verifies nothing.

---

## 5. Acceptance tests

Acceptance composes the whole toolkit: a scenario, the real system via the composition root, a fake only at the true edge, the domain API client to act.

Write the scenario in **gherkin** (Given/When/Then) in a `.feature` file and bind the steps with a cucumber-style library — add it as a dependency if the stack has one but it isn't installed; fall back to plain Given/When/Then blocks only when the stack genuinely has none. The `.feature` file and its step definitions can't drift apart.

```
Feature: Registration
  Scenario: Successful registration
    Given I am a new user
    When I register with valid account details
    Then I am granted access to my account
    And I receive a welcome email
```

```
fake   = new FakeEmailService()
system = new CompositionRoot({ emailService: fake })   // real everything except email
api    = new ApiClient(system.baseUrl)

beforeAll: system.webServer.start()
afterAll:  system.webServer.stop()

Given "I am a new user":            user = mother.defaultUser()
When  "I register...":              api.signUp(mother.toSignUpRequest(user))
Then  "I am granted access":        api.signIn(mother.toSignInRequest(user), { storeToken: true })
                                    assert api.getProfile().email == user.email
And   "I receive a welcome email":  assert fake sent an email to user.email
```

---

## 6. Decision guides

### Where does this test go?

- No real I/O → **unit** (inject an in-memory fake through a seam if it touches a boundary).
- Exercises one real external technology → **infrastructure**.
- Several units wired in-process, no external tech → **normal integration**.
- A business scenario through the public boundary → **acceptance**.
- The whole deployed system through its real entry point → **e2e**.
- A latency / throughput threshold under load → **performance**.

When several fit, default to the lowest.

### Reaching for a mock? Do this instead

| Temptation | Instead |
|---|---|
| Mock the email sender, assert it was called | `FakeEmailService` implementing the port; assert it **recorded** the email |
| Mock a repository to return a user | In-memory repository fake, a persisting builder, or just pass a built domain object |
| Mock the filesystem / network handle | Hand-written in-memory fake of the minimal port, injected via a seam |
| Stub HTTP responses for a client | Run the real system from the composition root and drive it through its real interface (API client or UI driver) |
| Spy to assert an internal method ran | Assert the **observable outcome** — returned value, stored state, or message sent |
| Re-declare inline literals in every test | A **builder** over an **object mother** |
