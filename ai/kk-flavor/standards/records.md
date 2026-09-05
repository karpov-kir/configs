# Project Records

A record agents **append to** across many runs rather than rewrite: a decision log, a playbook of how a repo is operated, a project's vocabulary, the constraints its work inherits. **This binds any skill that writes one.** The skill names three things: the file, the writer that stops two runs clobbering each other, and **the one point at which the record is pruned**. Two pruning points put two agents on the same cap with different readings of what it can afford to lose. That point owns the routine sweep of the whole record. An append into a full record still forces one move where it happened (**Reaching the cap**).

**Where nothing can overlap, the skill says so and writes by hand** — a record only a human ever triggers is that case.

A **generated** file is not one of these. Regenerated from a source that is itself bounded, it needs no cap and no promotion — pruning its source is the whole of it.

## Every entry is dated and counted

`<count>x | <date> | <the entry>`. The date is the **last time it was confirmed**, never the day it was written.

**Read the record before appending to it.** An entry restating one already there bumps that entry's count and date instead of adding a line.

**A run that reaches an entry and finds it still true bumps it**, appending nothing.

**Widen an entry rather than adding one beside it.** Where what you would append is a narrower, broader or sharper version of something already there, revise that entry to cover both cases.

## The cap evicts by what the record can afford to lose

Every record is capped, states its bound in the file itself, and **the writer holds that bound** — an append into a full record refuses. Where the record is hand-written under the exception above, the skill states the bound instead, and its own pruning point holds it.

**Eviction is the judge's** — `~/.kk-flavor/scripts/bloat-judge.sh record-entry` over every entry.

**Never evict by count, and never from the top of the file.** The count says how often the record has been needed, never how much the next agent needs the entry. Age is worse: the settled decisions everything rests on are old *because* nothing has needed to revisit them.

## Reaching the cap

The cap is a prompt to judge, never a queue to trim from the bottom. Work the moves in this order and stop at the first that applies:

- **Delete** what is no longer true, whatever its count or date — its subject gone from the code, or a later entry superseding it.
- **Promote** what a later agent must not lose (below).
- **Combine** two entries that carry one idea between them: revise the higher-count one into the wording covering both, then evict the other. The survivor keeps its count, so the fold costs the record no reach.
- **Evict** what the judge names, once the three above have found nothing — never an entry this run bumped.

**With a new entry in hand, it is judged alongside the incumbents.** A new entry the judge names is not recorded at all, and saying so is the whole of it; one it spares while naming no incumbent is turned away the same way — the cap holds.

**Say which move you made and on what.**

**Take every move here without asking.** A cap nothing may act on is a cap the file grows past for good. This departs from [skill-protocol.md](skill-protocol.md) → **Orchestrators — interactive first**, which routes a deletion that loses reasoning to the human: an evicted entry in a tracked record is still in the history, so nothing is lost. **The exception is a record whose header says a human owns its wording** — there every move is a proposal.

## Promotion is the exit upward

**A count that keeps rising is a rule nobody has written down yet.** A quiet entry you would not accept losing goes up too: **promote at any count**. Move it to whatever binds:

- how the project is built → the record holding its constraints
- what the project is for → the file holding its scope
- a domain term → the record holding its vocabulary
- how agents or the project work → the standard or `CLAUDE.md` owning that lane

**Promoting deletes the entry.**

**A file that receives promotions carries a test as well as any cap it has**, applied to every entry on every edit:

- **A principle, a constraint, a scope line** — it must name what it rules out that nothing else already rules out. One that rules out nothing is deleted, not reworded.
- **A vocabulary entry** — a term no artifact uses is deleted.
