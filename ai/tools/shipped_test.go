// The release workflow names the tools it builds. Nothing else does, so nothing else could notice a
// tool added here and not added there. That failure surfaces as a machine with no Go toolchain
// getting exit 2 from a skill, long after the release it was missing from.
package tools_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Built here rather than shipped, so a release does not carry them. go-mutate rewrites this module's
// own source and runs its suites; it is run from a checkout, by someone changing the code.
var developerOnly = map[string]bool{"go-mutate": true}

const workflow = "../../.github/workflows/release-tools.yml"

func TestEveryShippedToolIsInTheReleaseWorkflow(t *testing.T) {
	built := mainPackages(t)
	if len(built) == 0 {
		t.Fatal("found no main packages, so this case would pass against any workflow at all")
	}

	listed := shippedInWorkflow(t)
	if len(listed) == 0 {
		t.Fatalf("no SHIPPED list found in %s — the release would attach nothing", workflow)
	}

	for _, tool := range built {
		if !listed[tool] {
			t.Errorf("%s is a main package here but not in %s — a release would not carry it, and every "+
				"machine without Go would get exit 2 reaching for it", tool, workflow)
		}
		delete(listed, tool)
	}
	for tool := range listed {
		t.Errorf("%s is built by %s but is not a main package here — the release step would fail on it",
			tool, workflow)
	}
}

// The tool names a release should carry: one per directory holding a `package main`, named by the
// directory, or by its parent where main sits in a cmd/ subdirectory so the package beside it can be
// driven in-process by its suite. The walk itself is gitignore_test.go's, which needs the same
// directories under a different name.
func mainPackages(t *testing.T) []string {
	t.Helper()
	found := map[string]bool{}
	for _, dir := range mainPackageDirs(t) {
		if filepath.Base(dir) == "cmd" {
			dir = filepath.Dir(dir)
		}
		if name := filepath.Base(dir); !developerOnly[name] {
			found[name] = true
		}
	}
	var names []string
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func shippedInWorkflow(t *testing.T) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("reading %s: %v", workflow, err)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(trimmed, "SHIPPED:")
		if !ok {
			continue
		}
		for _, name := range strings.Fields(rest) {
			listed[name] = true
		}
	}
	return listed
}
