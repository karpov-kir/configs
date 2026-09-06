package ecocheck

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"kk-flavor/tools/shell"
)

// The width and the count a finding is bounded to before anything is printed. A finding quotes text
// this checker did not choose, into output an agent drafts a PR comment from; bounding here, on the
// one path every finding takes, keeps a scan added later from reopening it.
const (
	lineWidthCap    = 500
	printedLinesCap = 200

	// What a class closes with once the budget has shown as much of it as it will. Only a class
	// rankTable names gets one. The count is that class's own: a note spanning several kinds would
	// count findings the block above it is not made of.
	suppressedMarker = "more of this class, suppressed"

	// What the whole report closes on, counting the findings neither bound let through. Named for the
	// reason suppressedMarker is: the suite matches on it, and a case that spelled it itself goes
	// green the next time it is reworded.
	unshownMarker = "finding(s) not shown in total"
)

// A finding that means *this check did not check the tree you think* ranks above every reference
// finding: those name a broken or tampered-with check rather than a broken reference. Every test is
// on the head of the line, or a finding whose *link target* merely carries the substring promotes
// itself.
//
// The prefix that matches is also the finding's class, and keptWithinTheBudgets holds a floor of one
// line per class present. Classes come from this table and never from the heads the findings carry,
// so a reviewed tree can't widen the report by inventing one. The floor therefore costs at most one
// line per row below.
//
// Every kind this checker emits has a row, so a class holds one kind. That is what lets a class carry
// suppressedMarker's count. A kind with no row falls to the empty class, which still prints but stays
// noteless, so drift lands there rather than in a false count.
//
// A row names its kind through the same constant the emit site leads with, so the two cannot drift
// into different spellings — a head reworded in one place stops compiling in the other.
// TestEveryEmittedHeadHasARankTableRow is what then catches a kind added with no row at all, since a
// constant used at an emit site and left out of this table still compiles.
//
// A finding leading with a path the reviewed tree chose cannot be a row, so every emit site leads
// with its kind instead. TestTheRankTableGivesEachKindAClassItCanAfford holds what a row needs. No
// prefix is a prefix of another, and no rank carries more rows than findingCap, so the floor cannot
// spend a whole rank on itself.
var rankTable = []struct {
	prefix string
	rank   int
}{
	{syntaxError, 0},

	// Three rows, because scanSharedRegions emits three kinds and one row would hold all three as one
	// class. mutants.go anchors on the first two as a contiguous pair, so reordering them breaks that
	// anchor.
	{sharedRegionHasDrifted, 1},
	{sharedRegionNotChecked, 1},
	{sharedRegionWithoutCounterpart, 1},
	{directionScanReadNoFiles, 1},
	// Emitted by both bounded reads in shell.go and by the call-site scan's whole-file read in
	// subcommands.go. Each finding ends on what going unread cost: "it was NOT checked", "it was NOT
	// counted", or "no call site in it was seen".
	{fileTooLargeToScan, 1},
	{fileCouldNotBeRead, 1},
	{skillsNotMounted, 1},
	{skillNotMounted, 1},
	{skillMountedElsewhere, 1},
	{mountWithoutASkill, 1},

	{budgetFileRefused, 2},
	{scriptNamesMissingTest, 2},
	{scriptNamesAmbiguousTest, 2},
	{scriptNamesTooManySuites, 2},
	{basenameNotChecked, 2},
	{subcommandCallSitesNotChecked, 2},
	// Every way that scan can hold a script with subcommands and name none of them. Each leads
	// with the path of the file it is about, so without an entry here it lands at rank 5, sharing
	// one budget with `dangling link:`.
	{unreadDispatch, 2},
	// The bound this scan withheld subcommands under. Without an entry it sorts by byte order
	// against the basenames its own findings lead with, so a stub named `alpha.sh` buries it.
	{subcommandScanAtItsBound, 2},

	{scriptNotExecutable, 3},
	{skillNameDirMismatch, 3},

	{importRefused, 4},

	// Rank 5, where a reviewed branch decides how many findings it produces of any kind it likes. The
	// rows keep the kind that is cheapest to mass-produce from spending the whole rank on itself.
	{anyRepoNamesWorkflowFamily, 5},
	{bareRuleIDCitation, 5},
	{citationTargetNotRegular, 5},
	{danglingHomeRef, 5},
	{danglingLink, 5},
	{danglingPathRef, 5},
	{danglingSectionRef, 5},
	{budgetRefusalsSuppressed, 5},
	{injectListsMissingDoc, 5},
	{citationPathIsPattern, 5},
	// Both ways the family router can fail to claim its exception. One row, because the loop that
	// raises them runs once for the single lane named familyRouter and returns after either. So this
	// class holds one finding or none, and never two kinds at once.
	{familyRouterFinding, 5},
	{scriptDeclaresNoTestPosition, 5},
	// Each of these three also heads its own scan's "already shown, the rest are not listed" notice.
	// reportBoundReached leads with the class name for that reason, so the notice shares the row.
	{sharedLayerCitesLane, 5},
	{audienceNothingReads, 5},
	{sharedLayerNamesLane, 5},
	{sharedLayerReachesLaneByBasename, 5},
	// The quote is what separates this from the other `skill…` rows above and below: the tree's own
	// directory name follows it.
	{skillInNeitherFamily, 5},
	{skillDirWithoutSkillFile, 5},
	{skillWithoutDescription, 5},
	{subcommandDispatchDoesNotAccept, 5},
	{subcommandUsageDoesNotName, 5},
	{subcommandWithNoCallSite, 5},
	{uncheckableCitation, 5},
	{undelimitedSectionCitation, 5},
	{unknownSkillReferenced, 5},
	{unresolvableCitationPath, 5},
}

// Where a finding no row names lands. It is the last rank in the table, so a kind whose row is
// missing still sorts below every kind that has one. Add a rank past this one and that stops holding.
const unnamedClassRank = 5

func classify(line string) (rank int, prefix string) {
	for _, candidate := range rankTable {
		if strings.HasPrefix(line, candidate.prefix) {
			return candidate.rank, candidate.prefix
		}
	}
	return unnamedClassRank, ""
}

// Ordered before it is cut, never alphabetically: showing one class until the cap runs out lets a
// flood of crafted `dangling link:` lines bury a syntax error, a drifted shared region or a refused
// budget file.
func (c *checker) printFindings(out io.Writer) int {
	sorted := shell.SortUnique(c.findings)
	if len(sorted) == 0 {
		writeLinef(out, "wiring: clean")
		return 0
	}

	printedFindings := 0
	for i, line := range keptWithinTheBudgets(sorted) {
		if i >= printedLinesCap {
			break
		}
		// Marked, because this is the last cut a finding takes and so the only one the reader sees the
		// result of. Every message upstream that quotes a name the tree chose marks its own cut; this
		// bound runs after all of them and takes that mark off with the tail it removes, leaving a
		// shorter wrong line that reads as a whole one. Two cases in budget_test.go already match on
		// a finding's head rather than its tail for exactly this reason.
		bounded := shell.CutBytesMarked(line.text, lineWidthCap)
		writeLinef(out, "%s", bounded)
		// Counted from what was actually printed, never from either cap: two mechanisms hide
		// findings, so arithmetic against one alone contradicts the other. Notes are told apart by
		// their flag and never by matching the text. A finding quoting a committed path that holds the
		// note wording would otherwise count as a note, putting the reviewed tree in charge of this
		// tally.
		if !line.isNote {
			printedFindings++
		}
	}
	// A total, and worded as one. "Further" reads as an increment on the class lines above it, so a
	// reader adds the two and is told twice the number — once per class the floor gave a line to.
	if len(sorted) > printedFindings {
		writeLinef(out, "… %d "+unshownMarker+" — fix these and re-run", len(sorted)-printedFindings)
	}
	return 1
}

type rankedLine struct {
	rank   int
	text   string
	isNote bool
}

type findingClass struct {
	// Empty for everything rankTable does not name.
	prefix string
	rank   int
	total  int
	shown  int
}

type classifiedFinding struct {
	text   string
	class  *findingClass
	isKept bool
}

// What the report prints of a sorted, deduplicated finding list, and in what order.
//
// Two bounds, because two floods reach here. Each rank shows at most findingCap findings. And inside
// a rank there is a floor: every class present keeps one line. Without it the class the branch under
// review floods fills the rank by itself and picks which of the others a reviewer never sees, a
// drifted shared region among them. Each class then says how many of its own it withheld.
func keptWithinTheBudgets(sorted []string) []rankedLine {
	findings := classified(sorted)
	keepWithinEachRank(findings)
	return orderedByRank(findings)
}

func classified(sorted []string) []classifiedFinding {
	findings := make([]classifiedFinding, len(sorted))
	classes := map[string]*findingClass{}
	for i, finding := range sorted {
		rank, prefix := classify(finding)
		if classes[prefix] == nil {
			classes[prefix] = &findingClass{prefix: prefix, rank: rank}
		}
		classes[prefix].total++
		findings[i] = classifiedFinding{text: finding, class: classes[prefix]}
	}
	return findings
}

// The floor first: the byte-first finding of every class the tree holds. Then what is left of each
// rank's budget, in byte order across the classes sharing it.
func keepWithinEachRank(findings []classifiedFinding) {
	shownInRank := map[int]int{}
	isRankFull := func(rank int) bool { return shownInRank[rank] >= findingCap }
	keep := func(i int) {
		findings[i].isKept = true
		findings[i].class.shown++
		shownInRank[findings[i].class.rank]++
	}
	for i := range findings {
		if findings[i].class.shown > 0 || isRankFull(findings[i].class.rank) {
			continue
		}
		keep(i)
	}
	for i := range findings {
		if findings[i].isKept || isRankFull(findings[i].class.rank) {
			continue
		}
		keep(i)
	}
}

// The kept findings in rank order, each named class's suppression note on the line under the last
// finding that class shows. A class rankTable does not name gets no note: its findings are of several
// kinds, so "of this class" would be false whichever block the line landed under.
func orderedByRank(findings []classifiedFinding) []rankedLine {
	var kept []rankedLine
	isNoted := map[*findingClass]bool{}
	for _, finding := range findings {
		class := finding.class
		switch {
		case finding.isKept:
			kept = append(kept, rankedLine{rank: class.rank, text: finding.text})
		case class.prefix != "" && !isNoted[class]:
			isNoted[class] = true
			kept = append(kept, rankedLine{rank: class.rank, isNote: true,
				text: fmt.Sprintf("… and %d %s — fix these first", class.total-class.shown, suppressedMarker)})
		}
	}
	// Stable, so the byte order the deduplication left within each rank survives the reordering.
	sort.SliceStable(kept, func(a, b int) bool { return kept[a].rank < kept[b].rank })
	return kept
}

func writeLinef(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, format+"\n", args...)
}
