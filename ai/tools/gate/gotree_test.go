// Which units are keyed on the Go tool tree, checked in both directions. Over-keying costs a stale
// verdict on every unrelated Go edit and ~110 phantom inputs; under-keying reports a pass over a tool
// that changed underneath. The second is the dangerous one, so no case here narrows without a control
// asserting the narrowing stopped where it should.
package gate

import (
	"slices"
	"testing"
)

// Keyed on nothing in the tool tree, with what used to put it there anyway.
var notGoDriven = map[string]string{
	"shell:ai/install-project":                                 "a fake mise shim the suite writes into its own temp dir under $tmp_real/tools/",
	"shell:ai/project-skills":                                  "the same shim in that suite's fixture",
	"shell:ai/kk-flavor/skills/idsd-qualify/scripts/todo-gate": "a header comment naming eco-report's Go suite",
}

// The control, and the half that matters more: each of these does drive a Go tool, so a change that
// unkeyed the scan entirely would satisfy the map above and be caught only here. source-stamp is the
// one to keep in mind — it says so through a shell variable named `tools` and through comments, so on
// marker text alone it is the first suite to lose keying it needs.
var goDriven = map[string]string{
	"shell:ai/tools/source-stamp":                               "source-stamp.sh fingerprints the tree",
	"shell:ai/tools/install":                                    "the installer runs the tools built from it",
	"shell:ai/rtk-bootstrap":                                    "ai/bootstrap.sh executes ai/tools/install.sh",
	"shell:ai/kk-flavor/skills/kk-ecosystem/scripts/cite-graph": "cite-graph.sh runs the cite-graph tool",
}

func TestOnlyTheSuitesThatReachTheGoTreeAreKeyedOnIt(t *testing.T) {
	g := discoveredOverThisRepo(t)
	keyed := map[string]bool{}
	for _, u := range g.units {
		keyed[u.id] = slices.Contains(u.inputs, goTree)
	}
	for id, why := range notGoDriven {
		got, found := keyed[id]
		if !found {
			t.Errorf("no unit is called %s any more, so this expectation checked nothing. Rename it "+
				"here or drop it, but do not leave it looking like a passing case", id)
			continue
		}
		if got {
			t.Errorf("%s is keyed on %s, which it never reads — %s. Every Go source is then a phantom "+
				"input and the unit goes stale on each unrelated edit to one", id, goTree, why)
		}
	}
	for id, why := range goDriven {
		got, found := keyed[id]
		if !found {
			t.Errorf("no unit is called %s any more, so the control for the dangerous direction is "+
				"not running", id)
			continue
		}
		if !got {
			t.Errorf("%s is NOT keyed on %s, and %s. The gate would answer a cached pass over a tool "+
				"that changed under it", id, goTree, why)
		}
	}
}

func TestTheGoToolScanReadsCommandsAndNotProse(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want bool
	}{
		{"a tool run out of the tree", "ai/tools/resolve.sh eco-check\n", true},
		{"that same line commented out", "# ai/tools/resolve.sh eco-check\n", false},
		{"a tool named only in prose", "# the caller's side is pinned by eco-report's Go suite\n", false},
		{"a tool named in a command", "eco-report --check\n", true},
		{"a fixture path that merely holds the word", "cat > \"$tmp_real/tools/mise\" <<'MISE'\n", false},
		{"a comment after a real command", "run_it   # eco-check\n", true},
		{"an indented comment", "  # eco-stats --agent=claude\n", false},
	} {
		if got := drivesGoTool(c.body); got != c.want {
			t.Errorf("%s: drivesGoTool said %v, wanted %v, over %q", c.name, got, c.want, c.body)
		}
	}
}

// The suite alone is not the whole answer. Scanning only it left shell:ai/rtk-bootstrap unkeyed while
// the script it covers executes a tool out of the tree, and shell:ai/bootstrap keyed on that same
// script — one file, two units, opposite answers.
func TestTheScanReadsTheCoveredScriptAndNotOnlyTheSuite(t *testing.T) {
	suite := "# nothing in here names a tool\nbash \"$script\" --dry-run\n"
	script := "exec \"$repo/ai/tools/install.sh\"\n"

	if drivesGoTool(suite) {
		t.Fatal("the fixture suite matches on its own, so this case cannot show the script being read")
	}
	if !drivesGoTool(suite, script) {
		t.Error("a suite whose script runs a tool out of the tree is not keyed on it — the gate would " +
			"answer a cached pass over a change to that tool")
	}
	// And the other direction: two bodies that name nothing must still say no, or the case above
	// would pass over a scan that answers true for anything it is handed twice.
	if drivesGoTool(suite, "echo hello\n") {
		t.Error("the scan says yes over two bodies that name no tool at all")
	}
}
