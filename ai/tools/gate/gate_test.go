// Cases for the pre-commit gate. Three must not be weakened, and each is the gate reporting a pass it
// did not earn:
//
//   - a unit that FAILED must leave no record, or reverting an unrelated file later answers the broken
//     tree out of a green one;
//   - a unit that resolved to NO INPUT FILE must exit 2, because a rename or a typo silently narrowing
//     the gate looks exactly like a clean run;
//   - two units sharing one cache record must exit 2, because running either then reports the other
//     fresh over inputs nothing read.
//
// Every case drives the gate through its units-file seam rather than the real table: that reaches the
// run loop, the cache and every refusal in milliseconds, where discovering the real units means
// building go-mutate and listing both harnesses — the very work this exists not to do.
package gate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAUnitRunsAndIsThenFresh(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))

	f.run()
	f.expectCode(0)
	f.expectOut("ran ok")
	if got := f.runCount("ran.log"); got != 1 {
		t.Fatalf("the command ran %d times, wanted 1 — output:\n%s", got, f.out())
	}

	f.run()
	f.expectCode(0)
	f.expectOut("fresh")
	f.expectNotOut("ran ok")
	if got := f.runCount("ran.log"); got != 1 {
		t.Errorf("a fresh unit ran its command again (%d times)", got)
	}
}

func TestAnInputChangedByAByteRunsAgain(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))
	f.run()
	f.write("watched.txt", "two\n")
	f.run()
	f.expectCode(0)
	f.expectOut("ran ok")
	if got := f.runCount("ran.log"); got != 2 {
		t.Errorf("the command ran %d times after its input moved, wanted 2", got)
	}
}

// The command is part of the key, because it decides the verdict as much as the inputs do.
func TestAChangedCommandRunsAgainThoughItsInputsDidNot(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("first.log", 0))
	f.run()
	f.table("one\tcheck\twatched.txt\t" + marker("second.log", 0))
	f.run()
	f.expectCode(0)
	f.expectOut("ran ok")
	if got := f.runCount("second.log"); got != 1 {
		t.Errorf("the new command ran %d times, wanted 1", got)
	}
}

func TestAFailingUnitFailsTheGateAndRecordsNothing(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 1))

	f.run()
	f.expectCode(1)
	f.expectOut("FAILED")

	// The load-bearing half: a record written here would answer for the broken tree the moment someone
	// reverted an unrelated file.
	f.run()
	f.expectCode(1)
	f.expectOut("FAILED")
	if got := f.runCount("ran.log"); got != 2 {
		t.Errorf("the failing unit ran %d times over two runs, wanted 2 — a record was written for a failure", got)
	}
}

func TestAUnitExitingTwoIsNeverMeasuredRatherThanFailed(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 2))

	f.run()
	f.expectCode(2)
	f.expectOut("NO MEASURE")
	f.expectNotOut("FAILED")
	f.expectOut("not a pass")

	f.run()
	if got := f.runCount("ran.log"); got != 2 {
		t.Errorf("an unmeasured unit was answered from a record: ran %d times over two runs, wanted 2", got)
	}
}

// run-tests.sh's "ran, and refuses its own result" — the checkout moved underneath it. Read as a
// failure it would name the code for something a neighbouring agent did.
func TestAUnitExitingThreeRefusesItsResultRatherThanFailing(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 3))

	f.run()
	f.expectCode(2)
	f.expectOut("REFUSED")
	f.expectNotOut("FAILED")

	f.run()
	if got := f.runCount("ran.log"); got != 2 {
		t.Errorf("a refused unit was answered from a record: ran %d times, wanted 2", got)
	}
}

// A finding about the code outranks one about the machine.
func TestAFailureBesideAnUnmeasuredUnitExitsOne(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table(
		"broken\tcheck\twatched.txt\t"+marker("a.log", 1),
		"silent\tcheck\twatched.txt\t"+marker("b.log", 2),
	)
	f.run()
	f.expectCode(1)
	f.expectOut("FAILED")
	f.expectOut("NO MEASURE")
}

// A rename or a typo silently narrowing the gate looks exactly like a clean run, which is why this is
// treated as worse than a failure.
func TestAUnitResolvingToNoFileExitsTwo(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table(
		"real\tcheck\twatched.txt\t"+marker("a.log", 0),
		"phantom\tcheck\tnot-there.txt\t"+marker("b.log", 0),
	)
	f.run()
	f.expectCode(2)
	f.expectOut("NO INPUTS")
	f.expectOut("narrowed itself")
	// And it is not softened by the unit beside it that passed.
	f.expectNotOut("exit 0")
}

func TestATableWhoseEveryInputIsMissingExitsTwoAndRunsNothing(t *testing.T) {
	f := newFixture(t)
	f.table("phantom\tcheck\tnot-there.txt\t" + marker("ran.log", 0))
	f.run()
	f.expectCode(2)
	if got := f.runCount("ran.log"); got != 0 {
		t.Errorf("a unit with no inputs ran its command %d times", got)
	}
}

// A path git still lists and the filesystem no longer has. It reaches further in than the case above:
// the listing is not empty, so the refusal that fires is the manifest's own, and the run must stop
// there. Both refusals exit 2, so the wording is the assertion, not the exit code. Past the manifest
// guard the unit resolves to no key material and is reported as NO INPUTS.
func TestAnInputTrackedButGoneFromTheWorkingTreeStopsAtTheManifest(t *testing.T) {
	f := newFixture(t)
	f.write("gone.txt", "one\n")
	f.commit()
	if err := os.Remove(filepath.Join(f.root, "gone.txt")); err != nil {
		t.Fatalf("could not remove the fixture input: %v", err)
	}
	f.table("one\tcheck\tgone.txt\t" + marker("ran.log", 0))
	f.run()
	f.expectCode(2)
	f.expectOut("not one input path resolved to a file")
	f.expectNotOut("NO INPUTS")
	if got := f.runCount("ran.log"); got != 0 {
		t.Errorf("the gate ran a unit keyed on a file that is not there (%d times)", got)
	}
}

// Two units under one cache record share a verdict, so running either reports the other fresh over
// inputs nothing read. Asked about STEMS rather than ids, because the stem is the record's name.
func TestTwoIdsNamingOneCacheRecordExitTwo(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	// Different ids, one stem: every byte outside [A-Za-z0-9._-] flattens to a dash.
	f.table(
		"a:b\tcheck\twatched.txt\t"+marker("a.log", 0),
		"a/b\tcheck\twatched.txt\t"+marker("b.log", 0),
	)
	f.run()
	f.expectCode(2)
	f.expectOut("share one cache record")
	f.expectOut("different ids, one record name")
	if got := f.runCount("a.log") + f.runCount("b.log"); got != 0 {
		t.Errorf("a clashing table ran %d command(s) before refusing", got)
	}
}

// Ids are package-qualified — `mutants:go:eco-check/shell.go` — and a `/` in one names a directory the
// cache does not have, so every write for such a unit failed silently and `--mutants` reported a pass
// having recorded nothing.
func TestAnIdHoldingASlashStillRecords(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("mutants:go:pkg/file.go\tmutation\twatched.txt\t" + marker("ran.log", 0))

	f.run("--mutants")
	f.expectCode(0)
	f.expectOut("ran ok")

	f.run()
	f.expectOut("fresh")
	if got := f.runCount("ran.log"); got != 1 {
		t.Errorf("the unit ran %d times, wanted 1 — its record did not stick", got)
	}
}

func TestTheFastPathDefersMutationAndMutantsSettlesIt(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	// A check beside it, because a run whose ONLY unit is deferred has measured nothing and exits 2 by
	// the same guard that catches a table resolving to nothing. Every real table has checks in it; a
	// fixture without one would be asserting against a shape the gate never meets.
	f.table(
		"plain\tcheck\twatched.txt\t"+marker("check.log", 0),
		"mutants:shell:x\tmutation\twatched.txt\t"+marker("ran.log", 0),
	)

	f.run()
	f.expectCode(0)
	f.expectOut("DEFERRED")
	f.expectOut("ai/gate.sh --mutants")
	if got := f.runCount("ran.log"); got != 0 {
		t.Errorf("the fast path ran a mutation unit %d times", got)
	}

	f.run("--mutants")
	f.expectCode(0)
	f.expectOut("ran ok")
	if got := f.runCount("ran.log"); got != 1 {
		t.Errorf("--mutants ran it %d times, wanted 1", got)
	}
}

func TestMutantsDoesNotRunTheChecks(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("plain\tcheck\twatched.txt\t" + marker("ran.log", 0))
	f.run("--mutants")
	f.expectOut("not asked")
	if got := f.runCount("ran.log"); got != 0 {
		t.Errorf("--mutants ran a check %d times", got)
	}
	// Nothing ran and nothing was answered from cache, which is exit 2 and not a pass. Asserted here
	// rather than in a case of its own, because this table — every unit set aside — is the shape that
	// reaches that guard, and a run over it exiting 0 is the gate reporting a clean sweep it never took.
	f.expectCode(2)
	f.expectOut("nothing was measured and nothing was answered from cache")
}

// --full ignores a record and then refreshes it, so a unit that has started failing is caught rather
// than answered out of the green record it earned before.
func TestFullRunsAgainOverUnchangedInputsAndDropsTheRecordWhenItFails(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))
	f.run()
	f.expectCode(0)

	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 1))
	// Its inputs did not move, so only --full reaches it.
	f.run("--full")
	f.expectCode(1)
	f.expectOut("FAILED")

	// And the failing run dropped the record, so the next fast run measures rather than reporting fresh.
	f.run()
	f.expectCode(1)
	f.expectOut("FAILED")
}

// The case above cannot see the record being dropped: it moves the COMMAND to turn the unit red, and
// the command is part of the key, so the second run misses the green record rather than finding it
// removed. So this unit's verdict turns on a file that is not a declared input: the key holds still
// across all three runs, and the removal is the only thing between FAILED and a clean gate over it.
func TestAFailingRunDropsTheGreenRecordItAlreadyHad(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\tprintf 'x ' >> ran.log; test ! -f flip")

	f.run()
	f.expectCode(0)
	f.expectOut("ran ok")

	// --full, because the fast path would report the unit fresh and never reach the command at all.
	f.write("flip", "")
	f.run("--full")
	f.expectCode(1)
	f.expectOut("FAILED")

	f.run()
	f.expectCode(1)
	f.expectOut("FAILED")
	f.expectNotOut("inputs unchanged since it last passed")
	if got := f.runCount("ran.log"); got != 3 {
		t.Errorf("the command ran %d times over three runs, wanted 3 — the failing run left its green record behind", got)
	}
}

// A verdict is a statement about these inputs under THIS deciding code, so the digest of that code is
// in every key. Dropped from the key material, every verdict recorded by one gate binary answers for
// the next one — including the edit that broke a guard.
func TestAChangedGateBinaryIsNotAnsweredOutOfTheOldCache(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))

	f.runAs("digest-before")
	f.expectCode(0)
	f.expectOut("ran ok")

	f.runAs("digest-after")
	f.expectCode(0)
	f.expectOut("ran ok")
	f.expectNotOut("inputs unchanged since it last passed")
	if got := f.runCount("ran.log"); got != 2 {
		t.Errorf("the unit ran %d times across two gate binaries, wanted 2 — the digest is not in the key", got)
	}
}

// An input the gate cannot read used to vanish from the manifest silently, which takes it out of every
// key that declared it — its edits then stop invalidating anything, and the gate narrows itself.
func TestAnUnreadableInputExitsTwo(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.write("locked.txt", "secret\n")
	if err := os.Chmod(filepath.Join(f.root, "locked.txt"), 0o000); err != nil {
		t.Skipf("cannot drop read permission here: %v", err)
	}
	defer os.Chmod(filepath.Join(f.root, "locked.txt"), 0o644)
	if body, err := os.ReadFile(filepath.Join(f.root, "locked.txt")); err == nil {
		t.Skipf("this filesystem still reads a 0000 file (%d bytes), so the case cannot run", len(body))
	}
	f.table("one\tcheck\twatched.txt locked.txt\t" + marker("ran.log", 0))
	f.run()
	f.expectCode(2)
	f.expectOut("stop invalidating")
	if got := f.runCount("ran.log"); got != 0 {
		t.Errorf("the gate ran a unit whose input it could not hash (%d times)", got)
	}
}
