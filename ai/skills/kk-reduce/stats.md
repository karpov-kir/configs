# Ecosystem size

Appended by `kk-reduce` alone, via `scripts/stats.sh --append <note>` beside it: one row before a
campaign, whose note ends `, start`, and one after. **A delta across that pair is the campaign's own
cut, not drift** — drift is measured from a closing row forward.

`kk-reduce`'s own SKILL.md writes each row and defines what its note carries. **A column is a
measurement and is never edited.**

**A `+` on a row's always-loaded figure makes it a lower bound**: `stats.sh` named an `@import` it
could not resolve and left it uncounted. Read the delta between two marked rows as "at least this
much". From a marked row to an unmarked one, part of the rise is `stats.sh` resolving more rather
than the tree growing. The unmarked row's note says how much.

| date | prose | scripts | always-loaded | skills | what ran |
|---|---|---|---|---|---|
| 2026-08-07 | 22582 | 8040 | 1293 | 18 | kk-reduce campaign: prose -43%, always-loaded -47%; +kk-foreman, kk-reduce, kk-skillcraft |
| 2026-08-14 | 29843 | 30494 | 1852 | 20 | kk-reduce campaign: 40% of honest prose (md + script comments), always-loaded and script comments hardest, start [of the always-loaded figure, 139 words are imports this run resolved] |
| 2026-08-14 | 26876 | 21283 | 1578 | 20 | kk-reduce campaign: honest prose -26% (48266 to 35740), always-loaded -15%, script comments -52%; mutation harness was inert, now kills 25/25; drive gate 8/8. Open: check.sh's skill-dir scan has no case [of the always-loaded figure, 139 words are imports this run resolved] |
