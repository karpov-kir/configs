# Changing the Ecosystem

Rules for editing what agents read: skills, standards, prompts, templates, agent instructions. Every line here is context each future run pays for. The bar is never "is this true" — it is "does this change what an agent does".

These rules bind any edit, however small. **Run the instruction lane after a batch of edits here.** A rule half-duplicated by the clause beside it leaves a contradiction, and no scoped hunt catches it — the file was never the thing under review.

## Earn the place

- **Write shared instructions in terms of roles and capabilities.** Name providers only where their behavior, paths, formats or commands differ; keep those distinctions at the boundary that needs them.
- **Delete before you rephrase.** Keep what is essential, plus the supporting detail that makes it unambiguous. Drop the rest.
- **Nice-to-have is a cut** where no move saves it (**Move it before you cut it**, below) — a rule that fires rarely, or that a competent agent follows anyway.
- **A rule you add names what it replaces**, or says plainly that nothing covered this — in the instruction lane's account, never in the file, where **No evidence in a rule file** (below) cuts it.
- **The bar rises with how often the file loads.** An always-read doc takes only what applies to nearly every task; a trigger-loaded standard, its activity; a skill body, its lane. **Text behind a pointer costs that pointer's wording on every run that does not follow it** — a skill's `description:`, a line naming a doc, a branch naming its file are one object — so a target reached unreliably is a pointer to sharpen, never material to inline.

## No evidence in a rule file

Beyond [writing.md](writing.md) → **Density**: no anecdotes, counts, dates, "observed:", and the like. **Density's ban on justification lifts only for a rule an agent would otherwise override.**

## One home

Every rule lives in exactly one file; everywhere else cross-references it by path.

Owner user-scoped instructions are installed as independent copies from a discoverable template; keep the template outside repository `AGENTS.md` and `CLAUDE.md` entry points.

**A citation is an instruction to load a file, and costs what that file costs.** It earns its place only where the reader must open the target — a branch to take, or a delta too big to state here. **Beside a rule the citing file owns and states whole, the link is attribution: cut it.**

**Ownership is stated in the owned file, never broadcast to the files that do not own it.**

**A rule's home is the file whose reader would otherwise get it wrong** — not the file that proves it, and not the file that happens to depend on it.

**The shared layer — a standard, a template under `kk-flavor/`, agent instruction file — never names a skill, and never cites anything inside one**: not a section, not a file it owns, not a script it ships. A standard names the **lane**; the skill filling that lane binds itself to the name and cites the standard, never the reverse. Move the rule up and let the skill cite it there. Skill to skill, the citation is normal.

**`kk-flavor/skills/` and `kk-flavor/workers/` are the lane trees, and neither is the shared layer.** A worker's prompt is one skill's work addressed to one agent, so it names and cites that skill the way a skill does. Every other directory under `kk-flavor/` is shared by default, so one added tomorrow is held to the paragraph above with nothing to opt it in.

## Three kinds, two homes

**A worker is `kk-flavor/workers/<path>.md`. It has no door, and it is only ever dispatched** — `code-review`, `conform`, `reduce/arbitrate`. Nothing the human types reaches it, which is what makes the model its row assigns the model it actually runs at.

**An orchestrator and a session are both `kk-flavor/skills/<name>/SKILL.md` with a door**, and the difference between them is one declaration rather than one directory. An orchestrator dispatches every substantive step. A session holds model work itself, and takes the tier of the work it holds.

**A dispatch resolves into `workers/`.** That is the one edge that spends money, and it is the one the families rule below and [model-policy.md](model-policy.md) both price. Three other edges are spelled the same way and are not dispatches: **extension**, one session reading a second contract as its delta; **sequencing**, a pipeline naming the stage after it; and **orientation**, a pointer placing one skill against its neighbour. Forbidding those would duplicate whole contracts rather than simplify anything.

**A door on top of a worker dispatches it; it never reads it inline.** Read inline, the worker runs at the calling session's tier and its own row selects nothing — the defect the layer exists to remove. The exception is a session that must stay with the human through the work: it reads the contract itself and its own row applies.

**A worker's prompt is a file, never a directory.** Scripts a worker owns sit beside it at `kk-flavor/workers/<name>/`, so the lane that owns an instrument is still readable from its path.

## Family direction

**Inside the lane trees the dependency runs one way too: the any-repo family never names the workflow family, or anything it owns** — not a skill, not a section, not the directory that family keeps its state in. **A worker's task name carries no family prefix, so its path says which family it is in**: `kk-flavor/workers/idsd/` is the workflow family's and every other worker is any-repo. A workflow skill invokes an any-repo one and cites it. An any-repo skill saying what it is *not* names the capability, never the skill that has it. **A skill whose job is routing between the families is the one exception**, and it claims that exception in its own file.

**Every capability is any-repo, even where only one workflow invokes it; a workflow skill composes those and adds only its own machinery.** A capability living in the workflow family alone is one no repo outside that methodology can reach. **Which home it takes is **Three kinds, two homes** above, not this rule**: a capability nothing human enters is a worker, and one the router sends a human to is a skill. **A standard is not a substitute** — a standard is read when something routes a reader to it, where a skill is what someone invokes.

## Conventions a new file joins

- **A skill authored in this tree joins one of two families**, by prefix: `kk-` works in any repo, `idsd-` belongs to a single workflow and carries that workflow's own on-disk machinery. An installed tool skill is a skill outside this tree, not a third family inside it.
- **A skill the human always initiates carries `disable-model-invocation: true` for Claude Code and `policy.allow_implicit_invocation: false` in `agents/openai.yaml` for Codex.** These disable automatic invocation; only Claude's marker promises to remove the description from context. Model invocation is for the skill that must fire on work the human would not think to name.
- **A skill whose subject is this tree's own instructions carries `audience: maintainer`.** Omitting it means the skill works on anyone's project, so only the exceptions carry the key and a new skill inherits the right default by writing nothing. The install offers to skip the marked ones, and every reader of the marker — the installer, the mount scan — takes it from the frontmatter rather than a list, so discovery still holds when a fourth arrives. **A skill that judges work against `~/.kk-flavor/` is marked** however general its subject sounds: it cannot travel to a tree that does not have them.
- **Cite a section as `<file>.md → **Section**`.** That form is machine-checked; "its **Report** section" is not.
- **A dispatched worker is assigned a model, and a worker whose prompt is agent instructions keeps it in `kk-flavor/workers/`, one file per worker, named by path when it is dispatched rather than pasted into the prompt** — three kinds of row carry no file there, each named in [model-policy.md](model-policy.md) → **One row per skill, one per dispatch site**, which owns them and what checks each.
- **A prompt fragment several dispatches share stays with the skill that owns them, and never gets a row.** It is the one thing handed over verbatim instead of by path, because the contract the spawn runs does not name it. `models.json` is keyed on what spawns, so a row invented to make a fragment satisfy the worker census prices a dispatch that does not exist — and sets a tier that contradicts the sites actually handed that fragment.
- **A skill or worker that runs a script cites it by full path — `~/.kk-flavor/skills/<skill>/scripts/<x>.sh`, or `~/.kk-flavor/workers/<worker>/<x>.sh` — whenever the run's working directory can hold code the human did not write.** The test is where the script runs, not who owns it.
- **A machine-local override lives at `${XDG_CONFIG_HOME:-~/.config}/kk-flavor/<name>.conf`, never in this tree.** `~/.kk-flavor` is a symlink into the checkout, so a value tuned there would show up as a dirty working tree and travel to everyone on the next commit. One file per thing being overridden, one `<key> <value>` per line, comments on `#`. **An override that takes effect says so, on stderr, in the output of every command it changes.** **An override file that is present but unusable refuses**, never falling back to the tracked default in silence. A default quietly restored is indistinguishable from the override working.
- **An edit in a worktree is not what a spawned agent reads.** The mounts resolve to the main checkout, so a stage you spawn reads the landed version of every skill and standard, not the tree you are editing. Land it, or exercise it inline, before spawning anything to test it.
- **A new skill is not live until it is mounted** by a symlink pointing back at its directory here. Use `.agents/skills/<name>` for Codex or `.claude/skills/<name>` for Claude Code, under `~` machine-wide or under the project for a project install. **Mount or unmount only once the tree change it reflects has landed, as the last step of that landing and never a later tidy-up.** Mounts change immediately while tree edits are branch-local: mounted early a skill may be absent from the installed checkout; unmounted early it disappears while the tree still ships it. **Prove the mount by running the instruction lane's wiring check** — its mount scan runs only in the install, so a green from a worktree says nothing about mounts, in either direction.

## Move it before you cut it

- **Split** a branch not every run takes into a file the skill names at that branch. The pointer must sit where the branch is taken and say the file is the whole delta for that path.
- **Extract** a rule a script can assert into the script (**Prefer the mechanism**, below).
- **Reuse** — where two files state the same procedure, one skill owns it and the others invoke it, naming only their own delta. **Where neither can own it**, because each carries a scope the other must not inherit, the shared part becomes a new file both stack on. **That extraction pays only once the copies it replaced are gone** — until then it is a third home, and a routing surface on top. **A rule that departs from another file's rule moves with the sentence licensing the departure**, or lands where that sentence reaches it.
- **Demote** a rule that fires for one activity out of an always-read file into that activity's standard.
- **Promote** a rule the common path needs out of a file that path never loads, into one it already loads.

A move away from the common path is only a win when that path genuinely never needs the text — a rule that silently shapes behaviour on every run stays, however rarely it is quoted.

**A reference file earns its place when an agent takes something out of it, not when it reads it** — vendorable code, a template, a checklist it fills. Example code illustrating a rule the prose already states is a cut.

## Prefer the mechanism

- A rule a script can enforce belongs in the script; prose duplicating what a script already enforces is a deletion.
- **A number that multiplies what a run costs belongs in `~/.kk-flavor/models.json`, not in prose or a literal** ([model-policy.md](model-policy.md) → **Cost is a design constraint**).
- **A change to a shared script lands its call site in the same edit.**
- **A script is held to the bar it enforces** — converting prose into a script moves the cost rather than removing it.
- **A script is Go under `ai/tools/`, reached by the `shared:tool-stub` region; shell only where Go cannot yet run.** The install path and the stub settle whether Go can run, at the moment the script runs.
- **The conversion is a win only where the enforcement is known to fire.** Each shell script's header states its test position: the `-test.sh` that covers it, or `# untested: <why>`. The instruction lane's wiring check proves both — that the position is stated, and that the `-test.sh` it names exists.

## Memory

Memory uses the location named by the active agent instructions; do not assume it lives in their Memory section. It is a staging area: an entry stays there until it fits a standard or a skill, then moves into that file. Entries are authoritative — don't reorganize them.

## Approved means edited now

An improvement the human approves is applied to the file in this session, in their repo's working tree.
