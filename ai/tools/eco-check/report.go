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

	// What a class closes with once the budget has shown all of it that it will. Only a class
	// classify's table names gets one. Rank 5 is the single class for every kind that table does not
	// name, so a note there counts several kinds while sitting under whichever one byte order printed
	// last: 200 dangling links and 3 unfamilied skills print 40 links and a note saying 163. The
	// trailing "further finding(s) not shown" carries that remainder instead, attributing it to
	// nothing.
	suppressedMarker = "more of this class, suppressed"
)

// A finding that means *this check did not check the tree you think* ranks above every reference
// finding: those name a broken or tampered-with check rather than a broken reference. Every test is
// on the head of the line, or a finding whose *link target* merely carries the substring promotes
// itself.
//
// The prefix that matches is also the finding's class, and keptWithinTheBudgets holds a floor of one
// line per class present. Classes come from this table and never from the heads the findings carry,
// so a reviewed tree can't widen the report by inventing one. The floor therefore costs at most one
// line per row below, plus one for the class that holds everything no row names.
func classify(line string) (rank int, prefix string) {
	byRank := []struct {
		prefix string
		rank   int
	}{
		{"syntax: ", 0},
		{"shared region ", 1},
		{"direction scan read no files", 1},
		// Emitted by both bounded reads in shell.go; the finding's own text ends on "it was NOT
		// checked". At the default rank it shares one budget with `dangling link:` and sorts below
		// every one of them.
		{"file too large to scan: ", 1},
		{"file could not be read: ", 1},
		{"skills not mounted", 1},
		{"skill not mounted", 1},
		{"skill mounted elsewhere", 1},
		{"mount without a skill", 1},
		{"budget file refused", 2},
		{"script names a missing test", 2},
		{"script names an ambiguous test", 2},
		{"script names more suites than the scan reads", 2},
		{"basename not checked", 2},
		{"subcommand call sites not checked", 2},
		// Every way that scan can hold a script with subcommands and name none of them. Each leads
		// with the path of the file it is about, so without an entry here it lands at rank 5, sharing
		// one budget with `dangling link:`.
		{unreadDispatch, 2},
		// The bound this scan withheld subcommands under. Without an entry it sorts by byte order
		// against the basenames its own findings lead with, so a stub named `alpha.sh` buries it.
		{"subcommand call-site scan is at its", 2},
		{"script not executable", 3},
		{"skill name/dir mismatch", 3},
		{"import refused", 4},
	}
	for _, candidate := range byRank {
		if strings.HasPrefix(line, candidate.prefix) {
			return candidate.rank, candidate.prefix
		}
	}
	return 5, ""
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
	if len(sorted) > printedFindings {
		writeLinef(out, "… and %d further finding(s) not shown — fix these and re-run", len(sorted)-printedFindings)
	}
	return 1
}

type rankedLine struct {
	rank   int
	text   string
	isNote bool
}

type findingClass struct {
	// Empty for everything classify's table does not name.
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
// review floods fills the rank by itself and picks which of the others a reviewer never sees,
// `shared region ` among them. Each class then says how many of its own it withheld.
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
// finding that class shows.
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
