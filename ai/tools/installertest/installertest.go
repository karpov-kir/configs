// A fixture for an installer writes the same things the installer does, files, directories and
// symlinks under a home and a checkout. One of those writes once followed a live symlink out of the
// case's own tree and overwrote real config files in the working tree. Every suite grew the same
// guard afterwards, and four copies of a guard are four chances for one to guard something slightly
// different.

// The bound is the production one, by construction. `installer.tree` refuses a write whose nearest
// existing parent falls outside its write root. Writer refuses a fixture write by calling the same
// two functions, shell.NearestExistingParent and shell.IsWithin.

// The copies this replaces walked up with filepath.EvalSymlinks instead, and that stops at a regular
// file, a name under which no write can land. A fixture stopping there judged a write by a boundary
// the code under test lacks.

// Every refusal here is a GUARD. It says the case was about to write somewhere it never meant to,
// and it fails the case on the spot. A result would let the write happen and the assertion pass.

// Package installertest is the bounded tree the installer suites build their fixtures in.
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

// New is a writer over a temp directory of the case's own. The path is resolved physically, because
// t.TempDir hands back /var/folders/… on macOS and /var is itself a symlink to /private/var. An
// unresolved bound refuses every write made through a resolved path, and a guard that always fires
// is a guard somebody deletes.
func New(t *testing.T) *Writer {
	t.Helper()
	return &Writer{t: t, base: Physical(t, t.TempDir())}
}

// Base is the tree this writer bounds. A case hands the same tree to the run under test as its own
// write root. A run that escapes and a fixture that escapes are then caught by the same line.
func (w *Writer) Base() string { return w.base }

// The parent is what gets checked, because each writer creates the missing directories under it.
// That ancestor is the deepest point a symlink can still redirect. The check leaves the LAST
// component unexamined, and that is why every writer here asks RefuseExistingSymlink as well.

// ContainedParent refuses a path whose nearest existing parent is outside this tree.
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
// symlink, and the links these cases produce point into a checkout. A run leaves $home/.zshrc
// pointing at $repo/zsh/.zshrc, and a fixture write at that path afterwards lands in the real file.
// That is the single door ContainedParent leaves open.
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
	// A contained parent leaves dir itself unexamined, and MkdirAll follows a symlink there the same
	// way a file write does.
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

// Symlink refuses a name already taken by a link instead of forcing past it. `ln -s X Y` with Y
// already a symlink to a directory creates the link INSIDE Y, and that is how a stray link ends up
// in a checkout. Every fixture link is meant to be the first thing at its path.
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

// Physical resolves a directory the way every bound here is resolved. It is exported for a case with
// a second tree of its own to resolve, such as a linked worktree or a checkout the run is pointed
// at.
func Physical(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("the case's own temp directory does not resolve, so nothing could be contained: %v", err)
	}
	return resolved
}
