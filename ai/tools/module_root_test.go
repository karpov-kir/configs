// Go hashes every file a case opens inside the module root into that package's test cache entry, and
// skips what lies above the root. Where go.mod sits therefore decides which reads this repository's
// suites are keyed on.

// With go.mod at `ai/tools`, the suites in this package read the checkout and answered `ok (cached)`
// over a tree that had moved. The gate worked around it by forcing one package with `-count=1` on
// every run. At the repository root every file a case can open is inside the module, so the forcing
// is gone and a stale green cannot happen.

// The property is invisible while it holds. Move the module file down again, or carve a second module
// out of a subdirectory, and every suite still passes, having stopped noticing. This is the case that
// notices.
package tools_test

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheModuleRootIsTheRepositoryRoot(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Fatalf("no go.mod at the repository root: %v. Every suite in this module is then keyed on a "+
			"subtree, and each one reading a file above that subtree answers `ok (cached)` over a checkout "+
			"that has moved — silently, with the whole gate green.", err)
	}

	var extra []string
	for _, tracked := range trackedFiles(t) {
		if path.Base(tracked) == "go.mod" && tracked != "go.mod" {
			extra = append(extra, tracked)
		}
	}

	if len(extra) > 0 {
		t.Errorf("go.mod also at %s. A module file below the root takes everything under it into a module "+
			"of its own, and those packages stop being keyed on anything above it — which is the stale green "+
			"this layout exists to end. `ai/kk-flavor/standards/testing.md` rule 11 states the rule.",
			strings.Join(extra, ", "))
	}
}

// A developer's worktrees sit under this checkout and each carries a go.mod, and those are outside
// this repository's declarations. `-z`, because git C-quotes a path holding a quote or a non-ASCII
// byte, and a quoted name reaches no file.

// Lists every file this repository tracks, by repo-relative path. The listing reads the index, since
// the question here is what the repository DECLARES as a module, and an untracked go.mod is outside
// that.
func trackedFiles(t *testing.T) []string {
	t.Helper()
	listed, err := exec.Command("git", "-C", repoRoot, "ls-files", "--cached", "-z").Output()
	if err != nil {
		t.Fatalf("asking git what %s tracks: %v — nothing was measured", repoRoot, err)
	}
	files := strings.Split(strings.TrimSuffix(string(listed), "\x00"), "\x00")
	if len(files) < 2 {
		t.Fatalf("git lists %d tracked file(s) under %s, so this case asserts nothing — either the listing "+
			"is reaching the wrong tree, or it is not being read", len(files), repoRoot)
	}
	return files
}
