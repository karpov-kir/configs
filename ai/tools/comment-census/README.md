# comment-census

Counts the sentences a proposed comment rule would reach, before the rule is written down.

Five rule sets into this campaign, four rules had been specified as a word test or a length test and
every one had to be re-cut once somebody counted it. The counting was done by hand each time, into a
scratchpad that was gone by the next session. This is that counting, kept.

## Running it

```
JUDGE_EVAL_PLAIN=<a directory of reviewed source> go test ./comment-census/ -run TestCensusOverThePlainSet -v
```

The set is named by the environment variable and never by a committed path. It is somebody else's
code and this repository is public, so counts and the sentences a shape matched are all that leave
the run, and a case is named by its position in the sorted set.

The test asserts no bar. A shape's count is evidence a rule is cut against, and a bar invented here
would teach the next rule to clear it.

## The measurement of 2026-09-20

60 files, 476 comment blocks, 407 summaries over a declaration, 304 note sentences, 711 sentences.

| shape | count | of | verdict |
|---|---|---|---|
| note-connective | 131 | 304 | dropped, reaches clear prose |
| sentence over 15 words | 354 | 711 | dropped, reaches clear prose |
| so-clause | 46 | 304 | split three ways below |
| so-clause naming an element | 20 | 46 | the shape the pattern keeps |
| so-clause with a pronoun subject | 5 | 46 | reads clear, left alone |
| so-clause naming no element | 21 | 304 | the shape the pattern drops |
| counterfactual-consequence | 14 | 304 | landed as a check |
| of the 21, also counterfactual | 6 | 21 | the two rules are mostly apart |
| anthropomorphism | 2 | 711 | landed as a check |
| elided-verb | 1 | 711 | landed as a check |
| negated-case | 0 | 407 | no corpus scope, a writer step |
| coined-compound | 117 | 476 blocks | re-cut, most hits are code spans |
| note over 2 sentences | 21 | 476 blocks | |

Metaphor verbs: answer 30, reach 9, cover 5, sit in 3, settle 1, load-bearing 1. The writing
standard's other named tells — hide, climb, slip past, rubber-stamp, hedge, understate — fire zero
times over the 60 files. `answer` is the literal verb in a codebase that queries things, and `reach`
is literal about half the time, so both stay out. `cover`, `settle`, `sit in` and `load-bearing`
total ten and go in.

## What the numbers taught

A rule written as a blanket word test or length test reaches about half of good reviewed prose. A
rule that strikes a specific shape the text carries lands at a few percent and reads true in the
sample. Every rule this campaign has re-cut moved in that direction.

Two assumptions died on contact here. The connective ban was proposed to make notes simple and would
have rewritten 131 of 304 notes that nobody had flagged. The counterfactual clause was assumed to be
the subset of the clauses naming no element, and it is 6 of the 21.

## restates-code, measured before it was allowed to delete

The proposal was a check that deletes a summary whose content words the declaration beneath it
already spells, the writer's strike step turned into a tool. It was measured at 3 of 390 on summaries
the writer had produced. The population it would run against is different: the summaries a codebase
already holds. So it was measured there first.

| bodyWindow | restates-code | of |
|---|---|---|
| 8 lines | 2 | 407 |
| 20 lines | 3 | 407 |
| 40 lines | 4 | 407 |

The count is a function of how far past the declaration the strike reads, which is a number this
tool chose. That alone settles it: a check whose finding count moves with an arbitrary constant
reports, and it never deletes.

Reading the four at the widest window settles it again. Two are descriptions on catalogue constants,
where the constant's name carries the same words as its description because the description is what
the name was made from. Deleting those takes the description of an asset with it, which is the
provenance class rather than the restatement class. One more is a summary carrying a second clause
that the wide window struck by reaching code the summary never spoke about.

So `restates-code` lands as a report. The writer's own `none` stays the only thing that deletes a
block, and the strike step stays the writer's procedure.
