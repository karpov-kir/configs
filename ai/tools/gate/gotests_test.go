// A unit that cannot observe the module's `_test.go` files is not keyed on them. Two kinds qualify:
// the shell suites, which reach Go only through a binary `go build` produced, and `wiring`, whose
// checker skips test files by name when it reads Go sources. Keying on them retired 66 of 150 inputs'
// worth of good cached verdicts every time anyone touched a Go test.
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

func TestAUnitBlindToGoTestsIsNotKeyedOnThem(t *testing.T) {
	g := keyedGate()
	inputs := []string{goTree}

	blind := unit{id: "shell:tool-stub", kind: "check", inputs: inputs, cmd: "run", blindToGoTests: true}
	wholeTree := unit{id: "gotest", kind: "check", inputs: inputs, cmd: "run"}

	narrowKey, narrowLines := g.keyMaterial(blind)
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
func TestOnlyASuiteThatNeverCompilesTheModuleIsBlindToGoTests(t *testing.T) {
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
		if u.blindToGoTests != expected {
			t.Errorf("%s has blindToGoTests=%v, wanted %v. A suite that runs `go test` sees the "+
				"module's test files and must stay keyed on them; one that only execs a built tool "+
				"cannot", u.id, u.blindToGoTests, expected)
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

// `wiring` earns the flag for a different reason from the shell suites, and the reason lives in
// another package: eco-check skips `_test.go` when it reads Go sources. If that ever stops being
// true, this unit is keyed on files it does read and the gate answers a cached pass over them — so
// the claim is checked against eco-check's own source rather than restated here.
func TestWiringIsBlindToGoTestsAndEcoCheckStillSkipsThem(t *testing.T) {
	// The real registration, not a stand-in for it: the four Go checks as discoverUnits adds them.
	g := &gate{}
	g.addGoChecks()
	blind := map[string]bool{}
	for _, u := range g.units {
		blind[u.id] = u.blindToGoTests
	}
	for _, id := range []string{"gofmt", "vet", "gotest", "wiring"} {
		if _, found := blind[id]; !found {
			t.Fatalf("addGoChecks registered no unit called %s, so this case is checking a table that "+
				"no longer exists", id)
		}
	}
	if !blind["wiring"] {
		t.Error("the wiring unit is keyed on the module's test files. eco-check skips them when it " +
			"reads Go sources, so every one of them in the key retires a good cached verdict")
	}
	if blind["gotest"] {
		t.Error("the gotest unit is NOT keyed on the module's test files — it RUNS them, so this is " +
			"the gate answering a cached pass over a test that just changed")
	}

	// The other half of the claim, read where it is true rather than asserted from memory.
	source, err := os.ReadFile(filepath.Join("..", "eco-check", "subcommands.go"))
	if err != nil {
		t.Fatalf("reading eco-check's Go-source scan: %v", err)
	}
	if !strings.Contains(string(source), `strings.HasSuffix(shell.BaseName(file), "_test.go")`) {
		t.Error("eco-check no longer skips _test.go when it reads Go sources, so `wiring` may not be " +
			"blind to them. Either restore that skip or take the flag off the wiring unit — as it " +
			"stands the gate would answer a cached pass over files the check does read.")
	}
}
