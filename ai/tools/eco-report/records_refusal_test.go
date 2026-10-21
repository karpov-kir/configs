package ecoreport_test

import (
	"strconv"
	"strings"
	"testing"
)

func TestRecordRefusesEveryWriteItCannotResolve(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.runReport("record", "append", "project-decisions", "one entry")
	f.runReport("record", "append", "project-decisions", "another entry about locking")
	f.runReport("record", "append", "project-decisions", "a third entry about locking")
	path := recordFile(f, "project-decisions")
	before := f.read(path)

	cases := []struct {
		name   string
		args   []string
		says   string
		reason string
	}{
		{"an entry spanning several lines is refused", []string{"record", "append", "project-decisions", "first\nsecond"},
			"one line", "a second line is invisible to the count, to eviction and to every later bump"},
		{"an empty entry is refused", []string{"record", "append", "project-decisions", "   "},
			"empty", "an entry with nothing in it is a line the next agent cannot act on"},
		{"a record this tool does not own is refused", []string{"record", "append", "charter", "x"},
			"is not a record this tool writes", "the name reaches a path"},
		// The two halves of naming the copy deliberately. Either way round, the write lands in a file the
		// caller did not mean, and finalize is the first thing that would notice.
		{"a local record with no ship named is refused", []string{"record", "append", "local-decisions", "x"},
			"needs --intent", "a local write with no ship lands at the project root"},
		{"a project record given a ship is refused", []string{"record", "--intent", "001-a", "append", "project-decisions", "x"},
			"belongs to no single ship", "a project write under a ship's name is a local one the caller mistyped"},
		{"an operation that is not one of the seven is refused", []string{"record", "amend", "project-decisions", "x"},
			"append, bump, revise, evict, admit, classify and merge", "a typo must not fall through to a write"},
		{"a fourth argument to an op that takes three is refused", []string{"record", "bump", "project-decisions", "one entry", "stray"},
			"revise and admit take", "a silently dropped argument is as likely to be half the entry someone meant"},
		{"revise without its new text is refused", []string{"record", "revise", "project-decisions", "one entry"},
			"revise and admit take", "three of four arguments name no replacement, and the entry must not be left as it was while reporting success"},
		{"admit without the entry it beat is refused", []string{"record", "admit", "project-decisions", "one entry"},
			"revise and admit take", "a swap missing one of its halves would drop an entry for nothing, or land one for free"},
		{"admit into a record with room is refused", []string{"record", "admit", "project-decisions", "one entry", "a new one"},
			"is not full", "the swap is the move at a full record, and taken early it throws a live entry away for a slot the file already had"},
		{"revise with an empty new text is refused", []string{"record", "revise", "project-decisions", "one entry", "  "},
			"empty", "the guards on an entry's text bind the replacement as much as the original"},
		{"a call missing its text is refused with the usage", []string{"record", "append", "project-decisions"},
			"usage:", "two of three arguments cannot name an entry"},
		{"text matching no entry is refused", []string{"record", "bump", "project-decisions", "nothing says this"},
			"no entry", "bumping nothing would report success having changed nothing"},
		{"text matching two entries is refused", []string{"record", "bump", "project-decisions", "about locking"},
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
	f.runReport("record", "bump", "project-decisions", "indented, so not an entry")
	f.assertRefused("text on a line that is not an entry is refused")
	f.assertReports("IS in", "and the refusal says the text is in the file but not inside an entry")
	f.runReport("record", "bump", "project-decisions", "text that appears nowhere in this file")
	f.record("and that hint is absent when the text really is nowhere",
		f.status == 2 && !strings.Contains(f.out, "IS in"), f.evidence())

	// The date sits in every entry's prefix and in no entry's text, so it matches nothing. The hint
	// must not then claim it is on a line that is not an entry — those lines are entries.
	f.runReport("record", "bump", "project-decisions", today()[:7])
	f.record("and a search matching only an entry's prefix is not called a non-entry line",
		f.status == 2 && !strings.Contains(f.out, "IS in"), f.evidence())

	f.runReport("record", "bump", "project-decisions", "about locking")
	f.record("the ambiguous refusal quotes both candidates",
		strings.Contains(f.out, "another entry about locking") && strings.Contains(f.out, "a third entry about locking"),
		f.evidence())

	// A symlink at the record steers the write — and the truncation a rewrite does — wherever it
	// points, which is the rule assertWritePathsAreReal states for every other write in this tool.
	linked := newRepo(t)
	linked.mkdirAll(linked.scratch() + "/for-agents")
	outside := linked.base + "/elsewhere.md"
	linked.write(outside, "1x | 2026-01-01 | not this file\n")
	linked.symlink(outside, recordFile(linked, "project-decisions"))
	linked.runReport("record", "append", "project-decisions", "steered")
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
	escaped.mkdirAll(escaped.scratch() + "/for-agents")
	escaped.symlink("/nowhere\x1b[1A\x1b[2K/for-agents/decisions.md", recordFile(escaped, "project-decisions"))
	escaped.runReport("record", "append", "project-decisions", "steered")
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
	plantedPath := recordFile(planted, "project-decisions")
	planted.write(plantedPath, "# Decisions\n\n## Promotion candidates\n\n## Decisions\n\n1x | 2026-01-01 | hand-written "+esc+"\n")
	planted.runReport("record", "bump", "project-decisions", "hand-written")
	// Quoted, every piece of evidence in this case: a failure prints it, and the whole subject here is
	// text that drives the terminal it would print to.
	planted.record("an escape no `record` op ever saw still reaches the file",
		strings.Contains(planted.read(plantedPath), esc), strconv.Quote(planted.read(plantedPath)))
	planted.record("and is collapsed on the way back out, so the door is not the only guard",
		planted.status == 0 && !strings.Contains(planted.out, "\x1b"), strconv.Quote(planted.evidence()))

	f := newRepo(t)
	f.runReport("record", "append", "project-decisions", "a decision that ends here"+esc)
	path := recordFile(f, "project-decisions")
	f.record("an escape in an entry is collapsed rather than stored",
		f.status == 0 && !strings.Contains(f.read(path), "\x1b"), strconv.Quote(f.read(path)))
	f.record("and so is never echoed back",
		!strings.Contains(f.out, "\x1b"), strconv.Quote(f.evidence()))
	f.record("and the text it was hidden in survives",
		strings.Contains(f.read(path), "| a decision that ends here"), strconv.Quote(f.read(path)))

	// Control bytes and nothing else collapse to spaces, which is an entry with nothing in it.
	f.runReport("record", "append", "project-decisions", "\x01\x02")
	f.assertRefused("an entry of control bytes alone is refused as empty")

	// The record's stated cap counts entries, so one line of any length sits inside it. Without this,
	// a whole ticket appended as one "decision" is a permanent charge on every later agent's context —
	// and `discard` keeps playbook.md.
	before := f.read(path)
	f.runReport("record", "append", "project-decisions", strings.Repeat("A", 2001))
	f.assertRefused("an entry past the length one may hold is refused")
	f.assertReports("2001 bytes", "and the refusal says how long it was")
	f.record("and the record is unchanged", f.read(path) == before, strconv.Quote(f.read(path)))

	// The bound clears real practice by a wide margin, so it never refuses an entry someone meant: the
	// longest entry any of these records carries today is around 800 bytes, and one of that size still lands.
	f.runReport("record", "append", "project-decisions", strings.Repeat("A", 800))
	f.record("an entry the length real ones run to still lands",
		f.status == 0 && strings.Contains(f.read(path), "| "+strings.Repeat("A", 800)+"\n"), strconv.Quote(f.evidence()))
}
