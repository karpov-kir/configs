// The Go gate — gofmt, vet, test — is written out in two workflows. Why each of them must stay, and
// why sharing them would cost more than the three commands are worth, is in
// .github/workflows/gates.yml's own header.
//
// So the duplication is a decision, and this is its guard: two copies with a comment hoping to stay
// aligned is how a fix lands in one and not the other. The cases below hold the copies to each other
// and to what they should say — agreeing on the wrong thing is still green.
//
// This file and gates.yml are a unit and must land in the same commit: with only one workflow
// carrying a Gate step, they have nothing to hold to account and fail deliberately.
package tools_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const workflowsDir = "../../.github/workflows"

const gateSource = "gate/run.go"

// What the Gate's `go test` must carry, minus the bound. The parity case below passes just as happily
// on two copies that are wrong together, so each flag is pinned here: without `-count=1` a cached `ok`
// covers a package that fails, and without `./...` the gate runs a subset of the module.
//
// The bound is deliberately not in this list. It has one home — `goSuiteTimeout` in ai/tools/gate/run.go — and
// TestEveryWorkflowGateBoundsGoTestLikeTheGateScript derives it from there. Pinned here as well, the
// number would have a fourth home, and raising it would take a four-file edit with this case red until
// the last one landed. That is the drift both cases exist to stop, reintroduced by the guard against
// it.
var goSuiteFlags = []string{"-count=1", "./..."}

func TestEveryWorkflowGateRunsTheGoSuiteWithItsFlagsPinned(t *testing.T) {
	checked := 0
	for name, step := range gateSteps(t) {
		invocations := goTestLines(step)
		if len(invocations) == 0 {
			t.Errorf("the Gate step in %s runs no `go test`, so the run it gates is not this module's "+
				"suite.\n\n%s:\n%s", name, name, step)
			continue
		}
		for i, fields := range invocations {
			checked++
			for _, flag := range goSuiteFlags {
				if !hasField(fields, flag) {
					t.Errorf("`go test` invocation %d in %s's Gate step does not pass %s, so the run it "+
						"gates is not the one that was decided on — and this flag fails silently when "+
						"absent.\n\n%s:\n%s", i+1, name, flag, name, step)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatalf("no workflow under %s has a Gate step running `go test`, so this case held nothing to "+
			"account. That is the drift going unguarded while reading green.", workflowsDir)
	}
}

func hasField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

// The local gate is the third runner of that suite and the one a human actually watches. It cannot
// carry `goSuiteFlags` verbatim — it selects packages rather than running `./...`, and forces some with
// `-count=1` because the Go cache cannot see the fixtures' external inputs — so what is held here is
// the part that must not vary: no invocation of `go test` may go out without a timeout, for the
// reason goSuiteFlags above gives.
//
// Read from the Go source, which is where those invocations live now.
func TestTheLocalGateNeverRunsTheGoSuiteWithoutATimeout(t *testing.T) {
	const source = "gate/run.go"
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("reading %s: %v", source, err)
	}
	var found int
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		// The invocations only, never the prose about them: a comment naming `go test` is not a run.
		if strings.HasPrefix(trimmed, "//") || !strings.Contains(trimmed, `"test"`) {
			continue
		}
		found++
		if !strings.Contains(trimmed, "-timeout") {
			t.Errorf("%s runs the Go suite with no -timeout: %s", source, trimmed)
		}
	}
	// Zero invocations means this case is holding nothing to account — the runner moved, or the suite
	// did, and either way the assertion above passed over nothing.
	if found == 0 {
		t.Fatalf("found no `go test` invocation in %s, so this case checked nothing", source)
	}
}

func TestEveryWorkflowGateIsTheSameGate(t *testing.T) {
	gates := gateSteps(t)

	names := make([]string, 0, len(gates))
	for name := range gates {
		names = append(names, name)
	}
	sort.Strings(names)

	first := names[0]
	for _, name := range names[1:] {
		if gates[name] != gates[first] {
			t.Errorf("the Gate step in %s differs from the one in %s — the copies have drifted, so a change "+
				"landed in one and not the other.\n\n%s:\n%s\n%s:\n%s",
				name, first, first, gates[first], name, gates[name])
		}
	}
}

// Every workflow's step named `Gate`, by file name. Fewer than two is fatal for both cases above:
// either a gate lost its step name or a workflow lost its gate, and both leave the cases with
// nothing to hold to account. Not to be softened into a skip, which would leave the drift unguarded
// while reading green.
func gateSteps(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		t.Fatalf("reading %s: %v", workflowsDir, err)
	}

	gates := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(workflowsDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		if step, ok := gateStep(string(body)); ok {
			gates[entry.Name()] = step
		}
	}

	if len(gates) < 2 {
		t.Fatalf("found %d workflow(s) with a step named Gate under %s — fewer than two means these cases "+
			"are holding nothing to account.", len(gates), workflowsDir)
	}
	return gates
}

// The `go test` bound is one fact with two homes: `goSuiteTimeout` in ai/tools/gate/run.go, and the `-timeout`
// each workflow's Gate step passes. The workflows carry a comment pointing at the script instead of
// repeating its reasoning, and a pointer is only as good as something checking it still points at the
// same number. This is that check.
//
// Both ends have to be found or the case fails outright. A guard comparing two strings it could not
// locate compares nothing and reports green, which is the same defect as a test case that cannot fail
// — and the reason the bound exists at all is that a `go test` timeout reads as a deadlock rather than
// as a slow pass, so drift here is expensive to diagnose and cheap to prevent.
func TestEveryWorkflowGateBoundsGoTestLikeTheGateScript(t *testing.T) {
	script, err := os.ReadFile(gateSource)
	if err != nil {
		t.Fatalf("reading %s: %v", gateSource, err)
	}
	want := gotestTimeout(string(script))
	if want == "" {
		t.Fatalf("%s no longer declares goSuiteTimeout at the start of a line, so this case has nothing to "+
			"hold the workflows to and would pass over any value they carry. Restore the assignment, or "+
			"retire this case deliberately — do not leave it green over nothing.", gateSource)
	}

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		t.Fatalf("reading %s: %v", workflowsDir, err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(workflowsDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		step, ok := gateStep(string(body))
		if !ok {
			continue
		}
		bounds := goTestBounds(step)
		if len(bounds) == 0 {
			t.Errorf("the Gate step in %s runs no `go test`, so whatever it gates is not this module's "+
				"suite. Either the step lost its command or this case is looking at the wrong step.",
				entry.Name())
			continue
		}
		for i, bound := range bounds {
			checked++
			switch {
			case bound == "":
				t.Errorf("`go test` invocation %d in %s's Gate step carries no -timeout, so Go's 10m "+
					"default applies there while %s uses %s. eco-report alone has been measured past 10m "+
					"on a loaded runner, and overrunning prints a goroutine dump that reads as a hang "+
					"rather than a slow pass.", i+1, entry.Name(), gateSource, want)
			case bound != want:
				t.Errorf("`go test` invocation %d in %s's Gate step passes -timeout %s, but %s sets "+
					"goSuiteTimeout to %s — the two drifted, so the same suite is bounded differently "+
					"depending on who runs it.", i+1, entry.Name(), bound, gateSource, want)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no workflow under %s has a Gate step running `go test`, so this case held nothing to "+
			"account. That is the drift going unguarded while reading green.", workflowsDir)
	}
}

// The value the gate declares for goSuiteTimeout. It used to live in ai/gate.sh as `gotest_timeout=`;
// the gate is Go now and the bound moved with it. Line-based and anchored at column zero, so a mention
// inside a comment or a nested scope is not mistaken for the declaration itself.
func gotestTimeout(script string) string {
	const assign = `const goSuiteTimeout = "`
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, assign) {
			return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, assign)), `"`)
		}
	}
	return ""
}

// The -timeout each `go test` line in a Gate step carries — one entry per line, in order, empty where
// a line carries none. An empty slice means the step runs no `go test` at all, which says this case is
// reading the wrong thing rather than that anything drifted.
//
// A slice and not one value, because a step may run `go test` more than once. Collapsed to a single
// bound, one invocation carrying -timeout vouches for a sibling that carries none, and the case reports
// green over exactly the drift it exists to catch — a case that cannot fail, which is what this file's
// header declares war on. Comment lines are skipped, because the step's own comment names the flag it
// is explaining.
func goTestBounds(step string) []string {
	var bounds []string
	for _, fields := range goTestLines(step) {
		bound := ""
		for i := 2; i+1 < len(fields); i++ {
			if fields[i] == "-timeout" {
				bound = fields[i+1]
			}
		}
		bounds = append(bounds, bound)
	}
	return bounds
}

// The fields of each `go test` line in a Gate step, in order. Both cases read their own fact out of
// these, so neither can disagree with the other about which lines are the suite's. Comment lines are
// skipped, because a step's own comment names the flags it is explaining.
func goTestLines(step string) [][]string {
	var invocations [][]string
	for _, line := range strings.Split(step, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if fields[0] != "go" || fields[1] != "test" {
			continue
		}
		invocations = append(invocations, fields)
	}
	return invocations
}

// The body of the step named `Gate`, dedented, with trailing blank lines dropped. Line-based rather
// than parsed, because this module carries no YAML dependency and shipped_test.go reads its own
// workflow the same way.
func gateStep(body string) (string, bool) {
	var block []string
	seen, inBlock, indent := false, false, 0
	for _, line := range strings.Split(body, "\n") {
		switch {
		case !seen:
			if strings.TrimSpace(line) == "- name: Gate" {
				seen = true
			}
		case !inBlock:
			if strings.TrimSpace(line) == "run: |" {
				indent = len(line) - len(strings.TrimLeft(line, " "))
				inBlock = true
			}
		default:
			if strings.TrimSpace(line) == "" {
				block = append(block, "")
				continue
			}
			if len(line)-len(strings.TrimLeft(line, " ")) <= indent {
				return trimTrailingBlanks(block), true
			}
			block = append(block, line[indent+2:])
		}
	}
	if inBlock {
		return trimTrailingBlanks(block), true
	}
	return "", false
}

func trimTrailingBlanks(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
