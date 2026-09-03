package main

// The harness's own suite. A mutation harness cannot be covered by the list it runs — every entry in
// `mutants` measures some other package, and nothing in that list points back here — so what is
// covered is the part that decides things without running anything: how a suite's exit is read, and
// what preflight refuses and how it counts what it refused.
//
// That is where a harness fails quietly. A guard it wrongly calls `killed` is a proof nobody has, and
// a summary that miscounts its own refusals sends the reader hunting for a fault that is not there.
// Both print in the same columns as a run that worked.

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The file a mutant is aimed at, in the three shapes the cases below need: the anchor `if n > 0 {`
// present once, present twice, and gone. Which of the three a case takes is the whole of what that
// case is asking.
const (
	guardMatchedOnce  = "package p\n\nfunc f(n int) bool {\n\tif n > 0 {\n\t\treturn true\n\t}\n\treturn false\n}\n"
	guardMatchedTwice = "package p\n\nfunc f(n int) bool {\n\tif n > 0 {\n\t\treturn true\n\t}\n\treturn n > 0\n}\n"
	noGuardAtAll      = "package p\n\nfunc f(n int) bool {\n\treturn n > 0\n}\n"
)

// What preflight is told the suite holds: `./p/` and the one test the mutants below name. A fresh map
// each call, so no case can leave a name behind for the next.
func newHeldTests() map[string]map[string]bool {
	return map[string]map[string]bool{"./p/": {"TestBound": true}}
}

// The three answers a suite run can give about a removed guard. They print in one column, so a wrong
// reading here is indistinguishable from a right one — and one of the three is the harness crediting
// itself with a proof it never obtained.
func TestASuiteRunIsReadAsKilledOnlyWhenItActuallyRan(t *testing.T) {
	for _, c := range []struct {
		name        string
		suiteFailed bool
		output      string
		want        string
	}{
		{"a suite that went red observed the guard", true, "--- FAIL: TestThing\nFAIL\n", "killed"},
		{"a suite that stayed green did not", false, "ok  	pkg	0.2s\n", "KILLED NOTHING"},
		// The mutant never compiled, so nothing was measured. Read as a kill it manufactures the very
		// finding this harness exists to produce, and it does so in the column a reader scans for them.
		{"a mutant that did not build proves nothing", true, "# pkg\nx.go:4:2: undefined: foo\nFAIL	pkg [build failed]\n", "broken"},
		{"a type error in the mutated file proves nothing", true, "x.go:9:3: cannot use n (variable of type int)\n", "broken"},
		// A green suite is a green suite however loudly the log mentions building.
		{"build wording in a passing run is not a broken mutant", false, "ok  	pkg [build failed was printed by a test]\n", "KILLED NOTHING"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := verdictOf(c.suiteFailed, c.output); got != c.want {
				t.Errorf("verdictOf(%v, %q) = %q, want %q", c.suiteFailed, c.output, got, c.want)
			}
		})
	}
}

// A mutant that resolves is left alone. Without this the refusals below are satisfied by a preflight
// that refuses the whole list, which runs nothing and reports no problem.
func TestAResolvingMutantIsNotRefused(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "guard.go", guardMatchedOnce)
	list := []mutant{{label: "the bound", file: "guard.go", suite: "./p/", by: "TestBound", from: "if n > 0 {", to: "if n > -1 {"}}
	held := newHeldTests()

	if stale := staleMutants(list, dir, held); len(stale) != 0 {
		t.Fatalf("a mutant that resolves was refused: %+v", stale)
	}
}

func TestPreflightRefusesAMutantItCannotRunAsWritten(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "guard.go", guardMatchedTwice)
	held := newHeldTests()

	for _, c := range []struct {
		name   string
		m      mutant
		reason string
	}{
		// Nothing to edit: the guard was reworded and the anchor still names the old text.
		{"an anchor matching nothing", mutant{label: "gone", file: "guard.go", suite: "./p/", by: "TestBound", from: "if n >= 0 {", to: "x"}, "0 time(s)"},
		// Two matches edit a guard the mutant was not aimed at, and which one it hit is invisible.
		{"an anchor matching twice", mutant{label: "twice", file: "guard.go", suite: "./p/", by: "TestBound", from: "n > 0", to: "n > -1"}, "2 time(s)"},
		{"a file that is not there", mutant{label: "absent", file: "nowhere.go", suite: "./p/", by: "TestBound", from: "x", to: "y"}, "no file at"},
		// `go test -run` exits 0 on a filter matching nothing, so a renamed test comes back KILLED
		// NOTHING — loud about the suite when the fault is this list.
		{"a test its suite no longer holds", mutant{label: "renamed", file: "guard.go", suite: "./p/", by: "TestRenamedAway", from: "if n > 0 {", to: "if n > -1 {"}, "does not hold"},
	} {
		t.Run(c.name, func(t *testing.T) {
			stale := staleMutants([]mutant{c.m}, dir, held)
			if len(stale) != 1 {
				t.Fatalf("refused %d mutant(s), want 1: %+v", len(stale), stale)
			}
			if !strings.Contains(strings.Join(stale[0].reasons, "; "), c.reason) {
				t.Errorf("reasons %q name nothing about %q", stale[0].reasons, c.reason)
			}
		})
	}
}

// The command-line half of the stale-name defect preflight already refuses for `by`. A `-run` naming
// a test the suites do not hold selects nothing, `go test` exits 0 over it, and every mutant comes
// back KILLED NOTHING — 350 findings the harness invented.
func TestARunFilterNoSuiteCanAnswerIsRefused(t *testing.T) {
	held := map[string]map[string]bool{"./eco-check/": {"TestTheGravestFindingSurvivesAFlood": true}}

	for _, given := range []string{"", "TestTheGravest", "TestTheGravestFindingSurvivesAFlood/and_ranks_it"} {
		if why := unmatchedRunFilter(given, held); why != "" {
			t.Errorf("expected -run %q to be answerable, refused with %q", given, why)
		}
	}

	// The label of a mutant, which is what a reader reaches for and what no test is named.
	for _, given := range []string{"report: the per-class floor removed", "TestNoSuchCase", "Test("} {
		if unmatchedRunFilter(given, held) == "" {
			t.Errorf("expected -run %q to be refused, it was accepted", given)
		}
	}
}

// The count is of mutants, not of complaints. A renamed guard and a renamed test travel together, so
// one entry is regularly wrong twice — and counted twice it made the summary claim more broken anchors
// than the list has entries, sending the reader after a second fault that does not exist.
func TestAMutantWrongInTwoWaysIsStillOneMutant(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "guard.go", noGuardAtAll)
	held := newHeldTests()
	list := []mutant{{label: "wrong twice", file: "guard.go", suite: "./p/", by: "TestRenamedAway", from: "if n >= 99 {", to: "x"}}

	stale := staleMutants(list, dir, held)
	if len(stale) != 1 {
		t.Fatalf("counted %d mutants, want 1: %+v", len(stale), stale)
	}
	if len(stale[0].reasons) != 2 {
		t.Errorf("reasons = %q, want both the anchor and the test name", stale[0].reasons)
	}
}

// A suite whose test names could not be read at all is not evidence that a mutant naming one is
// stale. Refusing it here would report every mutant in that suite as broken, burying whatever is
// really wrong; main counts the unlistable suites apart and says so on its own line.
func TestAnUnlistableSuiteDoesNotCondemnItsMutants(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "guard.go", guardMatchedOnce)
	list := []mutant{{label: "the bound", file: "guard.go", suite: "./p/", by: "TestBound", from: "if n > 0 {", to: "if n > -1 {"}}

	if stale := staleMutants(list, dir, map[string]map[string]bool{}); len(stale) != 0 {
		t.Fatalf("a mutant was condemned for its suite being unlistable: %+v", stale)
	}
}

// A scope selects whole files and nothing else, and the suites the baseline runs shrink with it —
// otherwise a scoped run pays for compiling and listing suites no selected mutant can redden.
func TestAScopeSelectsItsFileAndNarrowsTheBaseline(t *testing.T) {
	list := []mutant{
		{label: "a", file: "one.go", suite: "./one/"},
		{label: "b", file: "two.go", suite: "./two/"},
		{label: "c", file: "one.go", suite: "./one/"},
	}
	selected, unmatched := selectByFile(list, "one.go")
	if len(unmatched) != 0 {
		t.Fatalf("one.go is in the list, yet it came back unmatched: %v", unmatched)
	}
	if len(selected) != 2 {
		t.Fatalf("selecting one.go took %d mutant(s), want the 2 that name it", len(selected))
	}
	if got := suitesNamed(selected); len(got) != 1 || got[0] != "./one/" {
		t.Fatalf("the scoped baseline is %v, want just ./one/ — the suite the selection can redden", got)
	}
}

// The refusal that keeps a typo from reading as a clean run. A name no mutant carries has to come
// back unmatched, because main exits 2 on that: silently selecting nothing would print a green over
// zero mutants, which is the one verdict this harness must never produce.
func TestAScopeNamingNoMutantIsRefusedRatherThanEmptied(t *testing.T) {
	list := []mutant{{label: "a", file: "one.go", suite: "./one/"}}
	selected, unmatched := selectByFile(list, "typo.go")
	if len(unmatched) != 1 || unmatched[0] != "typo.go" {
		t.Fatalf("unmatched is %v, want typo.go named so the caller can be refused", unmatched)
	}
	if len(selected) != 0 {
		t.Fatalf("a name no mutant carries selected %d mutant(s), want none", len(selected))
	}
}

// An empty scope is not a scope: it selects everything, so `-file ""` is the unscoped run rather than
// a run over nothing.
func TestAnEmptyScopeSelectsEverything(t *testing.T) {
	list := []mutant{{label: "a", file: "one.go"}, {label: "b", file: "two.go"}}
	if selected, _ := selectByFile(list, ""); len(selected) != 2 {
		t.Fatalf("an empty scope took %d of 2 mutant(s), want all of them", len(selected))
	}
}

// The unit listing is what `ai/gate.sh` builds its mutation units from, one per line, so a file
// missing from it is a whole unit that stops existing with nothing saying so. Every mutated file has
// to appear exactly once, carrying its own count and each suite it names.
func TestTheUnitListingNamesEveryFileOnceWithItsOwnCount(t *testing.T) {
	list := []mutant{
		{label: "a", file: "one.go", suite: "./one/"},
		{label: "b", file: "two.go", suite: "./two/"},
		{label: "c", file: "one.go", suite: "./one/"},
		{label: "d", file: "one.go", suite: "./other/"},
	}
	lines := unitLines(list, "/base")
	if len(lines) != 2 {
		t.Fatalf("the listing is %d line(s) over 2 distinct files: %v", len(lines), lines)
	}
	if lines[0] != "one.go\t./one/,./other/\t3\t/base/one.go" {
		t.Errorf("one.go's line is %q, want its 3 mutants and both suites it names, first-named first", lines[0])
	}
	if lines[1] != "two.go\t./two/\t1\t/base/two.go" {
		t.Errorf("two.go's line is %q", lines[1])
	}
}

// The listing and the scope have to agree, or the gate builds a unit it cannot run. Every file the
// listing names must select at least one mutant, and the union of those selections must be the whole
// list — otherwise some mutant belongs to no unit and nothing ever runs it.
func TestEveryListedFileSelectsAndTogetherTheyCoverEveryMutant(t *testing.T) {
	seen := 0
	for _, line := range unitLines(mutants, "/base") {
		file := strings.SplitN(line, "\t", 2)[0]
		selected, unmatched := selectByFile(mutants, file)
		if len(unmatched) != 0 {
			t.Errorf("the listing names %s, which selects nothing", file)
		}
		if len(selected) == 0 {
			t.Errorf("%s is listed and selects no mutant", file)
		}
		seen += len(selected)
	}
	if seen != len(mutants) {
		t.Errorf("the listed files cover %d of %d mutant(s), so some mutant is in no unit and never runs", seen, len(mutants))
	}
}

// The output a timeout leaves is the only thing that tells a manufactured kill from a real one. It
// has to survive the run rather than be read and dropped. Every other verdict carries none: an
// evidence block under a `killed` line would claim the suite said something it did not.
func TestOnlyATimeoutCarriesItsOutputOut(t *testing.T) {
	timedOut := "panic: test timed out after 20m0s\n\trunning tests:\n\tTestBound (20m0s)"
	if verdict, evidence := verdictWithEvidence(true, timedOut); verdict != "TIMED OUT" || evidence != timedOut {
		t.Errorf("a timed-out suite gave (%q, %d bytes of evidence), want TIMED OUT carrying all %d", verdict, len(evidence), len(timedOut))
	}
	for _, out := range []string{"--- FAIL: TestBound\nFAIL", "ok  \t./p/\t0.1s", "[build failed]"} {
		if _, evidence := verdictWithEvidence(true, out); evidence != "" {
			t.Errorf("a suite that did not time out carried evidence: %q", evidence)
		}
	}
	if verdict, evidence := verdictWithEvidence(false, "ok"); verdict != "KILLED NOTHING" || evidence != "" {
		t.Errorf("a green suite gave (%q, %q), want KILLED NOTHING and no evidence", verdict, evidence)
	}
}

// A timed-out suite exits non-zero with a panic that is neither a build failure nor a case going red.
// Read as a kill — which is what happened before this arm existed — the mutant is credited with a
// guard it never observed, and the harness manufactures the finding it exists to produce.
func TestASuiteThatRanOutOfTimeIsNotAKill(t *testing.T) {
	out := "panic: test timed out after 10m0s\n\trunning tests:\n\tTestSomething (10m0s)"
	if got := verdictOf(true, out); got != "TIMED OUT" {
		t.Fatalf("a timed-out suite came back %q, want TIMED OUT — %q credits a guard nothing observed", got, got)
	}
	// And it is neither a finding about the code nor an excuse for one.
	if shown, isBad := outcomeOf("TIMED OUT", false); isBad || shown != "TIMED OUT" {
		t.Fatalf("outcomeOf gave (%q, %v), want (TIMED OUT, false): it says nothing about the guard either way", shown, isBad)
	}
	if shown, isBad := outcomeOf("TIMED OUT", true); isBad || shown != "TIMED OUT" {
		t.Fatalf("a declared-unreachable mutant that timed out gave (%q, %v), want (TIMED OUT, false)", shown, isBad)
	}
	// The negative control: a suite that really did go red is still a kill.
	if got := verdictOf(true, "--- FAIL: TestSomething\nFAIL"); got != "killed" {
		t.Fatalf("a genuinely red suite came back %q, want killed", got)
	}
}

// Counting a timeout is not the same as showing it. A reader auditing for a manufactured kill needs
// `test timed out` in the log, and it appears only in the suite's own output. A duration alone cannot
// tell a slow compile from a timeout.
func TestATimedOutMutantPrintsTheSuitesOwnWords(t *testing.T) {
	selected := []mutant{{label: "the bound", file: "guard.go", suite: "./p/"}}
	evidence := "panic: test timed out after 20m0s\n\trunning tests:\n\tTestBound (20m0s)"

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not capture stdout: %v", err)
	}
	os.Stdout = writer
	_, _, unmeasured := report(selected, []result{{verdict: "TIMED OUT", elapsed: time.Second, evidence: evidence}})
	writer.Close()
	os.Stdout = stdout
	printed, _ := io.ReadAll(reader)

	if unmeasured != 1 {
		t.Errorf("a timed-out mutant counted %d as never measured, want 1", unmeasured)
	}
	if !strings.Contains(string(printed), "test timed out") {
		t.Errorf("the suite's own words never reached the log:\n%s", printed)
	}
	// The negative control, carrying the same evidence rather than an empty one. Hand in "" and this
	// passes even against a report that prints the block for every verdict, because there would be
	// nothing to print. Only the verdict may vary between the two halves.
	reader, writer, err = os.Pipe()
	if err != nil {
		t.Fatalf("could not capture stdout for the control: %v", err)
	}
	os.Stdout = writer
	report(selected, []result{{verdict: "killed", elapsed: time.Second, evidence: evidence}})
	writer.Close()
	os.Stdout = stdout
	printed, _ = io.ReadAll(reader)
	if strings.Contains(string(printed), "test timed out") {
		t.Errorf("a killed mutant printed timeout evidence it does not have:\n%s", printed)
	}
}

// The baseline the run is measured against has to cover every suite a mutant names, or a verdict of
// "this edit turned the suite red" is claimed over a suite that was never asked.
func TestEverySuiteAMutantNamesIsInTheBaseline(t *testing.T) {
	named := map[string]bool{}
	for _, s := range suitesNamed(mutants) {
		if named[s] {
			t.Errorf("%s is listed twice, so the baseline runs it twice", s)
		}
		named[s] = true
	}
	for _, m := range mutants {
		if !named[m.suite] {
			t.Errorf("%s names suite %s, which the baseline never runs", m.label, m.suite)
		}
	}
	if len(named) == 0 {
		t.Fatal("no suites at all — the baseline would assert nothing")
	}
}

// The baseline's flags, which the case above does not reach: it asks which suites run, never how.
// Dropping either flag is silent — every other case here stays green while the run every verdict is
// measured against changes meaning. Raising the timeout is an edit to this expectation, not a reason
// to drop the flag.
func TestTheBaselineRunPinsTheFlagsItsVerdictsRestOn(t *testing.T) {
	want := "test -count=1 -timeout " + suiteTimeout + " " + strings.Join(suitesNamed(mutants), " ")
	if got := strings.Join(baselineArgs(mutants), " "); got != want {
		t.Errorf("the baseline argv drifted — a verdict of \"this edit turned the suite red\" now rests "+
			"on a different run than the one documented.\n got: go %s\nwant: go %s", got, want)
	}
}

// The per-mutant run's argv, the one every printed verdict comes from. The no-filter branch is here
// because `-run ""` runs the same suite an omitted flag does, so nothing about the run would betray
// the flag creeping in — only this expectation would.
func TestTheMutantRunPinsTheFlagsItsVerdictsRestOn(t *testing.T) {
	for _, c := range []struct {
		name string
		m    mutant
		want string
	}{
		{
			"a mutant naming its test runs that test alone",
			mutant{suite: "./p/", by: "TestBound"},
			"test -overlay=/w/overlay.json -count=1 -failfast -timeout " + suiteTimeout + " -run TestBound ./p/",
		},
		{
			"a mutant naming no test runs its whole suite, with no empty -run",
			mutant{suite: "./p/"},
			"test -overlay=/w/overlay.json -count=1 -failfast -timeout " + suiteTimeout + " ./p/",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := strings.Join(mutantArgs(c.m, "/w/overlay.json", ""), " "); got != c.want {
				t.Errorf("the mutant argv drifted\n got: go %s\nwant: go %s", got, c.want)
			}
		})
	}

	// The hand-driving override, which has to be able to widen the filter as well as narrow it.
	widened := strings.Join(mutantArgs(mutant{suite: "./p/", by: "TestBound"}, "/w/overlay.json", ".*"), " ")
	if want := "test -overlay=/w/overlay.json -count=1 -failfast -timeout " + suiteTimeout + " -run .* ./p/"; widened != want {
		t.Errorf("a -run override no longer replaces the mutant's own test\n got: go %s\nwant: go %s", widened, want)
	}
}

// The shipped list, held to the rule preflight enforces at runtime: every mutant names a file that is
// there and a string that is in it exactly once. This is the one case here that reads the real list,
// and it is what stops the list rotting between full mutation runs — those take minutes and are not
// run per commit, so a guard reworded today is otherwise found by whoever next runs the harness.
func TestTheShippedMutantsAllResolveAgainstTheTree(t *testing.T) {
	pkgDir := "../eco-check"
	if _, err := os.Stat(pkgDir); err != nil {
		t.Skipf("the checker package is not beside this one: %v", err)
	}
	// Anchors only. Test names need `go test -list` per suite, which is the slow half and belongs to
	// the harness run rather than to this suite.
	var broken []string
	for _, m := range mutants {
		_, _, matches, err := m.anchor(pkgDir)
		switch {
		case err != nil:
			broken = append(broken, m.label+": no file at "+m.file)
		case matches != 1:
			broken = append(broken, m.label+": anchor matches "+strconv.Itoa(matches)+" time(s)")
		}
	}
	if len(broken) > 0 {
		t.Errorf("%d of %d mutant(s) no longer resolve:\n  %s", len(broken), len(mutants), strings.Join(broken, "\n  "))
	}
}

// The four outcomes, and which of them the exit code is owed to. The mismatches are the point: an
// undeclared survivor is the finding this harness exists to produce, and a declared mutant that got
// killed is a declaration that has gone false — reported loudly, because a declaration nobody
// re-checks is how a survivor keeps getting excused after the reason stopped holding.
func TestADeclarationExcusesASurvivorAndNeverAKill(t *testing.T) {
	for _, c := range []struct {
		name       string
		verdict    string
		isDeclared bool
		shown      string
		isBad      bool
	}{
		{"an ordinary kill", "killed", false, "killed", false},
		{"an undeclared survivor is the finding", "KILLED NOTHING", false, "KILLED NOTHING", true},
		{"a declared survivor is expected", "KILLED NOTHING", true, "unreachable", false},
		// The safeguard that keeps the list honest as the suite grows a case for it.
		{"a declared mutant that got killed is a stale claim", "killed", true, "STALE CLAIM", true},
		// A declaration says a guard is unobservable; it never says an edit may fail to compile.
		{"a mutant that did not build is a finding however it was declared", "broken", true, "broken", true},
		{"and undeclared too", "broken", false, "broken", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			shown, isBad := outcomeOf(c.verdict, c.isDeclared)
			if shown != c.shown || isBad != c.isBad {
				t.Errorf("outcomeOf(%q, %v) = %q, %v — want %q, %v",
					c.verdict, c.isDeclared, shown, isBad, c.shown, c.isBad)
			}
		})
	}
}

// The declaration list is an index into the mutant list, so it can rot in three ways, and each one
// leaves the run excusing a survivor for a reason that is no longer true. This is the check that runs
// per commit; the harness itself only ever asks at the start of a three-minute run.
func TestEveryUnreachableDeclarationStillNamesOneMutantAndSaysWhy(t *testing.T) {
	carried := map[string]int{}
	for _, m := range mutants {
		carried[m.label]++
	}
	for label, count := range carried {
		if count > 1 {
			t.Errorf("%d mutants share the label %q, so a declaration cannot name one of them", count, label)
		}
	}
	for _, u := range unreachableMutants {
		if carried[u.label] == 0 {
			t.Errorf("%q is declared unreachable, and no mutant carries that label", u.label)
		}
		// A declaration with no reason is indistinguishable from a mutant someone gave up on, which is
		// exactly what this list must never become.
		if strings.TrimSpace(u.why) == "" {
			t.Errorf("%q is declared unreachable with no reason given", u.label)
		}
	}
}

// A declaration excuses one label and nothing else. Asked of the lookup rather than of the list, since
// that is what the run branches on.
func TestOnlyADeclaredMutantIsExcused(t *testing.T) {
	if len(unreachableMutants) == 0 {
		t.Skip("nothing is declared unreachable, so there is no excusal to bound")
	}
	if why, ok := declaredUnreachable(unreachableMutants[0].label); !ok || why == "" {
		t.Errorf("declaredUnreachable(%q) = %q, %v — want the reason", unreachableMutants[0].label, why, ok)
	}
	// The bound: every other mutant must still be answerable for. Without this the lookup could return
	// true for anything and every survivor in the list would print as expected.
	excused := 0
	for _, m := range mutants {
		if _, ok := declaredUnreachable(m.label); ok {
			excused++
		}
	}
	if excused != len(unreachableMutants) {
		t.Errorf("%d mutants are excused, but %d are declared", excused, len(unreachableMutants))
	}
	if _, ok := declaredUnreachable("a label no mutant and no declaration carries"); ok {
		t.Error("an undeclared label was excused")
	}
}

func writeSource(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
