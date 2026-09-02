// The Go gate — gofmt, vet, test — is written out in two workflows. release-tools.yml gates the
// binaries a release attaches, because resolve.sh prefers a downloaded binary over the source beside
// it, so a release built from unchecked code is worse than no release. gates.yml gates a push,
// because before it a commit reached main with nothing checking it at all. Both must stay, and
// sharing them would mean a reusable workflow — more machinery than three commands are worth.
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

// What the Gate runs. The parity case below passes just as happily on two copies that are wrong
// together, so each flag is pinned here: without `-count=1` a cached `ok` covers a package that
// fails, and without `-timeout 30m` the eco-report suite runs past Go's 10m default on a loaded
// runner and panics with a goroutine dump that reads as a hang rather than as a slow pass. Raising
// the timeout is an edit to this constant, never a reason to drop the flag.
const goSuiteRun = "go test -count=1 -timeout 30m ./..."

func TestEveryWorkflowGateRunsTheGoSuiteWithItsFlagsPinned(t *testing.T) {
	for name, step := range gateSteps(t) {
		if !strings.Contains(step, goSuiteRun) {
			t.Errorf("the Gate step in %s does not run `%s`, so the run it gates is not the one that was "+
				"decided on — and both flags fail silently when absent.\n\n%s:\n%s", name, goSuiteRun, name, step)
		}
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
