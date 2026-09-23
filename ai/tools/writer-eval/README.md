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

k17 on 44d4720, the same way, came back 6 and 3. Four earlier readings across rule texts that
barely touch it read 7, 10, 3 and 4. So k17 alone spreads by three, one past the band this section gives, and
a move on k17 of three or fewer is inside what the instrument does there.

A floor is a share of the rolls in whole percent. Floors were counts once, written against five
rolls. The day the count went to fifteen, each of them asked for a third of its intent, and the table
was silent on that.

## What a run costs

390 calls for the full set at fifteen rolls, four in flight, and about half an hour. One run took 25 minutes and a later one 33. Between them two attempts never finished, at 2h51m and 4h18m, and both were stopped before any table came out. A run of nine cases took 61 minutes. The account is shared with every other session
on the machine, and the runs queue behind each other.

The four-minute deadline on a call did not hold. Both stalled runs ended with every slot in flight
held by a call past sixteen minutes, alive, a second and a half of processor time each, waiting on
the network. `WaitDelay` now closes the pipes shortly after the kill. The first full set run with it in place finished in 33 minutes, where the two before it had not finished at all. **That is evidence and not a diagnosis.** Where a run wedges again, read it before anything else lands. Check `pmset -g log` for sleep first. The call deadline runs on a clock that stops while the machine sleeps, and `ps` elapsed time does not. A run on 2026-09-23 took 3 h 40 m through fifteen-minute sleep cycles, with calls showing 33 minutes against a four-minute deadline. `caffeinate -i -s -w <pid>` holds the machine awake for one test process and ends when it does. The two stalled runs of 2026-09-22 ran before 08:31 that morning, and the power log shows the machine sleeping in every hour from midnight to eight. Sleep fits both stalls, so `WaitDelay` may not have been the fix, and the run after it may simply have run awake.

Read the run's own processes by the test binary's path and never by a name a shell wrapper also carries. A check for the wedge timed `go test` and `sed` as though they were calls, because `pgrep -f writer-eval.test` matches the wrapper's command line too. Whether that is the whole of it is unmeasured: the runs were
stopped, not diagnosed.

Where the full set will not fit, an isolation run is k03, the case under test and the plain half.
k03 is the guard. It is a negated case over four conditions, and the refactor lane is owed
it. A rule that talks the writer into writing empties k03 first. The full set runs once at landing.

## The rule text a run measured

The two rule files are read once, when the run starts, and every roll is given that text. The table
header prints a short hash of it. An edit made to a rule while a run is going reached the rolls after
it before this, and the table named the text it had measured nowhere.

## k11, and a floor a case has never met

k11 asks the writer to name both the caller's act on a copy and the site that makes the copy. The
note pattern is what carried it. That pattern demanded a consequence at every site, and at k11's site
the consequence is worth having.

Its no-change pair reads 2 and 2 of 15 against a floor of 60%, so the case has yet to meet its own
floor. The pattern's removal took it to 0. A rule telling the writer to state the bearing where the
code beneath would read as a mistake without it took it to 8. That is the second largest single move
this set has measured, and it is one roll under the floor.

The floor stays at 60%. A floor set to what a case just scored means whatever the last run gave, and
the pair rule this file opens with exists to stop that. k11 reads as failing, and the bar it fails is
one the writer has yet to be shown clearing at that site.

## The two halves

The labelled half is `testdata/cases/`, one file per site, each carrying what a reviewer decided and
why. The plain half reads a directory of reviewed source named by `JUDGE_EVAL_PLAIN` and takes the blocks
a reviewer left alone. It asks what share of them the writer writes for, and what share of those fail
a check. The set is named by the environment and by no committed path. A case there
is named by its position in the sorted set, and only counts leave the run.

## One run at a time

Two runs in flight together fail. On 2026-09-22 a full set and a single case ran at once, and the
writer row came back `exit status 1` on most calls. The table printed `0 of 15  error x15` for
twenty-odd cases, which reads exactly like a rule that broke everything. Read the `what came back`
column before believing a zero: `error` there is a run that did not happen.

A run also measures the rule files as they stood when it started, which `readRules` says in the
code. So a rule edit during a run leaves a table naming text the run measured nowhere, and the header
hash is the only thing that says so. Finish the run, or kill it and start again.

## What a case cannot measure

The writer a case runs is told it has no tools. Every case hands it the code, the facts, and
whatever `--- callers` and `--- tests` sections the case carries, and it answers from that alone. A
writer working a real change set has the repository: it follows a field's consumer into another
file and decides what it found there.

A case also hands the writer its callers in a `--- callers` section, which no run has. In a run the
strip emits no such section, and the writer finds a caller by `grep` under question 1. A case reads
what the writer does with callers it has, and says nothing about whether it would have found them.

So a defect that turns on what the writer goes and finds cannot go red here. Case k18 is that
shape. Four cuts of it, each at fifteen rolls, came back written 15, 15, 13 and 15 times. That
two-roll move sits inside the no-change band this file's first section gives. The rule k18 pins
landed on the evidence from the change set. The case pins the correct answer, and it guards against
no defect.

## The blind reader, measured and deleted

A reader saw a block and the line under it, and answered what that line does because of the block,
or `cannot say`. A `cannot say` went back to the writer. The bar came before the reader. It had to
answer `cannot say` on the ten blocks a reviewer flagged or a run left as a residual. Then at most one
note in twenty of the reviewed set could get that answer. It ran on sonnet, fifteen rolls a block.

The first brief asked what the line does because of the block, and 8 of 10 failed as the bar wants.
The two it passed it read off the code alone and left the fact out. A single allowed round asked for
the sentence to rest on the fact, with a constant's value counting as what a fact bears on. It then
connected nearly anything: 6 of 10, with k20, k21 and k07 passed on every roll. The bar deleted it,
and the plain half never ran. Its brief and harness stay on the `comment/read-gate` branch.

Three readings from it outlive it. A three-roll majority agreed with fifteen on every labelled block.
The invented consequence at k17 and k23 got through on one roll in fifteen under the first brief.
Under the second it got through on four at k17, so a model reader accepts that shape too. The branch
writer's own k07 block claims to tie its fact, and the reader found it tied on fifteen rolls of
fifteen.
