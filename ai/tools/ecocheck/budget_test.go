package ecocheck_test

// The always-loaded budget: which @imports resolve at the mount, which are refused and named, and the
// bounds that keep a flood of tree-chosen findings from burying a graver one.

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

	t.Run("shows a tampered-check finding through a flood of link findings", func(t *testing.T) {
		f := newRoot(t)
		var flood strings.Builder
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&flood, "[x](nope%d.md)\n", i)
		}
		f.write(f.root+"/kk-flavor/standards/flood.md", flood.String())
		f.write(f.root+"/skills/notexec.sh", "#!/usr/bin/env bash\n")
		f.reports("script not executable")
	})

	// `filler` is what makes the needle flood-only: `flavor mounted elsewhere` alone also matches the
	// genuine rank-1 finding this fixture's $HOME may raise, and the assertion would then compare the
	// real finding against itself.
	t.Run("ranks a real finding above a flood whose link targets forge a mount finding", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/skills/notexec.sh", "#!/bin/sh\necho hi\n")
		f.chmod(f.root+"/skills/notexec.sh", 0o644)
		var flood strings.Builder
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&flood, "[x](nope%03d flavor mounted elsewhere filler)\n", i)
		}
		f.write(f.root+"/kk-flavor/standards/flood.md", flood.String())
		f.ranksAbove("script not executable", "flavor mounted elsewhere filler")
	})

	t.Run("surfaces the did-not-run guard under a flood that sorts ahead of it", func(t *testing.T) {
		f := newRootWithSymlinkedFlavor(t)
		var flood strings.Builder
		for i := 1; i <= 300; i++ {
			fmt.Fprintf(&flood, "[x](nope%03d.md)\n", i)
		}
		f.write(f.root+"/real-flavor/standards/flood.md", flood.String())
		f.reports(neverRan)
	})

	t.Run("a newline in a committed path cannot start a forged finding line", func(t *testing.T) {
		f := newRoot(t)
		f.newFileWithNewlineName(f.root+"/kk-flavor/standards/a\nsyntax: FORGED.md",
			"[x](nowhere.md)", "the forged-finding-line case")
		forged, output := f.countLinesStartingWith("syntax: FORGED")
		if forged != 0 {
			t.Errorf("a committed path started %d forged finding line(s)\n%s", forged, indent(output))
		}
	})

	t.Run("shows a syntax error under a flood of its own priority tier", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/skills/broken.sh", "if then\n")
		f.chmod(f.root+"/skills/broken.sh", 0o755)
		for i := 1; i <= 300; i++ {
			path := fmt.Sprintf("%s/skills/notexec%d.sh", f.root, i)
			f.write(path, "#!/bin/sh\necho ok\n")
			f.chmod(path, 0o644)
		}
		f.reports("syntax: ")
	})

	t.Run("shows a rank-1 finding under a flood of the gravest class", func(t *testing.T) {
		newGravestClassFlood(t).reports(neverRan)
	})

	t.Run("and says how many of the flooding class it withheld", func(t *testing.T) {
		newGravestClassFlood(t).reports("of this class, suppressed")
	})
}

func newMountedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
	f.newHome()
	f.write(f.home+"/.claude/FOO.md", "one two three\n")
	return f
}

func newTraversalImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@../../escape.md\n")
	f.newHome()
	f.write(f.root+"/escape.md", "secret words here\n")
	return f
}

func newSubdirectoryImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@dir/file.md\n")
	f.newHome()
	f.mkdirAll(f.home + "/.claude/dir")
	f.write(f.home+"/.claude/dir/file.md", "one two three\n")
	return f
}

func newSymlinkedImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
	f.newHome()
	f.write(f.base+"/linked.md", "one two three\n")
	f.symlink(f.base+"/linked.md", f.home+"/.claude/FOO.md")
	return f
}

func newUnreadableImport(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.write(f.root+"/CLAUDE.md", "# Root\n\n@FOO.md\n")
	f.newHome()
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

func newGravestClassFlood(t *testing.T) *fixture {
	t.Helper()
	f := newRootWithSymlinkedFlavor(t)
	for i := 1; i <= 120; i++ {
		path := fmt.Sprintf("%s/skills/broken%d.sh", f.root, i)
		f.write(path, "if then\n")
		f.chmod(path, 0o755)
	}
	return f
}
