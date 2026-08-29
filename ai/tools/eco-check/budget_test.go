package ecocheck_test

// The always-loaded budget: which @imports resolve at the mount, and which are refused and named.

import (
	"fmt"
	"strings"
	"testing"
)

func TestImportResolvedAtTheMount(t *testing.T) {
	t.Run("counts an import the mount really holds", func(t *testing.T) {
		newMountedImport(t).doesNotReport(uncounted)
	})

	t.Run("and folds it into the budget's file count", func(t *testing.T) {
		newMountedImport(t).reports("across 3 files")
	})

	// The pattern requires a non-word character before the `@`, and that character is a rune, not a
	// byte. Cutting a fixed two carried the tail of a multi-byte one into the name: `é@alpha.md`
	// resolved as `@alpha.md`, which is nowhere, and landed on the census line instead.
	t.Run("counts an import behind a two-byte boundary rune", func(t *testing.T) {
		newMultiByteBoundaryImport(t).doesNotReport(uncounted)
	})

	t.Run("and one behind a three-byte em dash, folding both into the file count", func(t *testing.T) {
		newMultiByteBoundaryImport(t).reports("across 4 files")
	})

	t.Run("refuses to resolve when this checkout is not the installed one", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
		f.newHomeWithoutFlavorMount()
		f.write(f.home+"/.claude/FOO.md", "one two three\n")
		f.reports(uncounted)
	})

	// The file has to sit exactly where the traversal lands. Put it anywhere else and the case passes
	// on "no such file", proving nothing.
	t.Run("refuses a name that is a path rather than a bare filename", func(t *testing.T) {
		newTraversalImport(t).reports(uncounted)
	})

	t.Run("and reports that refusal instead of leaving it to read as drift", func(t *testing.T) {
		newTraversalImport(t).reports(refused)
	})

	t.Run("leaves a plain subdirectory import uncounted", func(t *testing.T) {
		newSubdirectoryImport(t).reports(uncounted)
	})

	t.Run("and does not report it as a probe", func(t *testing.T) {
		newSubdirectoryImport(t).doesNotReport(refused)
	})

	t.Run("refuses a symlink at the mount", func(t *testing.T) {
		newSymlinkedImport(t).reports(uncounted)
	})

	t.Run("and reports that one too, the shape nothing legitimate produces", func(t *testing.T) {
		newSymlinkedImport(t).reports(refused)
	})

	t.Run("refuses a file at the mount it cannot read", func(t *testing.T) {
		newUnreadableImport(t).reports(uncounted)
	})

	t.Run("and reports it, the shape that hid a short figure one tier down", func(t *testing.T) {
		newUnreadableImport(t).reports(refused)
	})

	t.Run("refuses a kk-flavor symlinked to the install, which would open the gate", func(t *testing.T) {
		f := newRootWithSymlinkedFlavor(t)
		f.newHomeWithoutFlavorMount()
		f.symlink(f.root+"/real-flavor", f.home+"/.kk-flavor")
		f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
		f.write(f.home+"/.claude/FOO.md", "one two three\n")
		f.reports(uncounted)
	})

	t.Run("refuses an import its carrier does not put at this mount", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n")
		f.appendTo(f.root+"/kk-flavor/inject.md", "@BAR.md\n")
		f.newHome()
		f.write(f.home+"/.claude/BAR.md", "one two three four five\n")
		f.reports(uncounted)
	})

	t.Run("treats a fenced mention in CLAUDE.md as prose, not as its import", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\n```\n@FOO.md\n```\n")
		f.appendTo(f.root+"/kk-flavor/inject.md", "@FOO.md\n")
		f.newHome()
		f.write(f.home+"/.claude/FOO.md", "one two three four five\n")
		f.reports(uncounted)
	})

	t.Run("treats a backticked mention in CLAUDE.md the same way", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/CLAUDE.md", "# Root\n\nnever write `@FOO.md` here\n")
		f.appendTo(f.root+"/kk-flavor/inject.md", "@FOO.md\n")
		f.newHome()
		f.write(f.home+"/.claude/FOO.md", "one two three four five\n")
		f.reports(uncounted)
	})

	t.Run("caps resolution attempts and names what it skipped", func(t *testing.T) {
		f := newRoot(t)
		f.newHome()
		var claudeMd strings.Builder
		claudeMd.WriteString("# Root\n\n")
		for i := 1; i <= 65; i++ {
			fmt.Fprintf(&claudeMd, "@f%d.md\n", i)
			f.write(fmt.Sprintf("%s/.claude/f%d.md", f.home, i), "one\n")
		}
		f.write(f.root+"/CLAUDE.md", claudeMd.String())
		f.reports(uncounted)
	})

	// Byte-order sorting is what fixes the two positions these cases turn on: `../x.md` first, because
	// `.` sorts below every letter, and `d01.md` 65th — one past the 64-attempt cap, with the 63 `b`
	// names filling it. Under a sort that puts punctuation away, `../x.md` itself lands past the cap
	// and both asserts flip.
	t.Run("reports the probe-shaped name the resolver did look at", func(t *testing.T) {
		newCappedImportSpread(t).reports("not counted: ../x.md")
	})

	t.Run("and carries no reason over to a name past the cap", func(t *testing.T) {
		newCappedImportSpread(t).doesNotReport("not counted: d01.md")
	})
}

// A root whose CLAUDE.md imports one name, plus the home its mount resolves at. What each case below
// varies is only what it puts at that mount.
func newRootImporting(t *testing.T, imported string) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@"+imported+"\n")
	f.newHome()
	return f
}

func newMountedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "FOO.md")
	f.write(f.home+"/.claude/FOO.md", "one two three\n")
	return f
}

// Two imports whose boundary character is not one byte: `é` is two and `—` is three.
func newMultiByteBoundaryImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\nsee é@alpha.md and —@beta.md\n")
	f.newHome()
	f.write(f.home+"/.claude/alpha.md", "one two three\n")
	f.write(f.home+"/.claude/beta.md", "four five six\n")
	return f
}

func newTraversalImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "../../escape.md")
	f.write(f.root+"/escape.md", "secret words here\n")
	return f
}

func newSubdirectoryImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "dir/file.md")
	f.mkdirAll(f.home + "/.claude/dir")
	f.write(f.home+"/.claude/dir/file.md", "one two three\n")
	return f
}

func newSymlinkedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRootImporting(t, "FOO.md")
	f.write(f.base+"/linked.md", "one two three\n")
	f.symlink(f.base+"/linked.md", f.home+"/.claude/FOO.md")
	return f
}

// The `isReadable` limb of the import refusal: a file at the mount this process cannot open. Root
// reads a mode-000 file regardless of the mode, and nothing else builds this condition — every other
// way of making the read fail (a missing path, a directory, a symlink) is refused by an earlier limb
// and never reaches this one. So the case declines rather than asserting against a file the tool
// reads happily.
func newUnreadableImport(t *testing.T) *fixture {
	t.Helper()
	if !modeDeniesRead(t) {
		t.Skip("this process reads a mode-000 file regardless of the mode (root, or CAP_DAC_OVERRIDE), so an unreadable file at the mount cannot be built here")
	}
	f := newRootImporting(t, "FOO.md")
	f.write(f.home+"/.claude/FOO.md", "one two three\n")
	f.chmod(f.home+"/.claude/FOO.md", 0o000)
	return f
}

func newCappedImportSpread(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.newHome()
	var claudeMd strings.Builder
	claudeMd.WriteString("# Root\n\n")
	for i := 1; i <= 63; i++ {
		fmt.Fprintf(&claudeMd, "@b%02d.md\n", i)
		f.write(fmt.Sprintf("%s/.claude/b%02d.md", f.home, i), "one\n")
	}
	claudeMd.WriteString("@../x.md\n@d01.md\n")
	f.write(f.root+"/CLAUDE.md", claudeMd.String())
	f.write(f.home+"/.claude/d01.md", "one\n")
	return f
}
