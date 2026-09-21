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

## The rules measurement cut, and what the cutting has in common

Three proposed rules were cut in one batch on 2026-09-20, and the campaign has cut one the same way
in every batch before it.

| the rule as proposed | what it reached | what landed |
|---|---|---|
| a note carries no connective | 131 of 304 notes | dropped; the notes it reached are clear |
| a sentence is at most fifteen words | 354 of 711 sentences | dropped, for the same reason |
| a summary restating its declaration is deleted | 2 to 4 of 407, by an arbitrary window | a report, and it deletes nothing |
| a compound the code spells is renamed | 12 of 129, all of them terms that stay | a check behind a `domain` allowlist |

Each was written as a blanket word test or length test, and each reached prose nobody had flagged.
The rules that survived every batch strike a shape the text carries: a counterfactual consequence at
14 of 304, a boolean written as a person at 3 of 711, an elided verb at 1 of 711.

Two rules also reached nothing at all, which is the other failure and looks like a clean measurement
until the case that prompted the rule is checked against it. The summary strike step as first written
reached neither block it was written for. The `anthropomorphism` check wanted the article beside the
word and so passed over "a scheme-blind yes", the one sentence it existed for.

So a proposed rule is counted twice before it lands: over the population it will run against, and
over the case that prompted it.

## The class no presence test reaches

Three mechanisms were cut at one class of claim and all three failed. The claim paraphrases what the
code does, in words the code does not spell: `combination` is what a loop inside a loop means, and
`owner` is what a call bound to its object means.

| the test | what it did |
|---|---|
| strike the claim's words against the declaration | 2 of 304, and it leaves the paraphrase standing |
| match the claim's nouns against a test's name | reaches one of five nouns on the case it was cut for |
| read the claim's grammatical subject | 158 of 304, because a domain noun is an identifier |

The third is the one worth keeping in mind. A codebase about devices and streams spells `device` and
`stream` in its code **because** they are the domain, so a note whose subject is a device reads as a
note about the code. There is no threshold between those, since the thing that makes a word a domain
noun is the thing that puts it in the code.

So a presence test over the code's own vocabulary cannot reach this class. What decides it is the
writer's judgement, and what holds that judgement is the fixture.

## A new check is run over its own comment

Three checks have reported their own documentation on the first run they were given. `coined-identifier`
found the compound in the sentence explaining it. `anthropomorphism` found the phrase it was cut for,
in the comment naming that phrase. Then `anthropomorphism` found "saying no more than itself", where
`no` is the ordinary word before a comparative, and that one was the check in the wrong.

So the first thing a new check is asked is its own comment. A finding there is one of two things: a
comment that should take the plain wording, or a false positive announcing itself before it reaches
anybody's diff. Both are worth having before the check lands.

## Why a fourth mechanism failed the same way

`paraphrased-identifier` was proposed for prose that spells an identifier out instead of naming it:
"the preferred key systems setting" where `preferredKeySystems` was meant. Measured by the fewest
camel humps an identifier needs before the check fires:

| minimum humps | hits | of |
|---|---|---|
| two | 41 | 304 notes |
| three | 1 | 304 notes |
| four | 0 | 304 notes |

At two humps every hit is a domain phrase: `DeviceClaim` as "device claim", `contentType` as "content
type", `adaptationSet` as "adaptation set". The identifier is named after the phrase, so the prose is
right and the identifier is the derivative. At three humps the single hit is a specification's term.

That is the same fact that defeated `restates-code`, the subject test and the mechanical rename: **an
identifier's words are the domain's words**, because that is where an identifier's words come from. A
test over the code's vocabulary cannot tell prose using the domain from prose copying the code.

Four mechanisms have now failed on it. The class is settled by the writer's judgement and held by the
fixture.
