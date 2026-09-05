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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures. The gate drives `git ls-files
	// --exclude-standard`, so a global core.excludesFile hides a fixture's own declared inputs and
	// every unit below takes the NO INPUTS refusal on correct code. NOSYSTEM alone does not stop it:
	// it blocks /etc/gitconfig, and ~/.gitconfig is the one that reaches in here.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Exit(m.Run())
}

type fixture struct {
	t      *testing.T
	root   string
	cache  string
	units  string
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "r")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	f := &fixture{t: t, root: root, cache: filepath.Join(base, "cache"), units: filepath.Join(base, "units")}
	return f
}

func (f *fixture) write(name, body string) {
	f.t.Helper()
	full := filepath.Join(f.root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", name, err)
	}
}

// One unit per line: id, kind, inputs, command — tab-separated, the shape the seam reads.
func (f *fixture) table(lines ...string) {
	f.t.Helper()
	if err := os.WriteFile(f.units, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		f.t.Fatalf("writing the units file: %v", err)
	}
}

// Pinned so a key does not move with the test binary, which would make every "fresh on the second
// run" case depend on nothing having recompiled in between.
func (f *fixture) run(args ...string) {
	f.t.Helper()
	f.runAs("test-digest", args...)
}

// The same run under a stated digest of the deciding code, for the one case that is about the digest.
func (f *fixture) runAs(digest string, args ...string) {
	f.t.Helper()
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run(args, Env{
		Root:       f.root,
		Cache:      f.cache,
		UnitsFile:  f.units,
		SelfDigest: digest,
	}, &f.stdout, &f.stderr)
}

// Commit whatever is in the fixture tree, so a later deletion reads as tracked-but-gone.
func (f *fixture) commit() {
	f.t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "seed"}} {
		cmd := exec.Command("git", append([]string{"-C", f.root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func (f *fixture) out() string { return f.stdout.String() + f.stderr.String() }

func (f *fixture) expectCode(want int) {
	f.t.Helper()
	if f.code != want {
		f.t.Errorf("exit %d, wanted %d — output:\n%s", f.code, want, f.out())
	}
}

func (f *fixture) expectOut(want string) {
	f.t.Helper()
	if !strings.Contains(f.out(), want) {
		f.t.Errorf("wanted %q in:\n%s", want, f.out())
	}
}

func (f *fixture) expectNotOut(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.out(), unwanted) {
		f.t.Errorf("%q appears in:\n%s", unwanted, f.out())
	}
}

// How many times a unit's command has run, counted by the command itself appending to a file. The
// observable that separates "reported fresh" from "ran again and passed", which the report's own
// wording cannot: both end in a clean gate.
func (f *fixture) runCount(name string) int {
	body, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(body)))
}

// A command that records that it ran, then exits with the given code.
func marker(name string, code int) string {
	return "printf 'x ' >> " + name + "; exit " + strconv.Itoa(code)
}

// --- the ordinary path ------------------------------------------------------------------------------

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

// --- a failure, and what must not be recorded ----------------------------------------------------------

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

// --- the exit-code vocabulary ---------------------------------------------------------------------------

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

// --- the two refusals that outrank everything -------------------------------------------------------------

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
// there. Both refusals exit 2, so the exit code alone cannot tell them apart — the wording is the
// assertion. Past the manifest guard the unit resolves to no key material at all and is reported as
// NO INPUTS, which names the table for a file the working tree lost.
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

// --- an id that is not a filename -----------------------------------------------------------------------

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

// --- the modes ------------------------------------------------------------------------------------------

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

// The case above cannot see the record being dropped, and neither could anything else here: it moves
// the COMMAND to turn the unit red, and the command is part of the key, so the second run misses the
// green record rather than finding it removed. Every guard is still green with the removal deleted.
//
// So the unit's verdict turns on a file that is not one of its declared inputs. The key then holds
// still across all three runs, the green record from the first is the one the third would answer out
// of, and the removal is the only thing standing between a FAILED run and a clean gate over it.
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

// --- the table itself ---------------------------------------------------------------------------------

func TestUnitsAndWhyPrintWithoutRunningAnything(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))

	f.run("--units")
	f.expectCode(0)
	f.expectOut("UNIT")
	f.expectOut("one")
	f.expectOut("stale")

	f.run("--why", "one")
	f.expectCode(0)
	f.expectOut("command:")
	f.expectOut("key:")
	f.expectOut("watched.txt")
	if got := f.runCount("ran.log"); got != 0 {
		t.Errorf("--units/--why ran the command %d times", got)
	}
}

func TestWhyRefusesAnUnknownUnit(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))
	f.run("--why", "nope")
	f.expectCode(2)
	f.expectOut("no unit is called 'nope'")
}

func TestAnUnknownArgumentRefuses(t *testing.T) {
	f := newFixture(t)
	f.write("watched.txt", "one\n")
	f.table("one\tcheck\twatched.txt\t" + marker("ran.log", 0))
	f.run("--nope")
	f.expectCode(2)
	f.expectOut("unknown argument")
}

// --- the name guard --------------------------------------------------------------------------------------

// A zero-byte `ai/a;true;#-test.sh` would run as `ai/run-tests.sh -s ai/a` then `true`, so the unit
// exits 0 and the gate writes a green record for a suite that never ran. The file's contents are
// empty, so nothing reviewing contents would see it; the executable part is the name.
func TestTheNameGuardRefusesWhatWouldBecomeSyntax(t *testing.T) {
	cases := []struct{ name, value string }{
		{"a semicolon", "ai/a;true;#-test.sh"},
		{"a space", "ai/a b-test.sh"},
		{"a quote", "ai/a'b-test.sh"},
		{"a leading dash", "-rf"},
		{"an empty name", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			if err := safeToken("suite", tc.value); err == nil {
				t.Errorf("%q was accepted as a name to build a command from", tc.value)
			}
		})
	}
	// And the control: an ordinary path is accepted, or the guard above would be refusing everything.
	if err := safeToken("suite", "ai/skills/kk-one/scripts/a-test.sh"); err != nil {
		t.Errorf("an ordinary suite path was refused: %v", err)
	}
}

func TestCheckPathAnswersWithoutRunningTheGate(t *testing.T) {
	f := newFixture(t)
	f.run("--check-path", "ai/ordinary-test.sh")
	f.expectCode(0)
	f.expectOut("can safely build a command from")

	f.run("--check-path", "ai/a;true;#-test.sh")
	f.expectCode(2)
	f.expectOut("cannot safely put in a command")
}

// --- the key ------------------------------------------------------------------------------------------

// The key is a statement about the inputs, and an empty one is a record name that says nothing about
// them — every run afterwards would answer out of it whatever the tree did.
func TestTheKeyIsNeitherEmptyNorSharedBetweenDifferentInputs(t *testing.T) {
	f := newFixture(t)
	f.write("a.txt", "one\n")
	f.write("b.txt", "two\n")
	f.table(
		"first\tcheck\ta.txt\t"+marker("a.log", 0),
		"second\tcheck\tb.txt\t"+marker("b.log", 0),
	)
	f.run("--why", "first")
	firstKey := keyFrom(t, f.out())
	f.run("--why", "second")
	secondKey := keyFrom(t, f.out())

	if firstKey == "" || secondKey == "" {
		t.Fatalf("a key came back empty: %q and %q", firstKey, secondKey)
	}
	if firstKey == secondKey {
		t.Errorf("two units over different inputs share one key: %q", firstKey)
	}
}

func keyFrom(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if key, ok := strings.CutPrefix(strings.TrimSpace(line), "key:"); ok {
			return strings.TrimSpace(key)
		}
	}
	return ""
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

// `--help` answers and stops. It used to return 0 from the argument parser, which run() reads as
// "these arguments are fine" — so help printed and then the entire gate ran, writing verdict records
// for someone who asked what the flags were.
func TestHelpAnswersWithoutRunningTheGate(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			f := newFixture(t)
			f.write("watched.txt", "one\n")
			f.table("only\tcheck\twatched.txt\t" + marker("ran.txt", 0))
			f.run(flag)
			f.expectCode(0)
			f.expectOut("usage: gate.sh")
			if n := f.runCount("ran.txt"); n != 0 {
				t.Errorf("%s executed %d unit(s). Help is not a run — a caller asking what the flags are "+
					"must not have verdicts recorded on their behalf", flag, n)
			}
			if entries, err := os.ReadDir(f.cache); err == nil && len(entries) != 0 {
				t.Errorf("%s left %d file(s) in the cache, so it reached the run loop", flag, len(entries))
			}
		})
	}
}

// The usage text has one home, and the stub's header quotes it. They drifted once: the binary grew
// --check-path and the header did not, so the flag existed and was documented nowhere a reader looks.
func TestTheGateStubDocumentsTheUsageItsBinaryPrints(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "gate.sh"))
	if err != nil {
		t.Fatalf("reading the stub: %v", err)
	}
	if !strings.Contains(string(body), usageLine) {
		t.Errorf("ai/gate.sh's header does not carry the line its binary prints.\n binary: %q\n"+
			"The header is what a reader opens first; a flag missing there is a flag nobody finds.",
			usageLine)
	}
}
