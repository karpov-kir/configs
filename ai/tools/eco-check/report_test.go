package ecocheck_test

// What the report does when the tree floods it: the gravest finding still has to reach the screen,
// and a name the tree chose must not be able to write a line of its own.

import (
	"fmt"
	"strings"
	"testing"

	"kk-flavor/tools/shell"
)

func TestTheGravestFindingSurvivesAFlood(t *testing.T) {
	t.Run("shows a tampered-check finding through a flood of link findings", func(t *testing.T) {
		f := newRoot(t)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%d.md)")
		f.write(f.root+"/skills/notexec.sh", "#!/usr/bin/env bash\n")
		f.reports("script not executable")
	})

	// The forged text has to be a prefix the rank table still carries, or the case asks nothing: a
	// checker ranking on the whole line instead of its head promotes only a finding whose text holds a
	// *ranked* phrase. `flavor mounted elsewhere` was that phrase until the gate subsumed the
	// flavor-mount comparison, and this case went on passing over a checker that ranks on Contains.
	//
	// `filler` is what makes the needle flood-only: `skill mounted elsewhere` alone also matches the
	// genuine rank-1 finding this fixture's $HOME may raise, and the assertion would then compare the
	// real finding against itself.
	t.Run("ranks a real finding above a flood whose link targets forge a mount finding", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/skills/notexec.sh", "#!/bin/sh\necho hi\n")
		f.chmod(f.root+"/skills/notexec.sh", 0o644)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d skill mounted elsewhere filler)")
		f.ranksAbove("script not executable", "skill mounted elsewhere filler")
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

	// A file past the read bound is the plainest member of the tampered-check class — its own finding
	// ends on "it was NOT checked" — and it went unranked, so it shared one budget with `dangling
	// link:` and sorted below every one of them. 300 crafted links then hid an unread 8 MiB file
	// completely: the report named no such file at all.
	t.Run("shows a file past the read bound under a flood of link findings", func(t *testing.T) {
		newOversizeUnderAFlood(t).reports("file too large to scan")
	})

	// Presence alone would pass again the day the rank is dropped and the flood happens to be one
	// line short of the cap, so the ordering is what this pins.
	t.Run("and ranks it above that flood rather than inside it", func(t *testing.T) {
		newOversizeUnderAFlood(t).ranksAbove("file too large to scan", "dangling link: ")
	})
}

// One file the read bound refuses, buried under more crafted links than the per-rank cap will show.
func newOversizeUnderAFlood(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
	f.writeOversize(f.root + "/kk-flavor/standards/huge.md")
	return f
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

// The last cut any finding takes, and so the one that decides what a reader actually sees. Every
// message upstream of it that quotes a name the tree chose now marks its own cut; this bound runs
// after all of them, so cutting here without a word puts a shorter wrong line back on the screen and
// takes the upstream mark off with the tail it removes.
//
// It has already cost this suite: two cases in budget_test.go match on a finding's head, and only one
// of them does so because its line really runs past this bound. A cut nothing announces is one every
// reader has to re-derive, and each re-derivation is another chance to get it wrong.
//
// A Read-always target that is absent is the shape that reaches this bound, because its finding
// quotes the listed name twice and caps neither.
func TestACutFindingLineSaysThatItWasCut(t *testing.T) {
	t.Run("marks a finding line the width bound cut", func(t *testing.T) {
		// Two segments rather than one long name: a single path component past 255 bytes comes back
		// ENAMETOOLONG, which is a different arm of the budget scan and a line short enough to fit.
		listed := "standards/" + strings.Repeat("z", 235) + "/" + strings.Repeat("z", 250) + ".md"
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/inject.md", "# Flavor\n\n## Read always\n\n- [a]("+listed+")\n")

		// The whole line, and pinned at both ends, rather than the marker alone: other findings on
		// this run mark their own cuts at their own bounds, so "a marker is somewhere in the output"
		// is satisfied whatever this bound does. The head is 17 bytes and the name reaches 480 more
		// before the 497 the marker leaves runs out — so the cut lands inside the name's second
		// segment, ahead of anything t.TempDir chose the length of.
		f.reports("\ninject.md lists 'standards/" + strings.Repeat("z", 235) + "/" +
			strings.Repeat("z", 234) + shell.CutMarker + "\n")
	})

	// The other direction. A mark on a line that was never cut says the line has a tail it does not
	// have, and a finding whose own text ends the line is the commonest thing in this report — so the
	// short line has to arrive whole, with its real ending and no marker.
	t.Run("and leaves a line that fits whole, ending where the finding does", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/kk-flavor/inject.md",
			"# Flavor\n\n## Read always\n\n- [a](standards/nowhere.md)\n")
		f.reports(" does not exist\n")
	})
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
