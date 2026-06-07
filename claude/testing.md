# How We Write Tests

Examples below are language-agnostic pseudo-code. Patterns matter, not the tools.

**Applying this document.** Follow these when a project already does, or when starting a new one; otherwise match the project's existing test conventions — consistency within a project wins.

---

## 1. Core philosophy

1. **Test behaviour, not implementation.** Assert observable outcomes — returned values, stored state, messages sent outward — never which internal method was called. Then a refactor that preserves behaviour leaves the test green.
2. **Treat test code as production code** — same naming, type-safety, single-responsibility, and DRY rules; doubles and helpers included.
3. **No mocks.** Reach for fakes, drivers, and builders instead (see §3).
4. **Design for testability *is* design for production** — the ports and composition root that let a test inject a fake are what keep production code decoupled.
5. **Cheapest level first.** Push each behaviour down to the cheapest level that can prove it: edge cases and branches in fast unit tests; only wiring and real-infrastructure risk need the slow levels. A logic-less wrapper over a tool (a db client, a connection) earns no test of its own — its users' infra/e2e tests exercise it, and mocking the tool to "cover" it proves nothing.

---

## 2. Test taxonomy

| Level | Verifies | Real collaborators / infra | Doubles |
|---|---|---|---|
| **Unit** (classical school) | One unit of *behaviour* | Real in-process collaborators; **no** real I/O | Fakes only for awkward/external edges |
| **Integration — normal** | More than one unit wired together | In-process; still no external tech | Fakes at the true edges |
| **Integration — infrastructure** | One real external technology adapter (e.g. DB, filesystem, server, broker) | The real technology | Real infra; fakes elsewhere |
| **Integration — acceptance** (gherkin) | A business scenario through the public boundary | Whole system, real entry point | Fakes only at true external edges (e.g. email) |
| **Integration — e2e** | The whole system through its real front door | Everything real | As few fakes as possible |
| **Performance** | A latency / throughput NFR (e.g. p99 < 50ms) under load | Whole system under load | As few as possible — measure the real path |

**Classical-school unit:** a unit of *behaviour*, not a single class — real in-process collaborators used directly, doubles only at external/awkward boundaries (e.g. clock, network, disk, email).

### Priority and adoption

Start with unit; add levels as the project needs them — the mix, and whether a level exists at all, depends on the project. Priority (importance and adoption order, not volume):

**unit → acceptance → integration → e2e → infra → perf**

Unit is usually the most numerous; acceptance is the next priority (it proves the system works for the user); performance is last (NFRs, not correctness).

### Commands and file names

One script per level, each with its own config file, matched by filename suffix:

| Script | Files it runs |
|---|---|
| `test:unit` | `*.unit.test.<ext>` |
| `test:acceptance` | `*.accept.test.<ext>` (+ `.feature`) |
| `test:integration` | `*.integ.test.<ext>` |
| `test:e2e` | `*.e2e.test.<ext>` |
| `test:infra` | `*.infra.test.<ext>` |
| `test:perf` | `*.perf.test.<ext>` |

`test` runs every level in priority order; `test:ci` does the same with coverage, which no other command generates. `test:unit:watch` reruns the unit suite on save, the only watch you need; the rest are separated for convenience (run one while working on it) and don't all need real infrastructure (§4). On an existing project follow whatever suffixes it already uses (common variants: `.spec`, `.ispec`, `.steps`/`.e2espec`).

**Placement.** Unit tests sit beside the code they cover (`foo.unit.test.ts` next to `foo.ts`); every other test and all test utilities (fakes, builders, object mothers, drivers) live in a `tests/` folder beside `src/`, not per-feature `testing/` subfolders.

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
    withEmail(email): this.email = email; return this
    build(): return { id: nextId(), firstName, email, ... }
```

Two flavours: **in-memory** (`build()` returns a domain object — unit tests) and **persisting** (`build()` writes to a real store — infra/acceptance). A persisting builder gives each entity unique keys — a fresh email and id — so cases isolate (§4).

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
| Against real infrastructure (DB, filesystem, server) | shared global setup: load env, migrate schema, start the real infra | unique keys per case, else reset shared state; tear down owned resources |

For tests against real infrastructure:
- **Isolate by uniqueness** — give each case unique ids and temp paths so cases never collide and run in parallel.
- **Reset and run serially** only when state is unavoidably shared (truncate tables, delete temp files).
- **Clean up what the test created** (`afterAll`/`afterEach`): disconnect, stop the server, remove temp files.
- **Provision the real thing**, don't fake the boundary you're testing.

**Where it lives.** That shared global setup is one class under `tests/setups/`; each level that needs it registers a thin entry (e.g. `infraTestsSetup`) as its config's global setup.

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

## 6. Coverage

**One number, fed by every level (§2).** A line first reached by an acceptance or e2e test still counts — don't add a unit test just to hit it. Target **≥ 80%**, branches included — raise the bar per project; the build fails below it.

**Include all of `src`; don't hand-list globs.** They rot when files move — a dangling glob measures nothing while the gate stays green. Mark each exception in the file, with the tool's ignore directive and a reason:

```
/* coverage ignore file — generated client */
```

**Exclude only what no level should cover** — e.g. a type-only module or a constant table. A file that does real work earns a test, not an exclusion — expense and "run by operators" are no excuse. Same for block and method exclusions: for a paid wrapper, test the request/response mapping on sample payloads and exclude only the live call, behind an opt-in infra test.

---

## 7. Decision guides

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
