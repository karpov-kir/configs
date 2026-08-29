package ecocheck_test

// What the report does when the tree floods it: the gravest finding still has to reach the screen,
// and a name the tree chose must not be able to write a line of its own.

import (
	"fmt"
	"testing"
)

func TestTheGravestFindingSurvivesAFlood(t *testing.T) {
	t.Run("shows a tampered-check finding through a flood of link findings", func(t *testing.T) {
		f := newRoot(t)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%d.md)")
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
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d flavor mounted elsewhere filler)")
		f.ranksAbove("script not executable", "flavor mounted elsewhere filler")
	})

	t.Run("surfaces the did-not-run guard under a flood that sorts ahead of it", func(t *testing.T) {
		f := newRootWithSymlinkedFlavor(t)
		f.floodWithLinks(f.root+"/real-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
		f.reports(neverRan)
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

// A committed filename holding a newline reaches the report as text. Printed raw it opens a line that
// reads exactly like one of the report's own findings.
func TestACommittedPathCannotForgeAFindingLine(t *testing.T) {
	f := newRoot(t)
	f.newFileWithNewlineName(f.root+"/kk-flavor/standards/a\nsyntax: FORGED.md",
		"[x](nowhere.md)", "the forged-finding-line case")
	forged, output := f.countLinesStartingWith("syntax: FORGED")
	if forged != 0 {
		t.Errorf("a committed path started %d forged finding line(s)\n%s", forged, indent(output))
	}
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
