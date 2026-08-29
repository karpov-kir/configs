// The Go gate — gofmt, vet, test — is written out in two workflows. release-tools.yml gates the
// binaries a release attaches, because resolve.sh prefers a downloaded binary over the source beside
// it, so a release built from unchecked code is worse than no release. gates.yml gates a push,
// because before it a commit reached main with nothing checking it at all. Both must stay, and
// sharing them would mean a reusable workflow — more machinery than three commands are worth.
//
// So the duplication is a decision, and this is its guard: two copies with a comment hoping to stay
// aligned is how a fix lands in one and not the other, which happened in this repo today to a shell
// script and its Go twin.
//
// This file and gates.yml are a unit and must land in the same commit. With only one workflow
// carrying a Gate step, the case below has nothing to hold to parity and fails — deliberately, and
// not to be softened into a skip. A skip here would leave the drift unguarded while reading green,
// which is the failure the whole case exists to prevent.
package tools_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const workflowsDir = "../../.github/workflows"

func TestEveryWorkflowGateIsTheSameGate(t *testing.T) {
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
		t.Fatalf("found %d workflow(s) with a step named Gate under %s — this case exists because two of "+
			"them run the same gate, so fewer than two means it is holding nothing to account. Either a "+
			"gate lost its step name or a workflow lost its gate.", len(gates), workflowsDir)
	}

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
