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
//
// That the LOCAL gate bounds its own `go test` is not here any more. It was a scan of gate.go's source
// lines for the literal, which read that file's text and never the commands it builds. It went red on
// the refactor that named the bound once, and `gate/gate_test.go`'s
// TestEveryGoCommandIsBoundedAboveTheBudget reads those commands instead.
package tools_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"configs/ai/tools/shell"
)

const repoRoot = "../.."

const workflowsDir = repoRoot + "/.github/workflows"

const gateSource = "gate/gate.go"

// What the Gate's `go test` must carry, minus the bound. The parity case below passes just as happily
// on two copies that are wrong together, so each flag is pinned here: without `-count=1` a cached `ok`
// covers a package that fails, and without `./...` the gate runs a subset of the module.
//
// The bound is deliberately not in this list. It has one home, `suiteTimeoutSeconds` in
// ai/tools/gate/gate.go, and TestEveryWorkflowGateBoundsGoTestLikeTheGate derives it from there.
// Pinned here too, raising it would mean one more file to edit, with this case red until that edit
// lands.
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

// suiteTimeoutSeconds, the const in ai/tools/gate/gate.go, is the bound's one home. It is the
// backstop every `go test` carries, so a suite that hangs on a runner fails in minutes, well inside
// Go's ten-minute default.

// It is NOT the 100 seconds `testing.md` rule 6 states. That number is a budget over the whole gate,
// and gate.go's own wall clock enforces it. A `go test` carrying the budget killed the package before
// the gate could report which one was slow.

// A pointer is only as good as something checking it still points at the same number, and this case is
// that check.
func TestEveryWorkflowGateBoundsGoTestLikeTheGate(t *testing.T) {
	gateBody, err := os.ReadFile(gateSource)
	if err != nil {
		t.Fatalf("reading %s: %v", gateSource, err)
	}
	const bound = "suiteTimeoutSeconds"
	want := constSecondsIn(string(gateBody), bound)
	if want == "" {
		t.Fatalf("%s no longer declares %s at the start of a line, so this case has nothing to "+
			"hold the workflows to and would pass over any value they carry. Restore the declaration, or "+
			"retire this case deliberately — do not leave it green over nothing.", gateSource, bound)
	}

	checked := 0
	for name, step := range gateSteps(t) {
		bounds := goTestBounds(step)
		if len(bounds) == 0 {
			t.Errorf("the Gate step in %s runs no `go test`, so whatever it gates is not this module's "+
				"suite. Either the step lost its command or this case is looking at the wrong step.",
				name)
			continue
		}
		for i, carried := range bounds {
			checked++
			switch {
			case carried == "":
				t.Errorf("`go test` invocation %d in %s's Gate step carries no -timeout, so Go's 10m "+
					"default applies there while %s uses %s. Overrunning prints a goroutine dump that "+
					"reads as a hang rather than as a suite over its budget, and this repository has "+
					"twice reported one as a red gate that was not one.", i+1, name, gateSource, want)
			case carried != want:
				t.Errorf("`go test` invocation %d in %s's Gate step passes -timeout %s, but %s sets "+
					"%s=%s — the two drifted, so the same suite is bounded differently "+
					"depending on who runs it.", i+1, name, carried, gateSource, bound, want)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no workflow under %s has a Gate step running `go test`, so this case held nothing to "+
			"account. That is the drift going unguarded while reading green.", workflowsDir)
	}
}

// Returns a `const <name> = <n>` seconds declaration, rendered the way `go test -timeout` spells one.
// The match is anchored at column zero, so a mention inside a comment or a nested scope is not
// mistaken for the declaration.
func constSecondsIn(source, name string) string {
	assign := "const " + name + " = "
	for _, line := range strings.Split(source, "\n") {
		if rest, found := strings.CutPrefix(line, assign); found {
			if value := strings.TrimSpace(strings.SplitN(rest, "//", 2)[0]); value != "" {
				return value + "s"
			}
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
// header declares war on.
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

// A script gets deleted or moved, the workflow invoking it is not touched, and every push after that
// goes red on a 127. This is the guard for that.
//
// What it can see is a field that looks like a path once shell wrapping is cut off, resolved against
// the step's own `working-directory`. An invocation spelled so that nothing path-shaped survives is
// skipped rather than reported, because a field ending in `.sh` inside a message is not something
// anyone runs — and the zero-invocations check below is what stops that leniency emptying the case.
func TestEveryScriptAWorkflowRunsExists(t *testing.T) {
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
		for _, script := range scriptsRun(string(body)) {
			checked++
			info, err := os.Stat(filepath.Join(repoRoot, script))
			if err != nil {
				t.Errorf("a step in %s runs %s, and no such file is in the checkout. The job exits 127 on "+
					"every push, and 127 says a file is missing rather than anything about the code the job "+
					"exists to gate.", entry.Name(), script)
				continue
			}
			if info.Mode()&0o111 == 0 {
				t.Errorf("a step in %s runs %s, which is in the checkout but not executable. The job exits "+
					"126, which is the same silence as 127 one cause further on.", entry.Name(), script)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no step under %s was seen to run a script, so this case held nothing to account.",
			workflowsDir)
	}
}

// A `#` ends the line here, so both a whole-line comment and the tail of one are skipped. A
// workflow's prose names scripts that deliberately no longer exist, and saying what a job used to run
// is how a removal gets explained.
func scriptsRun(body string) []string {
	var found []string
	seen := map[string]bool{}
	for _, step := range runBlocks(body) {
		for _, line := range strings.Split(step.body, "\n") {
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "#") {
					break
				}
				path := strings.TrimPrefix(unwrapShell(field), "./")
				if !strings.HasSuffix(path, ".sh") || !isPlainPath(path) {
					continue
				}
				path = filepath.Join(step.dir, path)
				if seen[path] {
					continue
				}
				seen[path] = true
				found = append(found, path)
			}
		}
	}
	return found
}

// The path inside a field a shell would run it from: `stamp="$(./source-stamp.sh` is an invocation,
// and dropping it because of the assignment and the substitution around it leaves this case covering
// three of the four scripts these workflows run while its own comment claims all of them.
//
// Only the opening wrappers are cut. A trailing one would make `"$(cat` end in a path shape it never
// had, and isPlainPath is what refuses whatever this leaves behind.
func unwrapShell(field string) string {
	for {
		cut := strings.LastIndexAny(field, "$(\"'`=")
		if cut < 0 {
			return field
		}
		field = field[cut+1:]
	}
}

// Fields ending in `.sh` that name no file are common in these steps: `*-test.sh` in a glob,
// `note="ai/gate.sh` inside a message. Reporting one as missing would be a finding against text
// nobody runs. So the narrow direction is deliberate: an oddly spelled invocation is skipped, and
// the zero-invocations check above stops that from emptying the case.
func isPlainPath(path string) bool {
	for _, b := range []byte(path) {
		switch {
		case shell.IsAlnumByte(b):
		case b == '.', b == '_', b == '-', b == '/':
		default:
			return false
		}
	}
	return path != ""
}

// Every `run:` step in a workflow, block form and one-liner alike, each with the directory it runs in.
// Line-based for the reason gateStep gives.
//
// `working-directory` is carried because a path in the step is relative to it, not to the checkout: a
// step that cds into `ai/tools` and runs `./source-stamp.sh` names a file that does not exist at the
// repo root. It may be declared before or after the `run:` it applies to, so a block takes whichever
// the surrounding step carries.
type runStep struct {
	body string
	dir  string
}

func runBlocks(body string) []runStep {
	var blocks []runStep
	var block []string
	inBlock, indent := false, 0
	dir, pending := "", ""
	flush := func() {
		if inBlock {
			blocks = append(blocks, runStep{trimTrailingBlanks(block), dir})
			block, inBlock = nil, false
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if inBlock {
			if trimmed == "" {
				block = append(block, "")
				continue
			}
			if len(line)-len(strings.TrimLeft(line, " ")) > indent {
				block = append(block, trimmed)
				continue
			}
			flush()
		}
		switch {
		// A new step clears what the last one declared; a `working-directory` inside one is held for
		// whichever `run:` that step has, before it or after.
		case strings.HasPrefix(trimmed, "- "):
			dir, pending = "", ""
			trimmed = strings.TrimPrefix(trimmed, "- ")
			if !strings.HasPrefix(trimmed, "working-directory:") && !strings.HasPrefix(trimmed, "run:") {
				continue
			}
			fallthrough
		default:
			switch {
			case strings.HasPrefix(trimmed, "working-directory:"):
				pending = strings.TrimSpace(strings.TrimPrefix(trimmed, "working-directory:"))
				dir = pending
			case trimmed == "run: |" || trimmed == "run: >":
				indent = len(line) - len(strings.TrimLeft(line, " "))
				dir, inBlock = pending, true
			case strings.HasPrefix(trimmed, "run: "):
				blocks = append(blocks, runStep{strings.TrimPrefix(trimmed, "run: "), pending})
			}
		}
	}
	flush()
	return blocks
}
