// A unit's declared inputs are a set. `--units` prints them and `--why` prints what they resolve to,
// so a count off either is usable only while the two agree.
package gate

import (
	"os"
	"path/filepath"
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
	g.addGuideCheck()
	checks := len(g.units)
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery over this repository exited %d: %s", code, said.String())
	}
	return g, suites, checks
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
	groups, err := groupMutants(line+line+line, "")
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
