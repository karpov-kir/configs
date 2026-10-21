package ecoreport_test

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// The file a record name writes. The project-*/local-* prefix names which copy, never the filename —
// the two are the same basename under different roots, which is the whole of what makes them mergeable.
func recordFile(f *fixture, name string) string {
	return f.scratch() + "/for-agents/" + strings.TrimPrefix(name, "project-") + ".md"
}

// A local record's file, inside the folder of the ship that owns it.
func localRecordFile(f *fixture, slug, name string) string {
	return f.shipDir(slug) + "/for-agents/" + strings.TrimPrefix(name, "local-") + ".md"
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func TestFirstWriteCreatesTheRecordWithItsHeader(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	// Every record this tool owns, with the cap its header has to state and the audience it has to
	// name. Written out here rather than read from recordKinds: a test that takes these from the code
	// it checks agrees with any values the code happens to hold, including a header stating one cap
	// while noteBound enforces another.
	headers := map[string]struct{ bound, audience string }{
		"project-decisions": {"100", "never presented to a human"},
		"project-playbook":  {"100", "never presented to a human"},
		"project-language":  {"100", "an agent keeps it current"},
	}
	for name, want := range headers {
		f.runReport("record", "append", name, "the first thing anyone wrote here")
		content := f.read(recordFile(f, name))
		// `records.md` → **The cap evicts by what the record can afford to lose** requires the record to
		// state its own bound, so the header a first append writes has to carry it.
		f.record(name+".md is created with its own header, stating its bound",
			f.status == 0 && strings.HasPrefix(content, "# ") && strings.Contains(content, "Full at "+want.bound+" entries"),
			content)
		// The header and the refusal are one file talking about one cap, so they cannot say different
		// things about what happens past it. `records.md` → **Reaching the cap** leaves an entry that
		// frees no slot unrecorded, which a header promising the newcomer an incumbent's place denies.
		f.record("and says what the cap does without promising the newcomer a place",
			strings.Contains(content, "an append refuses, and a new entry lands only if") &&
				!strings.Contains(content, "displaces"), content)
		// The four sit side by side and read alike, so the file itself is what tells a human opening it
		// whether the wording is theirs to own or an agent's to keep.
		f.record("and names who it is written for", strings.Contains(content, want.audience), content)
		f.record("and holds the entry, counted once and dated today",
			strings.Contains(content, "1x | "+today()+" | the first thing anyone wrote here\n"), content)
	}
}

// `records.md` → **Reaching the cap** makes revise the way an entry is sharpened, widened, or folded
// into the one it overlaps. It is the only op that changes an entry's text, so what it must not do is
// hand back a fresh count: rewording would otherwise be the way to lift an entry out of eviction's
// reach without ever confirming it.
func TestReviseReplacesTheTextAndKeepsTheCount(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "project-playbook", "p99 under 200ms on search")
	f.runReport("record", "append", "project-playbook", "p99 under 200ms on autocomplete")
	path := recordFile(f, "project-playbook")
	f.replaceLine(path, "1x | "+today()+" | p99 under 200ms on search", "4x | 2020-01-01 | p99 under 200ms on search")

	f.runReport("record", "revise", "project-playbook", "on search", "p99 under 200ms on every read endpoint")
	content := f.read(path)
	f.record("revise replaces the text, keeps the count and dates it today",
		f.status == 0 && strings.Contains(content, "4x | "+today()+" | p99 under 200ms on every read endpoint\n"), content)
	f.record("and the old wording is gone rather than left beside it",
		!strings.Contains(content, "| p99 under 200ms on search\n") && strings.Count(content, "x | ") == 2, content)
	// The replaced text has to reach stdout. `replacement` is the whole entry, so a caller that reads
	// "revise" as an edit-in-place loses every word it did not retype, out of an append-only record.
	// A ship found this by copying the log, restoring and diffing the whole file; nothing the tool
	// printed would have told it. The echo is what makes the loss visible at the moment it happens.
	f.record("and it echoes the entry it replaced, so the loss is not silent",
		strings.Contains(f.out, "it replaced: ") && strings.Contains(f.out, "p99 under 200ms on search"), f.evidence())

	// Folding two entries into one is revise-then-evict, so the pair has to end as a single entry.
	f.runReport("record", "evict", "project-playbook", "on autocomplete")
	f.record("evicting the folded-in entry leaves one covering both",
		f.status == 0 && strings.Count(f.read(path), "x | ") == 1, f.read(path))

	// Revising one entry into the exact text of another is the half-done fold: the record would hold the
	// same line twice, and every later bump or evict naming it would refuse as ambiguous.
	f.runReport("record", "append", "project-playbook", "WCAG 2.1 AA on every page")
	before := f.read(path)
	f.runReport("record", "revise", "project-playbook", "WCAG", "p99 under 200ms on every read endpoint")
	f.assertRefused("revising an entry into the text of another is refused")
	f.record("and the record is unchanged", f.read(path) == before, f.read(path))

	// Re-stating an entry's own current text is not a collision with itself.
	f.runReport("record", "revise", "project-playbook", "WCAG", "WCAG 2.1 AA on every page")
	f.record("revising an entry to its own text is allowed", f.status == 0, f.evidence())

	// Sharpening is the move that shortens a record, and the rewrite goes over the old bytes before it
	// trims them. A missed trim would leave the tail of the longer wording standing under the new one.
	f.runReport("record", "revise", "project-playbook", "WCAG", "WCAG AA")
	after := f.read(path)
	f.record("a revision that shortens an entry leaves no tail of the old wording",
		f.status == 0 && strings.HasSuffix(after, "| WCAG AA\n") && strings.Count(after, "x | ") == 2, after)

	// The shape that cost a ship its entry: the caller passes the fragment it wanted substituted in,
	// and revise takes it as the whole entry. Containment recognises exactly that and needs no
	// threshold — a real rewording is not a substring of what it replaces. This is the only write in
	// an append-only record that can remove text, so it is the one that must not do it in silence.
	beforeGuard := f.read(path)
	f.runReport("record", "revise", "project-playbook", "every read endpoint", "p99 under 200ms")
	f.record("a replacement contained in the entry is refused, and the record is untouched",
		f.status != 0 && f.read(path) == beforeGuard, f.evidence())
	f.record("and the refusal says revise takes the whole entry, and offers evict",
		strings.Contains(f.out, "WHOLE new entry") && strings.Contains(f.out, "record evict project-playbook"), f.evidence())
}

func TestBumpRaisesTheCountAndRedatesWithoutAddingALine(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "project-decisions", "settled once")
	f.runReport("record", "append", "project-decisions", "settled twice")
	path := recordFile(f, "project-decisions")
	f.replaceLine(path, "1x | "+today()+" | settled once", "2x | 2020-01-01 | settled once")

	f.runReport("record", "bump", "project-decisions", "settled once")
	content := f.read(path)
	f.record("bump raises the count and dates the entry today",
		f.status == 0 && strings.Contains(content, "3x | "+today()+" | settled once\n"), content)
	f.record("and adds no second entry for it",
		strings.Count(content, "| settled once") == 1, content)
	f.record("and leaves its neighbour alone",
		strings.Contains(content, "1x | "+today()+" | settled twice\n"), content)

	// An append of text already there is the same event, and `records.md` says it bumps rather than
	// adding a line. Only the identical text is caught — a restatement in other words is a judgment.
	f.runReport("record", "append", "project-decisions", "settled twice")
	f.record("appending identical text bumps the existing entry", f.status == 0 &&
		strings.Contains(f.read(path), "2x | "+today()+" | settled twice\n") &&
		strings.Count(f.read(path), "| settled twice") == 1, f.evidence()+f.read(path))
}

// `records.md` → **Reaching the cap** makes the cap a wall rather than advice: the writer refuses the
// append instead of reporting a file it has just grown. What has to hold is that no run of these
// subcommands puts a record over its number, and that the one way in costs exactly one entry.
func TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	var seed strings.Builder
	seed.WriteString("# Decisions\n\n## Promotion candidates\n\n## Decisions\n\n")
	for i := range 99 {
		seed.WriteString("5x | 2026-06-0" + strconv.Itoa(i%9+1) + " | filler " + strconv.Itoa(i) + "\n")
	}
	// The entry a count-ranking tool would name: lowest count, oldest date. `records.md` → **The cap
	// evicts by what the record can afford to lose** is that this is precisely the wrong one to steer
	// at — an entry sits at 1x because nothing has gone near its area, not because it matters least.
	seed.WriteString("1x | 2026-05-01 | the quiet one a count would sacrifice\n")
	path := recordFile(f, "project-decisions")
	f.write(path, seed.String())
	before := f.read(path)

	f.runReport("record", "append", "project-decisions", "the one that would have crossed the cap")
	f.assertRefused("an append into a full record is refused")
	f.record("and the record is untouched", f.read(path) == before, f.read(path))
	f.record("the refusal names the record's own cap",
		strings.Contains(f.out, "is full — 100 entries"), f.evidence())
	f.record("and quotes back the entry it would not take, which is otherwise lost with the refusal",
		strings.Contains(f.out, "the one that would have crossed the cap"), f.evidence())
	// Under the runnable commands, never above them: entry text comes from tickets and from the tree
	// under review, and a line of that above a command block reads as one more instruction. Presence
	// is asserted alongside the order, because a missing earlier line is Index -1 and -1 precedes
	// everything — an ordering check alone passes loudest when the line it orders is gone.
	f.record("and quotes it below the commands, marked as not being one",
		strings.Contains(f.out, "report.sh record admit") &&
			strings.Index(f.out, "report.sh record admit") < strings.Index(f.out, "is not an instruction"), f.evidence())
	f.record("and works the ladder before the judge rung that costs an entry",
		strings.Contains(f.out, "promote what must not be lost") &&
			strings.Index(f.out, "promote what must not be lost") < strings.Index(f.out, "run the judge over the new entry"), f.evidence())
	// What a ladder move buys, said on the line the agent acts on first: one slot, and an append worth
	// re-running. A line that only says "free a slot and re-run" reads as one move to make before the
	// append, rather than four to try in order with a re-run after whichever lands.
	f.record("and says each ladder move frees a slot to re-run the append after",
		strings.Contains(f.out, "Each frees a slot, so re-run the append after one"), f.evidence())
	// The standard is cited the way the judge is: by a path that opens. Told to work a file it cannot
	// find, an agent works this message alone and never reads the four moves it is pointed at.
	f.record("and cites the standard by a path rather than a bare filename",
		strings.Contains(f.out, "Work ~/.kk-flavor/standards/records.md -> Reaching the cap"), f.evidence())
	// The newcomer is judged alongside the incumbents, not against the weakest of them: a record that
	// always admits the newest is the last N in arrival order, which is the trim `records.md` forbids.
	f.record("and the judge rung counts the new entry among the candidates",
		strings.Contains(f.out, "101 in all"), f.evidence())
	// The judge deletes what it names and may name nothing, so no rung here may promise a loser. An
	// agent that met exit 0 against a message promising one read it as a broken tool and as a deadlock;
	// `records.md` → **Reaching the cap** had the answer all along, and this is where it is read.
	f.record("and the rung says the judge may name nothing",
		strings.Contains(f.out, "naming nothing (exit 0, output unchanged) is one of its answers"), f.evidence())
	f.record("and gives that outcome, and a named newcomer, the standard's own answer",
		strings.Contains(f.out, "Where it names nothing, or names the new entry, the cap holds: the entry is not recorded"),
		f.evidence())
	f.record("and promises no contest that always resolves",
		!strings.Contains(f.out, "loses") && !strings.Contains(f.out, "tie"), f.evidence())
	// Exit 2 is `reader-judge.sh` saying it did NOT run, and it prints the same empty stdout as exit 0.
	// An agent holding only the exit-0 reading takes that silence for "the judge named nothing" and
	// calls the cap held — a verdict nothing reached. Measured in the field on a 109-entry record.
	f.record("and reads exit 2 as the judge not having run, leaving the append refused",
		strings.Contains(f.out, "Exit 2 is not an answer: the judge did NOT run") &&
			strings.Contains(f.out, "the append stays"), f.evidence())
	// No entry is singled out, so nothing steers the agent at the lowest-count line before it has
	// judged the file.
	f.record("and no entry is named as the one to drop",
		!strings.Contains(f.out, "the quiet one a count would sacrifice") && !strings.Contains(f.out, "filler"),
		f.evidence())
	// decisions.md is the next agent's own, so `records.md` → **Reaching the cap** has it take every
	// move unasked and the refusal hands the commands straight over.
	f.record("and a record no human owns is not told to ask first",
		!strings.Contains(f.out, "propose each move"), f.evidence())

	// A restatement wants no slot, so a full record answers it with bump rather than sending the agent
	// to the judge, which would put a live entry out for a copy of itself.
	f.runReport("record", "append", "project-decisions", "filler 7")
	f.record("a restatement into a full record bumps without consuming a slot", f.status == 0 && strings.Contains(f.read(path), "6x | "+today()+" | filler 7\n"), f.evidence())
	f.record("and it is not sent to the judge", !strings.Contains(f.out, "is full"), f.evidence())

	// The judge's verdict, applied: one out for one in, in a single write.
	f.runReport("record", "admit", "project-decisions", "filler 42", "the one that won its place")
	content := f.read(path)
	f.record("admit lands the winner, counted once and dated today",
		f.status == 0 && strings.Contains(content, "1x | "+today()+" | the one that won its place\n"), f.evidence()+"\n"+content)
	f.record("and the entry it beat is gone", !strings.Contains(content, "| filler 42\n"), content)
	f.record("and the record is no bigger than its cap", strings.Count(content, "x | ") == 100, content)
	// A revise inherits the count; an admission is ground no run has reached, so a borrowed 5x here
	// would be read as evidence by every later prune.
	f.record("and the winner does not inherit the reach of the entry it displaced",
		!strings.Contains(content, "5x | "+today()+" | the one that won its place"), content)
	// A swap reported by its winner alone is a record silently shortened.
	f.record("and both halves of the swap are reported",
		strings.Contains(f.out, "admitted to decisions.md") && strings.Contains(f.out, "in place of: 5x | ") &&
			strings.Contains(f.out, "| filler 42"), f.evidence())

	// The wall is still up behind the swap: it freed no room.
	f.runReport("record", "append", "project-decisions", "a second one wanting in")
	f.assertRefused("the record is still full after the swap")
	f.record("so no run of these subcommands grows it past its cap",
		strings.Count(f.read(path), "x | ") == 100, f.read(path))

	// A revise may reword an entry into its own text and lose nothing, since it keeps the count. admit
	// lands at 1x, so the same move there is a silent reset of everything the entry had earned — and it
	// reads as a swap that displaced something, which is what makes it worth a refusal of its own.
	f.replaceLine(path, "1x | "+today()+" | the one that won its place", "9x | 2026-01-02 | the one that won its place")
	earned := f.read(path)
	f.runReport("record", "admit", "project-decisions", "the one that won its place", "the one that won its place")
	f.assertRefused("admitting an entry over its own text is refused")
	f.record("and the count it had earned is untouched", f.read(path) == earned, f.read(path))
}

// `oneMatchingEntry` resolves by substring, and `admit` made that a destructive selector. An entry
// whose text contains another entry's whole text shadows the shorter one: every string naming the
// short entry names the long one too, so it can never again be bumped, revised, evicted, or named as
// the entry the judge named — and entry text is seeded from tickets and from the tree under review, so
// the containing entry is writable rather than a freak of wording.
func TestAnEntryIsNotShadowedByALongerOneQuotingItWhole(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "project-decisions", "the lock goes on the record's own descriptor")
	f.runReport("record", "append", "project-decisions", "the lock goes on the record's own descriptor — superseded by TICKET-9")
	path := recordFile(f, "project-decisions")

	// Every substring of the short entry also sits inside the long one, so nothing but the exact text
	// can single it out.
	f.runReport("record", "bump", "project-decisions", "the lock goes on the record's own descriptor")
	f.record("an exact hit resolves rather than colliding with the entry quoting it whole",
		f.status == 0 && strings.Contains(f.read(path), "2x | "+today()+" | the lock goes on the record's own descriptor\n"),
		f.evidence()+"\n"+f.read(path))
	f.record("and the longer entry is left alone",
		strings.Contains(f.read(path), "1x | "+today()+" | the lock goes on the record's own descriptor — superseded by TICKET-9\n"),
		f.read(path))

	// The exact rule reaches only the exact text. A partial string still matching two entries is the
	// ambiguity this function exists to refuse, and it must not now resolve to one of them.
	f.runReport("record", "bump", "project-decisions", "the lock goes on")
	f.assertRefused("a partial string matching two entries is still refused as ambiguous")
	f.assertReports("2 entries", "as ambiguous rather than resolved to either")

	// And the exact rule fires only where it resolves. No op here writes a duplicate, but a record is a
	// plain file a template seeds and a human edits, so two entries holding one text is a state that
	// reaches this code — and picking either would be the guess the whole function exists to refuse.
	f.appendTo(path, "1x | 2026-01-01 | a line written twice by hand\n1x | 2026-01-02 | a line written twice by hand\n")
	before := f.read(path)
	f.runReport("record", "evict", "project-decisions", "a line written twice by hand")
	f.assertRefused("an exact text held by two entries is refused rather than resolved to one")
	f.record("and neither copy was removed", f.read(path) == before, f.read(path))
}

func TestAMutationNeverLosesALineItDidNotTarget(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	// The tail one of these records really carries: a block that is not in the entry format at all.
	// It is not an entry, so it is never counted, matched or evicted — and it must survive verbatim.
	block := "**A test timeout reads exactly like a hang.** `go test` without `-timeout` gives up at ten\nminutes and prints a goroutine dump.\n"
	path := recordFile(f, "project-playbook")
	f.write(path, "# Playbook\n\n1x | 2026-01-01 | keep me\n2x | 2026-02-02 | bump me\n1x | 2026-03-03 | evict me\n\n"+block)

	f.runReport("record", "bump", "project-playbook", "bump me")
	f.runReport("record", "evict", "project-playbook", "evict me")
	content := f.read(path)
	f.record("the untargeted entry survives both mutations",
		strings.Contains(content, "1x | 2026-01-01 | keep me\n"), content)
	f.record("and so does a block that is not in the entry format",
		strings.Contains(content, block), content)
	f.record("and the targeted ones did what was asked",
		strings.Contains(content, "3x | "+today()+" | bump me\n") && !strings.Contains(content, "evict me"), content)

	// A record whose last line has no newline of its own: the append must not land as that line's tail.
	tail := newRepo(t)
	tail.mkdirAll(tail.scratch())
	tailPath := recordFile(tail, "project-decisions")
	tail.write(tailPath, "# Decisions\n\n## Promotion candidates\n\n## Decisions\n\n1x | 2026-01-01 | no newline after me")
	tail.runReport("record", "append", "project-decisions", "an entry of my own")
	tail.record("an append to a record with no trailing newline is still its own entry",
		strings.Contains(tail.read(tailPath), "no newline after me\n1x | "+today()+" | an entry of my own\n"),
		tail.read(tailPath))
}

func TestAnEntryRoundTripsWhateverTextItCarries(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	// An entry is external text: it carries whatever the agent writing it wrote, including the
	// separator this format is built from — the parser splits on the first two, so the rest is text.
	awkward := "`gate.sh --full` | 90 % of runs — “quoted”, ünïcode, 🍦, a | pipe"
	f.runReport("record", "append", "project-decisions", awkward)
	path := recordFile(f, "project-decisions")
	f.record("the entry is stored exactly as written",
		strings.Contains(f.read(path), "1x | "+today()+" | "+awkward+"\n"), f.read(path))

	f.runReport("record", "bump", "project-decisions", "🍦")
	f.record("and is still findable by any part of itself",
		f.status == 0 && strings.Contains(f.read(path), "2x | "+today()+" | "+awkward+"\n"), f.evidence()+"\n"+f.read(path))
}

func TestAnEvictLeavesNoTailOfWhatItRemoved(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	path := recordFile(f, "project-decisions")
	// The evicted entry is far longer than what follows it, so a rewrite that writes the new content
	// and does not then trim the file leaves the tail of the old content standing — and that tail is
	// whole, well-formed entries the parser goes on to count, match and bump. Asserted as the file's
	// exact bytes rather than by counting entries, because a duplicated entry satisfies every count.
	head := "# Decisions\n\n## Promotion candidates\n\n## Decisions\n\n"
	keep := "1x | 2026-02-02 | one short entry\n2x | 2026-03-03 | another short entry\n"
	f.write(path, head+"1x | 2026-01-01 | "+strings.Repeat("a long entry that is about to go, ", 12)+"end\n"+keep)

	f.runReport("record", "evict", "project-decisions", "a long entry that is about to go")
	f.record("the record afterwards is exactly what was left, with no tail of the old content",
		f.status == 0 && f.read(path) == head+keep, f.evidence()+"\n"+strconv.Quote(f.read(path)))
}
