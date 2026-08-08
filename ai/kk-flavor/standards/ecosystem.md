# Changing the Ecosystem

Rules for editing what agents read: skills, standards, prompts, templates, `CLAUDE.md`. Every line here is context each future run pays for, so the bar is never "is this true" — it is "does this change what an agent does".

These rules bind any edit, however small. Run `/kk-ecosystem` after a batch of edits here.

## Earn the place

- **Delete before you rephrase.** Keep what is essential, plus the supporting detail that makes it unambiguous. Drop the rest.
- **Nice-to-have is a cut.** A rule that fires rarely, or that a competent agent follows anyway, buys nothing and dilutes the rules that matter.
- **A rule you add names what it replaces**, or says plainly that nothing covered this.
- **The bar rises with how often the file loads.** An always-read doc takes only what applies to nearly every task; a trigger-loaded standard, its activity; a skill body, its lane.

## No evidence in a rule file

Beyond [writing.md](writing.md) → Density: no anecdotes, counts, dates, or "observed:". Justify a rule only where an agent would otherwise override it.

## One home

Every rule lives in exactly one file; everywhere else cross-references it by path.

## Conventions a new file joins

- **A skill joins one of two families.** `kk-*` works in any repo; `idsd-*` belongs to the intent workflow and carries its `.idsd/` machinery. A name outside both is routed by nothing and checked by nothing.
- **A skill the human always initiates carries `disable-model-invocation: true`**, which drops its description out of every session's context. Model invocation is for the skill that must fire on work the human would not think to name.
- **Cite a section as `<file>.md → **Section**`.** That form is machine-checked; "its **Report** section" is not, and prose that drifts to the unchecked form shrinks the guard silently.
- **A skill that runs a script while its working directory holds code under review cites that script by full path** — `~/.claude/skills/<skill>/scripts/<x>.sh`. Everywhere else `scripts/<x>.sh` plus "(this skill's dir)" is enough. Inside a checkout of someone else's branch it is not: a relative name resolves against that checkout first, and whoever opened it chose what sits there.
- **A new skill is not live until it is mounted.** The loader reads `~/.claude/skills/<name>`, so a skill directory added here needs a symlink there pointing back at it. `kk-ecosystem`'s `scripts/check.sh` proves the mount, the family name, and every reference the new file makes and receives — run it as the last step of adding one, never as a later tidy-up.

## Move it before you cut it

Deleting is not the only way to stop paying for something. Look for these before you judge a rule dead — each keeps the instruction and stops it loading:

- **Split** a branch not every run takes into a file the skill names at that branch. The pointer must sit where the branch is taken and say the file is the whole delta for that path, or the rule is now absent rather than deferred.
- **Extract** a rule a script can assert into the script (**Prefer the mechanism**, below).
- **Reuse** — where two files state the same procedure, one skill owns it and the others invoke it, naming only their own delta. A third copy is the signal you already needed this.
- **Demote** a rule that fires for one activity out of an always-read file into that activity's standard.

A move is only a win when the common path genuinely never needs the text. A rule that silently shapes behaviour on every run stays, however rarely it is quoted.

**A reference file earns its place when an agent takes it, not when it reads it.** Vendorable code, a template, a checklist it fills — yes. Example code illustrating a rule the prose already states — no: it costs a file to maintain and drifts from the prose the moment either changes.

## Prefer the mechanism

- A rule a script can enforce belongs in the script; prose duplicating what a script already enforces is a deletion.
- **A change to a shared script lands its call site in the same edit.** A subcommand nothing invokes is invisible, not half-finished.

## Approved means edited now

An improvement the human approves is applied to the file in this session, in their repo's working tree.
