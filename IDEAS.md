# Ideas

Deferred proposals, not active agent instructions. Keep at most 20 open ideas; review and consolidate this backlog when the owner requests a review or before exceeding that limit.

## 3x | 2026-09-10 | Workers as their own layer, skills as the doors

The taxonomy is possible, needs no installer work, and the tree has already invented it five times under five names. `kk-patrol/SCOUT.md` and `FIXER.md` were worker prompts living inside the skill that spawns them, each mapping onto a row of its own: `patrol/scout`, `patrol/fixer`. **Step 1 — landed** moved those two. `kk-reduce/AGENT-BRIEF.md` looks like a third and is not — it is the brief handed to several of that campaign's agents, so it is a shared prompt fragment with no dispatch of its own and stays with its skill. `kk-build/technical-round.md`, `kk-handoff/handoff-prompt.md` and `kk-ecosystem/audit.md` are not workers and do not move — the first two are read inline by a session that needs the human, the third is a branch of its own skill. Nine more dispatch sites — `explore`, `facts`, `over-cut`, `arbitrate`, `fan-out`, `reconcile`, `converge`, `repair` and idsd-qualify's own `reconcile` — have a row in `models.json`, a paragraph in a `SKILL.md`, and no file of their own. `workers/` is the home those already want.

### Three kinds, two homes

A **worker** is `workers/<name>.md` or `workers/<group>/<name>.md`, has no door, is only ever dispatched, and its name carries no skill prefix: `code-review`, `explore`, `reduce/arbitrate`. An **orchestrator** and a **session** are both `skills/<name>/SKILL.md` with a door; the difference is one declaration, not a directory. An orchestrator dispatches every substantive step and is cheap. A session holds model work itself, for one of the three standing reasons — it decides with the human, it needs context only that session holds, or it performs an act with no undo — and takes the tier of the work it holds.

`models.json`'s two maps then line up with the two homes exactly: `workers` is the `workers/` tree, `sessions` is the `skills/` tree. Today the maps cut across one directory, which is why a name in the file cannot be read for what it is.

**A worker's row becomes true, which today it is not.** Nine skills declare `**Runs:** dispatched` while remaining mounted and `/`-invocable, so whenever the human types one it runs inline in their own session at their own tier and its row selects nothing. Moving them removes that path rather than documenting it.

### Possible with no install change

`~/.kk-flavor` is a symlink to the flavor directory, so `~/.kk-flavor/workers/<name>.md` resolves today. Dispatch already works this way: `kk-patrol` hands `~/.kk-flavor/workers/patrol/scout.md` to a spawned agent verbatim. The installers enumerate `skills/` only, so `workers/` is never mounted and a worker cannot be reached by `/name` or by model invocation — which is what makes its assignment the one that runs. No `agents/openai.yaml` per worker either; implicit invocation is not reachable.

### What Go should assert

The declaration-based checks become filesystem checks, and two new gates appear that prose cannot enforce.

Every `workers/**/*.md` has a `workers` row and every row has a file, both ways; every `skills/*/SKILL.md` has a `sessions` row, both ways. That retires the `**Dispatches:**` lines added for want of a home.

**An orchestrator may not hold the top tier.** An orchestrator claims every substantive step is dispatched; a top-tier row contradicts the claim. This is the offload signal as a test rather than a paragraph — the rule that a high tier in a coordinator means work belongs in a worker currently relies on someone reading the file and believing it.

**No skill dispatches a skill, and no worker reads one.** A dispatch resolves into `workers/`; a `skills/<x>` path inside `workers/**` fails. The second keeps a leaf a leaf.

`--graph` emits the workflow map — skills, their workers, and which skills extend or sequence which — generated so it cannot drift from the tree. `--cost <skill>` prints the per-run tier profile: one session at its tier plus each worker at its own. That is the central cost surface finished: readable, not only editable.

### The rule narrows to dispatch

"Skills never compose skills" over-reaches, because four different edges are all spelled as a path today. **Dispatch** spends money and is the one to forbid. **Extension** is one session reading two contracts — `idsd-build` is the ICE-shaped delta over `kk-build`, `idsd-qualify` over `kk-qualify` — and forbidding it duplicates the whole build contract into the workflow family. **Sequencing** is a pipeline naming its next stage, which `idsd-ship` must do. **Orientation** is a pointer placing a skill against its neighbour. The graph still simplifies as intended: every remaining skill-to-skill edge is a related contract, and every edge that costs money points into `workers/`.

This contradicts [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Family direction**, which currently requires every capability to be an any-repo skill that workflow skills compose. Capabilities become workers under this change and that section is rewritten with it, not around it.

### What each open question turns out to be

`kk-build` is a session and `kk-build/explore` a worker because `explore` was the one step inside it with no human in it. Phase 2 settles the stack with the human and Phase 5 checkpoints to them, so both stay; **Phase 4, the build loop, has no human in it either** and is the largest unoffloaded block in the tree. `workers/implement.md` is the missing file, and `kk-build` is a session that has not finished offloading rather than a session by nature.

`idsd-build` holds the top tier for `kk-build`'s reason and not its own: Phase 3 applies that build contract inline and inherits its loop. Its own work is Phase 1's gap round with the human, Phase 2's charter reading and Phase 4's checkpoint. It becomes an orchestrator the moment `implement` is a worker — the same defect counted twice.

`idsd-charter` running `kk-grill` is not a dispatch and needs no atomic grill worker. The session reads the grill contract and interrogates the human itself, which is the one composition that is correct, because a worker has no human to grill. `kk-grill` is not an orchestrator of one worker; it is a session, at the top tier, and `workers/facts.md` is the only part of it with no human in it. **A worker read inline is not a worker** — it is a section of the calling session, billed at that session's tier — so "a session that inlines one worker" is not a third category.

`kk-diagnose` stays a skill against the general rule: the router sends the human to it directly and before any diff exists, so it is an investigation with a door rather than a lane leaf. `kk-conform` is the harder case — it is dispatched by the quality pass and run **inline** by `kk-build` so that thread reaches the human. As a worker it must always dispatch and relay, which the build already does with every other return.

### The human reason splits three ways

A worker can reach the human already: it returns `blocked: <what you need>`, the orchestrator asks, and resumes that same worker by its ID with the answer ([skill-protocol.md](ai/kk-flavor/standards/skill-protocol.md) → **Orchestrators — interactive first**). A worker cannot ask directly — the question widget hands a spawned agent the skip filler rather than an answer — so the relay is the only shape, and it is a shape the tree already specifies.

**Relay does not create human access; it relocates model spend.** The orchestrator can only relay while it has a human of its own, so a nested stage or an unattended run dead-ends exactly where an inline session would. What changes is where the tokens are charged, and for an adaptive dialogue they are charged twice: the worker holds the design tree, the orchestrator holds every question and answer that passed through it, and each round costs a block-and-resume cycle. **Grilling is the one case where offloading raises the bill** — `kk-grill` asks a frontier, waits, and recomputes it from the answers, so the rounds cannot be batched into one return.

So `human` is not one reason. **Converses** — each question depends on the last answer — is immovable, and covers `kk-grill`, `idsd-charter`, `idsd-intent` and `idsd-build`'s gap round. **Asks once** — a single fact or decision the work cannot supply — is a `blocked:` relay, and offloadable. **Presents** — a result the human approves at the end — is a plain return that needs no relay at all, and covers `kk-qualify`'s residue and `kk-build`'s Phase 5 checkpoint. Only the first justifies an orchestrator holding a top tier, which sharpens the ceiling gate above: the declaration names which of the three, and the check reads it.

### The checkers are the bulk of it, not the skills

Moving the files is mechanical: 48 citations of the nine dispatched skills across 24 files, and one worker file per site. The work is in what enforces the current shape. Six `eco-check` files encode it across roughly 2,900 lines with their suites: `direction.go` bans the shared layer from naming a lane and needs the same three shapes for `workers/`; `families.go` enforces that the any-repo family never names the workflow family; `skills.go` scans skill directories for the frontmatter a worker will not have; `mounts.go` asserts both directions of the mount census, which nine removed mounts must not trip; `scripts.go` holds where a lane's scripts live; `budget.go` counts the descriptions that load every session, nine of which stop existing. The `model-policy` derivation gates get smaller rather than larger — a directory listing replaces the `**Runs:**` and `**Dispatches:**` declarations.

**Two design questions the checkers force, both answered under Decided.** Does a worker carry a family? `families.go` keys on the two prefixes, and `audit` is reachable from the workflow suite alone, so a family-neutral worker would leave a kk worker naming an idsd concept unchecked. And can a worker own scripts? `kk-ecosystem` ships three script-and-test pairs, which a flat worker file cannot hold.

### Where the architecture is written

`ai/field-guide.html` is the home and needs no new one. Its narrative half is hand-written in `tools/eco-guide/field-guide.template.html` and holds the walkthrough and the which-door table, so the three kinds and the dispatch rule are one new section there. Its inventory half is generated from frontmatter because a hand-kept catalogue of 27 skills drifts, and **the worker layer becomes a second generated inventory beside it** — one card per worker with the tier its row assigns, which is the workflow map a newcomer needs and cannot go stale. `guide.sh --check` already fails the gate when the committed page has drifted, so that property is inherited rather than built. `kk-skillcraft` learns the worker shape and the test for which kind a new thing is; `kk-ecosystem`'s `audit.md` learns to read the new structure.

### Decided

**Path grammar.** A worker groups under its parent skill's bare name and workflow-owned workers nest under `idsd/`: `workers/reduce/arbitrate.md`, `workers/patrol/scout.md`, `workers/idsd/audit.md`, with standalone any-repo workers flat at `workers/code-review.md`. `models.json` keys are those paths without `.md`, so no key carries a skill prefix. `idsd/` is declared in `families.go` the way the two prefixes already are, because the direction is the whole rule and cannot be derived from the name.

**Seven of the nine lose their door, `kk-edit` and `kk-ecosystem` keep theirs.** Those two are the only ones the router sends a human to; the other seven are reached only by a dispatch, so nothing anyone types goes away. Keeping `kk-ecosystem` a skill also settles the scripts question: it is the only one of the nine that ships any, so `check.sh`, `cite-graph.sh` and `ruleecho.sh` stay where they are and **no worker is ever a directory**.

**A door on top of a worker dispatches it; it never reads it inline.** Inline runs the worker at the calling session's tier, which is the exact falsehood the nine `dispatched` rows tell today, so an inline door would rebuild the hole one skill at a time. The single exception is the `converses` reason — a session that must stay with the human mid-work reads the contract itself and its own row applies, as `kk-build` does with conform. This keeps the escape hatch open: any door can come back later as a two-line skill that dispatches.

**`workers/implement.md` lands alone, after the structure.** It is the only change that alters how building feels, and it should be revertible without unpicking anything else.

**A commit stack per step, the record in this file.** No charter and no intents; each step lands green on its own and its outcome is written here before the next begins.

**The workflow family barely changes, and that is the design working.** Only `idsd-audit` becomes a worker, because it is the only `idsd-*` skill nothing human enters. `idsd-charter`, `idsd-intent` and `idsd-build` converse with the human and stay sessions; `idsd-ship`, `idsd-reactor` and `idsd-qualify` are orchestrators over stages they sequence. The family is already a set of doors over any-repo capabilities — [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Family direction** put the dispatchable work in the `kk-*` family years of edits ago, so that is where the workers come from. What the suite gains is tier, not thinness, and `idsd-finalize` joins the offload list: its `landing` reason covers the archive and the merge gate, not the drafting and stamp-check that precede them.

### Step 1 — landed

`ai/kk-flavor/workers/` exists and holds `patrol/scout.md` and `patrol/fixer.md`, the two prompts that were already worker prompts and already had rows. `models.json` rekeyed `kk-patrol/scout` and `kk-patrol/fixer` to `patrol/scout` and `patrol/fixer`, so no worker key carries a skill prefix, and `kk-patrol`'s `**Dispatches:**` line is gone because the directory declares those sites now. Both standards that stated the older declaration form now name `workers/` as well, since `kk-patrol` relies on it and declares nothing.

**`kk-reduce/AGENT-BRIEF.md` moved and was moved back**, which the conformance gate caught: I gave it a `reduce/scoped` row so the directory check would accept the file, which is the gate driving the data rather than describing it. Nothing dispatches that name — the brief goes to agents dispatching as `reduce/{fan-out,reconcile,converge}` — and the opus tier I invented for it contradicted `kk-reduce/over-cut` at sonnet, handed the same brief. It is a shared prompt fragment, not a worker, and step 1b decides where such a fragment lives once reduce's six sites have files.

In Go, `shippedWorkerFiles` derives the worker set by walking that directory, and both derivation gates read it: a file with no row fails, and a row whose file is missing fails. Both directions were run as controls and both fired.

**It changed a checker, against what I told you.** `direction.go` reads everything outside `skills/` as the shared layer, which must never name a lane — so the moment those files moved, every worker's mention of its own parent skill became a finding. `workers/` is now a second lane tree in both `sharedFilesNamed` and the basename census. Controlled both ways: a `kk-patrol` mention added to a standard still fires, and one added to a worker does not.

Left for step 1b: the nine sites still declared by `**Dispatches:**` in `kk-build`, `kk-grill`, `kk-reduce` and `idsd-qualify`. Two mechanisms coexist until then and 1b removes the legacy half. Two of the nine are the overrides Q5 settled, so 1b also implements the `worker` field in `policy.go`, and it settles the shared-fragment question the brief raised.

### Step 1 — the drive gate found a silent downgrade

The rekey left the old keys resolving. `patrol/fixer` was `kk-patrol/fixer` at opus; asked by its old name it fell back to the `kk-patrol` row and answered **sonnet, kind session**, exit 0 — so a stale caller either dispatches a tier lower than the work needs or reads `session`, sets no model, and inherits its parent's. Nothing in the answer says the name is dead. **Nine more keys are renamed in the steps after this one**, so the mechanism mattered more than the instance.

Two facts decided the fix. Only one stale reference existed — the `--task` flag's own help text — and **every sub-row in the file is explicit**, so nothing depends on the ancestor fallback to find a row at all. The fix step 1 took was enforcement alone: the help text is corrected, and `TestNoFileNamesATaskThePolicyDoesNotAssign` refuses any slash-shaped task name in the tree that the policy does not assign. It caught the one stale name still standing, which was inside its own explanatory comment, and it fires on a planted one and passes when that is removed.

**Since answered: the fallback is gone.** It was "a phase or section with no row of its own answers its skill's", it served no row, and it was the thing that turned a rename into a silent tier change. `Resolve` now does one exact lookup and refuses every unlisted key, which is the fail-visibly principle the rest of the file is built on, and [model-policy.md](ai/kk-flavor/standards/model-policy.md) states the refusal in place of the old guarantee. The name gate above stays as the second line: it catches a stale name in the tree before anything asks the resolver.

Not a regression, settled: a worker prompt may cite a lane other than its own without a finding, because the exemption is path-wide. The three files had that same freedom while they sat inside `skills/kk-patrol/`, and skill-to-skill citation is normal — what constrains crossing *families* is the open `families.go` question step two carries.

### Step 1 — what the conformance gate found

Requirements 1 and 2 delivered clean; the other five each carried a contradiction, and all of them were mine.

**The gate was driving the data.** `reduce/scoped` existed so the directory check would accept a file, not because anything dispatches that name — reverted, and the brief is back with its skill.

**A row with no file reported the wrong cause**, falling through to the fallback and wrong-map messages, neither of which says a file is absent — it now names that directly.

**I split a doc comment in half** inserting the worker helper, leaving one clause truncated and the test's own comment opening mid-sentence; healed, the helper moved below, and `TestEverySkillHasARow` renamed since it covers workers too.

**The lane-tree premise reached two of three call sites.** A shared file naming a worker's basename resolved its owner by walking `skills/` alone and would have reported an empty owner. Latent, since nothing shared names one today.

**Two checker files were delivery beyond the ask and carried no test** — `direction_test.go` covers exactly that scan and was untouched. Three cases added; two of them fail with the change reverted and pass with it, so they bind.

**Neither standard knew `workers/` existed** while `kk-patrol` already depended on it — both now name it, with the in-skill declaration marked as the older form being retired per site.

**The plan contradicted itself three ways** — the remaining-site count, the brief mapping onto `reduce/fan-out` against having a new row, and **Migration** still describing step one as ten sites plus five prompt files. All three reconciled at nine, though the review pass after that found one more copy of the wrong figure in this section's own paragraph.

The gate also reported a concurrent writer moving four files mid-pass. That was this session fixing the drive gate's finding while the conformance gate read, not a peer.

### Step 1 — what the round found

Seven lanes ran: conformance and drive as gates, then code-review, security-review and the instruction lane, a scoped re-review of the repairs, refactor, and edit. Every defect was mine.

**The exemption had no entrance.** Reading `workers/` as a lane tree stops a worker's mention of its own skill being a finding, but nothing caught a shared file steering a reader *into* one — so a lane-steering rule in a worker prompt, listed in the router's always-read block, would load in every session with the provenance check silent. The fix took three attempts: a bare directory segment broke the case holding that `--gate` knows no lane the commit does not carry; making the arm's prefix optional turned every blank line into a finding, because the `$^` empty-case sentinel matches an empty line once nothing precedes it; and a targeted replace stripped that sentinel from the wrong one of two functions with identical tails. The arm is now built from the paths the tree carries, over every file rather than prompts alone, since the exemption covers every file too.

**Two guards of mine had no case until someone checked.** The new stale-task gate counted a token as a task reference only when its first segment was itself a row — so the `patrol/*` family this step creates could never be checked, `kk-build/explore` was the only name it checked in the whole tree, and its own vacuity guard passed. Separately, deleting the lane-name character gate left the suite green. Both now have cases, and every new case has a control that fails on exactly its own change.

**Also closed:** a silent narrowing in the basename census, an owner lookup nothing tested, a filename with one invalid byte that would panic the tool before any finding printed, five reads of tree-chosen paths that followed symlinks, and a branch reachable only after the loop above had already errored. Refactor unified two escape mechanisms, extracted a fixture written four times, and named two effort tables. Edit brought long comment blocks from 22 to 10 and found a doc comment still claiming "worker prompts" after I widened its glob to every file.

Both standards briefly claimed every dispatched worker's prompt lives in `workers/`, which is true of two of twenty-two rows; both now name the migration window instead.

**Surfaced once, not fixed here:** the unknown-skill scan truncates a non-ASCII name — a `kk-drivé` directory reports `kk-driv`, a path no file matches. It predates this change set and belongs in its own pass.

### Migration

Land it in four steps, each green on its own. First the sites that already have prompts, then the rest as step 1b: create `workers/`, move each prompt in, retire that site's `**Dispatches:**` entry, and switch the Go checks to the directory. No door moves, so nothing the human types changes. See **Step 1 — landed** for what the first half actually cost. Second, the nine dispatched skills become worker files, `kk-diagnose` excepted, and their mounts are removed as the last step of that landing. Third, `**Runs:**` becomes `orchestrator` or `holds — <reason>`, and the orchestrator tier ceiling turns on; every skill it fails is either relabelled a session or has its held work dispatched. Fourth, `--graph` and `--cost`.

The reversal risk sits in step two and nowhere else: it removes `/kk-edit`, `/kk-code-review`, `/kk-refactor`, `/kk-conform`, `/kk-drive`, `/kk-ecosystem`, `/kk-security-review`, `/kk-skillcraft` and `/idsd-audit` as typed commands, leaving `kk-foreman` as the door. Cheaper than keeping them, because each of those paths today runs a worker's contract inline at a session's tier.

Standing cost: one new directory the installers ignore, four Go checks and two emitters to maintain, and a rewritten **Family direction**. Unmeasured: what the tier changes actually save, which needs a run of the quality pass before and after rather than an estimate from the file.

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
