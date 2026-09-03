// The Go gate — gofmt, vet, test — is written out in two workflows. Why each of them must stay, and
// why sharing them would cost more than the three commands are worth, is in
// .github/workflows/gates.yml's own header.
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

const gateScript = "../gate.sh"

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

// The `go test` bound is one fact with two homes: `gotest_timeout` in ai/gate.sh, and the `-timeout`
// each workflow's Gate step passes. The workflows carry a comment pointing at the script instead of
// repeating its reasoning, and a pointer is only as good as something checking it still points at the
// same number. This is that check.
//
// Both ends have to be found or the case fails outright. A guard comparing two strings it could not
// locate compares nothing and reports green, which is the same defect as a test case that cannot fail
// — and the reason the bound exists at all is that a `go test` timeout reads as a deadlock rather than
// as a slow pass, so drift here is expensive to diagnose and cheap to prevent.
func TestEveryWorkflowGateBoundsGoTestLikeTheGateScript(t *testing.T) {
	script, err := os.ReadFile(gateScript)
	if err != nil {
		t.Fatalf("reading %s: %v", gateScript, err)
	}
	want := gotestTimeout(string(script))
	if want == "" {
		t.Fatalf("%s no longer assigns gotest_timeout at the start of a line, so this case has nothing to "+
			"hold the workflows to and would pass over any value they carry. Restore the assignment, or "+
			"retire this case deliberately — do not leave it green over nothing.", gateScript)
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
					"rather than a slow pass.", i+1, entry.Name(), gateScript, want)
			case bound != want:
				t.Errorf("`go test` invocation %d in %s's Gate step passes -timeout %s, but %s sets "+
					"gotest_timeout=%s — the two drifted, so the same suite is bounded differently "+
					"depending on who runs it.", i+1, entry.Name(), bound, gateScript, want)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no workflow under %s has a Gate step running `go test`, so this case held nothing to "+
			"account. That is the drift going unguarded while reading green.", workflowsDir)
	}
}

// The value `ai/gate.sh` assigns to gotest_timeout. Line-based and anchored at column zero, so a
// mention inside a comment or a nested scope is not mistaken for the assignment itself.
func gotestTimeout(script string) string {
	const assign = "gotest_timeout="
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, assign) {
			return strings.TrimSpace(strings.TrimPrefix(line, assign))
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
	for _, line := range strings.Split(step, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if fields[0] != "go" || fields[1] != "test" {
			continue
		}
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
