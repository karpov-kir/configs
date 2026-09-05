package ecoreport_test

// The two shared records — one file per clone, written by every agent in every worktree. The subject
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
	for _, name := range []string{"decisions", "playbook"} {
		f.runReport("record", "append", name, "the first thing anyone wrote here")
		content := f.read(recordFile(f, name))
		// `records.md` → **The cap evicts by reach, not by age** requires the record to state its own
		// bound, so the header a first append writes has to carry it.
		f.record(name+".md is created with its own header, stating its bound",
			f.status == 0 && strings.HasPrefix(content, "# ") && strings.Contains(content, "Bound: roughly 40 lines"),
			content)
		f.record("and holds the entry, counted once and dated today",
			strings.Contains(content, "1x | "+today()+" | the first thing anyone wrote here\n"), content)
	}
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

func TestCrossingTheBoundIsReportedAndNothingIsDeleted(t *testing.T) {
	t.Parallel()
	f := newRepo(t)
	f.mkdirAll(f.scratch())
	var seed strings.Builder
	seed.WriteString("# Decisions\n\n")
	// 40 entries, none of them the candidate: every one is either higher-count or later-dated than the
	// one planted below.
	for i := range 40 {
		seed.WriteString("5x | 2026-06-0" + strconv.Itoa(i%9+1) + " | filler " + strconv.Itoa(i) + "\n")
	}
	// Lowest count wins, and the oldest date breaks a tie between equals. The decoy is planted FIRST
	// and the candidate last, so a rule that kept only the count — first lowest seen wins — names the
	// decoy here rather than agreeing with the real rule by accident.
	//
	// Neither is worded anything like the note's own line ("the entry to evict is:"): an assertion that
	// matched that wording would match whichever entry the tool named, and could never fail.
	seed.WriteString("1x | 2026-05-01 | stalest of the two lowest\n")
	path := recordFile(f, "decisions")
	f.write(path, seed.String())
	f.replaceLine(path, "5x | 2026-06-01 | filler 0", "1x | 2026-05-02 | lowest count but newer")

	f.runReport("record", "append", "decisions", "the one that crosses the bound")
	content := f.read(path)
	f.record("the entry lands even though the record is over its bound",
		f.status == 0 && strings.Contains(content, "| the one that crosses the bound\n"), f.evidence())
	f.record("crossing the bound is reported", strings.Contains(f.out, "past its bound"), f.evidence())
	f.record("the lowest-count oldest-dated entry is named as the candidate",
		strings.Contains(f.out, "1x | 2026-05-01 | stalest of the two lowest"), f.evidence())
	f.record("and the equally-low but newer one is not",
		!strings.Contains(f.out, "lowest count but newer"), f.evidence())
	f.record("and nothing was deleted",
		strings.Contains(content, "| stalest of the two lowest\n") && strings.Count(content, "x | ") == 42, content)

	// The candidate is named, never acted on: promotion and eviction are `records.md`'s judgment calls.
	f.runReport("record", "evict", "decisions", "stalest of the two lowest")
	after := f.read(path)
	f.record("evict, asked for explicitly, removes exactly that entry",
		f.status == 0 && !strings.Contains(after, "| stalest of the two lowest\n") && strings.Count(after, "x | ") == 41,
		f.evidence()+"\n"+after)
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
		{"an operation that is not one of the three is refused", []string{"record", "amend", "decisions", "x"},
			"append, bump and evict", "a typo must not fall through to a write"},
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
// echoes back on every append, bump and bound note. Two properties keep it from being a channel of its
// own: it cannot drive the terminal, and it cannot be any size at all.
func TestAnEntryCannotDriveTheTerminalOrRunAwayInLength(t *testing.T) {
	t.Parallel()
	// `ESC [ 1 A` then `ESC [ 2 K`: move up a line and erase it, so what a terminal shows is not what
	// the tool printed and not what the record holds.
	const esc = "\x1b[1A\x1b[2K"

	// The negative control, first: the echo replays whatever the file holds, so an escape that reaches
	// the file reaches the terminal. That is what makes collapsing it at the door, rather than at each
	// of the three echoes, the thing worth asserting below.
	planted := newRepo(t)
	planted.mkdirAll(planted.scratch())
	plantedPath := recordFile(planted, "decisions")
	planted.write(plantedPath, "# Decisions\n\n1x | 2026-01-01 | hand-written "+esc+"\n")
	planted.runReport("record", "bump", "decisions", "hand-written")
	// Quoted, every piece of evidence in this case: a failure prints it, and the whole subject here is
	// text that drives the terminal it would print to.
	planted.record("negative control: an escape already in the record is echoed straight to the terminal",
		strings.Contains(planted.out, esc), strconv.Quote(planted.evidence()))

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

	// The record's stated bound counts lines, so one line of any length sits inside it. Without this,
	// a whole ticket appended as one "decision" is a permanent charge on every later agent's context —
	// and `discard` keeps playbook.md.
	before := f.read(path)
	f.runReport("record", "append", "decisions", strings.Repeat("A", 2001))
	f.assertRefused("an entry past the length one may hold is refused")
	f.assertReports("2001 bytes", "and the refusal says how long it was")
	f.record("and the record is unchanged", f.read(path) == before, strconv.Quote(f.read(path)))

	// The bound clears real practice by a wide margin, so it never refuses an entry someone meant: the
	// longest entry either record carries today is around 800 bytes, and one of that size still lands.
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
	// Timed, because the hold below is a multiple of it. This case has to tell a write that waited
	// from one that was merely slow, and a fixed window measures that against a constant while the
	// machine moves. A second was that constant once: under a mutation run's concurrency this same
	// invocation ran past a second, and the case read the unfinished write as a wait. Taken here,
	// the baseline carries whatever load the contended write meets moments later.
	uncontendedAt := time.Now()
	f.runReport("record", "append", "decisions", "already here")
	uncontended := time.Since(uncontendedAt)
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
	// Written in the goroutine, read only after a receive on `landed`: the close publishes all three.
	var startedAt, landedAt time.Time
	var writeStatus int
	go func() {
		startedAt = time.Now()
		writeStatus = f.invoke(f.repo, io.Discard, io.Discard, []string{"record", "append", "decisions", "waited for the lock"})
		landedAt = time.Now()
		close(landed)
	}()
	// The multiple is what a merely slow write has to beat before it can pass for one that waited.
	hold := min(max(20*uncontended, time.Second), 10*time.Second)
	select {
	case <-landed:
	case <-time.After(hold):
	}

	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("could not release the lock: %v", err)
	}
	releasedAt := time.Now()
	handle.Close()
	select {
	case <-landed:
	case <-time.After(30 * time.Second):
		t.Fatal("the waiting write never completed after the lock was released")
	}
	// An ordering, not a deadline: the write began before the release, landed after it, and exited 0.
	// Starting before the release rules out a goroutine scheduled only afterwards, which would satisfy
	// the other two without ever contending. Exiting 0 rules out a refusal, which also lands after the
	// release.
	//
	// One gap stays open, and the hold cannot close it: the write could start on time and still
	// reach the flock call only after the release, bracketing it without ever contending. To do
	// that it would have to spend everything from its own start to the release on the work before
	// that call — twenty uncontended invocations, or ten seconds once the ceiling caps the hold.
	f.record("a write waits while another holds the record",
		writeStatus == 0 && startedAt.Before(releasedAt) && landedAt.After(releasedAt),
		fmt.Sprintf("exit %d, started at +%v, released at +%v, landed at +%v, uncontended %v\n%s",
			writeStatus, startedAt.Sub(uncontendedAt), releasedAt.Sub(uncontendedAt),
			landedAt.Sub(uncontendedAt), uncontended, f.read(path)))
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
