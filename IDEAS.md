# Ideas

Deferred proposals, not active agent instructions. Keep at most 20 open ideas; review and consolidate this backlog when the owner requests a review or before exceeding that limit.

## 3x | 2026-09-10 | Workers as their own layer, skills as the doors

The taxonomy is possible, needs no installer work, and the tree has already invented it five times under five names. `kk-patrol/SCOUT.md` and `FIXER.md` were worker prompts living inside the skill that spawns them, each mapping onto a row of its own: `patrol/scout`, `patrol/fixer`. **Step 1 — landed** moved those two. `kk-reduce/AGENT-BRIEF.md` looks like a third and is not — it is the brief handed to several of that campaign's agents, so it is a shared prompt fragment with no dispatch of its own and stays with its skill. `kk-build/technical-round.md`, `kk-handoff/handoff-prompt.md` and `kk-ecosystem/audit.md` are not workers and do not move — the first two are read inline by a session that needs the human, the third is a branch of its own skill. Nine more dispatch sites — `explore`, `facts`, `over-cut`, `arbitrate`, `fan-out`, `reconcile`, `converge`, `repair` and idsd-qualify's own `reconcile` — had a row in `models.json`, a paragraph in a `SKILL.md` and no file of their own; **Step 1b — landed** gave seven of them one and the other two a row that names whose prompt they run.

### Three kinds, two homes

A **worker** is `workers/<name>.md`, `workers/<group>/<name>.md`, or one segment deeper where both grouping rules apply at once (**Decided** → **Path grammar**). It has no door, is only ever dispatched, and its name carries no skill prefix: `code-review`, `explore`, `reduce/arbitrate`. An **orchestrator** and a **session** are both `skills/<name>/SKILL.md` with a door; the difference is one declaration, not a directory. An orchestrator dispatches every substantive step and is cheap. A session holds model work itself, for one of the three standing reasons — it decides with the human, it needs context only that session holds, or it performs an act with no undo — and takes the tier of the work it holds.

`models.json`'s two maps then line up with the two homes exactly: `workers` is the `workers/` tree, `sessions` is the `skills/` tree. Today the maps cut across one directory, which is why a name in the file cannot be read for what it is.

**A worker's row becomes true, which today it is not.** Ten skills declare `**Runs:** dispatched` while remaining mounted and `/`-invocable, so whenever the human types one it runs inline in their own session at their own tier and its row selects nothing. Moving them removes that path rather than documenting it.

### Possible with no install change

`~/.kk-flavor` is a symlink to the flavor directory, so `~/.kk-flavor/workers/<name>.md` resolves today. Dispatch already works this way: `kk-patrol` hands `~/.kk-flavor/workers/patrol/scout.md` to a spawned agent verbatim. The installers enumerate `skills/` only, so `workers/` is never mounted and a worker cannot be reached by `/name` or by model invocation — which is what makes its assignment the one that runs. No `agents/openai.yaml` per worker either; implicit invocation is not reachable.

### What Go should assert

The declaration-based checks become filesystem checks, and two new gates appear that prose cannot enforce.

Every `workers/**/*.md` has a `workers` row and every row has a file, both ways; every `skills/*/SKILL.md` has a `sessions` row, both ways. That retires the `**Dispatches:**` lines added for want of a home.

**An orchestrator may not hold the top tier.** An orchestrator claims every substantive step is dispatched; a top-tier row contradicts the claim. This is the offload signal as a test rather than a paragraph — the rule that a high tier in a coordinator means work belongs in a worker currently relies on someone reading the file and believing it.

**No skill dispatches a skill, and no worker dispatches one either.** A dispatch resolves into `workers/`. The second half was first written as "no worker *reads* a skill", to keep a leaf a leaf, and **Step 1b — landed** falsified it: reduce's editing workers are told to run the ecosystem pass, and that skill keeps its door, so a `skills/` path inside `workers/**` is legitimate.

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

Moving the files is mechanical: the seven that lose their doors are cited by path 19 times across 9 files, with bare-name mentions on top of that, and there is one worker file per site. The work is in what enforces the current shape. Six `eco-check` files encode it across roughly 2,900 lines with their suites: `direction.go` bans the shared layer from naming a lane and needs the same three shapes for `workers/`; `families.go` enforces that the any-repo family never names the workflow family; `skills.go` scans skill directories for the frontmatter a worker will not have; `mounts.go` asserts both directions of the mount census, which seven removed mounts must not trip; `scripts.go` holds where a lane's scripts live; `budget.go` counts the descriptions that load every session, seven of which stop existing. The `model-policy` derivation gates get smaller rather than larger — a directory listing replaces the `**Runs:**` and `**Dispatches:**` declarations.

**Two design questions the checkers force, both answered under Decided.** Does a worker carry a family? `families.go` keys on the two prefixes, and `audit` is reachable from the workflow suite alone, so a family-neutral worker would leave a kk worker naming an idsd concept unchecked. And can a worker own scripts? `kk-ecosystem` ships three script-and-test pairs, which a flat worker file cannot hold.

### Where the architecture is written

`ai/field-guide.html` is the home and needs no new one. Its narrative half is hand-written in `tools/eco-guide/field-guide.template.html` and holds the walkthrough and the which-door table, so the three kinds and the dispatch rule are one new section there. Its inventory half is generated from frontmatter because a hand-kept catalogue of 27 skills drifts, and **the worker layer becomes a second generated inventory beside it** — one card per worker with the tier its row assigns, which is the workflow map a newcomer needs and cannot go stale. `guide.sh --check` already fails the gate when the committed page has drifted, so that property is inherited rather than built. `kk-skillcraft` learns the worker shape and the test for which kind a new thing is; `kk-ecosystem`'s `audit.md` learns to read the new structure.

### Decided

**Path grammar.** A worker groups under its parent skill's bare name and workflow-owned workers nest under `idsd/`: `workers/reduce/arbitrate.md`, `workers/patrol/scout.md`, `workers/idsd/audit.md`, with standalone any-repo workers flat at `workers/code-review.md`. **The two rules compose where both apply**, which is how a site inside a workflow skill reaches three segments: `idsd-qualify`'s own `reconcile` is `workers/idsd/qualify/reconcile.md`. `models.json` keys are those paths without `.md`, so no key carries a skill prefix. `idsd/` is declared in `families.go`, derived from the workflow prefix the way that family's state directory already is, because the direction is the whole rule and nothing in a worker's own name carries it.

**Seven of the ten lose their door; `kk-edit`, `kk-ecosystem` and `kk-diagnose` keep theirs.** Those three are the only ones the router sends a human to; the other seven are reached only by a dispatch, so nothing anyone types goes away. Keeping `kk-ecosystem` a skill keeps `check.sh`, `cite-graph.sh` and `ruleecho.sh` where they are. It does **not** settle the scripts question, which this first read as it doing: three of the ten ship scripts, and `kk-refactor` is one of them and loses its door (**Step 2 — landed**). The rule that holds is narrower — **a worker's prompt is always a file, and any script it owns sits beside it** at `workers/<name>/`.

**A door on top of a worker dispatches it; it never reads it inline.** Inline runs the worker at the calling session's tier, which is the exact falsehood those ten `dispatched` rows tell today, so an inline door would rebuild the hole one skill at a time. The single exception is the `converses` reason — a session that must stay with the human mid-work reads the contract itself and its own row applies, as `kk-build` does with conform. This keeps the escape hatch open: any door can come back later as a two-line skill that dispatches.

**`workers/implement.md` lands alone, after the structure.** It is the only change that alters how building feels, and it should be revertible without unpicking anything else.

**A commit stack per step, the record in this file.** No charter and no intents; each step lands green on its own and its outcome is written here before the next begins.

**The workflow family barely changes, and that is the design working.** Only `idsd-audit` becomes a worker, because it is the only `idsd-*` skill nothing human enters. `idsd-charter`, `idsd-intent` and `idsd-build` converse with the human and stay sessions; `idsd-ship`, `idsd-reactor` and `idsd-qualify` are orchestrators over stages they sequence. The family is already a set of doors over any-repo capabilities — [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Family direction** put the dispatchable work in the `kk-*` family years of edits ago, so that is where the workers come from. What the suite gains is tier, not thinness, and `idsd-finalize` joins the offload list: its `landing` reason covers the archive and the merge gate, not the drafting and stamp-check that precede them.

### Step 1 — landed

`ai/kk-flavor/workers/` exists and holds `patrol/scout.md` and `patrol/fixer.md`, the two prompts that were already worker prompts and already had rows. `models.json` rekeyed `kk-patrol/scout` and `kk-patrol/fixer` to `patrol/scout` and `patrol/fixer`, so no worker key carries a skill prefix, and `kk-patrol`'s `**Dispatches:**` line is gone because the directory declares those sites now. Both standards that stated the older declaration form now name `workers/` as well, since `kk-patrol` relies on it and declares nothing.

**`kk-reduce/AGENT-BRIEF.md` moved and was moved back**, which the conformance gate caught: I gave it a `reduce/scoped` row so the directory check would accept the file, which is the gate driving the data rather than describing it. Nothing dispatches that name — the brief goes to agents dispatching as `reduce/{fan-out,reconcile,converge}` — and the opus tier I invented for it contradicted `kk-reduce/over-cut` at sonnet, handed the same brief. It is a shared prompt fragment, not a worker, and step 1b decides where such a fragment lives once reduce's sites are declared by the tree.

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

### Step 1b — landed

The nine remaining sites left the skills that spawned them. **Seven are files under `workers/`**: `build/explore`, `grill/facts`, `reduce/{over-cut,arbitrate,reconcile,converge}` and `idsd/qualify/reconcile`. **Two own no file, because their prompt is another worker's** — `reduce/fan-out` runs the ecosystem pass over a theme and `reduce/repair` the prose pass at the campaign's tier — and each names its owner in the `worker` field Q5 settled. Every worker key is now a path with no skill prefix, the `**Dispatches:**` form is gone from the tree, and the derivation gate reads the directory and that field alone.

**The `worker` field needed three refusals and a message, not one.** It is the only row shape that owns no prompt, so unchecked it spells a row naming nothing while the directory census accepts it. The three refusals are a target that is not a worker row, a target that is a session, and a target that re-tiers something else in turn — a chain whose file can only be found by reading two further rows. A row naming *itself* is a one-row cycle, which that third refusal already catches, so the branch that names it is a message rather than a fourth guard, and the case observes the message. `Resolve` returns the owner, so a caller learns which contract to hand the spawn and not only what it may spend.

**The shared fragment stays with its skill.** `AGENT-BRIEF.md` is that campaign's contract for its own editing agents, and the two workers that need it now cite it rather than being handed it verbatim — which they could not do while they had no file of their own to cite from. Phase 3's agents are still given it pasted, because their prompt is another skill's contract and that contract does not name the brief.

**Family direction reached the worker layer, a step early.** `families.go` scanned `skills/` alone, so this step's first workflow-owned worker would have landed unchecked, and every any-repo worker beside it. A worker's task name carries no family prefix, so its path is the only thing that can say which family it is in. `workers/idsd/` is the workflow family's and everything else is any-repo, so no third case is left for a neither-family finding to catch. The router's exception does not reach the tree, because a worker has nothing to route, and a case proves that citing that exception in a prompt buys no silence.

**A tenth dispatch site turned up on the way.** The build's interactive phase dispatched a bounded explorer that no row covered. Under step 1's refusal it could resolve no model at all, so it would have taken its session's. It is the same read-only bounded exploration the planning phase dispatches, so it answers to `build/explore` too, and that worker's file carries both asks.

**Seven control runs, each failing on exactly its own change**: the parse refusals, the session guard, the decision field, both directions of the file check, the worker family scan, and the directory that exempts the workflow family's own. The round below added eight more.

**Two of my own edits were the findings.** The new paragraph in [model-policy.md](ai/kk-flavor/standards/model-policy.md) named two skills to give the field an example, which is the shared layer naming a lane; it names the capability now. And [spawn-prompt.md](ai/kk-flavor/templates/spawn-prompt.md) opened every spawn with "apply the `<skill name>` contract", which nine of these dispatches no longer do.

Left for step 2, unchanged: the seven skills that lose their doors, the skillcraft lane learning the worker shape, and whether a worker may own scripts — answered under **Decided**, not yet enforced.

### Step 1b — what the gates found

**The repo gate went red on a mutation anchor, from the refactor and not the feature.** Extracting the shared glob pair out of `families.go` deleted the line that one of the 518 shipped mutants anchors on, so `TestTheShippedMutantsAllResolveAgainstTheTree` failed and `--mutants` would have refused its preflight. The tempting fix is to repoint the anchor, which that harness's own header warns is the trap: preflight asks only whether the anchor matches, and the deferred mutation run means a repoint goes green having proven nothing. What landed instead is one named list both lane trees read, so the pair has a single anchor and the existing script case kills it — verified by applying the mutation by hand. The gate's second red is the `wiring` unit, on the same untracked `ai/tools/eco-report/.git` debris as before, which is absent from this diff.

**The conformance gate passed four requirements and found five contradictions in three of them, every one prose against the tree.** One bullet carried two wrong counts: `bloat-judge` is a third row with no file under `workers/`, since a Go tool assembles its prompt, and [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) pointed at a section of [model-policy.md](ai/kk-flavor/standards/model-policy.md) that named only one of the three file-less forms. That section now names all three. **The `worker` field was also framed as re-tiering, which is false of one of its two users** — `reduce/fan-out`'s settings are byte-identical to the pass it dispatches, so it exists because it bills separately, not because it costs more; the standard and the skill both said otherwise. And **Family direction** still opened "inside the skill layer" while the checker had already reached the worker tree, so a worker author read a rule that said it covered the skill layer while the gate fired on them anyway.

**The shared-fragment decision was recorded and not enforced.** It sat in this file, whose own header says these are not agent instructions, so nothing stopped the next fragment from being given an invented row — the exact move step 1 made. [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Conventions a new file joins** now carries it as a convention: a fragment several dispatches share stays with the skill that owns them and never gets a row.

**The drive gate found the refusal message unable to tell a dead name from a never-existent one.** Its advice — "add it" — is actively wrong for a renamed worker: that worker already has a row under its new key, and a second row would fail the two-way file check. The message now says where a worker's task name comes from, so a stale caller reads the live name in the output instead of running a second command for it. The same round found `kk-grill` naming neither the task nor a per-spawn ledger path while explicitly allowing concurrent fact-finders — the one skill of the four whose dispatch paragraph was thin rather than merely relocated.

**Two duplications the move created, both removed at the copy rather than the home.** The file-partition rule now lives with the worker that produces the plan, and the campaign's fan-out phase cites it. The architecture read trigger stays with the skill, and the worker file carries what is actually its own — returning what decided between the alternatives.

**One rider, unasked and recorded here rather than left silent:** the mode-file probe in the derivation gate moved from `os.Stat` to an `Lstat` regular-file check, so a symlink cannot satisfy the census. It is the same hardening five reads took in step 1, inside a line this step already rewrote.

### Step 1b — what the round found

Five lanes over the change set: code review, security review and the instruction lane concurrently, then refactor and edit. Every defect was mine.

**The worst of them was a run licence nobody bounded.** `workers/grill/facts.md` told its agent to "run the command" to establish a fact, and `kk-grill` runs in any repository — including one the human is evaluating. The natural way to answer "which version is installed" is the wrapper that repository supplies: `./gradlew --version`, an `npm run` with a lifecycle hook, a `Makefile` target, a repo-local `bin/` on `PATH`. Each executes code that repository's author wrote, at the worker's privileges, while the caller reads the return as a read of the environment. The prompt's only guard was "you write nothing", which bounds edits and not execution. `workers/patrol/scout.md` had carried exactly this warning since before the move, and I did not carry it across. The bound is now in both prompts, and it is [ecosystem.md](ai/kk-flavor/standards/ecosystem.md) → **Conventions a new file joins**' own rule about a working directory that can hold code the human did not write.

**A second bound was deleted rather than moved.** The build's dispatch line said "a bounded **read-only** worker"; rewriting it to name the worker file dropped the word, and it appeared nowhere in either file afterwards. The worker's write ban now covers running as well as writing, since the prompt is the site's contract now and the skill line is not.

**The `worker` field can run a protected lane's prompt at a cheap row's tier**, which nothing could do before it. A row `{worker: kk-security-review, claude: haiku}` parses, validates and resolves, and reads as a new cheap site rather than as an edit to the protected row. A ceiling would need the tiers ordered, and the file carries no order, so what landed is the smaller enforced thing: the shipped set of such rows is pinned, and a third one fails the suite until somebody writes down what it is. The ordering goes with step three's ceiling gate, which needs it anyway.

**The override check ran on the wrong half of the rows.** It sat inside the sub-row branch, so a top-level row's `worker` field was never opened — a bare name claims someone else's file exactly as a sub-row does. It was widened, then widened again on the instruction lane's evidence that [model-policy.md](ai/kk-flavor/standards/model-policy.md)'s "the check runs both ways" was true only of names carrying a slash. **Every worker row now has to name a prompt**, by one of the three forms that standard lists or the file-owning one, and the tool-built exemption is one named row rather than a silent gap. That closes the hole the flat worker grammar would otherwise have landed in step two.

**Two of the new reduce workers contradicted the brief they are told to read first.** `reduce/reconcile` said to apply nothing it finds itself, where `AGENT-BRIEF.md` → **The plan's authority** says anything the plan did not consider is still its to cut. `reduce/converge` forbade its agent knowing what was cut, where that brief orders the arbitrated plan read whole — and that plan *is* the cut list. Each now states its departure rather than leaving an agent to pick between two files. Four more duplications were missed at the copy and cut there, and the template was half-generalised: its first line spoke of a worker prompt while its ledger slot and closing paragraph still said "skill".

**Nine worker prompts told their agent it would not be asked a follow-up**, which [skill-protocol.md](ai/kk-flavor/standards/skill-protocol.md) → **Orchestrators — interactive first** contradicts: a worker that returns `blocked:` is resumed by its ID with the answer. All nine say what is actually true — that nothing here is a conversation, and the one thing that reopens the context is that resume. Two of them predate this step.

**The refusal message I had just improved was false in a new way.** It told a stale caller that a worker's task name is its path under `workers/`, which is untrue of all three file-less forms, two of them added by this step. It is conditional now.

**One recommendation declined, with the evidence rather than silently:** the worker tree shares one finding bound where each skill gets its own, and a flooding prompt can hide another worker's finding behind it. No test can make that fail. Exhausting a bound of forty takes forty-one findings from one file, which already fills the printer's identical per-rank cap, so the second worker's finding is dropped either way and a per-file bound is unobservable.

**Eight more control runs, on top of the seven the step itself carried**: the hoisted glob list mutated by hand, each of the two new mutants the same way, the self-reference message, the pinned set of rows naming another worker's prompt, a flat worker row with no file, the tool-built exemption pointed at a name nothing uses, and the session mode-file probe. Each failed on exactly its own change and on nothing else.

**The refactor lane found the falsified framing still standing in the Go.** The standard and the skill stopped calling the field a re-tier once one of its two users turned out to re-tier nothing, and the code kept the word — in an accessor, a validator, a pinned set, six messages and five case names. All of them now say whose prompt the row dispatches, which is what the refusal message already said. It also renamed the skills-tree scan once a second tree existed beside it, and split a validator that had grown a mixed-level middle.

**Comment share came down to 21.7% against a 21.1% bar and stopped there, deliberately.** The same seven files measured 21.5% before this change touched them, so the overage is mostly inherited; every remaining comment ranked below the ones cut carries a trap, a bound or a mutation-design reason — the empty-sentinel arm, the basename ambiguity ordering, why a mutant reads `&& false`. Cutting to the number would delete the comments the rule exists to protect. Accepted as inherited; the two files carrying it (`direction.go`, `mutants.go`) are worth a prose pass of their own, which is not this step's.

**Two findings outside the ask, reported and not fixed.** The citation gate truncates a cited heading from the right, so a citation that *extends* a real heading by whole words resolves — a gate against paraphrase accepting paraphrase-by-extension. And `~/.kk-flavor` resolves to the main checkout, which has no `workers/` directory until this lands, so none of these dispatch sites can be exercised by spawning before the merge — step 1's patrol pair included. That is [ecosystem.md](ai/kk-flavor/standards/ecosystem.md)'s own **An edit in a worktree is not what a spawned agent reads**, and it is an argument for landing rather than a defect.

### Step 2 — landed

The seven lost their doors and became prompts: `code-review`, `security-review`, `conform`, `drive`, `refactor`, `skillcraft` and `idsd/audit`. Twenty skills remain where there were twenty-seven, the always-loaded description budget fell from 979 words over 21 skills to 673 over 14, and no mount removal was written — the installers enumerate `skills/`, and `unmount_stale` already drops a mount whose source directory is gone, so the seven directories disappearing *is* the removal.

**"It is the only one of the ten that ships any" was false.** **Decided** rested the whole no-worker-is-a-directory rule on `kk-ecosystem` being the only dispatched skill with scripts. Three of the ten ship them, and `kk-refactor` — which loses its door — ships `dup-literals.sh`. So the rule landed as the narrower true one: **a worker's prompt is always a file, and a script it owns sits beside it** at `workers/refactor/dup-literals.sh`. The stub's `tools_offset` is the one line that had to change, and the moved stub resolves and runs.

**Moving a script out of `skills/*/scripts/` broke two globs that nothing would have reported.** `kk-patrol`'s scout reaches its instruments by globbing that path, and the gate keys every stub-copying suite on the same one. Neither would have failed — the scout would silently stop finding the refactor lane's instrument, and the gate would serve a green from cache over a drifted stub. Both now name the two lane trees.

**A worker has no `audience:`, so the guide needed a rule the plan had not written.** The skills inventory leaves out `audience: maintainer`, and `kk-skillcraft` carried that marker into a file that has no frontmatter to hold it. The decision: **the worker inventory excludes nothing**, because that filter answers "would this reader ever invoke it" and for a worker the answer is always no. What the list is for is the tier map — what the tree spends when a skill hands a step away — and a maintainer-only lane is part of that bill. Revisit it if the page starts reading as two audiences.

**The worker inventory is generated from two sources that cannot flatter each other**: the brief's own first sentence, and the tier resolved through `model-policy`'s own `Resolve`. So the page cannot print a model the dispatch would not take, and `guide.sh --check` inherits the drift gate. Every worker opens `# <Name> brief`, which is what makes the first source readable at all.

**Seven control runs, each failing on exactly its own change** — the brief's first sentence, the tier coming from the policy rather than a constant, the family read off the path, an absent tier printed rather than omitted, the unusable-policy refusal, the zero-worker guard with the heading skip that feeds it, and the unreadable tree. **Two of them passed on the first attempt and were the finding**: the policy case deleted the file and was answered by the *parse* branch, and the empty-tree case deleted the directory and was answered by the *walk* branch, so neither had exercised the guard it named. Both were split into the branches they actually reach.

### Migration

Land it in four steps, each green on its own, with the first taken in two halves. First the sites that already had prompts, then the rest as step 1b: create `workers/`, move or author each prompt, retire that site's `**Dispatches:**` entry, and switch the Go checks to the directory. No door moves in either half, so nothing the human types changes. See **Step 1 — landed** and **Step 1b — landed** for what that cost. Second, seven of those ten become worker files — `kk-diagnose` and the two that keep doors are excepted — and their mounts are removed as the last step of that landing. See **Step 2 — landed**. Third, `**Runs:**` becomes `orchestrator` or `holds — <reason>`, and the orchestrator tier ceiling turns on; every skill it fails is either relabelled a session or has its held work dispatched. Fourth, `--graph` and `--cost`.

The reversal risk sits in step two and nowhere else: it removes `/kk-code-review`, `/kk-refactor`, `/kk-conform`, `/kk-drive`, `/kk-security-review`, `/kk-skillcraft` and `/idsd-audit` as typed commands, leaving `kk-foreman` as their door — the three that **Decided** keeps are not in that list. Cheaper than keeping them, because each of those paths today runs a worker's contract inline at a session's tier.

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

## 2x | 2026-09-11 | Where the gate's remaining wall clock is, and what not to try

A cold `ai/gate.sh --full` is its default lane end to end: the shell lane runs alongside and finishes inside it, so speeding the shell suites up does not move the gate at all. After grouping the mutation units by suite set, one unit is most of the whole run — two cold `--full` runs put `mutants:go:eco-report` at 630s of 1357s and at 658s of 1194s, 46% and 55%, where the next largest is `mutants:go:eco-check` at 163–174s and nothing else exceeds 60s. Any further reduction lives in that one unit, which means in the eco-report suite it runs as its baseline, not in the gate's own scheduling.

Three things not to try. **Widening the gate's lanes gains nothing**: `go-mutate` already runs its mutants `NumCPU-2` wide, so overlapping mutation units over-subscribes the machine rather than filling it, and the non-mutation checks total single-digit seconds warm. **`serialGroupFor`'s shell/non-shell boundary is not a scheduling choice** — it is containment, with a recorded incident where a suite escaped and overwrote real config files, and widening it would buy nothing anyway for the reason above. Read it as the gate's own lanes and nothing wider: it says nothing about how `ai/run-tests.sh` schedules suites inside one shell unit, which is a separate question with a separate answer below. **The mutation baseline cannot be left to Go's test cache** to avoid re-running per unit: measured on 2026-09-08, with `./eco-report` cached green, breaking a file the fixtures copy in from outside the module still answered `ok (cached)` while the same tree run with `-count=1` failed. A cached baseline is a green served over a red suite.

`ai/run-tests.sh` run on its own gains about 1.31x from running its suites several at a time, and its pole is `ai/bootstrap-test.sh` — 224s as the gate measures it, against a 348s whole-run. Further gain there lives inside that suite rather than in the runner. That scheduling stays in shell rather than moving into Go beside `gate/run.go`'s. The reason is in the runner's header: `ai/bootstrap.sh --verify` calls it on a machine that may have no Go and no downloaded binary.

That runner's own containment check proves less than its name suggests. `tree_state` is `git status` over the checkout, so it catches a suite writing into the repository and is blind to one that escapes its temp HOME and writes the real one — and the recorded incident reached the checkout only because a fixture write followed a symlink there. Overlap does not create that leak; a suite that escapes escapes alone just as well. It decides one default instead: `bootstrap.sh --verify` calls the runner immediately after writing `$HOME/.claude`, `$HOME/.kk-flavor` and `$HOME/.codex`, so that path takes a single lane and everything else keeps the 1.31x.

Figures are from one 12-core machine and move with load: the same gate has measured 116s and 2877s on identical code. Read the per-unit times and the lane totals, never a single wall clock.
