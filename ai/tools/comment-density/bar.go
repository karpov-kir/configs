// The bar half of the detector: what the host repo's own comment rate is, and how far over it a change
// set sits. It counts each changed file whole, so a second run over the same tree reproduces its verdict.
//
// A comment's bar is a ratio to the code it sits in. A PR body's is not: measured over player-testing's
// merged PRs, body length does not scale with the diff (words per changed line 3 at p50, 36 at p90), so a
// body takes an absolute bar read off the repo's own bodies. That thermometer is not built yet.
package density

import (
	"fmt"
	"sort"
	"strings"

	"kk-flavor/tools/diffscan"
	"kk-flavor/tools/shell"
)

// A block over this many lines reads as a wall, not a note. Their share is held apart from the line
// ratio: a set can sit under its ratio and still spend the whole allowance on one module header.
const longBlockLines = 4

// statsOf counts one file's whole content. A blank line ends a block: two comments with one between them
// are two things a reader meets, not one.
func statsOf(content string) stats {
	var counted stats
	run := 0
	closeRun := func() {
		if run == 0 {
			return
		}
		counted.blocks++
		if run > longBlockLines {
			counted.longBlocks++
		}
		run = 0
	}
	for _, raw := range shell.SplitLines(content) {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case line == "":
			closeRun()
		case isComment(line):
			counted.comments++
			run++
		default:
			counted.code++
			closeRun()
		}
	}
	closeRun()
	return counted
}

type rate struct {
	numerator   int
	denominator int
}

// allowance is integer arithmetic on the baseline's own counts, never through its ratio() as a float.
// That share of all lines, turned back into comments per code line, makes an exact allowance come out
// as 1.999: 2 on 4 code lines from a baseline of 1 per 2 truncates to 1, and the report asks for a line
// the bar does not need.
func (r rate) allowance(size int) int {
	if r.denominator == 0 {
		return 0
	}
	return r.numerator * size / r.denominator
}

// cutToRatio counts the comment lines that have to go. code is the fixed side: deleting comments never
// moves it. A baseline of only comments runs at a rate nothing exceeds.
func cutToRatio(set, base stats) int {
	if base.code == 0 {
		return 0
	}
	allowed := rate{numerator: base.comments, denominator: base.code}.allowance(set.code)
	if set.comments <= allowed {
		return 0
	}
	return set.comments - allowed
}

// baseline is the host repo's own shape, measured over the files this change does not touch. ceiling is
// the p90 of their per-file ratios, not the aggregate: one file in a change may carry a real explanation,
// and holding every file to the aggregate would spread comments evenly instead.
type baseline struct {
	stats   stats
	ceiling float64
	files   int
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	index := int(p * float64(len(sorted)))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// An unreadable file is skipped, not an error: the set is a population, and one missing member does not
// change what it says. read counts the files read at all, countable or not, so a caller can say how many
// were not.
func (h hostRepo) measure(paths []string, visit func(rel string, file stats)) (total stats, read int) {
	for _, rel := range paths {
		content, ok := h.readCapped(rel)
		if !ok {
			continue
		}
		read++
		file := statsOf(content)
		if file.total() == 0 {
			continue
		}
		total.add(file)
		visit(rel, file)
	}
	return total, read
}

func (h hostRepo) measureBaseline(paths []string) baseline {
	ratios := make([]float64, 0, len(paths))
	whole, _ := h.measure(paths, func(_ string, file stats) {
		ratios = append(ratios, file.ratio())
	})
	return baseline{stats: whole, ceiling: percentile(ratios, 0.9), files: len(ratios)}
}

// perFileCeiling is the ratio a file new since the diff's base may not exceed on its own. A file the
// repo already carried is not held to it: its density is the repo's own.
type perFileCeiling struct {
	isNew map[string]bool
	ratio float64
}

func (c perFileCeiling) isOver(rel string, file stats) bool {
	return c.isNew[rel] && file.ratio() > c.ratio
}

type fileOverCeiling struct {
	rel   string
	ratio float64
}

type changeSet struct {
	stats
	over []fileOverCeiling
	read int
}

func (h hostRepo) measureChangeSet(paths []string, ceiling perFileCeiling) changeSet {
	var set changeSet
	set.stats, set.read = h.measure(paths, func(rel string, file stats) {
		if ceiling.isOver(rel, file) {
			set.over = append(set.over, fileOverCeiling{rel: rel, ratio: file.ratio()})
		}
	})
	return set
}

func bar(out console, args []string, cwd string, cfg Config) int {
	if err := diffscan.RefuseNonRevisions(args, cwd); err != nil {
		return out.refuse(err)
	}
	host, err := newHostRepo(cwd, cfg.MaxFileBytes)
	if err != nil {
		return out.refuse(err)
	}
	revisions, pathspec := splitPathspec(args)
	changed, err := host.changedSources(revisions, pathspec)
	if err != nil {
		return out.refuse(err)
	}
	if len(changed) == 0 {
		out.note("no source file in this change set, so this run says nothing about it.")
		return exitClean
	}
	tracked, err := host.trackedSources()
	if err != nil {
		return out.refuse(err)
	}
	isNew, err := host.newSinceBase(revisions, changed)
	if err != nil {
		return out.refuse(err)
	}
	base := host.measureBaseline(without(tracked, changed))
	// No baseline is refused, never defaulted: a number invented here reads exactly like one measured.
	if base.files == 0 {
		return out.refuse(refusal("no file outside this change set carried countable lines, so the repo has no rate to hold it to"))
	}
	set := host.measureChangeSet(changed, perFileCeiling{isNew: isNew, ratio: base.ceiling})
	out.note("%d changed source file(s), %d read, %d skipped unread; %d file(s) outside the change in the baseline.",
		len(changed), set.read, len(changed)-set.read, base.files)
	if set.total() == 0 {
		return out.refuse(refusal("no changed source file could be read, so this run says nothing about the change set"))
	}
	return out.reportBar(base, set)
}

// Exit 1 means over the bar, and the report says how many lines: a share tells nobody what to delete. At
// most maxShown of the per-file lines are printed and the rest announced, for the reason at maxShown;
// every one of them is a finding.
func (c console) reportBar(base baseline, set changeSet) int {
	fmt.Fprintf(c.stdout, "host repo: %.1f%% comment lines, %.1f-line mean block, %.0f%% of blocks over %d lines (%d file(s) outside this change)\n",
		base.stats.ratio()*100, base.stats.meanBlock(), base.stats.longShare()*100, longBlockLines, base.files)
	fmt.Fprintf(c.stdout, "change set: %.1f%% comment lines (%d comment / %d code), %.1f-line mean block, %.0f%% of blocks over %d lines\n",
		set.ratio()*100, set.comments, set.code, set.meanBlock(), set.longShare()*100, longBlockLines)

	findings := len(set.over)
	if cut := cutToRatio(set.stats, base.stats); cut > 0 {
		findings++
		fmt.Fprintf(c.stdout, "over on lines: cut %d comment line(s) to reach %.1f%%\n", cut, base.stats.ratio()*100)
	}
	if allowed := (rate{numerator: base.stats.longBlocks, denominator: base.stats.blocks}).allowance(set.blocks); set.longBlocks > allowed {
		findings++
		fmt.Fprintf(c.stdout, "over on blocks: %d block(s) over %d lines against %d allowed\n", set.longBlocks, longBlockLines, allowed)
	}
	for i, file := range set.over {
		if i == maxShown {
			fmt.Fprintf(c.stdout, "… and %d further file(s) over the ceiling, not shown\n", len(set.over)-maxShown)
			break
		}
		fmt.Fprintf(c.stdout, "%s: %.0f%% against a %.0f%% ceiling\n",
			shell.CutBytesMarked(shell.Oneline(file.rel), maxPathBytes), file.ratio*100, base.ceiling*100)
	}
	if findings == 0 {
		return exitClean
	}
	return exitFound
}
