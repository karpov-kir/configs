// Which units are keyed on the Go tool tree, checked in both directions. Over-keying costs a stale
// verdict on every unrelated Go edit and ~110 phantom inputs; under-keying reports a pass over a tool
// that changed underneath. The second is the dangerous one, so no case here narrows without a control
// asserting the narrowing stopped where it should.
//
// Two of the entries below moved from one map to the other, and what moved them is worth keeping: a
// suite that stubs a tool is not one that drives it. `ai/bootstrap-test.sh` writes its own
// `ai/tools/install.sh` and its own `ai/run-tests.sh` into fixture repos — its last verify case deletes
// the runner outright — and asserts what bootstrap makes of their exit codes. Every path by which the
// checkout's tools could be reached is the path it replaces. The marker scan cannot see that, because
// the stub and the real thing are spelt the same; `# go-tools: none` in the suite is how the suite
// says which it wrote.
package gate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	// These two say so themselves, because nothing in their text could: both are `# go-tools: none`.
	"shell:ai/bootstrap":     "the ai/tools/install.sh it names is a stub it writes into its own fixture repo",
	"shell:ai/rtk-bootstrap": "ai/bootstrap.sh does run the installer, on a branch all four of this suite's invocations skip with --skip-tools",
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

// A suite that runs its subject against fixtures it builds in a temp directory drives no real Go tool,
// however many tool paths its text contains — `ai/bootstrap-test.sh` writes a stub `ai/tools/install.sh`
// into its own fixture repo and never reaches the checkout's. The marker scan cannot tell that path from
// one into the real tree, so it took all of `ai/tools` on the strength of it, and every edit to any Go
// file re-ran a 224-second suite that could not observe one.
//
// The declaration is what tells them apart, because nothing in the text does. Absent, the scan decides
// as before and the unit takes the tree — the safe direction, since narrowing a key wrongly is what
// reports a pass nobody earned.
func newGoTreeFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "r")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("making the fixture root: %v", err)
	}
	newGitRepo(t, root)
	stubsATool := "#!/bin/sh\nmkdir -p \"$tmp/repo/ai/tools\"\n" +
		"cat >\"$tmp/repo/ai/tools/install.sh\" <<'STUB'\ntrue\nSTUB\n"
	for name, body := range map[string]string{
		"ai/run-tests.sh":                "#!/bin/sh\ntrue\n",
		"ai/tools/eco-report/records.go": "package ecoreport\n",
		"ai/tools/cite-graph/main.go":    "package main\n",
		"ai/scanned.sh":                  "#!/bin/sh\ntrue\n",
		"ai/scanned-test.sh":             stubsATool,
		"ai/declared.sh":                 "#!/bin/sh\ntrue\n",
		"ai/declared-test.sh":            "# go-tools: none — the installer is stubbed into a temp repo\n" + stubsATool,
	} {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making the directory for %s: %v", name, err)
		}
		writeFixture(t, full, body)
	}
	return root
}

func TestASuiteDeclaringItDrivesNoGoToolIsNotKeyedOnTheToolTree(t *testing.T) {
	root := newGoTreeFixture(t)

	keys := keysOverTree(t, root)
	for _, id := range []string{"shell:ai/scanned", "shell:ai/declared"} {
		if keys[id] == "" {
			t.Fatalf("discovery produced no unit called %s, so every assertion below is about a unit "+
				"that does not exist", id)
		}
	}

	editFixture(t, filepath.Join(root, "ai/tools/eco-report/records.go"))
	moved := keysOverTree(t, root)

	// The control. Without it a declaration that narrowed every suite, or a scan that had stopped
	// taking the tree at all, would read exactly like the fix.
	if moved["shell:ai/scanned"] == keys["shell:ai/scanned"] {
		t.Errorf("editing a Go file left shell:ai/scanned's key where it was, and that suite declares " +
			"nothing. An undeclared suite naming a tool path must still take the whole tool tree: this " +
			"case can no longer tell a narrowed key from a scan that narrows everything.")
	}
	if moved["shell:ai/declared"] != keys["shell:ai/declared"] {
		t.Errorf("editing ai/tools/eco-report/records.go moved shell:ai/declared's key. That suite " +
			"declares `# go-tools: none`, so it drives no tool built from that file and cannot observe " +
			"the edit — re-running it is the wholesale keying this declaration exists to end.")
	}
}

// The two ways the declaration is refused rather than honoured. Both are the same rule from opposite
// sides: the gate narrows a key on a suite's word, so it takes that word only where the word is
// checkable and nothing in the suite already contradicts it.
func TestADeclarationTheGateWillNotNarrowOn(t *testing.T) {
	cases := []struct {
		name  string
		suite string
		body  string
		want  string
	}{
		{
			name:  "no reason given",
			suite: "ai/bare-test.sh",
			body:  "# go-tools: none\n#!/bin/sh\ntrue\n",
			want:  "with no reason after it",
		},
		{
			name:  "it lives in the tool tree",
			suite: "ai/tools/inside-test.sh",
			body:  "# go-tools: none — nothing real\n#!/bin/sh\ntrue\n",
			want:  "lives inside ai/tools",
		},
		{
			name:  "it runs the module's own suites",
			suite: "ai/compiles-test.sh",
			body:  "# go-tools: none — nothing real\n#!/bin/sh\ngo test ./...\n",
			want:  "runs `go test` or `go vet`",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newGoTreeFixture(t)
			writeFixture(t, filepath.Join(root, c.suite), c.body)
			writeFixture(t, filepath.Join(root, strings.Replace(c.suite, "-test.sh", ".sh", 1)), "#!/bin/sh\ntrue\n")

			said := &strings.Builder{}
			g := &gate{root: root, env: Env{Root: root}, stamp: "test-digest", errOut: said}
			code := g.discoverShellSuites()

			// Exit 2 and not 1: a declaration the gate cannot act on means it never decided this unit's
			// inputs, which is "it did not run" and never a finding about the code.
			if code != 2 {
				t.Fatalf("discovery exited %d over %s, wanted 2. A declaration the gate refuses must stop "+
					"it, not narrow the key anyway and report a pass nobody earned. It said: %s",
					code, c.suite, said.String())
			}
			if !strings.Contains(said.String(), c.want) {
				t.Errorf("the refusal does not say %q, so a reader is told a key was not decided and not "+
					"which line to fix. It said: %s", c.want, said.String())
			}
		})
	}
}
