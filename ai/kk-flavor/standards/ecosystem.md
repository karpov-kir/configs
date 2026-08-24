# Changing the Ecosystem

Rules for editing what agents read: skills, standards, prompts, templates, `CLAUDE.md`. Every line here is context each future run pays for. The bar is never "is this true" — it is "does this change what an agent does".

These rules bind any edit, however small. Run the ecosystem lane after a batch of edits here.

## Earn the place

- **Delete before you rephrase.** Keep what is essential, plus the supporting detail that makes it unambiguous. Drop the rest.
- **Nice-to-have is a cut.** A rule that fires rarely, or that a competent agent follows anyway, buys nothing and dilutes the rules that matter.
- **A rule you add names what it replaces**, or says plainly that nothing covered this.
- **The bar rises with how often the file loads.** An always-read doc takes only what applies to nearly every task; a trigger-loaded standard, its activity; a skill body, its lane.

## No evidence in a rule file

Beyond [writing.md](writing.md) → **Density**: no anecdotes, counts, dates, "observed:", and the like. Justify a rule only where an agent would otherwise override it.

## One home

Every rule lives in exactly one file; everywhere else cross-references it by path.

**Ownership is stated in the owned file, never broadcast to the files that do not own it.**

**The shared layer — a standard, a template under `kk-flavor/`, `CLAUDE.md` — never names a skill, and never cites anything inside one**: not a section, not a file it owns, not a script it ships. The dependency runs one way. A standard names the **lane**; the skill filling that lane binds itself to the name and cites the standard, never the reverse. Move the rule up and let the skill cite it there. Skill to skill, the citation is normal.

## Conventions a new file joins

- **A skill authored in this tree joins one of two families**, by prefix: one works in any repo, the other belongs to a single workflow and carries that workflow's own on-disk machinery. An installed tool skill is a skill outside this tree, not a third family inside it.
- **A skill the human always initiates carries `disable-model-invocation: true`**, which drops its description out of every session's context. Model invocation is for the skill that must fire on work the human would not think to name.
- **Cite a section as `<file>.md → **Section**`.** That form is machine-checked; "its **Report** section" is not.
- **A skill that runs a script cites it by full path — `~/.claude/skills/<skill>/scripts/<x>.sh` — whenever the run's working directory can hold code the human did not write.** The test is where the script runs, not who owns it.
- **A new skill is not live until it is mounted.** A directory added here needs a symlink at `~/.claude/skills/<name>` pointing back at it, and **the mount is proven by running the ecosystem lane's wiring check** — as the last step of adding one, never as a later tidy-up. That lane names the script.

## Move it before you cut it

- **Split** a branch not every run takes into a file the skill names at that branch. The pointer must sit where the branch is taken and say the file is the whole delta for that path.
- **Extract** a rule a script can assert into the script (**Prefer the mechanism**, below).
- **Reuse** — where two files state the same procedure, one skill owns it and the others invoke it, naming only their own delta.
- **Demote** a rule that fires for one activity out of an always-read file into that activity's standard.

A move is only a win when the common path genuinely never needs the text — a rule that silently shapes behaviour on every run stays, however rarely it is quoted.

**A reference file earns its place when an agent takes something out of it, not when it reads it** — vendorable code, a template, a checklist it fills. Example code illustrating a rule the prose already states is a cut.

## Prefer the mechanism

- A rule a script can enforce belongs in the script; prose duplicating what a script already enforces is a deletion.
- **A change to a shared script lands its call site in the same edit.**
- **A script is held to the bar it enforces** — converting prose into a script moves the cost rather than removing it.
- **The conversion is a win only where the enforcement is known to fire.** Each script's header states its test position: the `-test.sh` that covers it, or `# untested: <why>`. `check.sh` proves both — that the position is stated, and that the `-test.sh` it names exists.

## Memory

`CLAUDE.md`'s Memory section is a staging area: an entry stays there until it fits a standard or a skill, then moves into that file. Entries are authoritative — don't reorganize them.

## Approved means edited now

An improvement the human approves is applied to the file in this session, in their repo's working tree.
