// A unit's declared inputs are a set. Two views of one unit that disagree about how many files it is
// keyed on make any count taken from either unusable, and that is not hypothetical: a reviewer
// checking a keying claim off `--units` had to cross-check `--why` to get a figure worth quoting.
package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The real table, not a fixture. The collision guarded against here is two append sites reaching one
// file, and it is the repository's own suites that put a sibling script, a sourced library and a named
// peer on the same path. A fixture would have to stage the collision to catch it, which is the case
// asserting what it was handed rather than what discovery does.
func discoveredOverThisRepo(t *testing.T) *gate {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "run-tests.sh")); err != nil {
		t.Fatalf("%s is not this repository's root, so nothing below measures its units: %v", root, err)
	}
	g := &gate{root: root, env: Env{Root: root}, errOut: &strings.Builder{}}
	if code := g.buildUnits(); code != 0 {
		t.Fatalf("discovery over this repository exited %d: %s",
			code, g.errOut.(*strings.Builder).String())
	}
	return g
}

func TestNoUnitDeclaresAnInputTwice(t *testing.T) {
	g := discoveredOverThisRepo(t)

	// The control. A table of one unit, or of units carrying no inputs, would walk the loop below
	// having checked nothing and report a pass for it.
	if len(g.units) < 20 {
		t.Fatalf("discovery produced %d unit(s) over this repository, too few to be its real table",
			len(g.units))
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
}
