package tools_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The marker a suite writes when it deliberately has no scratch. Written out rather than inferred,
// because "I found no writes" is a claim about this scan's thoroughness, not about the suite.
const noScratchMarker = "# no scratch:"

func TestEveryShellSuiteOwnsItsScratch(t *testing.T) {
	suites := shellSuites(t)
	if len(suites) == 0 {
		t.Fatal("found no *-test.sh at all, so this case would pass over any suite in any state")
	}

	for _, suite := range suites {
		body, err := os.ReadFile(filepath.Join("..", "..", suite))
		if err != nil {
			t.Errorf("reading %s: %v", suite, err)
			continue
		}
		if !ownsScratch(t, string(body)) {
			t.Errorf("%s makes no scratch directory of its own, so it depends on being the only thing "+
				"running — and the gate runs this lane beside the Go suites. Use `mktemp -d`, source a "+
				"harness that does, or state `%s <why>` if this suite writes nothing at all.",
				suite, noScratchMarker)
		}
	}
}

func ownsScratch(t *testing.T, text string) bool {
	t.Helper()
	switch {
	case strings.Contains(text, "mktemp -d"):
		return true
	case strings.Contains(text, noScratchMarker):
		return true
	default:
		return sourcesAHarnessThatMktemps(t, text)
	}
}

func TestWhatCountsAsOwningScratch(t *testing.T) {
	for _, row := range []struct {
		name  string
		body  string
		owned bool
	}{
		{"mktemp of its own", "#!/bin/sh\nwork=$(mktemp -d)\n", true},
		{"a sourced harness that mktemps", "#!/bin/sh\n. \"$checkout/lib/test-harness.sh\"\n", true},
		{"a declared need for none", "#!/bin/sh\n" + noScratchMarker + " it writes nothing\n", true},
		{"a fixed path under /tmp", "#!/bin/sh\nwork=/tmp/suite-scratch\nmkdir -p \"$work\"\n", false},
		{"no scratch and no marker", "#!/bin/sh\necho hello\n", false},
		{"a harness that mktemps nothing", "#!/bin/sh\n. \"$checkout/lib/mount.sh\"\n", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := ownsScratch(t, row.body); got != row.owned {
				t.Errorf("ownsScratch = %v, wanted %v", got, row.owned)
			}
		})
	}
}

// Whether the suite sources a file that mktemps on its behalf. One level, which is all this tree
// does; a harness that delegated further would need this to follow it, and the case would say so by
// failing rather than by passing over the gap.
func sourcesAHarnessThatMktemps(t *testing.T, text string) bool {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ". ") && !strings.HasPrefix(line, "source ") {
			continue
		}
		// The path as written holds a shell variable for the checkout root; only its tail is fixed.
		fields := strings.Fields(line)
		sourced := strings.Trim(fields[len(fields)-1], `"'`)
		idx := strings.Index(sourced, "lib/")
		if idx < 0 {
			continue
		}
		body, err := os.ReadFile(filepath.Join("..", "..", sourced[idx:]))
		if err == nil && strings.Contains(string(body), "mktemp -d") {
			return true
		}
	}
	return false
}

// Every suite the gate would discover, from the same command it uses — `-z` and core.quotePath=false
// included. Split on whitespace instead, a name holding a space arrives as two names, this scan reads
// two files that do not exist, and the suite it was about goes unchecked. gate/units.go carries a
// mutant for that hazard, and this claim holds only while both spell the command the same way.
func shellSuites(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "-c", "core.quotePath=false", "ls-files", "-z",
		"--cached", "--others", "--exclude-standard", "--", "*-test.sh")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("listing the suites: %v", err)
	}
	var suites []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			suites = append(suites, name)
		}
	}
	return suites
}
