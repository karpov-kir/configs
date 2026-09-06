package ecocheck_test

// What the report does when the tree floods it: the gravest finding still has to reach the screen,
// and a name the tree chose must not be able to write a line of its own.

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	ecocheck "kk-flavor/tools/eco-check"
	"kk-flavor/tools/shell"
)

func TestTheGravestFindingSurvivesAFlood(t *testing.T) {
	t.Run("shows a tampered-check finding through a flood of link findings", func(t *testing.T) {
		f := newRoot(t)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%d.md)")
		f.write(f.root+"/skills/notexec.sh", "#!/usr/bin/env bash\n")
		f.reports(ecocheck.ScriptNotExecutable)
	})

	// The forged text has to be a prefix the rank table still carries, or the case asks nothing: a
	// checker ranking on the whole line instead of its head promotes only a finding whose text holds a
	// *ranked* phrase. A phrase the table stops carrying leaves this case green over such a checker.
	//
	// `filler` is what makes the needle flood-only: `skill mounted elsewhere` alone also matches the
	// genuine rank-1 finding this fixture's $HOME may raise, and the assertion would then compare the
	// real finding against itself.
	t.Run("ranks a real finding above a flood whose link targets forge a mount finding", func(t *testing.T) {
		f := newRoot(t)
		f.write(f.root+"/skills/notexec.sh", "#!/bin/sh\necho hi\n")
		f.chmod(f.root+"/skills/notexec.sh", 0o644)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300,
			"[x](nope%03d "+ecocheck.SkillMountedElsewhere+" filler)")
		f.ranksAbove(ecocheck.ScriptNotExecutable, ecocheck.SkillMountedElsewhere+" filler")
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
		f.reports(ecocheck.SyntaxError)
	})

	t.Run("shows a rank-1 finding under a flood of the gravest class", func(t *testing.T) {
		newGravestClassFlood(t).reports(neverRan)
	})

	t.Run("and says how many of the flooding class it withheld", func(t *testing.T) {
		newGravestClassFlood(t).reports(ecocheck.SuppressedMarker)
	})

	// The floor inside rank 5, the rank a branch under review picks the contents of. Without it, the
	// class byte order puts first spends the whole budget and every other kind there prints nothing.
	t.Run("shows every other rank-5 kind through a flood of one of them", func(t *testing.T) {
		newFloodedRankFive(t).reports(ecocheck.DanglingLink, dangling, unresolved, noPosition,
			unfamiliedSkill, ecocheck.SkillDirWithoutSkillFile, ecocheck.SkillWithoutDescription)
	})

	// The count is the flooding class's own: 45 raised, 33 shown once the other seven classes have
	// taken their floor line. Subtracting the rank's cap instead prints 5, which every presence case
	// above stays green over.
	t.Run("and closes that class with a count of its own withheld findings", func(t *testing.T) {
		newFloodedRankFive(t).reports("… and 12 " + ecocheck.SuppressedMarker)
	})

	// The second note on the same tree, over the class the floor left one line short of its two
	// findings. One note per class, each counting itself. A single note for the rank would say 13 on
	// both lines.
	t.Run("and gives a second flooded class in that rank its own count", func(t *testing.T) {
		newFloodedRankFive(t).reports("… and 1 " + ecocheck.SuppressedMarker)
	})

	// Exact, so the arithmetic is checkable: 54 findings, of which 41 print. That is the 40 rank 5's
	// budget allows, plus the rank-3 mismatch taking one from its own. The two notes print beside
	// them and are counted as neither.
	t.Run("and reports the whole remainder on the trailing line", func(t *testing.T) {
		newFloodedRankFive(t).reports("… 13 " + ecocheck.UnshownMarker)
	})

	// The remainder that is entirely the kind on screen: the note has to be right in that shape too.
	// Both lines say 60 here and only their wording tells them apart, so the case pins both.
	t.Run("and says as much on the class's own line when it is the only kind in the rank", func(t *testing.T) {
		f := newRoot(t)
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 100, "[x](nope%03d.md)")
		f.reports("… and 60 "+ecocheck.SuppressedMarker, "… 60 "+ecocheck.UnshownMarker)
	})

	// The number, not the presence of the line. The count has to be the class's own: subtracting the
	// rank's cap instead prints a plausible number for the flooding class and `-37` for this one, which
	// is why every case above stays green over it.
	t.Run("and counts a floored class's withheld findings against that class alone", func(t *testing.T) {
		newDriftUnderAFloodOfItsRank(t).reports("… and 2 " + ecocheck.SuppressedMarker)
	})

	// Tell a note from a finding by its printed text, and a committed path quoting the marker gets
	// subtracted from the count of what the reader was just shown. The report then claims it withheld
	// a finding on a tree that withheld none.
	//
	// The filename has to hold the marker whole or this case observes nothing, so build it from the
	// constant rather than copying the wording.
	t.Run("does not let a committed path pass itself off as a suppression note", func(t *testing.T) {
		f := newRoot(t)
		path := f.root + "/skills/" + ecocheck.SuppressedMarker + ".sh"
		f.write(path, "#!/bin/sh\necho hi\n")
		f.chmod(path, 0o644)
		f.doesNotReport(ecocheck.UnshownMarker)
	})

	// A file past the read bound is the plainest member of the tampered-check class — its own finding
	// ends on "it was NOT checked" — and it went unranked, so it shared one budget with `dangling
	// link:` and sorted below every one of them. 300 crafted links then hid an unread 8 MiB file
	// completely: the report named no such file at all.
	t.Run("shows a file past the read bound under a flood of link findings", func(t *testing.T) {
		newOversizeUnderAFlood(t).reports(ecocheck.FileTooLargeToScan)
	})

	// Presence alone would pass again the day the rank is dropped and the flood happens to be one
	// line short of the cap, so the ordering is what this pins.
	t.Run("and ranks it above that flood rather than inside it", func(t *testing.T) {
		newOversizeUnderAFlood(t).ranksAbove(ecocheck.FileTooLargeToScan, ecocheck.DanglingLink)
	})

	// The same property for a mount that outlived its skill, and here ordering is the only thing left
	// that can observe it. Every kind the table names has left the catch-all. A finding whose row is
	// dropped is then alone in its class, and the floor shows it whatever its rank, so presence passes
	// over exactly the mutation this case exists to catch. Where the line lands is what still moves.
	t.Run("ranks a mount that outlived its skill above a flood of link findings", func(t *testing.T) {
		f := newInstalledRoot(t)
		f.newMountPointingAt("idsd-gone", f.root+"/skills/idsd-gone")
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
		f.ranksAbove(ecocheck.MountWithoutASkill, ecocheck.DanglingLink)
	})

	// The same tally, on the tree that does carry notes — two of them, one per class the floor left
	// short. The rank-5 tree above cannot show this: it prints no note at all, so nothing there says
	// whether a note is still kept out of the count of findings shown.
	t.Run("and counts neither of a two-note tree's notes as a finding", func(t *testing.T) {
		newDriftUnderAFloodOfItsRank(t).reports("… 8 " + ecocheck.UnshownMarker)
	})

	// The same floor one rank up, where the findings are graver. Without it the drifted regions spend
	// the whole rank in byte order, and a region the drift check never ran on goes unmentioned.
	t.Run("shows an unchecked shared region through a flood of drifted ones", func(t *testing.T) {
		newFloodedSharedRegions(t).reports(ecocheck.SharedRegionNotChecked + "unchecked")
	})

	// `scriptNamed` answers a basename two scripts share with the basename itself — text the reviewed
	// tree wrote — and two findings led with it. Committing that pair under a name beginning with a
	// rank-1 prefix put the branch's own finding in that class, where byte order handed it the class's
	// one reserved line and left the real drift to a rank the same branch had flooded.

	t.Run("does not let a committed basename occupy another class's reserved line", func(t *testing.T) {

		newBasenameForgingAClass(t).reports(ecocheck.SharedRegionHasDrifted + "greet")
	})

	t.Run("shows a drifted shared region under a flood of another class in its rank", func(t *testing.T) {
		newDriftUnderAFloodOfItsRank(t).reports(ecocheck.SharedRegionHasDrifted + "greet")
	})

	// A floor that reserved the line and then appended it below every rank would satisfy presence. It
	// would also put the drift under the findings this report exists to rank it above.
	t.Run("and leaves it in its own rank, above what that rank outranks", func(t *testing.T) {
		newDriftUnderAFloodOfItsRank(t).ranksAbove(ecocheck.SharedRegionHasDrifted+"greet", noPosition)
	})

	// The third shared-region kind, whose row nothing tied to the head it is written for. A region
	// with no counterpart is what deleting one copy of a shared block produces — the tamper the drift
	// check exists to name — and the row is the whole of what keeps it out of the rank a branch under
	// review fills at will. Ordering, not presence: the floor shows the class whatever its rank, so
	// the line landing under the flood is all that is left to observe.
	t.Run("ranks a shared region whose counterpart is gone above a flood of link findings", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("kk-drive/scripts/one.sh", sharedRegions("one"))
		f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
		f.ranksAbove(ecocheck.SharedRegionWithoutCounterpart, ecocheck.DanglingLink)
	})
}

// Three regions rather than one, so the class is left at its floor line with something to withhold.
// `file too large to scan: ` sorts first and spends the rest of the rank. Within the drift class
// `greet` sorts first and is the one line shown; the two behind it are what its own count is of.
func newDriftUnderAFloodOfItsRank(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.floodWithOversize(f.root+"/kk-flavor/standards", ecocheck.FindingCap+5)
	f.newScript("kk-drive/scripts/one.sh", sharedRegions("one"))
	f.newScript("kk-humanize/scripts/two.sh", sharedRegions("two"))
	return f
}

// The rank-1 twin of newFloodedRankFive: one shared-region kind past the rank's whole budget, and a
// second kind present with one finding. `has drifted` sorts ahead of `not checked for drift`, so byte
// order alone shows 40 of the flood and none of the needle. The floor is the only thing that puts the
// unchecked region on the screen.
func newFloodedSharedRegions(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	var drifting, counterpart strings.Builder
	for i := 1; i <= ecocheck.FindingCap+5; i++ {
		name := fmt.Sprintf("drift%03d", i)
		fmt.Fprintf(&drifting, "# --- shared:%s ---\necho one\n# --- end shared:%s ---\n", name, name)
		fmt.Fprintf(&counterpart, "# --- shared:%s ---\necho two\n# --- end shared:%s ---\n", name, name)
	}
	drifting.WriteString(oversizeRegion("unchecked"))
	counterpart.WriteString(oversizeRegion("unchecked"))
	f.newScript("kk-drive/scripts/one.sh", drifting.String())
	f.newScript("kk-humanize/scripts/two.sh", counterpart.String())
	return f
}

// A region whose body runs past the compare bound, so the scan reports it rather than comparing it.
//
// Two lines past the bound, not up to it. regionsIn raises the flag on a line arriving after the body
// has already reached the cap, so a body stopping exactly at the cap is compared like any other. This
// fixture would then carry no needle at all.
func oversizeRegion(name string) string {
	var region strings.Builder
	line := "echo the body this region is too large to compare"
	fmt.Fprintf(&region, "# --- shared:%s ---\n", name)
	for body := 0; body <= ecocheck.SharedRegionBodyCap+2*(len(line)+1); body += len(line) + 1 {
		region.WriteString(line + "\n")
	}
	fmt.Fprintf(&region, "# --- end shared:%s ---\n", name)
	return region.String()
}

// The drift fixture, plus two stubs sharing one basename that opens with the drift class's own head.
// Their usage names one subcommand and their dispatch takes two, which is what makes the finding
// fire; the dispatch has to refuse under this basename, since the marker is what ties a dispatch to
// its stub.
//
// The basename has to be a live row of the rank table or the forgery reaches no class and the case
// asserts nothing. It was `shared region ` until that row became three, one per kind, and the name
// then matched nothing: the mutation this case exists to catch survived a full sweep while every
// assertion here stayed green. `aaa.sh` sorts ahead of `greet`, which is what takes the class's one
// reserved line rather than merely joining it.
func newBasenameForgingAClass(t *testing.T) *fixture {
	t.Helper()
	forgedName := ecocheck.SharedRegionHasDrifted + "aaa.sh"
	f := newDriftUnderAFloodOfItsRank(t)
	f.mkdirAll(f.root + "/tools/toy")
	f.write(f.root+"/tools/toy/toy.go", `package toy

func (r *run) dispatch() {
	switch r.arg(0) {
	case "alpha":
		r.alpha()
	case "beta":
		r.beta()
	default:
		r.refuse("usage: `+forgedName+` {alpha}")
	}
}
`)
	for _, skill := range []string{"kk-drive", "kk-humanize"} {
		f.newScript(skill+"/scripts/"+forgedName,
			"#!/usr/bin/env bash\n# untested: fixture\n#   usage: "+forgedName+" {alpha}\ntool=\"toy\"\ntrue")
	}
	return f
}

func sharedRegions(body string) string {
	var script strings.Builder
	for _, name := range []string{"greet", "hail", "wave"} {
		fmt.Fprintf(&script, "# --- shared:%s ---\necho %s\n# --- end shared:%s ---\n", name, body, name)
	}
	return script.String()
}

// Eight kinds sharing rank 5, one of them flooding it past the rank's whole budget. `dangling home
// ref: ` sorts first of the eight, so byte order alone shows 40 of it and names the other seven
// nowhere.
//
// The seven are one per scan that can reach this rank cheaply: a link, a section citation and an
// unresolvable citation path, a script with no test position, a skill in neither family, a skill
// directory with no SKILL.md, and a SKILL.md with no description.
func newFloodedRankFive(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.floodWithHomeRefs(f.root+"/kk-flavor/standards/flood.md", ecocheck.FindingCap+5)
	// Two dangling links rather than one, so a second class in the rank is left with something to
	// withhold and its note has a count of its own to get wrong.
	f.write(f.root+"/kk-flavor/standards/link.md", "[x](nowhere.md)\n")
	f.write(f.root+"/kk-flavor/standards/target.md", "# Target\n\n## Here\n")
	f.write(f.root+"/kk-flavor/standards/citer.md",
		"see [target.md](target.md) → **Nowhere** and [gone.md](gone.md) → **X**\n")
	f.newMountedSkill("zzz-outsider")
	f.newScript("kk-drive/scripts/one.sh", "true")
	return f
}

// One file the read bound refuses, buried under more crafted links than the per-rank cap will show.
func newOversizeUnderAFlood(t *testing.T) *fixture {
	t.Helper()
	f := newRoot(t)
	f.floodWithLinks(f.root+"/kk-flavor/standards/flood.md", 300, "[x](nope%03d.md)")
	f.writeOversize(f.root + "/kk-flavor/standards/huge.md")
	return f
}

// A region name is the branch's own text. The marker charset bounds what it may hold, and nothing
// bounds its length. Uncut, it reaches the printer's own 500-byte bound. What that bound removes is
// the detail saying what is wrong with the region, leaving a name and no defect.
//
// One case per arm of the scan, each asserting the marker together with the detail that has to
// survive it.
func TestALongSharedRegionNameIsCutBeforeItsDetail(t *testing.T) {
	long := strings.Repeat("z", 600)

	t.Run("marks a cut name and keeps the drift detail after it", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("kk-drive/scripts/one.sh", sharedRegion(long, "one"))
		f.newScript("kk-humanize/scripts/two.sh", sharedRegion(long, "two"))
		f.reports(shell.CutMarker + " — 2 copies, 2 distinct versions")
	})

	t.Run("and keeps the counterpart detail", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("kk-drive/scripts/one.sh", sharedRegion(long, "one"))
		f.reports(shell.CutMarker + " — 1 copy, and the marker names one no file carries")
	})

	t.Run("and keeps the reason the drift check did not run", func(t *testing.T) {
		f := newRoot(t)
		f.newScript("kk-drive/scripts/one.sh", oversizeRegion(long))
		f.reports(shell.CutMarker + " — it is too large to compare")
	})

	// Without this the three above would pass on a checker that marked every name, cut or not.
	t.Run("and leaves a name that fits whole, with no mark on it", func(t *testing.T) {
		newDriftUnderAFloodOfItsRank(t).doesNotReport(ecocheck.SharedRegionHasDrifted + "greet" + shell.CutMarker)
	})
}

func sharedRegion(name, body string) string {
	return fmt.Sprintf("# --- shared:%s ---\necho %s\n# --- end shared:%s ---\n", name, body, name)
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

// What a suppression note may claim, driven through the cut directly rather than through a tree. The
// counts are a function of the finding list alone. A tree that floods two classes of one rank and
// starves a third is cheap to build, but states its arithmetic nowhere a reader can check it.
func TestASuppressionNoteCountsOnlyItsOwnClass(t *testing.T) {
	t.Run("counts one flooded class's own withheld findings", func(t *testing.T) {
		flooded := manyFindings(firstRankFiveClasses(t, 1)[0], ecocheck.FindingCap+7)
		assertNotesAre(t, ecocheck.KeptLines(flooded), 7)
	})

	// Two classes of 40 in one rank of 40: the floor keeps one of each, the 38 lines left go to the
	// byte-first, and the counts differ by an order of magnitude. One note for the rank would say 41
	// once; a note subtracting the rank's cap would say 0 twice.
	t.Run("and gives two flooded classes in one rank a count each", func(t *testing.T) {
		classes := firstRankFiveClasses(t, 2)
		flooded := append(manyFindings(classes[0], ecocheck.FindingCap),
			manyFindings(classes[1], ecocheck.FindingCap)...)
		assertNotesAre(t, ecocheck.KeptLines(flooded), 1, 39)
	})

	// A class the rank table does not name holds every kind no row names, so no count it printed could
	// be true of the block it sits under. It stays noteless, and the report's trailing line carries
	// that remainder instead.
	t.Run("and leaves a class the rank table does not name without one", func(t *testing.T) {
		unnamed := manyFindings("no row in the rank table names this: ", ecocheck.FindingCap+5)
		assertNotesAre(t, ecocheck.KeptLines(unnamed))
	})
}

// Every class in a rank keeps a line however hard one of them floods it. This is what makes a note's
// "of this class" readable: the block it sits under is that class's, and no kind present is missing
// from the screen for the reader to wonder about.
func TestTheFloorKeepsALineForEveryClassInARank(t *testing.T) {
	classes := rankFiveClasses(t)
	flooded := manyFindings(classes[0], ecocheck.FindingCap*2)
	for _, class := range classes[1:] {
		flooded = append(flooded, manyFindings(class, 1)...)
	}
	shown := map[string]bool{}
	for _, line := range ecocheck.KeptLines(flooded) {
		for _, class := range classes {
			if strings.HasPrefix(line, class) {
				shown[class] = true
			}
		}
	}
	for _, class := range classes {
		if !shown[class] {
			t.Errorf("%q was present and the report showed none of it", class)
		}
	}
}

// Every rank-5 class prefix in byte order. Read from the table rather than written out, so a reworded
// kind cannot leave a case building findings of a class that no longer exists. Sorted, because every
// case above reasons about which classes byte order fills the rank with.
func rankFiveClasses(t *testing.T) []string {
	t.Helper()
	var prefixes []string
	for _, class := range ecocheck.RankTable() {
		if class.Rank == 5 {
			prefixes = append(prefixes, class.Prefix)
		}
	}
	sort.Strings(prefixes)
	return prefixes
}

// The first `count` of those, which is what byte order fills the rank with.
func firstRankFiveClasses(t *testing.T, count int) []string {
	t.Helper()
	prefixes := rankFiveClasses(t)
	if len(prefixes) < count {
		t.Fatalf("rank 5 holds %d classes and this case needs %d", len(prefixes), count)
	}
	return prefixes[:count]
}

// Distinct findings of one class, numbered so byte order within the class is the numeric one.
func manyFindings(prefix string, count int) []string {
	var findings []string
	for i := 1; i <= count; i++ {
		findings = append(findings, fmt.Sprintf("%s%04d", prefix, i))
	}
	return findings
}

// The suppression notes among the printed lines, by the count each carries, in printed order. Matched
// on the marker constant rather than on a wording written out here. A case comparing text it spelled
// itself goes green the next time the note is reworded, and stops observing anything.
func assertNotesAre(t *testing.T, lines []string, want ...int) {
	t.Helper()
	var found []string
	for _, line := range lines {
		if strings.Contains(line, ecocheck.SuppressedMarker) {
			found = append(found, line)
		}
	}
	if len(found) != len(want) {
		t.Fatalf("expected %d suppression note(s), got %d:\n%s", len(want), len(found), indent(strings.Join(found, "\n")))
	}
	for i, withheld := range want {
		if !strings.Contains(found[i], fmt.Sprintf("… and %d %s", withheld, ecocheck.SuppressedMarker)) {
			t.Errorf("note %d should have withheld %d:\n%s", i, withheld, indent(found[i]))
		}
	}
}
