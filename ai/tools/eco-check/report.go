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
	lineWidthCap     = 500
	printedLinesCap  = 200
	suppressedMarker = "of this class, suppressed"
)

// A finding that means *this check did not check the tree you think* ranks above every reference
// finding: those name a broken or tampered-with check rather than a broken reference. Every test is
// on the head of the line, or a finding whose *link target* merely carries the substring promotes
// itself.
func rank(line string) int {
	byRank := []struct {
		prefix string
		rank   int
	}{
		{"syntax: ", 0},
		{"shared region ", 1},
		{"direction scan read no files", 1},
		{"flavor not mounted", 1},
		{"flavor mounted elsewhere", 1},
		{"skills not mounted", 1},
		{"skill not mounted", 1},
		{"skill mounted elsewhere", 1},
		{"budget file refused", 2},
		{"script names a missing test", 2},
		{"script names an ambiguous test", 2},
		{"script names more suites than the scan reads", 2},
		{"basename not checked", 2},
		{"subcommand call sites not checked", 2},
		// The bound this scan withheld subcommands under. It reached the screen before only because it
		// sorted ahead of the basename its own findings led with, which is byte order and not a rank: a
		// tree whose stub is named `alpha.sh` buried it under the findings it exists to qualify.
		{"subcommand call-site scan is at its", 2},
		{"script not executable", 3},
		{"skill name/dir mismatch", 3},
		{"import refused", 4},
	}
	for _, candidate := range byRank {
		if strings.HasPrefix(line, candidate.prefix) {
			return candidate.rank
		}
	}
	return 5
}

// Ordered before it is cut, never alphabetically: showing one class until the cap runs out lets a
// flood of crafted `dangling link:` lines bury a syntax error, a drifted shared region or a refused
// budget file. Capped per class as well as in total, because rank 5 is also the cheapest to
// mass-produce: each rank shows at most findingCap and says how many of its own it withheld.
func (c *checker) printFindings(out io.Writer) int {
	sorted := shell.SortUnique(c.findings)
	if len(sorted) == 0 {
		writeLinef(out, "wiring: clean")
		return 0
	}

	total := map[int]int{}
	for _, finding := range sorted {
		total[rank(finding)]++
	}
	type rankedLine struct {
		rank int
		text string
	}
	shown := map[int]int{}
	var kept []rankedLine
	for _, finding := range sorted {
		r := rank(finding)
		shown[r]++
		switch {
		case shown[r] <= findingCap:
			kept = append(kept, rankedLine{rank: r, text: finding})
		case shown[r] == findingCap+1:
			kept = append(kept, rankedLine{rank: r, text: fmt.Sprintf("… and %d more %s — fix these first",
				total[r]-findingCap, suppressedMarker)})
		}
	}
	// Stable, so the byte order the deduplication left within each rank survives the reordering.
	sort.SliceStable(kept, func(a, b int) bool { return kept[a].rank < kept[b].rank })

	printedFindings := 0
	for i, line := range kept {
		if i >= printedLinesCap {
			break
		}
		bounded := shell.CutBytes(line.text, lineWidthCap)
		writeLinef(out, "%s", bounded)
		// Counted from what was actually printed, never from either cap: two mechanisms hide
		// findings, so arithmetic against one alone contradicts the other.
		if !strings.Contains(bounded, suppressedMarker) {
			printedFindings++
		}
	}
	if len(sorted) > printedFindings {
		writeLinef(out, "… and %d further finding(s) not shown — fix these and re-run", len(sorted)-printedFindings)
	}
	return 1
}

func writeLinef(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, format+"\n", args...)
}
