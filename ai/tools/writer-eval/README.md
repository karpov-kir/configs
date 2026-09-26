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
`WRITER_EVAL_CASE` narrows a run to the cases whose name starts with one of its comma-separated
prefixes. `WRITER_EVAL_ROLLS` sets
the roll count, `WRITER_EVAL_PARALLEL` a run's workers, `WRITER_EVAL_SLOTS` the calls in flight across
every run, `WRITER_EVAL_FULL` a run of every roll kept as its rules' column, and `WRITER_EVAL_DUMP` a
file to write every raw return to. `WRITER_EVAL_RESUME` reads back the rolls that dump already holds
under the same rules and prompt, and asks only the rest: a usage limit stopped item 20's table at case
21 of 47, and the resumed run kept the 305 rolls it had read.

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
it. A rule that talks the writer into writing empties k03 first.

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

## What a change is measured on

A full table costs tens of dollars, so it runs seldom. A change lands on the cases it targets, Kirill's
eight (k15, k19 to k23, k25, k26), and every case whose rule paragraph it touches, all at fifteen rolls
with the bar-aware stop, read against the kept column for main's rules. A full table runs only where a
change edits text every case reads, such as the record contract, the fates or the exemplars, or where
no column is kept for main's rules.

A proven bystander, a case holding 14 or 15 of 15 on the kept column that the change neither targets
nor counts among the eight, is read short: five rolls, and five of five passes it. One miss sends it
to the full read. A case truly at 14 of 15 passes five of five about seven times in ten, so a proven
case costs about 9.5 calls against 15. A target and one of the eight always get the full read: a case truly at
12 of 15 passes five of five one time in three, and that is the regression the bar is there to catch. The table marks it `5/5 (short)`, and a short read is never kept
in a column. `WRITER_EVAL_TARGETS` names, by prefix, the cases whose rule paragraph a change rewrites,
so they get the full read with the cases the branch edits.

Replayed on the rolls of main's full table, the same 46 cases cost 690 calls read in full, 519 with the
bar-aware stop, and 382 with short reads too. Of 19 proven bystanders, one went on to the full read.

The column for main's rules at 4cc6a9eb8cae was read on 2026-09-25 at 16 workers: 690 calls, every one
answered by claude-opus-5-5, none retried, 15m44s wall, $35.83. 33 of 46 cases cleared their floor.

## Runs share the account through slots

Two runs used to fail together: on 2026-09-22 a full set and a single case ran at once, and most calls
came back `exit status 1`. What they shared was the account's capacity, so every call of every run on
the machine now takes one of 16 lock files under the user's cache (`WRITER_EVAL_SLOTS`), and each run
starts 16 workers (`WRITER_EVAL_PARALLEL`). A killed run's locks go with its process. A full table at
32 slots on 2026-09-25 failed 404 of 690 calls after a clean 72-call test there, so the default stays
at 16 until a full table runs clean at 32. A call failing for any reason but a usage limit is retried
once, and the header counts the retries. A usage limit stops the table and names the account.

Measured on 2026-09-25 on Opus, 72 calls, the check off, every roll run:

| workers | wall | errors | mean call |
|---|---|---|---|
| 4 | 366s | 0 | 20s |
| 8 | 195s | 0 | 21s |
| 16 | 117s | 0 | 23s |
| 32 | 81s | 0 | 29s |

The account starts queueing past 16, and 32 gains on a short burst but not over a full table. A roll costs about 3.5k tokens before the
brief and $0.08 in all on Opus, so a full table of 46 cases at 15 rolls runs about $55. Every table
prints its own figure: wall time, calls, tokens, cost, and the model that answered.

A run measures the rule files as they stood when it started, which `readRules` says in the code. So a
rule edit during a run leaves a table naming text the run measured nowhere, and the header hash is the
only thing that says so. Finish the run, or kill it and start again.

A table stops a case once it has the count the bar needs: its floor, or the kept column's count less
two. It stops the whole table at the first case that can no longer reach that count.
`WRITER_EVAL_FULL=1` runs every roll and keeps the table under `testdata/columns/` as the column for its
rules, so main's column is measured once per rule set.

## The record check runs inside a roll

The pipeline's writer runs the record check on each block before it writes it. It rewrites for each
finding, and after two rewrites it answers `none` and sends the facts to the human. The eval's writer
used to read the check instead of running it, so no check ever sent a block back here. Run 10's check
refused one block twice over a defect in the check itself, and the fact reached the human. The case
for that site read 14 of 15, because no roll met the check.

So a roll now runs that loop. The writer returns its record beside the block, the harness pipes both
to the checkout's `voice-check.sh --profile=comment --source --record -`, and a finding goes back to
the writer as its next turn. A block still refused after two rewrites is scored as the pipeline ends
it: `none`, with `does not fit` routed. The table counts the rolls the check sent back.

A roll can now cost three calls instead of one, and every column measured before 2026-09-24 was read
without the loop. A move between such a column and a later one mixes the rule's change with the loop's.

`WRITER_EVAL_CHECK=off` runs each roll as one call again. Two near-identical rule sets read under the
loop on 2026-09-24 moved unrelated cases by up to seven rolls, so a rule change is measured against an
older column with the check off until the loop's own band is recorded.

## The harness now reads the pipeline, and batch3 is retired

Until 2026-09-24 the eval's writer never produced a record, its identifier list lacked the hyphenated
names the strip writes, and its prompt restated the brief in its own words. The pipeline's writer
fills the record, reads that list and reads the brief alone. The harness now gives the eval's writer
the same, and it keeps only what the brief cannot say: that no tool runs, and the shape it parses.

So batch3, main's column of 2026-09-23, is retired. It was read on the old harness, and the same
rules read differently on this one. Main's rules on the one-call harness, before its prompt lost the
restatements, read k23 at 1, k03 at 3 and k01 at 4, where batch3 read 10, 13 and 15. Those three
fail on main's rules under the faithful harness, and item 19 takes them first. After the prompt lost
its restatements, the branch read k23 0, k03 4, k01 1, k08 9 and k13 12, and main's rules read k23 0.
The rest of that pair was not run.

Item 18's rules read within two of main's rules on the same one-call harness at every case but l08,
11 against 14, with its floor at 9. The same case read 1 of 15 on the branch after the prompt lost its
restatements, on over-the-sentence-ceiling and rename. Both causes are item 19's: the sentence ceiling
that a second fact at the site's branch pushes past, and the audit reading a description as a term.

## The loop's band

With the check loop on, main's rules at 7d412e50 were read twice on nine cases, every roll, on
2026-09-25. Eight of nine moved by one roll or less:

| case | first | second |
|---|---|---|
| k06 | 7 | 6 |
| k08 | 8 | 9 |
| k14 | 7 | 6 |
| k19 | 15 | 15 |
| k21 | 15 | 15 |
| k23 | 15 | 15 |
| k35 | 15 | 15 |
| l05 | 7 | 11 |
| l11 | 14 | 15 |

l05 moved by four, and the loop sent back only one or two of its rolls. Its writers split on the brief's
drop of a claim a reader takes for granted. The pair cost $34 with no retry.

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

## A restating judge, built and left unrun

A second judge read a block beside its code and answered whether the block added anything the code
did not show. It was for l03, a claim that paraphrases the body in words the code does not spell. It
was built on 2026-09-23 and never run. The blind reader above had shown that a model asked to connect
a block to its code connects almost anything. l03 had meanwhile reached 14 of 15 under the writer's own
per-slot drop. A judge there was machinery over a case that already held.

## A spelling check, measured and left unbuilt

A comment word missing from both the repository's tree and the system dictionary was measured as a
finding on 2026-09-23, over the 60 reviewed files. It flagged 158 occurrences over 88 distinct words in
notes a reviewer left standing. Most were ordinary lower-case words the dictionary lacks, 23 were a
product's or a vendor's name, and 9 were acronyms. It measured the dictionary's age, and a dictionary is
a kept list, which no source of these checks may be. A coined compound the tree does not spell stays the
finding. Ordinary words are the register scan's ground.
