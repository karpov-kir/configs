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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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
	writeSource(t, dir, "guard.go", "package p\n\nfunc f(n int) bool {\n\tif n > 0 {\n\t\treturn true\n\t}\n\treturn false\n}\n")
	list := []mutant{{label: "the bound", file: "guard.go", suite: "./p/", by: "TestBound", from: "if n > 0 {", to: "if n > -1 {"}}
	held := map[string]map[string]bool{"./p/": {"TestBound": true}}

	if stale := staleMutants(list, dir, held); len(stale) != 0 {
		t.Fatalf("a mutant that resolves was refused: %+v", stale)
	}
}

func TestPreflightRefusesAMutantItCannotRunAsWritten(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "guard.go", "package p\n\nfunc f(n int) bool {\n\tif n > 0 {\n\t\treturn true\n\t}\n\treturn n > 0\n}\n")
	held := map[string]map[string]bool{"./p/": {"TestBound": true}}

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

// The count is of mutants, not of complaints. A renamed guard and a renamed test travel together, so
// one entry is regularly wrong twice — and counted twice it made the summary claim more broken anchors
// than the list has entries, sending the reader after a second fault that does not exist.
func TestAMutantWrongInTwoWaysIsStillOneMutant(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "guard.go", "package p\n\nfunc f(n int) bool {\n\treturn n > 0\n}\n")
	held := map[string]map[string]bool{"./p/": {"TestBound": true}}
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
	writeSource(t, dir, "guard.go", "package p\n\nfunc f(n int) bool {\n\tif n > 0 {\n\t\treturn true\n\t}\n\treturn false\n}\n")
	list := []mutant{{label: "the bound", file: "guard.go", suite: "./p/", by: "TestBound", from: "if n > 0 {", to: "if n > -1 {"}}

	if stale := staleMutants(list, dir, map[string]map[string]bool{}); len(stale) != 0 {
		t.Fatalf("a mutant was condemned for its suite being unlistable: %+v", stale)
	}
}

// The baseline the run is measured against has to cover every suite a mutant names, or a verdict of
// "this edit turned the suite red" is claimed over a suite that was never asked.
func TestEverySuiteAMutantNamesIsInTheBaseline(t *testing.T) {
	named := map[string]bool{}
	for _, s := range suitesNamed() {
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
	want := "test -count=1 -timeout 30m " + strings.Join(suitesNamed(), " ")
	if got := strings.Join(baselineArgs(), " "); got != want {
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
			"test -overlay=/w/overlay.json -count=1 -failfast -run TestBound ./p/",
		},
		{
			"a mutant naming no test runs its whole suite, with no empty -run",
			mutant{suite: "./p/"},
			"test -overlay=/w/overlay.json -count=1 -failfast ./p/",
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
	if want := "test -overlay=/w/overlay.json -count=1 -failfast -run .* ./p/"; widened != want {
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

// The four outcomes, and which of them the exit code is owed to. The diagonal is the point: an
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
		// the whole failure mode the list has to not become.
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
