# Ideas

Deferred proposals, not active agent instructions. Keep at most 20 open ideas; review and consolidate this backlog when the owner requests a review or before exceeding that limit.

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

Typed stage ingestion is implemented in the existing Go report tool. Each completed stage submits one result instead of separate `stage-returned` and `no-items` calls. The public skills stay focused; the submission and recovery procedure lives in `idsd-qualify/stage-results.md`.

Results bind to the qualification attempt, HEAD, candidate and worktree. The tool preserves accepted finding text and IDs, rejects stale or duplicate completion, and recovers interrupted submissions. The caller still reconciles findings and reviews repairs; the tool cannot prove a reviewer found every defect.

A three-trial local comparison of clean four-stage bookkeeping used 8 report invocations instead of 12. Median elapsed time increased from 7.24s to 9.17s on a tiny fixture. These are sequential runs on the same development machine, without this task's heavy tests running; background system load remains uncontrolled. Retained evidence, candidate validation and durable writes add work. This proves fewer calls and stronger result accounting, not lower latency or provider cost. Coordinator turns, provider tokens and real-project end-to-end savings remain unmeasured.

Gate reuse is deferred. Finalize still reruns build gates because report stamps hold no command, tool, dependency or environment evidence that could justify avoiding them. Keep those reruns until an explicit receipt establishes equivalent inputs and a successful real execution. Store evidence in scratch, separate from human project records. Include commands, working directory, exit status, candidate/dependency identity, relevant tool/configuration identity and logs without secrets. External or otherwise unbounded inputs require rerunning the gate.

Before building gate reuse, measure repeated gates in representative releases. Compare saved runtime with receipt validation and maintenance costs; exercise changed commands, dependencies, toolchains and environments as negative controls. Keep local-model evaluation separate from this decision.

Standing cost of typed ingestion: a versioned format, retained manifests, caller migration and crash-recovery tests. In-flight releases use a frozen old tool bundle until completion; new passes use the typed contract. No scheduler, provider runtime or gate cache was added.

## 2x | 2026-09-11 | Where the gate's remaining wall clock is, and what not to try

A cold `ai/gate.sh --full` is its default lane end to end: the shell lane runs alongside and finishes inside it, so speeding the shell suites up does not move the gate at all. After grouping the mutation units by suite set, one unit is most of the whole run — two cold `--full` runs put `mutants:go:eco-report` at 630s of 1357s and at 658s of 1194s, 46% and 55%, where the next largest is `mutants:go:eco-check` at 163–174s and nothing else exceeds 60s. Any further reduction lives in that one unit, which means in the eco-report suite it runs as its baseline, not in the gate's own scheduling.

Three things not to try. **Widening the gate's lanes gains nothing**: `go-mutate` already runs its mutants `NumCPU-2` wide, so overlapping mutation units over-subscribes the machine rather than filling it, and the non-mutation checks total single-digit seconds warm. **`serialGroupFor`'s shell/non-shell boundary is not a scheduling choice** — it is containment, with a recorded incident where a suite escaped and overwrote real config files, and widening it would buy nothing anyway for the reason above. Read it as the gate's own lanes and nothing wider: it says nothing about how `ai/run-tests.sh` schedules suites inside one shell unit, which is a separate question with a separate answer below. **The mutation baseline cannot be left to Go's test cache** to avoid re-running per unit: measured on 2026-09-08, with `./eco-report` cached green, breaking a file the fixtures copy in from outside the module still answered `ok (cached)` while the same tree run with `-count=1` failed. A cached baseline is a green served over a red suite.

`ai/run-tests.sh` run on its own gains about 1.31x from running its suites several at a time, and its pole is `ai/bootstrap-test.sh` — 224s as the gate measures it, against a 348s whole-run. Further gain there lives inside that suite rather than in the runner. That scheduling stays in shell rather than moving into Go beside `gate/run.go`'s. The reason is in the runner's header: `ai/bootstrap.sh --verify` calls it on a machine that may have no Go and no downloaded binary.

That runner's own containment check proves less than its name suggests. `tree_state` is `git status` over the checkout, so it catches a suite writing into the repository and is blind to one that escapes its temp HOME and writes the real one — and the recorded incident reached the checkout only because a fixture write followed a symlink there. Overlap does not create that leak; a suite that escapes escapes alone just as well. It decides one default instead: `bootstrap.sh --verify` calls the runner immediately after writing `$HOME/.claude`, `$HOME/.kk-flavor` and `$HOME/.codex`, so that path takes a single lane and everything else keeps the 1.31x.

Figures are from one 12-core machine and move with load: the same gate has measured 116s and 2877s on identical code. Read the per-unit times and the lane totals, never a single wall clock.
