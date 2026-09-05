package ecoreport_test

// The four shared records — one file per clone, written by every agent in every worktree. The subject
// is the write nobody can see fail: a lost entry leaves a well-formed file, so the first case proves
// the loss is observable before asserting it does not happen.

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func recordFile(f *fixture, name string) string {
	return f.scratch() + "/" + name + ".md"
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func TestConcurrentRecordWritesAllSurvive(t *testing.T) {
	t.Parallel()
	const writers = 24

	// The negative control, first: the hand-run read-modify-write these subcommands replace. Both
	// writers read the same bytes and then both write, which is what two agents in two worktrees do
	// when nothing serialises them. If this does not lose an entry, the case below proves nothing.
	lost := newRepo(t)
	lost.mkdirAll(lost.scratch())
	path := recordFile(lost, "decisions")
	lost.write(path, "1x | 2026-01-01 | seed\n")
	var readers, writes sync.WaitGroup
	readers.Add(2)
	writes.Add(2)
	for i := range 2 {
		go func() {
			defer writes.Done()
			held, _ := os.ReadFile(path)
			readers.Done()
			readers.Wait() // both have read; now both write, as two unsynchronised agents do
			_ = os.WriteFile(path, append(held, []byte("1x | 2026-01-01 | hand "+strconv.Itoa(i)+"\n")...), 0o600)
		}()
	}
	writes.Wait()
	after := lost.read(path)
	lost.record("negative control: an unserialised read-modify-write loses one of two entries",
		strings.Contains(after, "hand 0") != strings.Contains(after, "hand 1"), after)

	// The same shape through the tool. Each goroutine is its own invocation opening its own descriptor,
	// so they contend for the flock exactly as separate processes do.
	f := newRepo(t)
	f.runReport("record", "append", "decisions", "seed")
	var running sync.WaitGroup
	for i := range writers {
		running.Add(1)
		go func() {
			defer running.Done()
			f.invoke(f.repo, io.Discard, io.Discard, []string{"record", "append", "decisions", "writer " + strconv.Itoa(i)})
		}()
	}
	running.Wait()

	content := f.read(recordFile(f, "decisions"))
	missing := []string{}
	for i := range writers {
		if !strings.Contains(content, "| writer "+strconv.Itoa(i)+"\n") {
			missing = append(missing, strconv.Itoa(i))
		}
	}
	f.record("every concurrent append lands",
		len(missing) == 0, "missing writers: "+strings.Join(missing, ",")+"\n"+content)
	f.record("and the entry that was there before them all is still there",
		strings.Contains(content, "| seed\n"), content)
	f.record("and no line is a splice of two entries",
		strings.Count(content, "x | ") == writers+1, content)
}

func TestFirstWriteCreatesTheRecordWithItsHeader(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	// Every record this tool owns, with the cap its header has to state and the audience it has to
	// name. Written out here rather than read from recordKinds: a test that takes these from the code
	// it checks agrees with any values the code happens to hold, including a header stating one cap
	// while noteBound enforces another.
	headers := map[string]struct{ bound, audience string }{
		"decisions":   {"100", "never presented to a human"},
		"playbook":    {"100", "never presented to a human"},
		"language":    {"100", "an agent keeps it current"},
		"constraints": {"50", "The human owns every line"},
	}
	for name, want := range headers {
		f.runReport("record", "append", name, "the first thing anyone wrote here")
		content := f.read(recordFile(f, name))
		// `records.md` → **The cap evicts by what the record can afford to lose** requires the record to
		// state its own bound, so the header a first append writes has to carry it.
		f.record(name+".md is created with its own header, stating its bound",
			f.status == 0 && strings.HasPrefix(content, "# ") && strings.Contains(content, "Full at "+want.bound+" entries"),
			content)
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
	f.runReport("record", "append", "constraints", "p99 under 200ms on search")
	f.runReport("record", "append", "constraints", "p99 under 200ms on autocomplete")
	path := recordFile(f, "constraints")
	f.replaceLine(path, "1x | "+today()+" | p99 under 200ms on search", "4x | 2020-01-01 | p99 under 200ms on search")

	f.runReport("record", "revise", "constraints", "on search", "p99 under 200ms on every read endpoint")
	content := f.read(path)
	f.record("revise replaces the text, keeps the count and dates it today",
		f.status == 0 && strings.Contains(content, "4x | "+today()+" | p99 under 200ms on every read endpoint\n"), content)
	f.record("and the old wording is gone rather than left beside it",
		!strings.Contains(content, "| p99 under 200ms on search\n") && strings.Count(content, "x | ") == 2, content)

	// Folding two entries into one is revise-then-evict, so the pair has to end as a single entry.
	f.runReport("record", "evict", "constraints", "on autocomplete")
	f.record("evicting the folded-in entry leaves one covering both",
		f.status == 0 && strings.Count(f.read(path), "x | ") == 1, f.read(path))

	// Revising one entry into the exact text of another is the half-done fold: the record would hold the
	// same line twice, and every later bump or evict naming it would refuse as ambiguous.
	f.runReport("record", "append", "constraints", "WCAG 2.1 AA on every page")
	before := f.read(path)
	f.runReport("record", "revise", "constraints", "WCAG", "p99 under 200ms on every read endpoint")
	f.assertRefused("revising an entry into the text of another is refused")
	f.record("and the record is unchanged", f.read(path) == before, f.read(path))

	// Re-stating an entry's own current text is not a collision with itself.
	f.runReport("record", "revise", "constraints", "WCAG", "WCAG 2.1 AA on every page")
	f.record("revising an entry to its own text is allowed", f.status == 0, f.evidence())

	// Sharpening is the move that shortens a record, and the rewrite goes over the old bytes before it
	// trims them. A missed trim would leave the tail of the longer wording standing under the new one.
	f.runReport("record", "revise", "constraints", "WCAG", "WCAG AA")
	after := f.read(path)
	f.record("a revision that shortens an entry leaves no tail of the old wording",
		f.status == 0 && strings.HasSuffix(after, "| WCAG AA\n") && strings.Count(after, "x | ") == 2, after)
}

func TestBumpRaisesTheCountAndRedatesWithoutAddingALine(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "decisions", "settled once")
	f.runReport("record", "append", "decisions", "settled twice")
	path := recordFile(f, "decisions")
	f.replaceLine(path, "1x | "+today()+" | settled once", "2x | 2020-01-01 | settled once")

	f.runReport("record", "bump", "decisions", "settled once")
	content := f.read(path)
	f.record("bump raises the count and dates the entry today",
		f.status == 0 && strings.Contains(content, "3x | "+today()+" | settled once\n"), content)
	f.record("and adds no second entry for it",
		strings.Count(content, "| settled once") == 1, content)
	f.record("and leaves its neighbour alone",
		strings.Contains(content, "1x | "+today()+" | settled twice\n"), content)

	// An append of text already there is the same event, and `records.md` says it bumps rather than
	// adding a line. Only the identical text is caught — a restatement in other words is a judgment.
	f.runReport("record", "append", "decisions", "settled twice")
	f.assertRefused("appending an entry already there is refused")
	f.assertReports("record bump", "and the refusal names bump as what to do instead")
	f.record("and the record is unchanged", f.read(path) == content, f.read(path))
}

// `records.md` → **Reaching the cap** makes the cap a wall rather than advice: the writer refuses the
// append instead of reporting a file it has just grown. What has to hold is that no run of these
// subcommands puts a record over its number, and that the one way in costs exactly one entry.
func TestAFullRecordRefusesTheAppendAndAdmitIsTheWayIn(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	var seed strings.Builder
	seed.WriteString("# Decisions\n\n")
	for i := range 99 {
		seed.WriteString("5x | 2026-06-0" + strconv.Itoa(i%9+1) + " | filler " + strconv.Itoa(i) + "\n")
	}
	// The entry a count-ranking tool would name: lowest count, oldest date. `records.md` → **The cap
	// evicts by what the record can afford to lose** is that this is precisely the wrong one to steer
	// at — an entry sits at 1x because nothing has gone near its area, not because it matters least.
	seed.WriteString("1x | 2026-05-01 | the quiet one a count would sacrifice\n")
	path := recordFile(f, "decisions")
	f.write(path, seed.String())
	before := f.read(path)

	f.runReport("record", "append", "decisions", "the one that would have crossed the cap")
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
	f.record("and works the ladder before the contest that costs an entry",
		strings.Contains(f.out, "promote what must not be lost") &&
			strings.Index(f.out, "promote what must not be lost") < strings.Index(f.out, "run the judge over the new entry"), f.evidence())
	// The newcomer is judged alongside the incumbents, not against the weakest of them: a record that
	// always admits the newest is the last N in arrival order, which is the trim `records.md` forbids.
	f.record("and the contest counts the new entry among the candidates",
		strings.Contains(f.out, "lowest of the 101"), f.evidence())
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
	// off to hold a contest that would put a live entry out for a copy of itself.
	f.runReport("record", "append", "decisions", "filler 7")
	f.assertRefused("a restatement into a full record is refused")
	f.assertReports("record bump", "as the restatement it is, not as a record with no room")
	f.record("and it is not sent to the contest", !strings.Contains(f.out, "is full"), f.evidence())

	// The contest's verdict, applied: one out for one in, in a single write.
	f.runReport("record", "admit", "decisions", "filler 42", "the one that won its place")
	content := f.read(path)
	f.record("admit lands the winner, counted once and dated today",
		f.status == 0 && strings.Contains(content, "1x | "+today()+" | the one that won its place\n"), f.evidence()+"\n"+content)
	f.record("and the entry it beat is gone", !strings.Contains(content, "| filler 42\n"), content)
	f.record("and the record is no bigger than its cap", strings.Count(content, "x | ") == 100, content)
	// A revise inherits the count; an admission is ground no run has reached, so a borrowed 5x here
	// would be read as evidence by every later contest.
	f.record("and the winner does not inherit the reach of the entry it displaced",
		!strings.Contains(content, "5x | "+today()+" | the one that won its place"), content)
	// A swap reported by its winner alone is a record silently shortened.
	f.record("and both halves of the swap are reported",
		strings.Contains(f.out, "admitted to decisions.md") && strings.Contains(f.out, "in place of: 5x | ") &&
			strings.Contains(f.out, "| filler 42"), f.evidence())

	// The wall is still up behind the swap: it freed no room.
	f.runReport("record", "append", "decisions", "a second one wanting in")
	f.assertRefused("the record is still full after the swap")
	f.record("so no run of these subcommands grows it past its cap",
		strings.Count(f.read(path), "x | ") == 100, f.read(path))

	// A revise may reword an entry into its own text and lose nothing, since it keeps the count. admit
	// lands at 1x, so the same move there is a silent reset of everything the entry had earned — and it
	// reads as a swap that displaced something, which is what makes it worth a refusal of its own.
	f.replaceLine(path, "1x | "+today()+" | the one that won its place", "9x | 2026-01-02 | the one that won its place")
	earned := f.read(path)
	f.runReport("record", "admit", "decisions", "the one that won its place", "the one that won its place")
	f.assertRefused("admitting an entry over its own text is refused")
	f.record("and the count it had earned is untouched", f.read(path) == earned, f.read(path))
}

// `oneMatchingEntry` resolves by substring, and `admit` made that a destructive selector. An entry
// whose text contains another entry's whole text shadows the shorter one: every string naming the
// short entry names the long one too, so it can never again be bumped, revised, evicted, or named as
// the loser of a contest — and entry text is seeded from tickets and from the tree under review, so
// the containing entry is writable rather than a freak of wording.
func TestAnEntryIsNotShadowedByALongerOneQuotingItWhole(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "decisions", "the lock goes on the record's own descriptor")
	f.runReport("record", "append", "decisions", "the lock goes on the record's own descriptor — superseded by TICKET-9")
	path := recordFile(f, "decisions")

	// Every substring of the short entry also sits inside the long one, so nothing but the exact text
	// can single it out.
	f.runReport("record", "bump", "decisions", "the lock goes on the record's own descriptor")
	f.record("an exact hit resolves rather than colliding with the entry quoting it whole",
		f.status == 0 && strings.Contains(f.read(path), "2x | "+today()+" | the lock goes on the record's own descriptor\n"),
		f.evidence()+"\n"+f.read(path))
	f.record("and the longer entry is left alone",
		strings.Contains(f.read(path), "1x | "+today()+" | the lock goes on the record's own descriptor — superseded by TICKET-9\n"),
		f.read(path))

	// The exact rule reaches only the exact text. A partial string still matching two entries is the
	// ambiguity this function exists to refuse, and it must not now resolve to one of them.
	f.runReport("record", "bump", "decisions", "the lock goes on")
	f.assertRefused("a partial string matching two entries is still refused as ambiguous")
	f.assertReports("2 entries", "as ambiguous rather than resolved to either")

	// And the exact rule fires only where it resolves. No op here writes a duplicate, but a record is a
	// plain file a template seeds and a human edits, so two entries holding one text is a state that
	// reaches this code — and picking either would be the guess the whole function exists to refuse.
	f.appendTo(path, "1x | 2026-01-01 | a line written twice by hand\n1x | 2026-01-02 | a line written twice by hand\n")
	before := f.read(path)
	f.runReport("record", "evict", "decisions", "a line written twice by hand")
	f.assertRefused("an exact text held by two entries is refused rather than resolved to one")
	f.record("and neither copy was removed", f.read(path) == before, f.read(path))
}

// The cap belongs to the record, not to this tool: `records.md` has each file state its own, and
// constraints.md is held at half of decisions.md. A refusal quoting one figure for all four would be
// right about decisions.md and wrong everywhere else, and nothing in the header case above would see
// it — that one reads the file, this one reads what the tool said about it.
//
// It is also where constraints.md's exception has to land. `records.md` → **Reaching the cap** lets an
// agent take every move at the cap unasked except on a record whose header says a human owns the
// wording, and constraints.md's header — written by this same tool — says exactly that.
func TestTheCapCarriesTheRecordsOwnNumberAndItsOwner(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	var seed strings.Builder
	seed.WriteString("# Constraints\n\n")
	for i := range 50 {
		seed.WriteString("2x | 2026-06-01 | constraint " + strconv.Itoa(i) + "\n")
	}
	path := recordFile(f, "constraints")
	f.write(path, seed.String())

	f.runReport("record", "append", "constraints", "the one that would have crossed the tighter cap")
	f.assertRefused("a full record refuses the append whatever its number")
	f.record("the refusal quotes this record's own cap rather than another record's",
		strings.Contains(f.out, "is full — 50 entries") && !strings.Contains(f.out, "is full — 100"), f.evidence())
	f.record("and a record whose header says a human owns it is told to propose, not act",
		strings.Contains(f.out, "propose each move"), f.evidence())

	// A record already over its cap is one edited by hand, or one whose cap was lowered under it —
	// nothing these subcommands do can put one there. It still has to say so on every write it accepts.
	f.appendTo(path, "2x | 2026-06-01 | the fifty-first, put there by hand\n")
	f.runReport("record", "bump", "constraints", "constraint 49")
	f.record("an over-cap record is reported against its own number, and nothing is deleted",
		f.status == 0 && strings.Contains(f.out, "over its cap of 50") &&
			strings.Count(f.read(path), "x | ") == 51, f.evidence())
	f.record("and the note names the ladder to work rather than a verdict",
		strings.Contains(f.out, "promote what") && strings.Contains(f.out, "evict what the judge names"), f.evidence())
	f.record("and names no entry as the one to drop",
		!strings.Contains(f.out, "fifty-first") && !strings.Contains(f.out, "constraint 7"), f.evidence())
	f.record("and the human's record is told to propose each move",
		strings.Contains(f.out, "propose each move"), f.evidence())
}

func TestRecordRefusesEveryWriteItCannotResolve(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "decisions", "one entry")
	f.runReport("record", "append", "decisions", "another entry about locking")
	f.runReport("record", "append", "decisions", "a third entry about locking")
	path := recordFile(f, "decisions")
	before := f.read(path)

	cases := []struct {
		name   string
		args   []string
		says   string
		reason string
	}{
		{"an entry spanning several lines is refused", []string{"record", "append", "decisions", "first\nsecond"},
			"one line", "a second line is invisible to the count, to eviction and to every later bump"},
		{"an empty entry is refused", []string{"record", "append", "decisions", "   "},
			"empty", "an entry with nothing in it is a line the next agent cannot act on"},
		{"a record this tool does not own is refused", []string{"record", "append", "charter", "x"},
			"decisions|playbook", "the name reaches a path"},
		{"an operation that is not one of the five is refused", []string{"record", "amend", "decisions", "x"},
			"append, bump, revise, evict and admit", "a typo must not fall through to a write"},
		{"a fourth argument to an op that takes three is refused", []string{"record", "bump", "decisions", "one entry", "stray"},
			"revise and admit take", "a silently dropped argument is as likely to be half the entry someone meant"},
		{"revise without its new text is refused", []string{"record", "revise", "decisions", "one entry"},
			"revise and admit take", "three of four arguments name no replacement, and the entry must not be left as it was while reporting success"},
		{"admit without the entry it beat is refused", []string{"record", "admit", "decisions", "one entry"},
			"revise and admit take", "a swap missing one of its halves would drop an entry for nothing, or land one for free"},
		{"admit into a record with room is refused", []string{"record", "admit", "decisions", "one entry", "a new one"},
			"is not full", "the swap is the move at a full record, and taken early it throws a live entry away for a slot the file already had"},
		{"revise with an empty new text is refused", []string{"record", "revise", "decisions", "one entry", "  "},
			"empty", "the guards on an entry's text bind the replacement as much as the original"},
		{"a call missing its text is refused with the usage", []string{"record", "append", "decisions"},
			"usage:", "two of three arguments cannot name an entry"},
		{"text matching no entry is refused", []string{"record", "bump", "decisions", "nothing says this"},
			"no entry", "bumping nothing would report success having changed nothing"},
		{"text matching two entries is refused", []string{"record", "bump", "decisions", "about locking"},
			"2 entries", "the wrong one rewrites reasoning the next agent reads as settled"},
	}
	for _, one := range cases {
		f.runReport(one.args...)
		f.assertRefused(one.name)
		f.assertReports(one.says, one.name+" — and says why: "+one.reason)
		f.record(one.name+" — and the record is unchanged", f.read(path) == before, f.read(path))
	}
	// A line that is not an entry — indented here — still holds text the agent can see in the file, so
	// the refusal has to say which of the two situations it is rather than send them to check their
	// typing.
	f.appendTo(path, "   1x | 2026-01-01 | indented, so not an entry at all\n")
	f.runReport("record", "bump", "decisions", "indented, so not an entry")
	f.assertRefused("text on a line that is not an entry is refused")
	f.assertReports("IS in", "and the refusal says the text is in the file but not inside an entry")
	f.runReport("record", "bump", "decisions", "text that appears nowhere in this file")
	f.record("and that hint is absent when the text really is nowhere",
		f.status == 2 && !strings.Contains(f.out, "IS in"), f.evidence())

	// The date sits in every entry's prefix and in no entry's text, so it matches nothing. The hint
	// must not then claim it is on a line that is not an entry — those lines are entries.
	f.runReport("record", "bump", "decisions", today()[:7])
	f.record("and a search matching only an entry's prefix is not called a non-entry line",
		f.status == 2 && !strings.Contains(f.out, "IS in"), f.evidence())

	f.runReport("record", "bump", "decisions", "about locking")
	f.record("the ambiguous refusal quotes both candidates",
		strings.Contains(f.out, "another entry about locking") && strings.Contains(f.out, "a third entry about locking"),
		f.evidence())

	// A symlink at the record steers the write — and the truncation a rewrite does — wherever it
	// points, which is the rule assertWritePathsAreReal states for every other write in this tool.
	linked := newRepo(t)
	linked.mkdirAll(linked.scratch())
	outside := linked.base + "/elsewhere.md"
	linked.write(outside, "1x | 2026-01-01 | not this file\n")
	linked.symlink(outside, recordFile(linked, "decisions"))
	linked.runReport("record", "append", "decisions", "steered")
	linked.assertRefused("a symlinked record is refused")
	// On the message, not just the exit: O_NOFOLLOW refuses this open too, so a case asserting only the
	// refusal passes with recordPath's lstat guard gone and nothing observes it.
	linked.assertReports("always a regular file", "by the guard that names the rule, not by the open failing")
	linked.record("and nothing was written through it",
		!strings.Contains(linked.read(outside), "steered"), linked.read(outside))

	// The refusal quotes where the link points, and a target is arbitrary text that never has to
	// resolve — so it is external text on the same footing as an entry, and the case above sends it to
	// the terminal. Anyone who can write the scratch directory can plant one; the agent reading the
	// refusal is who the erased lines are for.
	escaped := newRepo(t)
	escaped.mkdirAll(escaped.scratch())
	escaped.symlink("/nowhere\x1b[1A\x1b[2K/decisions.md", recordFile(escaped, "decisions"))
	escaped.runReport("record", "append", "decisions", "steered")
	escaped.assertRefused("a symlinked record whose target holds an escape is still refused")
	escaped.record("and the target is collapsed rather than driving the terminal",
		!strings.Contains(escaped.out, "\x1b"), strconv.Quote(escaped.evidence()))
}

// An entry is external text that lands in a file every agent in every worktree reads, and that the tool
// echoes back on every op it accepts and on the over-cap note. Two properties keep it from being a
// channel of its own: it cannot drive the terminal, and it cannot be any size at all.
func TestAnEntryCannotDriveTheTerminalOrRunAwayInLength(t *testing.T) {
	t.Parallel()
	// `ESC [ 1 A` then `ESC [ 2 K`: move up a line and erase it, so what a terminal shows is not what
	// the tool printed and not what the record holds.
	const esc = "\x1b[1A\x1b[2K"

	// The door is not the only guard, and this is why. A record is a plain file a template seeds and a
	// human edits, so a line no `record` op ever saw can hold an ESC — `entryText` never touched it. The
	// planted entry stays in the file byte for byte, because rewriting an entry nobody asked to change
	// would be the worse fault; what must not happen is it reaching the terminal on the way back out.
	planted := newRepo(t)
	planted.mkdirAll(planted.scratch())
	plantedPath := recordFile(planted, "decisions")
	planted.write(plantedPath, "# Decisions\n\n1x | 2026-01-01 | hand-written "+esc+"\n")
	planted.runReport("record", "bump", "decisions", "hand-written")
	// Quoted, every piece of evidence in this case: a failure prints it, and the whole subject here is
	// text that drives the terminal it would print to.
	planted.record("an escape no `record` op ever saw still reaches the file",
		strings.Contains(planted.read(plantedPath), esc), strconv.Quote(planted.read(plantedPath)))
	planted.record("and is collapsed on the way back out, so the door is not the only guard",
		planted.status == 0 && !strings.Contains(planted.out, "\x1b"), strconv.Quote(planted.evidence()))

	f := newRepo(t)
	f.runReport("record", "append", "decisions", "a decision that ends here"+esc)
	path := recordFile(f, "decisions")
	f.record("an escape in an entry is collapsed rather than stored",
		f.status == 0 && !strings.Contains(f.read(path), "\x1b"), strconv.Quote(f.read(path)))
	f.record("and so is never echoed back",
		!strings.Contains(f.out, "\x1b"), strconv.Quote(f.evidence()))
	f.record("and the text it was hidden in survives",
		strings.Contains(f.read(path), "| a decision that ends here"), strconv.Quote(f.read(path)))

	// Control bytes and nothing else collapse to spaces, which is an entry with nothing in it.
	f.runReport("record", "append", "decisions", "\x01\x02")
	f.assertRefused("an entry of control bytes alone is refused as empty")

	// The record's stated cap counts entries, so one line of any length sits inside it. Without this,
	// a whole ticket appended as one "decision" is a permanent charge on every later agent's context —
	// and `discard` keeps playbook.md.
	before := f.read(path)
	f.runReport("record", "append", "decisions", strings.Repeat("A", 2001))
	f.assertRefused("an entry past the length one may hold is refused")
	f.assertReports("2001 bytes", "and the refusal says how long it was")
	f.record("and the record is unchanged", f.read(path) == before, strconv.Quote(f.read(path)))

	// The bound clears real practice by a wide margin, so it never refuses an entry someone meant: the
	// longest entry any of these records carries today is around 800 bytes, and one of that size still lands.
	f.runReport("record", "append", "decisions", strings.Repeat("A", 800))
	f.record("an entry the length real ones run to still lands",
		f.status == 0 && strings.Contains(f.read(path), "| "+strings.Repeat("A", 800)+"\n"), strconv.Quote(f.evidence()))
}

func TestAMutationNeverLosesALineItDidNotTarget(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	// The tail one of these records really carries: a block that is not in the entry format at all.
	// It is not an entry, so it is never counted, matched or evicted — and it must survive verbatim.
	block := "**A test timeout reads exactly like a hang.** `go test` without `-timeout` gives up at ten\nminutes and prints a goroutine dump.\n"
	path := recordFile(f, "playbook")
	f.write(path, "# Playbook\n\n1x | 2026-01-01 | keep me\n2x | 2026-02-02 | bump me\n1x | 2026-03-03 | evict me\n\n"+block)

	f.runReport("record", "bump", "playbook", "bump me")
	f.runReport("record", "evict", "playbook", "evict me")
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
	tailPath := recordFile(tail, "decisions")
	tail.write(tailPath, "1x | 2026-01-01 | no newline after me")
	tail.runReport("record", "append", "decisions", "an entry of my own")
	tail.record("an append to a record with no trailing newline is still its own entry",
		strings.Contains(tail.read(tailPath), "no newline after me\n1x | "+today()+" | an entry of my own\n"),
		tail.read(tailPath))
}

func TestRecordsLandWhereTheRepoModePutsThem(t *testing.T) {
	t.Parallel()
	throwaway := newRepo(t)
	throwaway.runReport("record", "append", "decisions", "throwaway")
	throwaway.record("in throwaway mode the record is outside the working tree",
		throwaway.isFile(throwaway.sharedIdsd()+"/decisions.md") && !throwaway.exists(throwaway.treeIdsd()+"/decisions.md"),
		throwaway.evidence())
	throwaway.record("and the tree stays clean", throwaway.treeIsFreeOfScratch(), throwaway.indexState())

	committed := newCommittedRepo(t)
	committed.runReport("record", "append", "decisions", "committed")
	committed.record("in committed mode it is the tracked one in the tree",
		committed.isFile(committed.treeIdsd()+"/decisions.md"), committed.evidence())
}

func TestAnEntryRoundTripsWhateverTextItCarries(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	// An entry is external text: it carries whatever the agent writing it wrote, including the
	// separator this format is built from — the parser splits on the first two, so the rest is text.
	awkward := "`gate.sh --full` | 90 % of runs — “quoted”, ünïcode, 🍦, a | pipe"
	f.runReport("record", "append", "decisions", awkward)
	path := recordFile(f, "decisions")
	f.record("the entry is stored exactly as written",
		strings.Contains(f.read(path), "1x | "+today()+" | "+awkward+"\n"), f.read(path))

	f.runReport("record", "bump", "decisions", "🍦")
	f.record("and is still findable by any part of itself",
		f.status == 0 && strings.Contains(f.read(path), "2x | "+today()+" | "+awkward+"\n"), f.evidence()+"\n"+f.read(path))
}

func TestARecordWriteWaitsForTheLockRatherThanRacingIt(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "decisions", "already here")
	path := recordFile(f, "decisions")

	// Held SHARED, deliberately. A writer taking an exclusive lock must wait for it; one that took a
	// shared lock of its own would sail straight past. That is the difference this case exists to
	// see, and a test holding an exclusive lock could not see it — both would block on that.
	handle, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("could not open the record: %v", err)
	}
	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_SH); err != nil {
		t.Fatalf("could not lock the record: %v", err)
	}

	landed := make(chan struct{})
	go func() {
		f.invoke(f.repo, io.Discard, io.Discard, []string{"record", "append", "decisions", "waited for the lock"})
		close(landed)
	}()
	// A second, against the milliseconds the same invocation takes with nothing holding the file. A
	// machine slow enough to spend a second here would report a pass it has not earned, so this case
	// belongs with a mutation run rather than standing alone.
	select {
	case <-landed:
		f.record("a write waits while another holds the record", false,
			"it appended while a lock was held:\n"+f.read(path))
	case <-time.After(time.Second):
		f.record("a write waits while another holds the record",
			!strings.Contains(f.read(path), "waited for the lock"), f.read(path))
	}

	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("could not release the lock: %v", err)
	}
	handle.Close()
	select {
	case <-landed:
	case <-time.After(30 * time.Second):
		t.Fatal("the waiting write never completed after the lock was released")
	}
	f.record("and lands as soon as the lock is released",
		strings.Contains(f.read(path), "| waited for the lock\n"), f.read(path))
}

func TestOnlyAnAppendCreatesARecord(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	for _, op := range []string{"bump", "evict"} {
		f.runReport("record", op, "playbook", "nothing is here yet")
		f.assertRefused(op + " on a record that is not there is refused")
		f.assertReports("there is no", op+" says the record is not there")
		// Not tidiness. `playbook.md` is on survivingContent's durable list, so a nought-byte one left
		// by a typo keeps a throwaway `.idsd/` standing for good — the mode's zero-traces contract
		// broken by a command that refused.
		f.record("and "+op+" leaves no empty record behind",
			!f.exists(recordFile(f, "playbook")), joinLines(f.find(f.scratch())))
	}
	// Nor the directory: a refusal that had already created one is a command that did nothing and still
	// left a trace, the same shape as the nought-byte record above.
	f.record("and neither op created the scratch directory", !f.exists(f.scratch()), joinLines(f.find(f.repo+"/.git")))

	f.runReport("record", "amend", "playbook", "an operation nobody spelled right")
	f.assertRefused("an unknown operation is refused")
	f.record("and it too leaves no scratch directory behind", !f.exists(f.scratch()), joinLines(f.find(f.repo+"/.git")))

	// With the scratch directory already there, an absent record is the ONLY thing left in the way — so
	// this is the case that observes the `creating` gate itself. Above, the open fails on the missing
	// parent whether or not O_CREATE is set, which is why those assertions pass with the gate removed.
	existing := newRepo(t)
	existing.mkdirAll(existing.scratch())
	existing.runReport("record", "bump", "playbook", "still nothing is here")
	existing.assertRefused("a bump into an existing scratch directory is still refused when the record is absent")
	existing.assertReports("there is no", "and says the record is not there, not that no entry matched")
	existing.record("and creates no empty record in it",
		!existing.exists(recordFile(existing, "playbook")), joinLines(existing.find(existing.scratch())))

	f.runReport("record", "append", "playbook", "and this is the call that creates it")
	f.record("while an append creates it", f.status == 0 && f.isFile(recordFile(f, "playbook")), f.evidence())
	// 0700, as `init` creates this tree: the records carry a project's decisions and are read from a
	// shared scratch root, so nothing outside this account has business in them.
	mode, err := os.Stat(f.scratch())
	f.record("and the scratch directory it created is reachable by nobody else",
		err == nil && mode.Mode().Perm()&0o077 == 0,
		fmt.Sprintf("%v mode %v", err, mode.Mode().Perm()))
}

func TestARecordIsNeverWrittenWhereGitCanReachIt(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.writeOverride("root " + f.repo + "/inside-the-tree\n")
	f.runReport("check-ignore")
	f.assertRefused("fixture: check-ignore refuses a scratch root inside the working tree")

	// `record` is the only command outside `init` that creates a file under the scratch directory, so
	// it has to ask the same question. Without that it writes where check-ignore refuses to, and the
	// entries land in the human's own `git add -A`.
	f.runReport("record", "append", "decisions", "this must not reach the working tree")
	f.assertRefused("and record refuses that layout rather than writing into it")
	status, _ := f.git("status", "--porcelain")
	f.record("so nothing of the record reaches git",
		!strings.Contains(status, "inside-the-tree"), "git status --porcelain:\n"+status)
}

func TestAnEvictLeavesNoTailOfWhatItRemoved(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	path := recordFile(f, "decisions")
	// The evicted entry is far longer than what follows it, so a rewrite that writes the new content
	// and does not then trim the file leaves the tail of the old content standing — and that tail is
	// whole, well-formed entries the parser goes on to count, match and bump. Asserted as the file's
	// exact bytes rather than by counting entries, because a duplicated entry satisfies every count.
	head := "# Decisions\n\n"
	keep := "1x | 2026-02-02 | one short entry\n2x | 2026-03-03 | another short entry\n"
	f.write(path, head+"1x | 2026-01-01 | "+strings.Repeat("a long entry that is about to go, ", 12)+"end\n"+keep)

	f.runReport("record", "evict", "decisions", "a long entry that is about to go")
	f.record("the record afterwards is exactly what was left, with no tail of the old content",
		f.status == 0 && f.read(path) == head+keep, f.evidence()+"\n"+strconv.Quote(f.read(path)))
}
