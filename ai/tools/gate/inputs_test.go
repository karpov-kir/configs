// A unit's declared inputs are a set. `--units` prints them and `--why` prints what they resolve to,
// so a count off either is usable only while the two agree.
package gate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

// The real table, not a fixture. The collision guarded against here is two append sites reaching one
// file, and only this repository's own suites produce it: a sibling script is often also the library
// it sources, and a fixture copy is a second path to the same input. A staged fixture would assert
// what the case handed it rather than what discovery does.
//
// The Go checks and the shell suites, not discoverGoMutants: it runs `go build` and writes a binary
// into the tree. The case below covers the mutation units' own append site directly instead.
// Returns the table, the suite count it should hold one unit for, and how many checks were already
// registered before those units — so a caller compares two counted numbers, never a written-down one.
func discoveredOverThisRepo(t *testing.T) (*gate, int, int) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "run-tests.sh")); err != nil {
		t.Fatalf("%s is not this repository's root, so nothing below measures its units: %v", root, err)
	}
	said := &strings.Builder{}
	g := &gate{root: root, env: Env{Root: root}, errOut: said}
	listed, err := g.listFiles("*-test.sh")
	if err != nil || len(listed) == 0 {
		t.Fatalf("listing this repository's suites: %v", err)
	}
	// Deduplicated the way discoverShellSuites deduplicates its own listing. `git ls-files --cached`
	// names a path once per stage, so during an unmerged suite the raw count runs ahead of the table
	// and the control below would blame a table that is correct.
	suites := len(shell.SortUnique(listed))
	g.addGoChecks()
	if code := g.addGuideCheck(thisModulesImports(t, g)); code != 0 {
		t.Fatalf("the guide unit did not register: %s", said.String())
	}
	g.addModelCheck()
	checks := len(g.units)
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery over this repository exited %d: %s", code, said.String())
	}
	return g, suites, checks
}

// The guide unit has to be keyed on the main package its command builds. Hand-listed, it named the
// library packages and not `cmd/eco-guide`, so editing main.go left the verdict fresh.
func TestTheGuideUnitIsKeyedOnTheCommandItRuns(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	var guide *unit
	for i := range g.units {
		if g.units[i].id == "guide" {
			guide = &g.units[i]
		}
	}
	if guide == nil {
		t.Fatal("no unit called guide, so this case would pass against any key at all")
	}
	for _, want := range []string{ecoGuideCommand, "ai/tools/eco-guide", "ai/tools/eco-root", "ai/tools/shell"} {
		if !slices.Contains(guide.inputs, want) {
			t.Errorf("guide is not keyed on %s, which its command is built from, so an edit there leaves "+
				"the verdict fresh over a binary nothing rebuilt", want)
		}
	}
	// The narrowness half: this check builds one command, not the module.
	if slices.Contains(guide.inputs, goTree) {
		t.Error("guide is keyed on the whole tool tree, so any Go edit at all re-runs it")
	}
}

// A graph that cannot answer for that command refuses, rather than keying on the three paths left.
func TestAGuideUnitTheGraphCannotAnswerForRefuses(t *testing.T) {
	said := &strings.Builder{}
	g := &gate{errOut: said}
	if code := g.addGuideCheck(map[string][]string{}); code != 2 {
		t.Fatalf("addGuideCheck exited %d over a graph naming no package, want 2", code)
	}
	if len(g.units) != 0 {
		t.Errorf("it registered %d unit(s) anyway, keyed on less than the command builds", len(g.units))
	}
}

// This repository's own graph, read as discovery reads it. `go list` writes nothing, so a case may
// take this where it may not call discoverGoMutants.
func thisModulesImports(t *testing.T, g *gate) map[string][]string {
	t.Helper()
	listing, err := g.listModulePackages()
	if err != nil {
		t.Fatalf("listing this module's packages: %v", err)
	}
	reached, err := moduleImports(listing, g.root)
	if err != nil {
		t.Fatalf("reading this module's import graph: %v", err)
	}
	return reached
}

func TestNoUnitDeclaresAnInputTwice(t *testing.T) {
	g, suites, checks := discoveredOverThisRepo(t)

	// The control: one unit per discovered suite, over and above the checks registered before them.
	// Both numbers are counted here rather than written down, so adding a Go check does not fail this
	// case with a total that names the wrong cause. A table short of a unit would walk the loop below
	// having checked less than it looks like.
	if got := len(g.units) - checks; got != suites {
		t.Fatalf("discovery produced %d shell unit(s) over this repository's %d suite(s)", got, suites)
	}
	for _, u := range g.units {
		if len(u.inputs) == 0 {
			t.Errorf("%s declares no inputs at all, so it checked nothing here", u.id)
			continue
		}
		seen := map[string]bool{}
		var twice []string
		for _, in := range u.inputs {
			if seen[in] {
				twice = append(twice, in)
			}
			seen[in] = true
		}
		if len(twice) > 0 {
			t.Errorf("%s declares %v more than once, so a count read off `--units` is %d too high. "+
				"Dedupe where the unit is built — addUnit — and not at whichever append site collided",
				u.id, twice, len(u.inputs)-len(seen))
		}
	}
}

// The mutation units are built somewhere else entirely, and they left this file's reach when the
// helper above stopped calling buildUnits. groupMutants appends to one group's inputs once per mutant
// over that suite set, so a set holding many mutants over one file is where the widest duplication
// lives — and it is g.add, not groupMutants, that has to collapse it. Staged over the listing format
// the harness emits rather than run through `go build`.
func TestAMutantGroupReachesAUnitWithNoInputTwice(t *testing.T) {
	const line = "eco-report/records.go\t./eco-report/\tTestSomething\tai/tools/eco-report/records.go\n"
	groups, err := groupMutants(line+line+line, "", suiteCompiles)
	if err != nil {
		t.Fatalf("grouping three mutants over one file: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("three mutants over one suite set produced %d group(s), wanted 1", len(groups))
	}

	// The control, and it is the point of the case: grouping itself repeats the file, once per mutant.
	// If that ever stops being true this case proves nothing about g.add, so it has to be asserted.
	if count(groups[0].inputs, "ai/tools/eco-report/records.go") < 2 {
		t.Fatalf("grouping no longer repeats a file across its mutants, so registering it below cannot "+
			"show the dedupe doing anything: %v", groups[0].inputs)
	}

	g := &gate{}
	g.add(groups[0].id, "mutation", groups[0].inputs, "run")
	if len(g.units) != 1 {
		t.Fatalf("registering one group produced %d unit(s)", len(g.units))
	}
	if n := count(g.units[0].inputs, "ai/tools/eco-report/records.go"); n != 1 {
		t.Errorf("the mutation unit declares its file %d times. groupMutants appends per mutant, so a "+
			"suite set with many mutants over one file is the widest duplication in the table", n)
	}
}

func count(values []string, want string) int {
	n := 0
	for _, v := range values {
		if v == want {
			n++
		}
	}
	return n
}

// Why deduping is safe at all, and the half a display fix would leave unstated: a duplicate never
// reached a key. `linesUnder` walks the manifest and takes each line at most once however many times
// a path is declared. If that stops being true, deduping the declared inputs stops being tidying and
// silently retires every cached verdict in the table.
func TestADuplicatedInputDoesNotMoveAUnitsKey(t *testing.T) {
	g := &gate{stamp: "test-digest", manifest: []manifestLine{
		{hash: "aaa", path: goSource},
		{hash: "ccc", path: shellFile},
	}}
	once := unit{id: "shell:x", kind: "check", inputs: []string{goTree, shellFile}, cmd: "run"}
	twice := unit{id: "shell:x", kind: "check", inputs: []string{goTree, shellFile, shellFile, goTree}, cmd: "run"}

	onceKey, onceLines := g.keyMaterial(once)
	twiceKey, twiceLines := g.keyMaterial(twice)

	// The control: the key has to be built over something. Two units resolving to nothing would agree
	// here and the case would pass over a manifest that matched neither.
	if len(onceLines) != 2 {
		t.Fatalf("the singly-declared unit resolved to %v, wanted both fixture files", pathsIn(onceLines))
	}
	if onceKey != twiceKey {
		t.Errorf("declaring an input twice moved the key (%s against %s), so deduping declared inputs "+
			"retires every cached verdict rather than only correcting what --units prints",
			onceKey[:12], twiceKey[:12])
	}
	if len(twiceLines) != len(onceLines) {
		t.Errorf("the duplicate reached the key material: %v against %v",
			pathsIn(twiceLines), pathsIn(onceLines))
	}

	// The half the two comparisons above cannot show: a keyMaterial that ignored its lines entirely
	// would satisfy both. A unit over strictly fewer files has to key differently.
	narrower := unit{id: "shell:x", kind: "check", inputs: []string{shellFile}, cmd: "run"}
	if narrowKey, _ := g.keyMaterial(narrower); narrowKey == onceKey {
		t.Error("a unit keyed on one of the two files hashes the same as one keyed on both, so the key " +
			"is not built from the inputs at all and the comparisons above prove nothing")
	}
}

// The gotest unit has to be keyed on the stubs its suite opens. `stub_usage_test.go` discovers every
// stub in the repository and reads each one, all of them outside this module, and Go's test cache
// cannot see any of them: without the key, editing a stub's header leaves the unit fresh and the drift
// check answers out of a cache over a file it never re-read — a check that cannot fire, reported as a
// pass.
//
// Asserted against the built unit rather than against the source text. `gate_script_test.go` holds the
// two halves of the wiring to each other by parsing this file; what that cannot say is whether a
// particular path ended up in the list, which is the fact a reader of `--why gotest` relies on.
func TestTheGotestUnitIsKeyedOnTheStubsItsSuiteReads(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)

	var gotest *unit
	for i := range g.units {
		if g.units[i].id == "gotest" {
			gotest = &g.units[i]
		}
	}
	if gotest == nil {
		t.Fatal("no unit called gotest, so this case would pass against any key at all")
	}
	if len(extStubs) == 0 {
		t.Fatal("extStubs is empty, so the loop below asserts nothing")
	}
	for _, stub := range extStubs {
		if !slices.Contains(gotest.inputs, stub) {
			t.Errorf("gotest is not keyed on %s, so an edit to that stub's header leaves this unit fresh "+
				"and stub_usage_test.go compares a line nothing re-read", stub)
		}
	}
}

// `go list`'s answer over the real module, which is what every unit's key now rests on. Two directions,
// and only one is the cheap mistake: eco-report's suite compiles repo-key, so that pair must be there,
// while cadence compiles nothing but itself, so a graph answering "the whole module" fails here.
func TestThisModulesGraphSaysWhatASuiteCompiles(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	reached := thisModulesImports(t, &gate{root: root})
	if !slices.Contains(reached["ai/tools/eco-report"], "ai/tools/repo-key") {
		t.Errorf("the graph does not say eco-report's suite compiles repo-key, so editing repokey.go "+
			"leaves mutants:go:eco-report fresh: %v", reached["ai/tools/eco-report"])
	}
	if got := reached["ai/tools/cadence"]; len(got) != 1 || got[0] != "ai/tools/cadence" {
		t.Errorf("cadence's suite compiles nothing else in this module, and the graph answers %v — a "+
			"unit keyed on that re-runs on edits that cannot move its verdict", got)
	}
}
