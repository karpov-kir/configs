# Ideas

Deferred proposals, not active agent instructions. Keep at most 20 open ideas; review and consolidate this backlog when the owner requests a review or before exceeding that limit.

## 3x | 2026-09-14 | Type every edge, then collapse the tooling that reads them

`--graph` prints `reads` for 27 edges because a path citation carries no kind. `cite-graph` prints 10
cycles and judges none, because nothing says which files are layered. Three packages — `eco-check`,
`eco-guide`, `cite-graph` — each parse citations with their own regexes. The cost of that showed up
the moment `**Extends:**` landed: `idsd-ship` ran four contracts inline, declared one, and billed
1 row where it reached 12. Nothing in the tree could have caught it.

**One copy is the rule the whole design follows from.** A declaration beside prose that says the same
thing is the form [model-policy.md](ai/kk-flavor/standards/model-policy.md) → **One row per skill, one
per dispatch site** already refuses — *a line beside the prose can forget to mention itself*. So the
declaration is the instruction, the sentence that used to carry it is cut
([ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Prefer the mechanism**), and there is nothing
left for a validator to find false. That is what removes the "is the label true" problem rather than
answering it with a second checker.

**Declare only what cannot be derived**, measured against the tree: a `workers/` path citation is a
dispatch because a worker prompt has no other use (58 today); a `skills/X` citation where X holds a
`workers` row is a dispatch because the row is the declaration; a `standards/` citation is always a
read. What nothing can know is whether X runs *inside* this session — 31 edges across 17 files.

**Two nets find the sites a declaration is owed at, and the first needs no word list.** A citation of
another skill's *whole* `SKILL.md` is an instruction to open and run that contract; a citation of a
section of it is a pointer at one passage. That is [ecosystem.md](ai/kk-flavor/standards/ecosystem.md)
→ **One home**'s own reading of what a citation costs, applied to the one question the text cannot
answer. Measured against the sixteen declarations: fifteen cite a whole contract; one whole-contract
citation is not an extension — `kk-reduce`'s brief, which tells *other* agents to run `kk-ecosystem`;
and two declarations cite only sections. **Citation shape cannot rot**, because there is no list to
forget to extend, which is the whole failing of the net below. Part of the fit is convention this
section created rather than discovered — the first step converted bare names to paths — and that is a
reason to hold it with a check, not to trust it.

**The verb rule is the second net, for a bare name carrying no path at all.** Measured: 20 such sites
outside table cells, against 130 bare skill names in prose. Banning bare names entirely was costed and
rejected — it would bloat 27 files to satisfy a parser, and would bill every skill for every neighbour
it names. The verb rule caught every `idsd-ship` miss and nine more undeclared extensions, which is
what took the tree from seven declarations to sixteen; `idsd-finalize` was the largest, able to re-run
the whole qualify pass and billing 1 row where it reached 11. **It has to carry `through` and `via`** —
nine sites use those and no listed verb, six of them undeclared extensions — and it will still never
be complete, because no closed set of English verbs is. A missed verb fails silently, which is exactly
how those six sat in plain sight, and is why this is the narrower net rather than the only one. A
table cell is exempt structurally rather than by naming skills, which covers `kk-foreman`'s Route and
`kk-qualify`'s Lanes without an exemption list that can rot.

**Standards get a layer instead of typed edges** — one declaration per file, 18 of them, not an edge
census. Three layers read out of the citation graph: **base** (core-principles, live-systems, writing,
human-writing, code-style), **craft** (testing, architecture/core, project, building, git, records,
browser), **process** (skill-protocol, quality-pipeline, model-policy, ecosystem, streaming). Almost
every current cycle is inside a layer, which is peers cross-referencing and not a defect by
`cite-graph`'s own header. `writing → human-writing → code-style → ecosystem → writing` crosses base
into process and is a real finding to fix. **Do not tune the layers until the cycles pass** — layers
chosen to make today's tree legal measure nothing.

The steps, each revertible alone. The first has landed: the grammar is `**Extends:** <skill> —
<when>`, sixteen edges across eight skills declare one, and no prose states an edge any more.

1. **`**Layer:**` on the 18 standards**, and the one cycle that crosses.
2. **The checks in `eco-check`**, on the citation parser it already has: both nets above, an
   extension cycle, a downward layer citation, and every `**Extends:**` held to a `**Runs:**` reason
   that licenses holding the work. Lands the inline worker read and the exception inventory below.
   **Neither net decides the edge** — each only says a human must have answered here, and the
   declaration is the answer.
3. **Collapse the three citation parsers.** `eco-guide/graph.go` and `cite-graph/read.go` drop their
   own regexes; `--graph` and `--cost` read declarations and the prose inference is deleted;
   `cite-graph`'s CYCLES becomes a verdict rather than a list.
4. **Audit all 22 Go packages for consolidation.** A campaign, not a pass — `eco-check` alone is 17
   files, and `eco-stats`, `rule-echo`, `eco-report` and `eco-guide` all walk the same tree. Run it
   after the grammar lands, so the shared reader exists to consolidate onto.
5. **Then `workers/build/implement.md`**, written under the grammar from the start.

**Six sites read a priced lane inline, and nothing declares it.** `idsd-qualify`, `idsd-finalize`,
`kk-pr` and `kk-ecosystem` each apply `kk-edit` inline; `kk-pr`'s review mode and `kk-build` read
`workers/conform.md` the same way. Each of those rows selects nothing and the work bills at the
caller's tier — the defect the worker layer exists to remove — while `--graph` prices all six as
dispatches, so the tool and the prose disagree today. `**Extends:**` cannot reach them: `kk-edit`
holds a `workers` row and a worker is not a skill.

**It is the same relation, so it takes the same declaration.** Reading `workers/conform.md` inline is
one session reading a second contract and running it in its own — `**Extends:**`'s definition
exactly — and only the target's shape differs. So the grammar's target widens to a worker path, and
`--cost` stops pricing a declared read as a dispatch: the row it shows is the caller's, which is what
the run actually pays. The check then holds each declaration to the declaring skill's own `**Runs:**`
reason, so a claim is never better than the reason beside it.

**Measured against that reason, not one of the five is licensed today.** `idsd-qualify` declares
`orchestrator`, `idsd-finalize` and `kk-pr` declare `landing`, `kk-ecosystem` declares `dispatched`.
The reason they all *give in prose* is that the text is already in the session, which is
`session-context` — a reason [model-policy.md](ai/kk-flavor/standards/model-policy.md) → **Cost is a
design constraint** defines and none of them claims. So each is a `**Runs:**` reason to correct or,
by that same section's *a reason that covers part of what the skill does is the offload signal*, work
that should be dispatched after all — and `kk-ecosystem`, being `dispatched` itself, is almost
certainly the latter. **Write the check first and resolve the five against it, one at a time.**
Growing the exception in [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Three kinds, two
homes** to admit `session-context` may then turn out to be unnecessary: it admits `converses` today,
which already covers `kk-build`, the only site that was ever legal.

**An exception lives in the file that takes it, and the inventory of them is generated.** A
hand-maintained list of which skills may read a lane inline is a second copy of what the skill already
says, and nothing can catch it going stale — the failure this whole section exists to remove.
`models.json` is central precisely because it is the *only* copy: a skill never names its model. An
edge kind cannot move that way, because the agent reading that phase acts on it and will not open a
config mid-task. **The test is who acts on it** — tooling only, and it belongs in a config file;
the reading agent, and it belongs in the file. What the central file is for is review, and that is
met by generating it: `guide.sh` emits `ai/declarations.md`, one line per declaration and per
exception, committed and held by the `--check` that already guards `field-guide.html`. Enforcement
comes from the check, unification from the single parser step 3 leaves, visibility from a file nobody
writes by hand.

**A receipt is the only thing that cannot be wrong, and nothing records one.** `--cost` is a ceiling
computed before a run; what no tool reaches is which contracts a session actually loaded. That is the
one check a wrong declaration cannot survive, and it is also the only way to answer whether the tier
changes saved anything — the question below has been open for four steps. It needs a recording
mechanism that does not exist, so it waits until the declarations it would audit are in the tree.

## 1x | 2026-09-10 | Two things left over from the worker layer

The layer landed in four steps over 2026-09-10..14: `workers/` as a home of its own, seven skills
moved into it, `**Runs:**` split into `orchestrator` / `holds — <reason>` / `dispatched` with the
orchestrator tier ceiling, and `guide.sh --graph` / `--cost <skill>` as the readable cost surface. The
reasoning is in the commits; [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Three kinds, two
homes** and [model-policy.md](ai/kk-flavor/standards/model-policy.md) → **Cost is a design constraint**
are where the rules live now. Two things were deliberately left out of that stack.

**`workers/implement.md`.** `kk-build`'s Phase 4 is the largest block of model work in the tree with no
human in it, and it is still inline. Offloading it makes `kk-build` an orchestrator: its Phase 2 asks
one message per stack choice and its Phase 5 presents for approval, and both are relays rather than a
conversation, so `converses` stops holding once the loop leaves. The ceiling then takes its row down.
**It does not do the same for `idsd-build`** — that skill's Phase 1 recomputes what remains open
between rounds, which is `converses` by the letter of the rule, so after this change its opus is
bought by its own conversation rather than by an inline build loop. Whether Phase 1 alone earns opus
is a separate question with no evidence either way. It lands alone and after the structure, because it
is the only change that alters how building feels and it should be revertible without unpicking
anything else.

**What the tier changes actually save is still unmeasured.** Four steps in, the saving is an argument
from the file rather than a number: it needs a real quality pass run before and after, not an estimate.
`guide.sh --cost` now makes the structure readable, which is the nearest thing to evidence so far and
is not evidence. Measure it before there is a fifth step to argue about.

## 2x | 2026-09-08 | Local model helpers with agent fallback

Status: the central model policy now exists; local inference still needs an acceptance benchmark before integration. The follow-up inspection found no Ollama, llama.cpp server or MLX server on PATH, and no Ollama or LM Studio application at their usual installation paths. No runtime or model was installed, and no inference benchmark was run. The owner asked to be contacted before installation.

The inspected machine is a MacBook Pro with an Apple M2 Max and 32 GB of unified memory, verified through system_profiler. This is a plausible machine for a small quantized helper model. Start with bounded text tasks, not implementation, correctness/security review, instruction semantics, or decisions that authorize skipping those lanes. Those roles retain the original task’s selected model.

### Hardware and model scope

Start by testing one 4–9B model at roughly 4-bit quantization, short inputs, and one concurrent request. A 14B model is a comparison candidate if quality justifies its extra memory and latency. Ideal weight storage is approximately parameters × bits / 8: about 4 GB for an 8B model at 4 bits, 7 GB for 14B, and 16 GB for 32B. These are weight-only arithmetic estimates, not download sizes or measured runtime requirements. Quantization metadata, unquantized tensors, KV cache, buffers, macOS and development tools need additional memory. A 32B model might fit some configurations, but is a poor first choice alongside active development on this 32 GB machine.

The current first comparison candidates are Qwen3.5 4B and 9B in Ollama's Q4_K_M packages, listed at 3.4 GB and 6.6 GB respectively. These are package sizes, not runtime memory estimates. Start with the smaller candidate; test the larger only if its quality could justify the added resources. Pin the actual artifact digest, quantization and runtime version in the experiment record. Model-card benchmarks do not establish quality or speed for these pipeline tasks on this Mac. [Qwen3.5 4B](https://ollama.com/library/qwen3.5:4b), [Qwen3.5 9B](https://ollama.com/library/qwen3.5:9b).

### First tasks

| Task | Local role | Acceptance condition |
|---|---|---|
| Extract fields from a supplied passage | Bounded extraction | Validate schema and source-backed values |
| Rewrite a short draft | Editorial proposal | Preserve names, numbers, negation, commitments and exceptions |
| Summarize supplied failure logs | Navigation aid | Keep evidence references; do not certify the root cause |
| Suggest duplicate prose or labels | Candidate generation | Treat suggestions as proposals, not deletion or skip authority |

Use deterministic Go code for hashes, routing known states, formatting, schema checks and process waiting. Local inference is not a reason to replace simpler tools. Do not preserve unnecessary three-vote communication checks merely because a local model makes them cheaper. Do not make the local model an autonomous coordinator in the first experiment.

### Runtime choice

Try Ollama first for integration simplicity: a local HTTP endpoint lets the same Go adapter serve Claude and Codex without embedding an inference engine. Its chat API supports schemas, thinking controls for supported models, keep-alive, and timing/token counters. Validate the returned JSON independently; valid JSON does not prove factual correctness. [Chat API](https://docs.ollama.com/api/chat), [structured outputs](https://docs.ollama.com/capabilities/structured-outputs).

Compare MLX-LM if Apple-silicon performance becomes the limiting factor; it supports local generation and quantization but adds Python environment management. llama.cpp is another option with Metal acceleration and a server interface. Keep the Go boundary independent of the runner so this comparison does not require rewriting workflow logic. [MLX-LM](https://github.com/ml-explore/mlx-lm), [llama.cpp](https://github.com/ggml-org/llama.cpp).

Bind to loopback and select an explicitly local artifact. Ollama also supports cloud models, so a localhost URL alone does not guarantee local inference. Its local-only setting, OLLAMA_NO_CLOUD=1, disables cloud features. Concurrent contexts consume additional memory; start with one loaded model and one inference slot. [Ollama configuration](https://docs.ollama.com/faq).

### Go interface and fallback

The useful interface is role-based rather than a model name supplied at every call:

```text
TryLocal(ctx, task{role, input}, policy)
  -> Completed{output, model_digest, timings}
   | NeedsAgent{reason, task_id}
```

The shared model configuration would map an eligible role to a preferred local profile and a fallback policy. Model IDs stay there alongside Claude/Codex profiles. Endpoint, memory/concurrency limits and deadlines are runtime configuration, not repeated skill prose. Protected roles never gain local routing through a global default.

The adapter checks role eligibility, endpoint health, model presence, input/context limits, resource availability and the request deadline. Availability means more than a running server. Do not download a model or wait through a long queue in the request path. On timeout, cancel or drain the local attempt before releasing its inference slot; do not assume closing an HTTP connection stopped GPU work. Validate the completed response and return a specific fallback reason when the attempt fails. Avoid repeated local retries after an unavailable or unsuitable model.

There are two different fallback mechanisms:

* Inside a Claude/Codex task, NeedsAgent returns control to that task. The caller completes the bounded work itself or dispatches its configured cloud helper. A Go function cannot synchronously invoke the current conversation’s reasoning without an explicit callable interface. The local failure need not spawn a separate cloud session.
* A standalone Go runner can call a configured provider API or CLI adapter and return its answer synchronously. That is a separate inference/session, with its own authentication, usage and supplied context. It is not the currently running agent and does not inherit its conversation automatically.

Cloud fallback must be explicit in the role policy. Local-only inputs must return a blocked/deferred result instead of being sent off-device. Preserve the original task model when fallback requires protected judgment. Record the selected backend and reason; a model claiming confidence is not an acceptance check.

### Economics and later experiment

Local inference removes provider inference charges for successful local work, but consumes RAM, power, thermals and time. It can compete with builds and other applications. A cold model load or long local attempt followed by cloud fallback can be slower and more expensive overall. Keep a model warm only across a useful burst, then unload it; measure plugged-in and normal-development conditions before choosing residency or battery policy.

Measure cold/warm latency, peak memory pressure and swap, impact on concurrent builds, output acceptance, fallback rate and provider usage avoided. Sequential latency is approximately local-attempt time plus fallback probability × cloud time. When a cloud coordinator still spends turns constructing and checking every local query, those cloud costs remain. Savings are most plausible for repeated bounded work whose results can be accepted without a complete second cloud pass.

Later trial: use 30–50 representative extraction/editing/log tasks with known expected outcomes, including lost-negation and altered-number cases. Compare the local candidate with the configured cloud helper, then test unavailable server, missing model, busy queue, oversized input, malformed output and timeout/cancellation. Separately evaluate semantic errors that pass schema validation. Keep the feature disabled unless it preserves required quality and improves measured cost or responsiveness under ordinary machine load.

Standing cost: model downloads of several GB, a local runner to update, memory residency while active, one Go adapter and a maintained evaluation set. Prepare the acceptance cases before requesting installation; build the production adapter only after the benchmark earns it.

## 1x | 2026-09-08 | Reusable gate evidence

Typed stage ingestion shipped; `idsd-qualify/stage-results.md` is its procedure. What it bought was fewer report calls and stronger result accounting, not lower latency — coordinator turns, provider tokens and real-project savings stayed unmeasured, and a three-trial comparison had median elapsed time rising from 7.24s to 9.17s on a tiny fixture.

Gate reuse is what remains, and it is deferred. Finalize still reruns build gates because report stamps hold no command, tool, dependency or environment evidence that could justify avoiding them. Keep those reruns until an explicit receipt establishes equivalent inputs and a successful real execution. Store evidence in scratch, separate from human project records. Include commands, working directory, exit status, candidate/dependency identity, relevant tool/configuration identity and logs without secrets. External or otherwise unbounded inputs require rerunning the gate.

Before building gate reuse, measure repeated gates in representative releases. Compare saved runtime with receipt validation and maintenance costs; exercise changed commands, dependencies, toolchains and environments as negative controls. Keep local-model evaluation separate from this decision.

Standing cost of typed ingestion: a versioned format, retained manifests, caller migration and crash-recovery tests. In-flight releases use a frozen old tool bundle until completion; new passes use the typed contract. No scheduler, provider runtime or gate cache was added.

## 2x | 2026-09-11 | Where the gate's remaining wall clock is, and what not to try

A cold `ai/gate.sh --full` is its default lane end to end: the shell lane runs alongside and finishes inside it, so speeding the shell suites up does not move the gate at all. After grouping the mutation units by suite set, one unit is most of the whole run — two cold `--full` runs put `mutants:go:eco-report` at 630s of 1357s and at 658s of 1194s, 46% and 55%, where the next largest is `mutants:go:eco-check` at 163–174s and nothing else exceeds 60s. Any further reduction lives in that one unit, which means in the eco-report suite it runs as its baseline, not in the gate's own scheduling.

Three things not to try. **Widening the gate's lanes gains nothing**: `go-mutate` already runs its mutants `NumCPU-2` wide, so overlapping mutation units over-subscribes the machine rather than filling it, and the non-mutation checks total single-digit seconds warm. **`serialGroupFor`'s shell/non-shell boundary is not a scheduling choice** — it is containment, with a recorded incident where a suite escaped and overwrote real config files, and widening it would buy nothing anyway for the reason above. Read it as the gate's own lanes and nothing wider: it says nothing about how `ai/run-tests.sh` schedules suites inside one shell unit, which is a separate question with a separate answer below. **The mutation baseline cannot be left to Go's test cache** to avoid re-running per unit: measured on 2026-09-08, with `./eco-report` cached green, breaking a file the fixtures copy in from outside the module still answered `ok (cached)` while the same tree run with `-count=1` failed. A cached baseline is a green served over a red suite.

`ai/run-tests.sh` run on its own gains about 1.31x from running its suites several at a time, and its pole is `ai/bootstrap-test.sh` — 224s as the gate measures it, against a 348s whole-run. Further gain there lives inside that suite rather than in the runner. That scheduling stays in shell rather than moving into Go beside the gate's own in `gate/run.go`. The reason is in the runner's header: `ai/bootstrap.sh --verify` calls it on a machine that may have no Go and no downloaded binary.

That runner's own containment check proves less than its name suggests. `tree_state` is `git status` over the checkout, so it catches a suite writing into the repository and is blind to one that escapes its temp HOME and writes the real one — and the recorded incident reached the checkout only because a fixture write followed a symlink there. Overlap does not create that leak; a suite that escapes escapes alone just as well. It decides one default instead: `bootstrap.sh --verify` calls the runner immediately after writing `$HOME/.claude`, `$HOME/.kk-flavor` and `$HOME/.codex`, so that path takes a single lane and everything else keeps the 1.31x.

Figures are from one 12-core machine and move with load: the same gate has measured 116s and 2877s on identical code. Read the per-unit times and the lane totals, never a single wall clock.
