// The bar half of the detector: what the host repo's own comment rate is, and how far over it a change
// set sits. It counts each changed file whole, so a second run over the same tree reproduces its verdict.
//
// A comment's bar is a ratio to the code it sits in. A PR body's is not: body length does not scale with
// the diff, so a body takes an absolute bar read off the repo's own bodies. Measured over one repository's
// merged PRs, words per changed line ran 3 at p50 and 36 at p90. That thermometer is not built yet.
package density

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"kk-flavor/tools/diffscan"
	gitrepo "kk-flavor/tools/repo"
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

// measure counts these paths where this run's content lives: the working tree, or the revision a closed
// range pinned it to, which contentRevision decides. Reading the tree under a range's file list would
// measure today's files against yesterday's names and hand back a plausible number with no error.
func (h hostRepo) measure(paths []string, visit func(rel string, file stats)) (stats, int, error) {
	return h.measureAt(h.contentRev, paths, visit)
}

// measureAt is measure at one named revision, and an empty rev is the working tree.
//
// At a revision the whole list goes to git in one call, because asking per file costs a process per
// file and over a checkout of a few hundred that bill, not the counting, is the mode's entire runtime.
// A symlink there is a blob holding its target string, counted as one code line, so nothing outside the
// repository is opened on this path either.
//
// An unreadable file is skipped, not an error: the set is a population, and one missing member does not
// change what it says. A call that fails outright is a different thing and is returned — the batch
// either answered or did not, and swallowing that would leave a whole baseline silently empty. read
// counts the files read at all, countable or not, so a caller can say how many were not.
func (h hostRepo) measureAt(rev string, paths []string, visit func(rel string, file stats)) (stats, int, error) {
	var total stats
	read := 0
	// A NUL byte marks the file binary whichever side it came from, and a binary file is not read at all
	// rather than read and counted.
	count := func(rel, content string) {
		if strings.IndexByte(content, 0) >= 0 {
			return
		}
		read++
		file := statsOf(content)
		if file.total() == 0 {
			return
		}
		total.add(file)
		visit(rel, file)
	}
	if rev == "" {
		for _, rel := range paths {
			if content, ok := h.readCappedInTree(rel); ok {
				count(rel, content)
			}
		}
		return total, read, nil
	}
	err := h.git.ContentsAt(h.root, rev, paths, h.maxBytes, func(rel string, content []byte) {
		count(rel, string(content))
	})
	if err != nil {
		return stats{}, 0, gitRefusal("could not read the files at "+rev, err)
	}
	return total, read, nil
}

// carried are files this change touched but did not create. They stay in the baseline at their
// pre-change content: that content is the repo's, and dropping it lets one edit to a comment-heavy file
// lower the very rate the change is then held to.
func (h hostRepo) measureBaseline(paths, carried []string, rev string) (baseline, error) {
	ratios := make([]float64, 0, len(paths)+len(carried))
	collect := func(_ string, file stats) {
		ratios = append(ratios, file.ratio())
	}
	whole, _, err := h.measure(paths, collect)
	if err != nil {
		return baseline{}, err
	}
	// At the base revision, not the run's content revision: these files are in the baseline for the
	// content they had BEFORE the change, which is the only reading the change cannot move.
	before, _, err := h.measureAt(rev, carried, collect)
	if err != nil {
		return baseline{}, err
	}
	whole.add(before)
	return baseline{stats: whole, ceiling: percentile(ratios, 0.9), files: len(ratios)}, nil
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
	// mass is every changed file's comment count, so the report can say where the overage sits. A file
	// the change did not create is marked carried: its comments are counted here because the file lands
	// with them, but they are the repo's and `code-style.md` reports them rather than charging them.
	mass []fileMass
	// chargeable weights each file by how much of its comment mass this change wrote. Carried mass is reported and never
	// charged, so the overage — which counts every changed file whole — is the workings and this is the
	// figure a reader acts on. They differ by more than the overage itself on a change that brushes a
	// comment-heavy file.
	chargeable stats
	read       int
}

type fileMass struct {
	rel      string
	comments int
	// written is the share of this file's landed comment lines that this change wrote. Authorship is
	// measured over the population being graded: a comment-only rewrite touches almost no code, so a
	// whole-file share would call it inherited and charge nothing for a mass entirely the change's own.
	// It is a fraction of the file as it stands, never a ratio of one delta to another.
	written float64
}

func (h hostRepo) measureChangeSet(paths []string, ceiling perFileCeiling, authored map[string]int) (changeSet, error) {
	var set changeSet
	var err error
	set.stats, set.read, err = h.measure(paths, func(rel string, file stats) {
		if ceiling.isOver(rel, file) {
			set.over = append(set.over, fileOverCeiling{rel: rel, ratio: file.ratio()})
		}
		// A file the change created carries no older line, and an untracked one has no diff to read at
		// all, so its whole comment mass is this change's by construction rather than by counting.
		written := 1.0
		if file.comments > 0 && !ceiling.isNew[rel] {
			written = min(float64(authored[rel])/float64(file.comments), 1)
		}
		set.chargeable.add(stats{
			comments: int(float64(file.comments)*written + 0.5),
			code:     int(float64(file.code)*written + 0.5),
		})
		if file.comments > 0 {
			set.mass = append(set.mass, fileMass{rel: rel, comments: file.comments, written: written})
		}
	})
	return set, err
}

// authoredComments counts, per file, the comment lines this change added. Paired with the file's landed
// comment count it gives authorship as a share of what is there, which is bounded whichever way the
// change went — where a rate built from the diff alone inverts on a change that only deleted.
func (h hostRepo) authoredComments(revisions, changed []string) (map[string]int, error) {
	// Scoped as a pathspec to the files already resolved as changed, so the patch covers the population
	// being graded and nothing else.
	named := revisions
	if len(named) == 0 {
		named = []string{"HEAD"}
	}
	diff, err := h.git.Patch(h.root, named, changed)
	if err != nil {
		return nil, gitRefusal("could not read which comment lines this change wrote", err)
	}
	authored := map[string]int{}
	var result diffscan.Result
	if err := result.WalkDiff(diff, func(added diffscan.AddedLine) {
		if isComment(strings.TrimLeft(added.Text, shell.SpaceBytes)) {
			authored[added.File]++
		}
	}); err != nil {
		return nil, err
	}
	return authored, nil
}

// carriers names the files holding the first half of the change set's comment mass, heaviest first. Half
// rather than a chosen count: it answers "where is this" without a number invented to make a report fit.
func (c changeSet) carriers() []fileMass {
	ranked := append([]fileMass(nil), c.mass...)
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].comments != ranked[j].comments {
			return ranked[i].comments > ranked[j].comments
		}
		return ranked[i].rel < ranked[j].rel
	})
	running := 0
	for i, file := range ranked {
		running += file.comments
		if running*2 >= c.comments || i+1 == maxShown {
			return ranked[:i+1]
		}
	}
	return ranked
}

func bar(out console, git gitrepo.Git, args []string, cwd string, cfg Config) int {
	if err := diffscan.RefuseNonRevisions(git, args, cwd); err != nil {
		return out.refuseArguments(err)
	}
	host, err := newHostRepo(git, cwd, cfg.MaxFileBytes)
	if err != nil {
		return out.refuse(err)
	}
	revisions, pathspec := splitPathspec(args)
	host.contentRev = contentRevision(revisions)
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
	baseRev, err := host.baseRevision(revisions)
	if err != nil {
		return out.refuse(err)
	}
	isNew, err := host.newSinceBase(revisions, changed)
	if err != nil {
		return out.refuse(err)
	}
	carried := make([]string, 0, len(changed))
	for _, rel := range changed {
		if !isNew[rel] {
			carried = append(carried, rel)
		}
	}
	base, err := host.measureBaseline(without(tracked, changed), carried, baseRev)
	if err != nil {
		return out.refuse(err)
	}
	// No baseline is refused, never defaulted: a number invented here reads exactly like one measured.
	if base.files == 0 {
		return out.refuse(refusal("no file outside this change set carried countable lines, so the repo has no rate to hold it to"))
	}
	authored, err := host.authoredComments(revisions, changed)
	if err != nil {
		return out.refuse(err)
	}
	set, err := host.measureChangeSet(changed, perFileCeiling{isNew: isNew, ratio: base.ceiling}, authored)
	if err != nil {
		return out.refuse(err)
	}
	out.note("%d changed source file(s), %d read, %d skipped unread; %d file(s) in the baseline.",
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
	fmt.Fprintf(c.stdout, "measured by: comment-density build %s, tree %s\n", toolBuild(), toolTree())
	fmt.Fprintf(c.stdout, "host repo: %.1f%% comment lines, %.1f-line mean block, %.0f%% of blocks over %d lines (%d file(s) in the baseline)\n",
		base.stats.ratio()*100, base.stats.meanBlock(), base.stats.longShare()*100, longBlockLines, base.files)
	fmt.Fprintf(c.stdout, "change set: %.1f%% comment lines (%d comment / %d code), %.1f-line mean block, %.0f%% of blocks over %d lines\n",
		set.ratio()*100, set.comments, set.code, set.meanBlock(), set.longShare()*100, longBlockLines)

	findings := len(set.over)
	if cut := cutToRatio(set.stats, base.stats); cut > 0 {
		findings++
		fmt.Fprintf(c.stdout, "over on lines: cut %d comment line(s) to reach %.1f%%\n", cut, base.stats.ratio()*100)
	}
	if cutToRatio(set.stats, base.stats) > 0 {
		if owed := cutToRatio(set.chargeable, base.stats); owed > 0 {
			fmt.Fprintf(c.stdout, "chargeable: %d comment line(s), in the files this change wrote\n", owed)
		} else {
			fmt.Fprintf(c.stdout, "chargeable: nothing chargeable — the overage is in files this change did not write\n")
		}
		for _, file := range set.carriers() {
			note := ""
			if file.written < 0.5 {
				note = " — mostly the repo's, so report it rather than charge it"
			}
			fmt.Fprintf(c.stdout, "%s: %d comment line(s), %.0f%% written here%s\n",
				shell.CutBytesMarked(shell.Oneline(file.rel), maxPathBytes), file.comments, file.written*100, note)
		}
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

// Reported unknown rather than omitted: a line that vanishes with the identity leaves its absence
// meaning either no stamp or an older binary, and the reader cannot tell which. `resolve.sh` carries
// what the stamp is and why it, rather than the binary's bytes.
// The checkout the stub resolved through, reported unknown rather than omitted for the reason toolBuild
// gives. A different fact from the build: source can hash identically to its own tree and that tree
// still be a commit nobody else has, which is what makes two readings taken apart incomparable.
func toolTree() string {
	if tree := os.Getenv("ECO_TOOL_TREE"); tree != "" {
		return tree
	}
	return "unknown"
}

func toolBuild() string {
	if stamp := os.Getenv("ECO_TOOL_BUILD"); stamp != "" {
		return stamp
	}
	return "unknown"
}
