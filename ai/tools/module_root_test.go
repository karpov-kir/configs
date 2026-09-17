// Where go.mod sits decides which reads this repository's suites are keyed on, and nothing else in the
// checkout would notice it moving.
//
// Go hashes every file a case opens inside the module root into that package's test cache entry, and
// skips what lies above it. With go.mod at `ai/tools` the suites that read the checkout — the whole of
// this package — answered `ok (cached)` over a tree that had moved, and the gate bought its way out by
// forcing one package with `-count=1` on every run. At the repository root nothing a case can open is
// above it, so the forcing is gone and a stale green has nowhere left to come from.
//
// That property is invisible while it holds: move the module file down again, or carve a second module
// out of a subdirectory, and every suite still passes — it just stops noticing. This is the case that
// notices.
package tools_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Directories with no Go source in them, skipped so the walk does not stat a repository's worth of git
// objects and build output on every run.
var notWalked = map[string]bool{".git": true, "bin": true, "dist": true, "node_modules": true}

func TestTheModuleRootIsTheRepositoryRoot(t *testing.T) {
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Fatalf("no go.mod at the repository root: %v. Every suite in this module is then keyed on a "+
			"subtree, and each one reading a file above that subtree answers `ok (cached)` over a checkout "+
			"that has moved — silently, with the whole gate green.", err)
	}

	var extra []string
	err := filepath.WalkDir(repoRoot, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if notWalked[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		relative, err := filepath.Rel(repoRoot, name)
		if err != nil {
			return err
		}
		if relative != "go.mod" {
			extra = append(extra, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s for module files: %v — nothing was measured", repoRoot, err)
	}

	if len(extra) > 0 {
		t.Errorf("go.mod also at %s. A module file below the root takes everything under it into a module "+
			"of its own, and those packages stop being keyed on anything above it — which is the stale green "+
			"this layout exists to end. `ai/kk-flavor/standards/testing.md` rule 11 states the rule.",
			strings.Join(extra, ", "))
	}
}
