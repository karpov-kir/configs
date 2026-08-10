# Ecosystem size history

Appended by `scripts/stats.sh --append <note>`. One row per pass that changed the tree.
Read it to answer *when did this last shrink, and has it grown since* — the question that
decides whether a scoped `kk-ecosystem` pass is enough or a `kk-reduce` campaign is owed.

What the columns mean, and what a `+` on the always-loaded figure means: `kk-foreman`'s SKILL.md,
step 1.

| date | prose | scripts | always-loaded | skills | what ran |
|---|---|---|---|---|---|
| 2026-08-07 | 22582 | 8040 | 1293 | 18 | kk-reduce campaign: prose -43%, always-loaded -47%; +kk-foreman, kk-reduce, kk-skillcraft |
| 2026-08-08 | 25228 | 10911 | 1466 | 19 | 13-lens holistic review + tier 1-3 fixes: merge gate closed, kk-pr-review fork/secret guards, check.sh +5 checks, 40+ prose findings applied |
| 2026-08-08 | 24951 | 10960 | 1466 | 19 | kk-ecosystem over the session's changed .md: one-home reconciliation across router/standards/idsd, 2 tighten lanes, 4 unchecked citations repaired |
| 2026-08-08 | 26240 | 13835 | 1517 | 19 | kk-foreman chain: code+security review x2, refactor, ecosystem lane, humanize over comments. 12 script defects fixed — promote staged the report, both scanners had false-cleans, diff-header forgery, symlink writes, discard deleted a hand-written charter |
| 2026-08-08 | 26795 | 13835 | 1520 | 19 | kk-foreman: kk-ecosystem over the working-tree diff (skillcraft 4 dirs, tighten 6 artifacts). Hoisted the self-finishing rule to skill-protocol 'Finish in the lanes your edits opened' from 5 scattered copies; foreman/quality-pipeline/kk-reduce now consume a returned handoff; idsd-build step 5 spawns kk-code-review instead of hand-rolling it; kk-reduce HANDOFF renamed CROSS-SCOPE to free the term |
| 2026-08-09 | 26992 | 13835 | 1520+ | 19 | kk-foreman round 2: kk-ecosystem over the 5 residue files (skillcraft 3 dirs, tighten 5). Receiver-side handoff duty given one home in skill-protocol; spawn-prompt.md gained the handoff slot that makes the depth-1 terminator enforceable; kk-reduce AGENT-BRIEF gained a HANDOFF return field. NOTE: always-loaded under-reports - ai/CLAUDE.md pulls @RTK.md (140 words, untracked) which check.sh does not walk, so the real figure is ~1660 |
| 2026-08-10 | 27230 | 17217 | 1520+ | 19 | check.sh/stats.sh hardening, 6 review rounds. Budget now names uncounted @imports (+ marker on the ledger cell). 13 defects fixed, 8 security: path traversal and leaf-symlink escapes into the budget, inject.md read despite refusal (/dev/zero hang), unbounded output (2.7MB -> 100KB), quadratic scans (>2min -> 2s), both scanners silenced by a PR-authored '* -diff' (now --text). Counting findings in the scanners introduced an awk mod-256 exit wrap and guarded it in the same edit; at HEAD 'found' was a flag, so that was never a live bug. 4 shared regions now fenced and drift-checked by check.sh rather than by comment |
