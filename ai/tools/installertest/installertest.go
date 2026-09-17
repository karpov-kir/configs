// Package installertest is the bounded tree the installer suites build their fixtures in.
//
// It exists because a fixture for an installer writes the same things the installer does — files,
// directories and symlinks under a home and a checkout — and one of those writes once followed a live
// symlink out of the case's own tree and overwrote real config files in the working tree. Every suite
// grew the same guard afterwards, and four copies of a guard are four chances for one of them to
// guard something slightly different.
//
// The bound is the production one, by construction rather than by resemblance. `installer.tree`
// refuses a write whose nearest existing parent is not within its write root; Writer refuses a
// fixture write by calling the same two functions, shell.NearestExistingParent and shell.IsWithin.
// The copies this replaces climbed with filepath.EvalSymlinks instead, which stops at a regular file
// — a name no write can ever land under — so a fixture stopping there judged a write by a boundary
// the code under test does not have.
//
// Every refusal here is a GUARD, not a result: it says the case was about to write somewhere it never
// meant to, and it fails the case on the spot rather than letting the write happen and the assertion
// pass.
package installertest

import (
	"os"
	"path/filepath"
	"testing"

	"configs/ai/tools/shell"
)

// Writer builds a case's fixture inside one tree and refuses anything that would land outside it.
type Writer struct {
	t    *testing.T
	base string
}

// New is a writer over a temp directory of the case's own. Physical, because t.TempDir hands back
// /var/folders/… on macOS while /var is itself a symlink to /private/var — and a bound that is not
// resolved refuses every write made through a resolved path, which is a guard that always fires and
// therefore a guard somebody deletes.
func New(t *testing.T) *Writer {
	t.Helper()
	return &Writer{t: t, base: Physical(t, t.TempDir())}
}

// Base is the tree this writer bounds, which is also what a case hands the run under test as its own
// write root: the two bounds are the same tree, so a run that escapes and a fixture that escapes are
// caught by the same line.
func (w *Writer) Base() string { return w.base }

// ContainedParent refuses a path whose nearest existing parent is outside this tree.
//
// The parent rather than the path: the write below creates the missing directories under it, so that
// ancestor is the deepest thing a symlink could still redirect. It says nothing about the LAST
// component, which is why every writer here asks RefuseExistingSymlink as well.
func (w *Writer) ContainedParent(path string) {
	w.t.Helper()
	parent := shell.NearestExistingParent(path)
	if parent == "" {
		w.t.Fatalf("refusing to write %s — no directory above it resolves\n"+
			"this is the containment guard, not a failing case", path)
	}
	if !shell.IsWithin(parent, w.base) {
		w.t.Fatalf("refusing to write %s — its nearest existing parent resolves to %s, outside %s\n"+
			"this is the containment guard, not a failing case", path, parent, w.base)
	}
}

// RefuseExistingSymlink turns away a write at a name that is already a link. A write follows a
// symlink, and the links these cases produce point into a checkout — a run leaves $home/.zshrc
// pointing at $repo/zsh/.zshrc, and a fixture write at that path afterwards lands in the real file.
// That is the one door ContainedParent does not cover.
func (w *Writer) RefuseExistingSymlink(path string) {
	w.t.Helper()
	if value, err := os.Readlink(path); err == nil {
		w.t.Fatalf("refusing to write %s — it already exists as a symlink to %s\n"+
			"this is the containment guard, not a failing case", path, value)
	}
}

func (w *Writer) MkdirAll(dir string) {
	w.t.Helper()
	w.ContainedParent(dir)
	// The parent being contained says nothing about dir itself, and MkdirAll follows a symlink there
	// the same way a file write does.
	w.RefuseExistingSymlink(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		w.t.Fatalf("the fixture could not create %s: %v", dir, err)
	}
}

func (w *Writer) Write(path, body string) {
	w.t.Helper()
	w.ContainedParent(path)
	w.RefuseExistingSymlink(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.t.Fatalf("the fixture could not create the parent of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		w.t.Fatalf("the fixture could not write %s: %v", path, err)
	}
}

// Symlink refuses a name already taken by a link rather than forcing past it: `ln -s X Y` where Y is
// already a symlink to a directory creates the link INSIDE Y, which is how a stray link ends up in a
// checkout. Every fixture link is meant to be the first thing at its path.
func (w *Writer) Symlink(source, target string) {
	w.t.Helper()
	w.ContainedParent(target)
	w.RefuseExistingSymlink(target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		w.t.Fatalf("the fixture could not create the parent of %s: %v", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		w.t.Fatalf("the fixture could not link %s at %s: %v", target, source, err)
	}
}

func (w *Writer) RemoveAll(path string) {
	w.t.Helper()
	w.ContainedParent(path)
	if err := os.RemoveAll(path); err != nil {
		w.t.Fatalf("the fixture could not remove %s: %v", path, err)
	}
}

// Physical resolves a directory the way every bound here is resolved. Exported for a case that has to
// resolve a second tree of its own — a linked worktree, a checkout the run is pointed at.
func Physical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("the case's own temp directory does not resolve, so nothing could be contained: %v", err)
	}
	return resolved
}
