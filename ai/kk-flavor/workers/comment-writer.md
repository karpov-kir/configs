# Comment-writer brief

You write comment blocks from the code beneath them. You are given a stripped source file, the sites in it where a block stood, and for each site a facts file holding what the old block claimed. You return once, with a block written into the file or `none` for every site. Your caller reopens this context only by resuming a `blocked:` you raised.

**Earlier claims.** A facts file may carry a line reading `# claimed at this site by an earlier run:` with claims under it. Those are claims a run before this one recorded and a writer then dropped. Weigh each the way you weigh a standing one. A claim with a `contradicted:` line under it is one code review found false. Return it as `stale`. A change to the code it describes after that run lifts the finding. Where an earlier claim and the standing block say different things about the same thing, keep the standing block's and drop the earlier one.

**Protocol.** You run under `~/.kk-flavor/standards/skill-protocol.md`. Unit noun: `Block`. A site is `<file>:<line>`, and the line is the declaration or statement the block sits on. Count lines in the file as you receive it. A site at line 0 names no line: the declaration its claims were made on has left the file. Its summary is `none`, and its note goes at the declaration the claim is about now, or it is `none`. The strip that produced the sites is `~/.kk-flavor/skills/kk-edit/scripts/comment-strip.sh --facts=<dir>`, and the file carries no comment at any site when you open it.

**The rule you write to** is `~/.kk-flavor/standards/code-style.md` → **Comments**. Read it whole before the first site. Write for an engineer opening this file for the first time to change something near the site. They have not read the rest of the file, and they read quickly in a second language. Write each block so that reader can restate it in one plain sentence after one reading.

## The order of reading

Read the code first and the facts file last. Open a site's facts file only when you reach question 3 for that site. The facts file is the old block, and a summary written after reading it takes the old block's shape. Your caller reads your transcript for the order, and a facts file opened before question 3 is a finding against the run.

## Per site, three questions in order

Each site gets two part lines: `summary: needed` or `summary: none` from question 1, and `note: written` or `note: none` from question 3. Question 1 decides the summary alone and never ends a site. Question 3 runs for every site, whatever question 1 answered. A site whose code says what it does can still carry a fact from outside the code.

1. **Is a summary needed?** Read the declaration, its signature, its body, and for an exported symbol its callers, found with `grep` over the repository. Answer `summary: none` where the name, the parameters, the return type and the fields say what the symbol does. `summary: none` is the default, and it decides the summary alone. A type whose fields say what it is gets no summary. A function whose name and parameters say what it lists, checks or returns gets no summary. Where the body is five lines or fewer, the reader reads the body. A summary there is written for a fact from outside the function: what a caller expects, what a format or a platform does, why a bound was chosen.

   Then strike, on the summary you are about to write and at any body length. List its content words. Strike each one that appears in the identifier, a parameter name, the return type or the body. A plural or a verb form counts as the same word. A comparison in the body counts as its word: `> 0` spells positive, `=== 1` spells exactly one, and returning `undefined` spells none. Strike also the verbs a summary opens with: checks, whether, returns, lists, says, gives, declares, holds, names, reads, writes, takes, yields, produces, provides, gets, sets. A summary with no word left is `none`. Probes for this question live in `~/.kk-flavor/workers/comment-writer/tests/summary-verdicts.md`.
2. **The summary.** Write one where the name and signature leave a return case, a unit, an ordering a caller depends on, a side effect or a precondition unsaid. Fill the summary pattern the rule gives: `<Verb> <what>` and, where there is a case, `, or <value> when <case>`. Use the identifier's names and the domain's words. Say what the symbol decides or returns. A summary on a member of an interface says what every implementation does, and behaviour one implementation alone has goes on that implementation. Do not say why.
3. **The note.** This question runs for every site, including one whose summary is `none`. Open the facts file. Read each sentence in it as a claim, and check the claim against the code. Drop a claim the code in this file contradicts, and return it as `stale`. A claim about anything you cannot read here is never `stale`: keep it as the note and return `unverified: <site>: <claim>` for code review. An archived claim is its author's, and you keep it ungraded. A claim you cannot check is kept, and it is never a reason to answer `does not fit`. A run marked a claim about a vendor's asset `stale` on 2026-09-21, and a reviewer fetched the asset and found the claim true. One claim is never `stale`. It is the claim that this code and something outside it agree, in the words copies, mirrors, stays in step with, must match or kept in sync with. Where the two have parted, the parting is a correctness finding and the claim is what made it findable. Keep the claim as the note, in wording that states the obligation. A note saying this table `must match` that one states what is owed. A note saying it copies that one states only what is. The reader who changes one side is owed the first. Return `invariant diverged: <site>: <claim>` for code review. Where a comparison against the other side could be written, return `carried by <test to write>` too. Code carries such a claim only where the code makes the parting impossible: a type, a derivation, a single place both sides are read from. A check running over the outside thing reports a parting and leaves it standing, and a check able to fail is not a carrier. A writer routing the claim to one deletes the sentence that made the parting findable.

   Then two drops you take as steps. Take the claim's content words, with the opening verbs struck as in question 1. The claim is shown by the body where every one of those words appears in the site's identifier, its parameters, its return type or its body lines. Drop it and return `shown by the body: <claim>`.

   A claim about a row of data belongs to the row. Where the declaration under the block holds values alone, write the note there, with `bears_on` the declared name and `does: none`. An unread field or constant carries the fact to no reader, so the fact stays a comment at its row. Use it in place of `does not fit` on a data site.

   Then read the change set's tests. A test carries the claim where its name or its describe path holds the claim's subject and its verb. It carries the claim too where that name holds every content noun of it. Drop it and return `carried by <test name>: <claim>`.

   Neither step reaches a claim that paraphrases the code in words the code does not spell. That judgement stays yours, and `~/.kk-flavor/workers/comment-writer/tests/` holds the cases it is measured on. Drop a claim that justifies a decision a reader would take for granted, which `~/.kk-flavor/standards/code-style.md` → **Comments** already names. A reader meeting two places a value can sit reads both, and the claim defends what they were going to do. That holds where the claim names a format that allows both places: the format is the fact, and reading every place it allows is no act a note explains. Drop a claim a test, a lint rule, a rename or an extraction would carry, and return it as `carried by <what>` for the refactor lane. Before keeping a claim, `grep` the change set's tests for the fact's nouns, and answer `carried by <test>` where a test names it. A language mechanism, an ordering and a branch routing are `carried by` before they are a note. A fact about the world outside this code is never `carried by <test>`. That fate is for a claim about what this code itself does: what it returns, which branch runs, an ordering it imposes. A test guards such a claim against regression, and the reader sees it in the code.

   Keep a claim only where it states a fact about the world outside this code: a device, a platform, a format, a vendor's asset, a specification, a library's behaviour. A claim about this code's own behaviour is shown by the code or carried by a test. Write the claim as the fact it is, and fill the note's record before you write a word of prose. The record is three slots. `fact:` is the claim from outside this code. `bears_on:` is one identifier declared at this site or inside its body, and never a name from anywhere else. `does:` is what that identifier does that the fact explains: a literal verb and a value, a return or a branch you read off the body. The identifier does not act on the fact. The fact is why the identifier does what it does, and `does:` is that act. Where the fact is why this code consults something at all, the act is the consulting: the call it makes, the value it reads, the branch it keeps. The declaration the block sits on is always one of the identifiers you may name. On a one-line declaration — a constant, a field, an enum member — `bears_on` is the declared name and `does:` may be `none`, because the value beneath is the tie. That holds for a fact about the declaration's own value. A fact explaining an act goes to the declaration performing the act, even where the strip offered it at the constant. Moving a fact chooses between declarations here, and it is never a reason to answer `none`. A fact that fits no declaration in this file goes back as `does not fit`. A fact explaining an act a declaration in this file performs is never `does not fit`. The act can be one that keeps a state from arising, such as removing a container whole so that no removal of its members leaves it empty. It can be what the code leaves out, such as a split that keeps the part before a separator. Where the act is the declaration consulting something at all, the declaration under the site performs it, and the fact stays there. A fact about the history of what the declaration consults is that case. A tie that reads diffuse is never a reason to route the fact to `does not fit`. A reader editing a namespace constant makes no mistake the fact prevents, and a reader editing the lookup that matches by that namespace does. `does:` is required under a function, a method, a branch or a call, and you fill it from the statements beneath. Under an enum, an interface, a type, a constant or a field, `does:` is `none`. The fact names the declaration by its identifier, and a member where the fact is about one. Read each slot against the body on its own. A `fact:` the body shows makes the record `none`, the same drop as before. `does:` is read off the body by definition, and it is never a reason to drop a claim. Where the only claim left after the drops is about this code's own behaviour, there is no fact, and the site is `note: none`. A name from outside the site's own declaration carries what it is in plain words at its first mention: `preferredSettlements, the ledger's list of allowed schemes`. A caller `grep` finds in the repository is one of the code's own elements. A caller no `grep` finds is a hypothetical actor. Leave that actor out, keep the claim, and return `unverified: <site>: <claim>` for review. The fact may be a caller's behaviour. What the callers do with the result is a fact from outside this function. Where it is why this function does what it does, it is the `fact:`, `bears_on` is this function, and `does:` is this function's act. `bears_on` is never a caller. The test is which identifier performs the verb in `does:`. Where that is a caller, the fact belongs at the caller. Write it at the caller's own declaration where the caller is in this file. Where the caller is in another file of the change set, return `belongs at <file>:<identifier>: <fact>` and write no note here. Never give your reason as a pointer at another block. `for the reason {@link X} gives` and `see <X> for why` leave the reason at neither block, and the comment profile reports both shapes. The reason is written where it is read. Leave out a choice the code never made, and leave out what other code would do. A sentence after the first names its own subject and object by the code's noun. `that`, `this`, `it` and `such a` reaching back make a reader hold both sentences to read the second. Two sentences is the ceiling.

   Write like the blocks under **How these read**, which are notes a reviewer left standing in a set of reviewed code. Return a fact that needs more as `does not fit: <site>: <fact>`, and write no note for it. That line goes to the human who asked for the change. `for the PR body` takes a fact about the change itself alone: what it changed and why. A fact about the world never goes into the body, where the change is described. Answer `note: written` or `note: none`.

   The block is the summary and the note together. The site is `none` only where both parts are `none`.

Copy no sentence from the facts file into the block. Write each kept fact again from the code and the claim. Copy a line carrying only a doc tag (`@param`, `@returns`, `@throws`, `@example`) unchanged where the file's other blocks carry them, and write no new one.

## How these read

Ten notes a reviewer left standing in a set of reviewed code, in this repository's own domain, and an
eleventh written to the same record. Each
one on a constant, a field or an enum member states the fact and stops, and the value beneath it is
the tie. Each one on a function, a branch or a call carries its tie in words.

```ts
// Publishes the outcome of a posting whose settlement failed. The ledger's own claims decide which token it gets.
import { LedgerClaim } from './LedgerClaims';

// `LedgerBook.SETTLED` is missing on some ledger builds here, so the value is spelled out.
const SETTLED = 2;

// A ledger ignoring `currency` answers about the rate and leaves the period unasked, so `readRateFields` reads both fields.
async function readRateFields(

// `formatFault`, the formatter for a fault's message, appends `(code [Unreadable])` to readable messages too.
// `isPlaceholderFaultText` matches only a message that opens with the placeholder, and not one the formatter appended it to.
export function isPlaceholderFaultText(formatted: string): boolean {

// The posting format, which carries the precision: `AMT2` is two decimals and `AMT4` is four.
format: string | null;

// XML Name rules cap no element name, and whoever served the export chose this one.
const PUBLISHED_ELEMENT_NAME_CHARACTER_LIMIT = 40;

// `0` is what a fetch reports for a request that never reached a server, and this branch keeps only a status from 100 up, so `0` falls out.
if (typeof status === 'number' && status >= 100 && status < 600) {

// One ledger build lacks `entry-precision`, the field defined against an entry, and falls through to the book-wide one.
const PRECISION_FIELDS = ['entry-precision', 'book-precision'];

// The posting service answered 5xx: a server erroring on a request it accepted cannot be the ledger's doing.
UnmeasuredPostingServerFailed = 'UNMEASURED_POSTING_SERVER_FAILED',

// A ledger export predating these fields ignores them and answers about the entry type only, and `reads` holds what such an export returns.
stubReadingLedger({ supported: true, reads });

// Every caller removes postings from the ledger while it walks the result.
// `snapshotPostings` copies the live list into a plain array.
export function snapshotPostings(list: LivePostingList): Posting[] {
```

The eleventh reads its fact from the callers you found. A caller's act that explains this
function's act is the `fact:`, and the copy is the act the callers' removal explains. `belongs at` is
for a caller's act the fact explains.

The block is the record written as two plain sentences: the fact, then the tie. The tie's subject is
the `bears_on` identifier, and it takes the spelling the code gives it. No connective is required and `so` is not
the default. The tie says what the code beneath does, and you read it off the body. A consequence in the world
is something you would be inventing. A one-line declaration whose `does:` is
`none` gets the fact alone, and the value under it is the tie a reader reads.

Four blocks a reviewer read on 2026-09-22 each stated a fact and stopped, on a function, a branch, a
vendor-prefixed branch and a function again. The reviewer's question on each was a version of "and
what?". Every register check passed all four, because their prose was sound.

Read what each one does. The first is a file header. It says what every declaration in the file
is for, and it leaves the file's own name unsaid. The third, the fourth, the seventh and the tenth
stand on a declaration with a body, and each of those names the identifier its tie is about. The
third joins its two on `so` and the fourth writes two sentences, so the connective is a choice. The sixth explains a number no consequence explains: a specification permits anything
and a person chose forty. The second warns that a build here lacks the name. The ninth says what an
answer means. None of them invents something to put after a `so`.

## Words

- Use the identifier's name or the domain's own word. Where the facts file coined a word for a thing the code names, use the code's name. Where the code lacks a name for the thing, return `rename: <the thing>` and leave it out of your sentence.
- A hyphenated pair inside an identifier is the code's own coined compound. It is a rename finding, and your prose takes the plain phrase instead. Return `rename: <the identifier>`.
- A boolean is a value. Write the call and the value it returns. Leave out a yes, an answer and a question as nouns. Leave out a device that says something.
- Write absent, unlisted or undefined where the facts file said a thing lacks a name.
- Put one idea in a sentence, and keep it under 20 words.
- Leave out semicolons, bold, bullets, headings, a contrast spine, and `never` as emphasis.
- Name the actor of every verb.
- Name a language mechanism by the language's own word: `this` binding, closure, promise, iterator, generator. A metaphor for one is a word the reader has to translate.

## The audit, before you return a block

The audit is a step you take before the block leaves your hands. A block whose audit carries a rewrite is rewritten and audited again, and it is never returned with the line still on it.

List every noun phrase in the block and classify each one:

- `identifier` where the word is in the site's `identifiers.txt`. Look it up with `grep -ixF '<the phrase>' identifiers.txt`, which matches the phrase whole and in any case. The strip writes that file beside the facts file. It holds every hyphenated name the repository spells in a path or on a line of code. It holds each name as the code spells it and in lower case, so a lookup matches either.
- `plain` where every word of it is an ordinary English word and it carries no hyphen.

- `path` where the phrase sits in backticks and holds a `/` or a file extension. It names a file in another repository, and a maintainer needs it to find that file. An identifier the same sentence places in such a path is `path` too: `` `LedgerMap` in `src/core/ledger/Ledgers.ts` ``.

These three are the only classes. A domain phrase in ordinary English words, such as `the posting interface`, is `plain`. A name the tree does not spell, such as another system's own identifier, stays out of the sentence, or comes back as `rename:` so the code spells it. Anything else is `neither` and a rewrite. Classify by presence in those lists. A coined compound reads as ordinary English to the writer who chose it, so judgement passes over it.

Then list every verb and classify each as `literal` or `figure`. These four are a `figure` wherever they appear, because a set of reviewed code was counted for them: cover, settle, sit in, load-bearing. Read them off that list, the way the noun phrases are read off theirs, and leave the judgement out. Any other verb is `literal` where it is an action its named subject performs: uses, returns, removes, reads, copies, throws. A `figure` is a rewrite. A verb the sentence borrows from an earlier clause is elided, and it is written again: `as soon as the document removes it`, never `as soon as the document does`.

Then read each sentence you wrote back against the file. A word of exclusivity — only, no other, every, always — claims something of every site in this file that handles the same identifier. `only` is the word, and `alone` after a noun is a rewrite the comment profile reports. Read those sites. Drop the word where one of them contradicts it, and keep the sentence. A word this file contradicts is an edit to the sentence, and it is never a reason to answer `note: none`. Where dropping the word leaves the claim saying something you cannot check here, keep the sentence and return `unverified: <site>: <claim>`. The stale step reads the facts file against the code, and it never reaches a sentence you wrote fresh.

Then count the note's sentences, and leave the summary out of that count. Two is the ceiling, and a fact may share one sentence with its consequence. Two facts explaining two acts in one function are two notes, each above the statement performing its act, a branch included, with one record each. Two acts can sit in one statement, such as a branch whose condition and call each have a reason. One note above it then carries both, each fact in one sentence with its act: `<fact>, so <identifier> <act>`. A fact that explains no act here stays out of both. Run 10 kept one of two such facts and sent the other to the human, and the branch condition the second one explains lost its reason. At three the facts need more room than a note, so the note is `none` and they go back as `does not fit`. The block's own bound is four prose lines.

Return the audit lines beside the block, one per line, as `term: <phrase> — identifier|plain|path` and `verb: <word> — literal|figure`.

## Check each block before you write it

Run the edit lane's voice check over the block on stdin: `voice-check.sh --profile=comment --source --record -`, the script under `~/.kk-flavor/skills/kk-edit/scripts/`. Pipe the note's three slots, then a line reading `---`, then the block with the declaration it will sit on and that declaration's body under it. The check reads the record against both: `bears_on` names a declaration under the block and the block spells it, and `does` shares a word with the body. The prose profile leaves out bare-identifier, long-block and coined-identifier, and a run on 2026-09-21 put a block past it twice that the comment profile refused both times. Rewrite the block for every finding it prints. Write `none` for a block still carrying a finding after two rewrites, and return its facts as `does not fit`. Show the two rewrites: `attempt 1: <the part> - <the finding>` and `attempt 2: <the part> - <the finding>`, one to a line, above the part's `none`. A part you set out to write and answered `none` for, with no two attempts under it, is a part you skipped, and your caller returns it to you. The gate applies to each part on its own.

A `long-line` finding is a line to wrap at the width it names. Wrap it and run the check again.

Then read the block once as the engineer opening this file for the first time, and restate it in one plain sentence. Rewrite a block you cannot restate. Write `none` for a block you still cannot restate after the second rewrite, and return its facts as `does not fit`.

A sentence a reviewer suggested reaches you as a fact at its site, and you answer it the way you answer any site. A sentence under `# code review:` in the facts file says what the code does, as a reviewer read it. Where it contradicts a clause of the old block, drop that clause, and write the note from the review's sentence where you can tie it to the code.

The site the strip offers is where a block stood. The block goes where its fact belongs. Write it above the declaration the fact is about, anywhere in this file, and answer with that line. A claim about a function is written at that function and names it. A file header keeps what spans the file, and a header saying what the file is for is `none` where the file's name says it. A header you write keeps one blank line between it and the code under it. A claim that fits no declaration in this file goes back as `does not fit`. Where the site's declaration declares members, a member's own line is one of the declarations you may choose.

Write the block in the comment syntax the file's other blocks use, and change no other line.

## Verdict

`Block N/M <file>:<line> | OK` followed by the block you wrote, verbatim, in a fenced code block, or `Block N/M <file>:<line> | none`. A site is `none` only where both parts are.

Under each verdict line, its two part lines: `summary: needed` or `summary: none`, and `note: written` or `note: none`. A lane reading your return counts the parts, so a site that lost its note to the summary's verdict can be seen.

After the verdict lines, write one line per routed claim, in one of these shapes:

- `stale: <site>: <claim>` for a claim the code here contradicts.
- `carried by <what>: <site>: <claim>` for a claim something else in the tree holds.
- `rename: <site>: <the thing>` for a name the refactor lane changes.
- `belongs at <file>:<identifier>: <fact>` for a fact a caller in another file acts on.
- `does not fit: <site>: <fact>` for a fact about the world that no note here holds.
- `for the PR body: <site>: <fact>` for a fact about the change itself.
- `for code review: <path>:<line> <sentence>` for a defect you noticed in the code, such as a value that can be null. It is a code-review finding, and it never becomes a comment. It reports the code, and a check you could not run is a fact about your run, which stays out of it.
- `none: <site>: <the reason>: <claim>` for a claim you dropped for any reason no other line names, such as a claim a reader would take for granted.

A fact has exactly four fates: written in a block, `none` with its reason on a routed line, `belongs at`, or `does not fit`. A line of your own wording outside these shapes reaches no lane.

## Do not

- Write a comment because the old one existed. The default is `none`.
- End a site at question 1. That question decides the summary, and question 3 runs whatever it answered.
- Read the facts file before question 3.
- Run `git diff`, `git show` or `git log` over the change's range before question 3, where the output holds a comment line. The strip clears the tree and leaves the history, so a block you read there is the block you were sent to replace. Output whose lines are all code shows you no block, and it leaves the run clean.
- Change a line of code. A code change you want is a `rename:` or `carried by` line for the refactor lane.
- Rewrite a block the strip left standing. The strip removes what you are there to replace, so a
  block still in the file is one a check reads, and your words would take its reader's input away.
- Put a blank line inside the block that opens a file. A header scan ends at the first line of
  code, so the lines under the blank lose their reader, and the loss is silent.
