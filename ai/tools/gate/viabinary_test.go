// A unit whose only reach into the Go tree is a binary `resolve.sh` compiled is not keyed on the
// module's `_test.go` files. `go build` leaves those out of every binary, so editing one cannot change
// what such a unit observes — and keying on them retired 66 of 150 inputs' worth of good cached
// verdicts every time anyone touched a Go test.
//
// The narrowing is the dangerous direction: a gate that stops watching a file it should watch reports
// a pass it did not earn, and looks exactly like a clean run. So every case here checks BOTH sides —
// that the test file went, and that the ordinary source beside it stayed.
package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	goSource   = "ai/tools/shell/path.go"
	goTestFile = "ai/tools/shell/path_test.go"
	shellFile  = "ai/tools/tool-stub-test.sh"
)

// A gate holding one manifest, so two units differing only in the flag can be compared against the
// same files.
func keyedGate() *gate {
	return &gate{
		stamp: "test-digest",
		manifest: []manifestLine{
			{hash: "aaa", path: goSource},
			{hash: "bbb", path: goTestFile},
			{hash: "ccc", path: shellFile},
		},
	}
}

func pathsIn(lines []manifestLine) []string {
	var out []string
	for _, line := range lines {
		out = append(out, line.path)
	}
	return out
}

func holds(lines []manifestLine, want string) bool {
	for _, line := range lines {
		if line.path == want {
			return true
		}
	}
	return false
}

func TestAUnitReachingGoThroughABinaryDropsTheModulesTestFiles(t *testing.T) {
	g := keyedGate()
	inputs := []string{goTree}

	viaBinary := unit{id: "shell:tool-stub", kind: "check", inputs: inputs, cmd: "run", viaCompiledBinary: true}
	wholeTree := unit{id: "gotest", kind: "check", inputs: inputs, cmd: "run"}

	narrowKey, narrowLines := g.keyMaterial(viaBinary)
	wholeKey, wholeLines := g.keyMaterial(wholeTree)

	// The control, and it comes first: a unit without the flag must still hold the test file. Without
	// this the case passes just as well over a manifest that never had one.
	if !holds(wholeLines, goTestFile) {
		t.Fatalf("the unflagged unit is not keyed on %s, so this case would pass over a filter that "+
			"does nothing. Its inputs resolved to %v", goTestFile, pathsIn(wholeLines))
	}
	if holds(narrowLines, goTestFile) {
		t.Errorf("the flagged unit is still keyed on %s. `go build` never compiles it into the binary "+
			"this unit runs, so a change there retires a verdict that is still good", goTestFile)
	}
	// The half that matters more. Narrowing past the ordinary sources is the gate quietly deciding it
	// no longer watches the code the binary is built from.
	if !holds(narrowLines, goSource) {
		t.Errorf("the flagged unit lost %s from its key, which IS compiled into the binary it runs. "+
			"The gate would then answer a cached pass over a tool that changed. Its inputs resolved to %v",
			goSource, pathsIn(narrowLines))
	}
	if !holds(narrowLines, shellFile) {
		t.Errorf("the flagged unit lost %s, which is neither Go nor a test file — the filter is "+
			"matching more than `_test.go`", shellFile)
	}
	if narrowKey == wholeKey {
		t.Error("both units computed the same key, so the flag changed nothing and every assertion " +
			"above is about one code path")
	}
}

// The flag is set at discovery, from the suite's own body, and this is where it can be set wrongly.
func TestOnlyASuiteThatNeverCompilesTheModuleGetsTheFlag(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	suites := map[string]string{
		// Reaches a tool the way the six real ones do: through the stub, which resolve.sh builds.
		"binary-test.sh": "#!/bin/sh\nai/tools/resolve.sh cite-graph\n",
		// Reaches the same tool AND compiles the module's own suites, so it sees `_test.go` and must
		// stay keyed on them.
		"compiles-test.sh": "#!/bin/sh\nai/tools/resolve.sh cite-graph\n(cd ai/tools && go test ./...)\n",
		// Names no tool at all, so it never took the tree and the flag is irrelevant to it.
		"plain-test.sh": "#!/bin/sh\necho hello\n",
	}
	for name, body := range suites {
		writeFixture(t, filepath.Join(root, name), body)
	}

	g := &gate{root: root, env: Env{Root: root}}
	if code := g.discoverShellSuites(); code != 0 {
		t.Fatalf("discovery exited %d over a tree holding three suites", code)
	}

	want := map[string]bool{"shell:binary": true, "shell:compiles": false, "shell:plain": false}
	seen := map[string]bool{}
	for _, u := range g.units {
		seen[u.id] = true
		expected, known := want[u.id]
		if !known {
			t.Errorf("discovery invented a unit called %s", u.id)
			continue
		}
		if u.viaCompiledBinary != expected {
			t.Errorf("%s has viaCompiledBinary=%v, wanted %v. A suite that runs `go test` sees the "+
				"module's test files and must stay keyed on them; one that only execs a built tool "+
				"cannot", u.id, u.viaCompiledBinary, expected)
		}
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("discovery produced no unit for %s, so its expectation above checked nothing", id)
		}
	}
}

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
