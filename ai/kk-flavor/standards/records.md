# Project Records

A record agents **append to** across many runs rather than rewrite: a decision log, a playbook of how a repo is operated, a project's vocabulary, the constraints its work inherits. **Binding on any skill that writes one.** The skill names three things: the file, the writer that stops two runs clobbering each other, and **the one point at which the record is pruned**. Two pruning points put two agents on the same cap with different readings of what it can afford to lose. That point owns the routine sweep of the whole record. An append into a full record still forces one move where it happened (**Reaching the cap**). That is not a second reading — both work the ladder below, on the one scale it names.

**Where nothing can overlap, the skill says so and writes by hand** — a record only a human ever triggers is that case.

A **generated** file is not one of these. Regenerated from a source that is itself bounded, it needs no cap and no promotion — pruning its source is the whole of it.

## Every entry is dated and counted

`<count>x | <date> | <the entry>`. The date is the **last time it was confirmed**, never the day it was written.

**Read the record before appending to it.** An entry restating one already there bumps that entry's count and date instead of adding a line.

**A run that reaches an entry and finds it still true bumps it**, appending nothing.

**Widen an entry rather than adding one beside it.** Where what you would append is a narrower, broader or sharper version of something already there, revise that entry to cover both cases.

## The cap evicts by what the record can afford to lose

Every record is capped, states its bound in the file itself, and **the writer holds that bound** — an append into a full record refuses. Where the record is hand-written under the exception above, the skill's own pruning point is what holds it instead.

**Eviction chooses by score** — `~/.kk-flavor/scripts/score.sh cut record-entry "<an entry the next agent would be worse off without>"` over every entry, and what falls at or below the line goes, the oldest date breaking a tie.

**Never evict by count, and never from the top of the file.** The count says how often the record has been needed, never how much the next agent needs this entry — an entry can be load-bearing and sit at `1x` for a month because nothing has gone near that area. Age alone is worse: it deletes the settled decisions everything rests on, which are old *because* nothing has needed to revisit them.

## Reaching the cap

The cap is a prompt to judge, never a queue to trim from the bottom. Work the moves in this order and stop at the first that applies:

- **Delete** what is no longer true — its subject gone, or a later entry superseding it.
- **Promote** what a later agent must not lose (below).
- **Combine** two entries that carry one idea between them: revise the higher-count one into the wording covering both, then evict the other. The survivor keeps its count, so the fold costs the record no reach.
- **Evict** what the score cuts, once the three above have found nothing.

**With a new entry in hand, it is scored alongside the incumbents.** The lowest of the N+1 loses, the oldest date breaking a tie. **A tie against the new entry is lost by the new entry** — broken the other way, arrival order is the tiebreak. A new entry that loses is not recorded at all, and saying so is the whole of it; one that wins takes the loser's place.

**Say which move you made and on what.**

**Take every move here without asking.** A cap nothing may act on is a cap the file grows past for good. This departs from [skill-protocol.md](skill-protocol.md) → **Orchestrators — interactive first**, which routes a deletion that loses reasoning to the human: that rule reaches a deletion whose reasoning dies with it, and an evicted entry in a tracked record is still in the history. **The exception is a record whose header says a human owns its wording** — there every move is a proposal.

## Promotion is the exit upward

**A count that keeps rising is a rule nobody has written down yet.** A quiet entry you would not accept losing goes up too: **promote at any count**, the `1x` one included. Move it to whatever binds:

- how the project is built → the record holding its constraints
- what the project is for → the file holding its scope
- a domain term → the record holding its vocabulary
- how agents or the project work → the standard or `CLAUDE.md` owning that lane

**Promoting deletes the entry.**

## Deletion is not eviction

Delete an entry outright, whatever its count or date, when its subject is gone from the code, when a later entry supersedes it ([writing.md](writing.md) → **Density**), or when it was promoted. Eviction reaches only an entry that is still true and still unreached.

**A file that receives promotions carries a test as well as any cap it has**, applied to every entry on every edit:

- **A principle, a constraint, a scope line** — it must name what it rules out that nothing else already rules out. One that rules out nothing is deleted, not reworded.
- **A vocabulary entry** — a term no artifact uses is deleted.
