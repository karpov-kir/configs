// A unit's declared inputs are a set. `--units` prints them and `--why` prints what they resolve to,
// so any count read off either is only usable while the two agree.
package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The real table, not a fixture. The collision guarded against here is two append sites reaching one
// file, and it is the repository's own suites that put a sibling script, a sourced library and a file
// copied into a fixture on the same path. A fixture would have to stage the collision to catch it,
// which is the case asserting what it was handed rather than what discovery does.
//
// The Go checks and the shell suites, and deliberately not discoverGoMutants: that one runs `go build`
// and writes a binary into the tree, which no case here needs — the mutation units' inputs come from
// a static table, not from any of the append sites this file is about.
func discoveredOverThisRepo(t *testing.T) (*gate, int) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ai", "run-tests.sh")); err != nil {
		t.Fatalf("%s is not this repository's root, so nothing below measures its units: %v", root, err)
	}
	g := &gate{root: root, env: Env{Root: root}, errOut: &strings.Builder{}}
	suites, err := g.listFiles("*-test.sh")
	if err != nil || len(suites) == 0 {
		t.Fatalf("listing this repository's suites: %v", err)
	}
	g.addGoChecks()
	g.addGuideCheck()
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery over this repository exited %d: %s",
			code, g.errOut.(*strings.Builder).String())
	}
	return g, len(suites)
}

func TestNoUnitDeclaresAnInputTwice(t *testing.T) {
	g, suites := discoveredOverThisRepo(t)

	// The control, counted rather than guessed: one unit per suite plus the five Go and guide checks.
	// A table missing any of them would walk the loop below having checked less than it looks like.
	if want := suites + 5; len(g.units) != want {
		t.Fatalf("discovery produced %d unit(s) over this repository's %d suite(s), wanted %d",
			len(g.units), suites, want)
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

	// The half the two comparisons above cannot show: a keyMaterial that ignored its lines entirely
	// would satisfy both. A unit over strictly fewer files has to key differently.
	narrower := unit{id: "shell:x", kind: "check", inputs: []string{shellFile}, cmd: "run"}
	if narrowKey, _ := g.keyMaterial(narrower); narrowKey == onceKey {
		t.Error("a unit keyed on one of the two files hashes the same as one keyed on both, so the key " +
			"is not built from the inputs at all and the comparisons above prove nothing")
	}
}
