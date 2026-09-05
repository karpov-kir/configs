package gate

// What the gate answers without running a unit: the table listing, --why, --help, --check-path,
// the name guard that decides what may be built into a command, and the key those answers echo.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
