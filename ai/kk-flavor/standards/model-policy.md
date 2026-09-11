# Model policy

`~/.kk-flavor/models.json` assigns a model to every skill and every dispatch site, and is the only
place to change one. Skills name a task; they never embed a model.

**Two of its maps assign models, because there are two kinds of assignment.** `workers` is the model a dispatch
actually sets. `sessions` is the tier a human should start that session at, since nothing can change
the model of a session already running — **editing a session row changes no dispatch.** They are two
maps rather than one with a marker so that no row can be mistaken for the other kind; a name in both
is refused, and a session row carrying a roll count is refused with it. Run `~/.kk-flavor/scripts/model-policy.sh --help` for the
resolver's arguments.

## One row per skill, one per dispatch site

**Every skill has a row, every dispatch site has a row, and the Go suite proves both against the
skills tree.** A skill or site without one would run at its caller's tier, which is the silent
inheritance this file exists to remove. Which map a skill's own row sits in follows from how that
skill runs, and the Go suite checks the split (**What the policy can and cannot reach** below); a site
always sits under `workers`.

**A dispatch site is a worker or a named mode the skill hands work to — not a phase.** Phases are the
wrong unit: most skills have none, a phase that runs inline takes the session's model and no row can
change it, and an interactive phase cannot be delegated at all. What costs money is what gets
dispatched, so that is what the file is keyed on, and a site gets a row even where its tier matches
its skill's — the file is a cost surface, and a spending site invisible in it cannot be tuned.

**A worker's own file under `workers/` is its declaration, and its path is its task name.** The check
runs both ways: no file without a row, no row naming a file that is not there. A directory cannot
forget to mention itself, which is what makes this provable rather than inspected — an inventory kept
elsewhere drifts from the tree, and matching a skill's prose instead would accept any row whose name
happens to appear in that skill's frontmatter `description:`.

**Three kinds of worker row own no file there, and each resolves to its prompt another way.** One
**names another worker's prompt in a `worker` field**, because it dispatches an existing pass under
its own accounting — a campaign re-running one is the case, and the row exists whether or not the tier
differs, since a site that bills separately is a site the cost surface has to show. That field must
name a row owning a prompt, one hop and never a chain, so every site still resolves to exactly one
file. The second is a worker **that is still a mounted skill**, running from its own `SKILL.md` until
that move finishes. The third is one **whose prompt a Go tool assembles** rather than reading it from
the tree; `bloat-judge` is the only one, and its row is a required input rather than a declaration
(**What the policy can and cannot reach** below).

**A site is never declared in a skill's prose.** A line beside the prose can forget to mention itself,
and that older form kept a site's tier and its prompt in two places.

**A task the policy does not list is refused, and nothing falls back to anything.** A name whose path
starts with a listed skill is refused with the rest: that ancestor fallback is what let a renamed
worker keep answering, on its skill's row, which is a session's — so the settings came back marked
unusable, the dispatch set no model and took its caller's tier, exit status 0, with nothing saying the
name was dead. **So a new phase or mode that needs its own tier needs a row before it can ask**, and
the tree may not name a task the policy does not assign; the Go suite checks the tree both ways.

**A name carrying a `/` is a sub-row, and two kinds of thing take one.** A worker sub-row is a
separate spawn, declared by whichever form above resolves its prompt. A sub-row for a named *path*
through one session bills nothing of its own — it earns a row only by needing a different tier than
the skill around it, and the mode file that path reads is its evidence.

The decision reports which kind of row answered, so a caller that gets `session` back knows the
settings are not its to apply.

## Cost is a design constraint

**Prefer a tool the policy can steer over one that hardcodes what it spends.** A model tier, a call
count, a worker cap or any other multiplier of the bill belongs in `models.json` — a tier in one of the
assignment maps, a count in `limits` — never in a literal in Go or a number in a skill's prose. Where
a new tool decides how much model to use, it takes that
decision from the policy and fails visibly when the policy does not answer
([ecosystem.md](ecosystem.md) → **Prefer the mechanism** is the same move for rules a script can
assert).

**Then spend the smallest model that still does the work.** The tier is chosen per task, never once for
a run: **cheap where a wrong answer surfaces in the next step, protected where a wrong answer looks
exactly like a right one.** A scout's miss shows up as a round that found nothing; a review's false
green is merged. That asymmetry, not the difficulty of the task, is what sets the tier.

**A worker that writes its own checklist and then grades against it has no denominator, so its misses
are structurally invisible** — the return reads as complete whatever it left out, because the list it
was counted against came from the same pass. Such a task stays protected however mechanical the
grading half looks.

**Inline is a claim that needs a reason, and there are only three.** The work decides *with* the
human, so a worker that has none cannot do it; it needs context only that session holds; or it
performs the act with no undo. A skill declares which, and anything else is dispatched. **The reason
is what makes the classification arguable** — agreement between a skill and the policy proves only
that they say the same thing, and a wrong `inline` passes every consistency check ever written for it.
**A reason that covers part of what the skill does is the offload signal**: the landing is inline, the
pass that precedes it need not be.

**A coordinator that needs a high tier is a coordinator holding work that belongs in a worker.** Read
the row as a finding, not a setting: the tier is what the inline work costs, so the fix is to dispatch
that work and then lower the row. **Lowering it first only underpowers the work** — the session still
does it, and now does it worse. A coordinator whose every substantive step is dispatched has nothing
left that a cheap tier cannot carry.

**Orchestrating is cheap; running a gate is not, and a coordinator can do both.** One that only
schedules, routes and relays reports is light work. One that also holds a gate itself, or reconciles
its workers' findings into the only record of them, is doing that gate's work at that gate's stakes —
and takes its tier. Read what a coordinator does when it is invoked bare, not what it does when
everything below it is dispatched.

**A cheap coordinator is safe only once every site under it names its own model**, and no worker is
lowered to compensate for its parent. Its own session is charged on every turn, so a long-lived
coordinator compares its model against its own row before it schedules anything, says what the
difference costs, and leaves the human to decide whether to restart cheaper — that tier is theirs to
set, not a row's (**What the policy can and cannot reach** below).

## What the policy can and cannot reach

Two kinds of control, and the difference decides where effort is worth spending:

- **Required input** — a Go tool under `ai/tools/` loads the policy and cannot run without it.
  `bloat-judge` is the only one today: it takes its model, its effort and its roll count from the
  `bloat-judge` task, and refuses to vote when the roll count is missing. **That the policy is
  required does not make the model it names the one that runs** — see the paragraph below.
- **Declared** — an agent reads the assignment and dispatches accordingly: the leaf skills the quality
  pass dispatches, and every declared dispatch site. Nothing verifies the model that actually ran, so
  these rows are a convention the agent keeps rather than a gate — but a model *is* selected.
- **A session** — everything under `sessions`. The skill runs in whatever session invoked it, so
  nothing sets its model and the value is advice about how that session should have been started.
  Which rows belong there is derived rather than judged: **each skill declares `**Runs:** dispatched`
  or `**Runs:** inline — <reason>`** beside its frontmatter, and the Go suite checks the two maps
  against those declarations both ways.

**A client's own model setting beats the policy, and no flag reaches past it.** A `model` pin in the
Claude client's settings overrides `--model` on the CLI, a `--settings` file passed beside it, and safe
mode. So a row selects a model only where the human has pinned none — and because nothing anywhere
compares the model requested against the model served, an overridden row looks exactly like a working
one. **This is why requested and observed stay
separate in every record**: the observed half is the only thing that could have caught it.

**Two levers sit outside the file, and no row can move them.** The session an orchestrator runs in
takes the model the human chose before the skill loaded, so a cheap reactor or patrol loop is bought
with that choice and not with an assignment. The judge's roll deadline stays machine-local in
`bloat-judge.conf` on purpose (`ai/tools/bloat-judge/deadline.go`) — a timeout tuned in the tree would
travel to everyone on the next commit.

## Resolving

Supply the client and the task. The resolver returns the requested settings and the policy digest; it
never launches an agent and never reports what ran. Keep requested and observed settings apart in any
record. Reuse a resolution while client, task and policy are unchanged — not once per file or command.

Which fields a transport carries differs, and a field it cannot carry is dropped in silence:

| Transport | Model | Effort |
| --- | --- | --- |
| Claude CLI (`claude -p --model`) | yes | no |
| Claude subagent dispatch | yes, from a fixed set of aliases | no |
| Codex CLI | yes | yes |

**So a Claude row states a model** — the policy refuses one carrying an effort alone, because it reads
as a saving and changes nothing. Keep Claude models as the client's aliases (`haiku`, `sonnet`,
`opus`), which both Claude transports accept where a dated API id is rejected by subagent dispatch.

Unknown tasks and unsupported selections fail visibly. Do not substitute a cheaper model or another
provider. An explicit user model change updates the relevant run; a background config edit affects new
runs, not work already dispatched.

## Work and results

Use native leaf workers and completion waits where the client has them. Deterministic commands, schema
validation and waiting need no model worker at all — the cheapest call is the one not made.

**A lower tier and a smaller read are separate savings, and a locating sweep earns both.** Where the
client offers a read-only search worker that returns excerpts rather than whole files, work that only
has to find something uses it: the tier sets the price per token and the worker sets how many there
are. A worker that reads whole files to report paths pays full price for context it never uses.

Supply bounded context: the requirement, resolved scope, relevant decisions, artifact paths and the
return contract. Full conversation history is justified only where that reasoning is a required input.
Return findings and evidence, not exploration transcripts.

Record task, requested and observed model/effort, policy digest, scope, candidate identity, elapsed
time and provider-reported usage where available. Preserve missing usage as unavailable, and keep
cached input distinct from uncached. A completed worker is not a passed review.

Evaluate a cheaper assignment on preserved meaning, accepted results, retries, cost and elapsed time
before adopting it. **Judge cost per completed task, not per call** — a cheap worker that needs a
second pass cost more than the expensive one. Local inference remains deferred; the resolver starts no
local service and offers no cloud fallback.
