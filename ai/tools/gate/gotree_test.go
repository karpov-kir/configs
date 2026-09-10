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
	// The other two the comment skip is holding: drop that skip and three units re-key, so all three
	// have to be named here for the revert to redden.
	"shell:ai/run-tests-concurrency": "this suite's own header naming ai/tools/gate/units.go",
	"shell:lib/install-registry":     "lib/install-registry.sh's header naming a bloat-judge source",
}

// The control, and the half that matters more: each of these does drive a Go tool, so a change that
// unkeyed the scan entirely would satisfy the map above and be caught only here. source-stamp is the
// one to keep in mind — it says so through a shell variable named `tools` and through comments, so on
// marker text alone. Every unit the current logic keys is listed, so a narrowing cannot slip past by
// unkeying one nobody wrote down. Which arm holds each is worth knowing: source-stamp, ai/tools/install,
// resolve and tool-stub are keyed by their path, the rest only by what their text says.
var goDriven = map[string]string{
	"shell:ai/tools/source-stamp":                               "source-stamp.sh fingerprints the tree, and says so only through a $tools variable",
	"shell:ai/tools/install":                                    "the installer runs the tools built from it",
	"shell:ai/tools/resolve":                                    "it lives in the tool tree and resolves the tools out of it",
	"shell:ai/tools/tool-stub":                                  "it copies the stubs that reach the tools",
	"shell:ai/rtk-bootstrap":                                    "ai/bootstrap.sh runs tools/install.sh, and this suite names no tool itself",
	"shell:ai/bootstrap":                                        "the same script, and this suite also stubs ai/tools/install.sh and asserts on it",
	"shell:ai/kk-flavor/skills/kk-ecosystem/scripts/cite-graph": "cite-graph.sh runs cite-graph through tools/resolve.sh",
	"shell:ai/kk-flavor/skills/kk-ecosystem/scripts/ruleecho":   "ruleecho.sh runs rule-echo the same way",
}

func TestOnlyTheSuitesThatReachTheGoTreeAreKeyedOnIt(t *testing.T) {
	g, _, _ := discoveredOverThisRepo(t)
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
		// Copied from ai/bootstrap.sh — the `$repo` root is what `ai/tools/` cannot match. Reword the
		// diagnostics that quote the full path and shell:ai/rtk-bootstrap unkeys without this.
		{"the installer reached through a variable root", "  \"$repo/tools/install.sh\"\n", true},
		// The clause that decides blindness as well as keying. A suite compiling the module observes its
		// test files, so it takes the tree and is NOT blind to them.
		{"a suite that runs the module's own suites", "(cd ai/tools && go test ./...)\n", false},
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
