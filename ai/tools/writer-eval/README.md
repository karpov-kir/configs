# writer-eval

Puts a labelled site to the real comment-writer row and scores what comes back. A rule change is
then read against cases somebody already decided, and not against the next block anybody writes.

## Running it

```
WRITER_EVAL=1 go test ./writer-eval/ -run TestWriterEval -v
WRITER_EVAL=1 WRITER_EVAL_CASE=k11 go test ./writer-eval/ -run TestWriterEval -v
JUDGE_EVAL_PLAIN=<a directory of reviewed source> WRITER_EVAL=1 go test ./writer-eval/ -run TestWriterEvalOverThePlainSet -v
```

Every roll spends a model call, so the suite skips both halves until `WRITER_EVAL` asks for them.
`WRITER_EVAL_CASE` narrows a run to the cases whose name starts with it. `WRITER_EVAL_ROLLS` sets
the roll count, `WRITER_EVAL_PARALLEL` the calls in flight, and `WRITER_EVAL_DUMP` a file to write
every raw return to.

## A floor is set from a no-change pair

**A floor is read from two runs of the unchanged tree. The run a rule just got does not set one.**

The roll count was five until 2026-09-21, and five cannot read this set. A run at fifteen rolls, with no rule
change at all, gave l01 6, l03 5 and k03 6. Those same three cases had read 5 of 5, 3 of 5
and 4 of 5 that day. A third of the labelled set sits near a coin, and at five rolls a coin prints as
certainty.

Two rounds of rule rewording were spent chasing a move the instrument had made. l03 came back 0 of 5
and then 4 of 5 over two runs whose rule text differed by one sentence that case never reads.

So: run the set twice on the unchanged tree, and the gap between the two columns is the smallest move
a rule is allowed to claim. A case's floor is set under its lower column, and a rule that moves a case
by less than the pair's own spread stays unmeasured.

The pair measured on 690b548, at fifteen rolls, with the tree the same for both columns:

| case | first | second |
|---|---|---|
| k03 | 7 | 8 |
| l01 | 7 | 9 |
| l02 | 9 | 10 |
| l03 | 5 | 6 |
| l04 | 11 | 11 |
| k11 | 2 | 2 |
| k12 | 1 | 2 |
| k13 | 2 | 0 |
| k14 | 4 | 5 |

**The spread is 0 to 2 of 15.** A rule that moves a case by two rolls or fewer has shown no more than
the instrument does. At five rolls the same cases swung by four, so the count is what bought the band.

A floor is a share of the rolls in whole percent. Floors were counts once, written against five
rolls. The day the count went to fifteen, each of them asked for a third of its intent, and the table
was silent on that.

## What a run costs

360 calls for the full set at fifteen rolls, four in flight. One run took 25 minutes. The next two
attempts at the same run never finished, at 2h51m and 4h18m, and both were stopped before any table
came out. A run of nine cases took 61 minutes. The account is shared with every other session
on the machine, and the runs queue behind each other.

The four-minute deadline on a call did not hold. Both stalled runs ended with every slot in flight
held by a call past sixteen minutes, alive, a second and a half of processor time each, waiting on
the network. `WaitDelay` now closes the pipes shortly after the kill. That bounds the case where the process dies
and the read goes on. Whether that is the whole of it is unmeasured: the runs were
stopped, not diagnosed.

Where the full set will not fit, an isolation run is k03, the case under test and the plain half.
k03 is the guard. It is a negated case over four conditions, and the refactor lane is owed
it. A rule that talks the writer into writing empties k03 first. The full set runs once at landing.

## The two halves

The labelled half is `testdata/cases/`, one file per site, each carrying what a reviewer decided and
why. The plain half reads a directory of reviewed source named by `JUDGE_EVAL_PLAIN` and takes the blocks
a reviewer left alone. It asks what share of them the writer writes for, and what share of those fail
a check. The set is named by the environment and by no committed path. A case there
is named by its position in the sorted set, and only counts leave the run.
