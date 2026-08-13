# Ecosystem size

Appended by `kk-reduce` alone, via `scripts/stats.sh --append <note>` beside it: one row before a
campaign, whose note ends `, start`, and one after. **A delta across that pair is the campaign's own
cut, not drift** — drift is measured from a closing row forward.
Not a per-pass log — git holds the history, this holds the measurements.
Read it to answer *when did this last shrink, and has it grown since* — the question that
decides whether a scoped `kk-ecosystem` pass is enough or a `kk-reduce` campaign is owed.

What a `+` on the always-loaded figure means: `~/.claude/skills/kk-foreman/SKILL.md` → **1. Read the state before choosing**, which reads this file.
What a note carries: `kk-reduce`'s own SKILL.md, which writes it. **A column is a measurement and is
never edited.**

**Rows dated 2026-08-08 to 2026-08-13 were bad measurements and are gone, git included** — don't hunt
for them. The 2026-08-07 row predates the defect and stands.

| date | prose | scripts | always-loaded | skills | what ran |
|---|---|---|---|---|---|
| 2026-08-07 | 22582 | 8040 | 1293 | 18 | kk-reduce campaign: prose -43%, always-loaded -47%; +kk-foreman, kk-reduce, kk-skillcraft |
