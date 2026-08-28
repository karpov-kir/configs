# How We Write Tests

Follow this on a new project or one that already does; otherwise match the project's existing test conventions.

## 1. Core philosophy

1. **Test behaviour, not implementation.** Assert observable outcomes — returned values, stored state, messages sent outward — never which internal method was called. **Bind the test to the module's published surface** ([architecture/core.md](architecture/core.md) → **Module depth**).
2. **Treat test code as production code**, doubles and helpers included.
3. **No mocks.** Reach for fakes, drivers, and builders instead (§3).
4. **Cheapest level first.** Push each behaviour down to the cheapest level that can prove it: edge cases and branches in fast unit tests; only wiring and real-infrastructure risk need the slow levels.
5. **A test that reimplements the behaviour it asserts agrees with itself, not with the code.** Import the real thing and drive it.
6. **A suite slow enough to schedule around stops being run.** Under a second is the working bar; tens of seconds is the ceiling, and past it a pass starts deferring the suite to "later" and shipping on the assumption it would have passed. The cost is almost never the assertions: it is process spawns and fixture I/O, and both come out — build fixtures in-process and call the code instead of executing it. **Measure where the time goes before optimising anything**, because the answer is rarely where it feels.

7. **A negative control proves three things, and only the third is ever checked** ([core-principles.md](core-principles.md) → **5. Verify the effect, not the report of it**): that the mutation *applied*, that the code still *builds*, and that the failure names the case you meant. A replacement whose whitespace does not match the file never lands, so the run tests unmutated code and reports green. A mutation that leaves a variable unused does not compile, and `FAIL … [build failed]` reads near enough to a red test to accept at a glance. And a case can go red for the wrong reason — a fixture that fails earlier than the branch under test passes its control while testing nothing, which only a failure message specific enough to betray it will show. **A harness reporting that everything survived is far likelier broken than the code is bulletproof** — a survivor is supposed to be a finding, so a broken harness manufactures findings that look like rigour.
8. **Two cases a scenario list routinely misses.** The non-ASCII or special-character one, wherever code lists or round-trips a name from outside the system. And where the deliverable is a **mapping**, every row of it — produce the full table, input to resulting state, and validate each row rather than the ones a scenario happened to name.

## 2. Test taxonomy

| Level | Verifies | Real collaborators / infra |
|---|---|---|
| **Unit** (classical school) | One unit of *behaviour* | Real in-process collaborators; **no** real I/O |
| **Integration — normal** | More than one unit wired together | In-process; still no external tech |
| **Integration — infrastructure** | One real external technology adapter (e.g. DB, filesystem, server, broker) | The real technology |
| **Integration — acceptance** (gherkin) | A business scenario through the public boundary | Whole system, real entry point |
| **Integration — e2e** | The whole system through its real front door | Everything real |
| **Performance** | A latency / throughput NFR (e.g. p99 < 50ms) under load | Whole system under load |

Fakes stand only at awkward or external edges (clock, network, disk, email), and fewer of them as the level rises — e2e and performance measure the real path. Start with unit and add levels as the project needs them.

### Commands and file names

One script per level, each with its own config file, matched by filename suffix: `test:unit` → `*.unit.test.<ext>`, `test:acceptance` → `*.accept.test.<ext>` (+ `.feature`), `test:integration` → `*.integ.test.<ext>`, and likewise `test:e2e`, `test:infra`, `test:perf`. Plus `test` (every level) and `test:ci` (the same with coverage — no other command generates it).

**Placement.** Unit tests sit beside the code they cover (`foo.unit.test.ts` next to `foo.ts`); every other test and all test utilities (fakes, builders, object mothers, drivers) live in a `tests/` folder beside `src/`, not per-feature `testing/` subfolders.

## 3. The anti-mock toolkit

Ports and the composition root are reused production patterns ([architecture/core.md](architecture/core.md)); the rest are test constructs.

- **Ports** — substitute at the domain's boundary interfaces.
- **Fakes** — a *working* in-memory implementation of a port, never a stub returning canned values. Give a fake its own test only where a bug in it could pass silently.
- **Drivers** — an intent-level handle on an interface with no domain abstraction of its own (a UI, a rendered page, a raw protocol), wrapping queries, clicks and waits in domain-named methods.
- **Composition root** — build the real graph, override only the edge under test; for a function-level seam, pass the fake as an option.
- **Builders** — fluent test data with named-constant, deterministic defaults, so a test states **only what it cares about**. In-memory `build()` returns a domain object; a persisting one writes to a real store with unique keys per entity (§4).
- **Object mothers** — canonical named instances built on builders, plus transforms between shapes. A mother gives *the* default case; a builder gives one that differs.
- **Spies** — only to confirm an outward effect, never to stand in for the unit. Prefer asserting recorded state; spy only where no state reflects the effect, e.g. batching.

## 4. Setup strategy

Tests on fakes need no provisioning and no isolation. Tests against real infrastructure (DB, filesystem, server) take a shared global setup — load env, migrate schema, start the real infra — plus per-case isolation: unique ids and temp paths, else reset and run serially. Tear down what the test created.

**Where it lives.** That shared global setup is one class under `tests/setups/`; each level that needs it registers a thin entry (e.g. `infraTestsSetup`) as its config's global setup.

**A fixture that denies access must not rest on a permission bit.** Root ignores mode bits, and CI runs as root often enough that `chmod 000` builds nothing there — the case then asserts against a file the process reads happily, and reports either a confusing red or a pass earned by something other than its name. Build the refusal so that no user bypasses it: a path that does not exist, a directory where a file is required, a dangling symlink, a path component that is not a directory. Where the behaviour under test *is* the readability check, none of those substitute — they are refused a limb earlier and never reach it. Probe whether the mode really denies this process, and skip with the reason stated rather than assert. Probe rather than read `id -u`: `CAP_DAC_OVERRIDE` without root, and a filesystem that drops the bit, behave the same way.

**Silence logs.** Default the logger to its off level (`LogLevel.Silent`) in the global setup; raise it in a single test only when debugging needs the output.

Performance tests run the real system under load (autocannon / k6) and must **fail when the NFR threshold is breached** — drive the endpoint the NFR names, not a cheap proxy.

## 5. Acceptance tests

Acceptance composes the whole toolkit: the real system via the composition root, a fake only at a true external edge, the domain API client to act. Write the scenario in **gherkin** (Given/When/Then) in a `.feature` file and bind the steps with a cucumber-style library — add it as a dependency if the stack has one but it isn't installed; fall back to plain Given/When/Then blocks only when the stack genuinely has none.

## 6. Coverage

**One number, fed by every level (§2)** — don't add a unit test to hit a line an acceptance or e2e test already reached. Target **≥ 80%**, branches included — raise the bar per project; the build fails below it.

**Include all of `src`; don't hand-list globs.** Mark each exception in the file itself, with the tool's ignore directive and a reason.

**Exclude only what no level should cover** — e.g. a type-only module or a constant table. A file that does real work earns a test, not an exclusion.
