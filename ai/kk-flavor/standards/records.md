# Project Records

A record agents **append to** across many runs rather than rewrite: a decision log, a repeat-tracking findings file, a playbook of how a repo is operated. **Binding on any skill that writes one** — the skill names the file and sets its bound.

A **generated** file is not one of these. Regenerated from a source that is itself bounded, it needs no cap and no promotion — pruning its source is the whole of it.

## Every entry is dated and counted

`<count>x | <date> | <the entry>`. The date is the **last time it was confirmed**, never the day it was written.

**Read the record before appending to it.** An entry restating one already there bumps that entry's count and date instead of adding a line.

## The cap evicts by reach, not by age

Bound the record, and state its bound in the file itself. **Evict the lowest count first, the oldest date breaking a tie** — never from the top of the file, and never on age alone. Age alone deletes the settled decisions everything rests on, which are old *because* nothing has needed to revisit them.

## Promotion is the exit upward

**A count that keeps rising is a rule nobody has written down yet.** Move it to whatever binds:

- how the project is built → the record holding its principles and gate commands
- what the project is for → the record holding its scope
- a domain term → the record holding its vocabulary
- how agents work → the standard or `CLAUDE.md` owning that lane

**Promoting deletes the entry.**

## Deletion is not eviction

Delete an entry outright, whatever its count or date, when its subject is gone from the code, when a later entry supersedes it ([writing.md](writing.md) → **Density**), or when it was promoted. Eviction reaches only an entry that is still true and still unreached.

## The promotion targets carry a test, not a cap

A record that **receives** promotions holds what the project has settled, so no line cap. Bound it by a test applied to the whole file on every edit:

- **A principle, a gate command, a scope line** — it must name what it rules out that nothing else already rules out ([ecosystem.md](ecosystem.md) → **Earn the place**). One that rules out nothing is deleted, not reworded.
- **A vocabulary entry** — a term no artifact uses is deleted.
