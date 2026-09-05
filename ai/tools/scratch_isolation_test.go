// The gate runs the shell lane beside `gotest`, and a suite on a fixed path would race whatever else
// reaches for that path — a sibling if the lane is ever widened, or one of the Go suites now. The
// failure would read as a flaky case rather than as a collision.
//
// `mktemp -d` is what gives each run a directory nothing else can name. This case holds every suite to
// it, so nothing in this tree depends on being the only thing running.
//
// Sourcing counts: the two bootstrap suites take their scratch from lib/test-harness.sh, which mktemps
// once for whichever suite sourced it. A suite that needs no scratch at all says so in a line this
// reads, rather than being silently exempt — an exemption nobody has to state is one that spreads.
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

// Whether a suite's text shows it working in scratch of its own — directly, through a harness it
// sources, or by declaring it needs none.
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

// The three shapes that pass and the one that does not. Without this the case above can only fail on
// a real suite going wrong, so nothing proves it would notice.
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
// included. Split on whitespace instead, a name holding a space arrives as two names and this scan
// reads two files that do not exist, while the suite it was about goes unchecked. That is the same
// hazard gate/units.go carries a mutant for, and the claim above is only true while both spell the
// command the same way.
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
