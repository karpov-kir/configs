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
	"regexp"
	"sort"
	"strings"

	"kk-flavor/tools/diffscan"
	"kk-flavor/tools/shell"
)

// A block over this many lines reads as a wall, not a note. Their share is held apart from the line
// ratio: a set can sit under its ratio and still spend the whole allowance on one module header.
const longBlockLines = 4

// A file header states the call order, lifecycle and error modes a published surface owes, and the
// rule allows it twice a block's length. voice.go holds the same two numbers.
const longHeaderLines = 8

// statsOf counts one file's whole content. A blank line ends a block: two comments with one between them
// are two things a reader meets, not one.
//
// A block's LENGTH is its prose lines, while its share of the file is its comment lines. The two counts
// differ on a `/**`, a `*/` and a doc tag line, and they differ on purpose: those lines cost the reader
// a line of screen, so they belong in the density ratio, and they carry no sentence, so they are not
// what "a block over four lines" is about. The voice check's own long-block test counts the same way —
// one rule stated in one place would be better still, but two instruments disagreeing about which
// blocks are long is the thing that cannot stand.
func statsOf(content string) stats {
	lines := shell.SplitLines(content)
	var counted stats
	run, prose := 0, 0
	seen := false
	// past is the index just after the block that is closing, which is where the declaration it might
	// sit on begins.
	closeRun := func(past int) {
		if run == 0 {
			return
		}
		counted.blocks++
		// A file header is allowed what the rule allows it, here as in the voice check. Held to a
		// block's limit, this package's own eight-line headers count as long blocks while the voice
		// check and code-style.md both allow them — one rule, two verdicts, which is the thing a
		// single instrument may not do.
		limit := longBlockLines
		if !seen {
			limit = longHeaderLines
		}
		if prose > limit {
			counted.longBlocks++
		}
		// Every prose line is a note, except the first of a block that sits on a declaration, which is
		// its summary. The rule requires a summary wherever a name leaves something to say and forbids
		// it paying the bar, so counting it would let a mandate inflate the number it is judged by — a
		// module of small exported functions is summary-dense by construction. A block that sits on a
		// statement has no summary to exempt, and its first line is a note like the rest.
		notes := prose
		if prose > 0 && sitsOnDeclaration(lines, past) {
			notes--
		}
		counted.notes += notes
		run, prose = 0, 0
	}
	for i, raw := range lines {
		at := i + 1
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case isShebang(at, line):
			// An interpreter directive is not a comment, here as in the voice check. It stands above
			// the file header rather than displacing it, so `seen` stays false and the header below
			// keeps the allowance the voice check gives it.
			closeRun(i)
		case line == "":
			closeRun(i)
		case isComment(line):
			counted.comments++
			run++
			if isProseLine(stripMarker(line)) {
				prose++
				counted.prose++
			}
		default:
			counted.code++
			closeRun(i)
			seen = true
		}
	}
	closeRun(len(lines))
	return counted
}

// sitsOnDeclaration says the block ending just before `from` is attached to a declaration: its next
// non-blank line opens with a declaration keyword, is a name followed by `:`, is a name and parameter
// list followed by a return type or a body, or ends in `{`.
// A further comment line ends the search — a block with only another comment under it declares nothing.
//
// A heuristic, and shaped by the languages this tool reads. It will call some lines declarations that
// are not, and miss some that are; what it decides is whether one line of a block is a summary or a
// note, so a misjudgement moves the bar by one line either way.
func sitsOnDeclaration(lines []string, from int) bool {
	for at := from; at < len(lines); at++ {
		line := strings.TrimSpace(lines[at])
		if line == "" {
			continue
		}
		if isComment(strings.TrimLeft(lines[at], shell.SpaceBytes)) {
			return false
		}
		return reDeclaration.MatchString(line)
	}
	return false
}

var reDeclaration = regexp.MustCompile(
	`^(?:export|func|type|const|let|var|class|interface|enum|public|private|protected|static|async|def|function|readonly)\b` +
		`|^[A-Za-z_$][\w$]*\s*:` +
		`|^[A-Za-z_$][\w$]*\s*\([^)]*\)\s*[:{]` +
		`|\{\s*$`)

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
// cutAcrossClasses sums each class's own overage against its own rate. One number over the whole set
// would let a class under its rate pay for one over it, which is the averaging this change removes.
func cutAcrossClasses(set changeSet, base baselines) int {
	total := 0
	for class, holds := range set.byClass {
		total += cutToRatio(holds, base.forClass(class).stats)
	}
	return total
}

func cutToRatio(set, base stats) int {
	if base.code == 0 {
		return 0
	}
	allowed := rate{numerator: base.notes, denominator: base.code}.allowance(set.code)
	if set.notes <= allowed {
		return 0
	}
	return set.notes - allowed
}

// baseline is the host repo's own shape, measured over the files this change does not touch. ceiling is
// the p90 of their per-file ratios, not the aggregate: one file in a change may carry a real explanation,
// and holding every file to the aggregate would spread comments evenly instead.
type baseline struct {
	stats   stats
	ceiling float64
	files   int
}

// classOf is the top-level directory a file sits in, and the population it is compared against. A
// library file and a test suite are written to different rates by every repository that has both, so
// one whole-repo number holds a library to a suite tree's density and asks it to cut what the comment
// rule requires. The comparison is like with like or it is not a comparison.
func classOf(rel string) string {
	if at := strings.IndexByte(rel, '/'); at >= 0 {
		return rel[:at]
	}
	return "."
}

// baselines is the repo's rate as a whole and per directory class. A class with no untouched file has
// no rate of its own, and its files fall back to the whole — measured, never invented.
type baselines struct {
	whole   baseline
	byClass map[string]baseline
}

func (b baselines) forClass(class string) baseline {
	if own, ok := b.byClass[class]; ok && own.files > 0 && own.stats.code > 0 {
		return own
	}
	return b.whole
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

// carried are files this change touched but did not create. They stay in the baseline at their
// pre-change content: that content is the repo's, and dropping it lets one edit to a comment-heavy file
// lower the very rate the change is then held to.
func (h hostRepo) measureBaseline(paths, carried []string, rev string) baselines {
	ratios := make([]float64, 0, len(paths)+len(carried))
	perClass := map[string][]float64{}
	classStats := map[string]stats{}
	take := func(rel string, file stats) {
		ratios = append(ratios, file.noteRatio())
		class := classOf(rel)
		perClass[class] = append(perClass[class], file.noteRatio())
		holds := classStats[class]
		holds.add(file)
		classStats[class] = holds
	}
	whole, _ := h.measure(paths, take)
	for _, rel := range carried {
		content, ok := h.readCappedAt(rev, rel)
		if !ok {
			continue
		}
		file := statsOf(content)
		whole.add(file)
		take(rel, file)
	}
	built := baselines{
		whole:   baseline{stats: whole, ceiling: percentile(ratios, 0.9), files: len(ratios)},
		byClass: map[string]baseline{},
	}
	for class, values := range perClass {
		built.byClass[class] = baseline{stats: classStats[class], ceiling: percentile(values, 0.9), files: len(values)}
	}
	return built
}

// perFileCeiling is the ratio a file new since the diff's base may not exceed on its own. A file the
// repo already carried is not held to it: its density is the repo's own.
type perFileCeiling struct {
	isNew map[string]bool
	base  baselines
}

func (c perFileCeiling) isOver(rel string, file stats) bool {
	return c.isNew[rel] && file.noteRatio() > c.base.forClass(classOf(rel)).ceiling
}

func (c perFileCeiling) ratioFor(rel string) float64 {
	return c.base.forClass(classOf(rel)).ceiling
}

type fileOverCeiling struct {
	rel     string
	ratio   float64
	ceiling float64
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
	// byClass is the set's own shape per directory class, so each part is held to the rate of the
	// population it belongs to rather than to one number for the whole repository.
	byClass map[string]stats
	// untouched counts files this change touched without adding a comment line. Their blocks are the
	// repository's, not the change's, so they are reported and never counted: one import edit to a
	// comment-heavy file would otherwise bring its whole mass into the numerator.
	untouched int
}

type fileMass struct {
	rel      string
	comments int
	notes    int
	code     int
	// written is the share of this file's landed comment lines that this change wrote. Authorship is
	// measured over the population being graded: a comment-only rewrite touches almost no code, so a
	// whole-file share would call it inherited and charge nothing for a mass entirely the change's own.
	// It is a fraction of the file as it stands, never a ratio of one delta to another.
	written float64
}

func (h hostRepo) measureChangeSet(paths []string, ceiling perFileCeiling, authored map[string]int) changeSet {
	var set changeSet
	set.byClass = map[string]stats{}
	_, set.read = h.measure(paths, func(rel string, file stats) {
		// A file this change touched by no comment line contributes nothing. Its comments are the
		// repository's and it is already in the baseline; counting it here charges the change for
		// prose it did not write.
		if !ceiling.isNew[rel] && authored[rel] == 0 {
			set.untouched++
			return
		}
		set.stats.add(file)
		holds := set.byClass[classOf(rel)]
		holds.add(file)
		set.byClass[classOf(rel)] = holds
		if ceiling.isOver(rel, file) {
			set.over = append(set.over, fileOverCeiling{rel: rel, ratio: file.noteRatio(), ceiling: ceiling.ratioFor(rel)})
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
			set.mass = append(set.mass, fileMass{rel: rel, comments: file.comments, notes: file.notes, code: file.code, written: written})
		}
	})
	return set
}

// authoredComments counts, per file, the comment lines this change added. Paired with the file's landed
// comment count it gives authorship as a share of what is there, which is bounded whichever way the
// change went — where a rate built from the diff alone inverts on a change that only deleted.
func (h hostRepo) authoredComments(revisions, changed []string) (map[string]int, error) {
	// Scoped to the files already resolved as changed, which both narrows the diff and supplies the `--`
	// that keeps a tree holding a file named like a revision from making the command ambiguous. Diff's
	// bare form must stay ambiguous there: it is how a path passed where a revision belongs is refused
	// rather than silently scanned against the index.
	named := revisions
	if len(named) == 0 {
		named = []string{"HEAD"}
	}
	args := append(append([]string{}, named...), "--")
	args = append(args, changed...)
	diff, err := gitOutput(h.root, append([]string{
		"-c", "core.quotePath=false", "diff", "--no-ext-diff", "--no-textconv", "--no-color",
		"--no-relative", "--text", "--src-prefix=a/", "--dst-prefix=b/",
	}, args...)...)
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

func bar(out console, args []string, cwd string, cfg Config) int {
	if err := diffscan.RefuseNonRevisions(args, cwd); err != nil {
		return out.refuseArguments(err)
	}
	host, err := newHostRepo(cwd, cfg.MaxFileBytes)
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
	base := host.measureBaseline(without(tracked, changed), carried, baseRev)
	// No baseline is refused, never defaulted: a number invented here reads exactly like one measured.
	if base.whole.files == 0 {
		return out.refuse(refusal("no file outside this change set carried countable lines, so the repo has no rate to hold it to"))
	}
	authored, err := host.authoredComments(revisions, changed)
	if err != nil {
		return out.refuse(err)
	}
	set := host.measureChangeSet(changed, perFileCeiling{isNew: isNew, base: base}, authored)
	out.note("%d changed source file(s), %d read, %d skipped unread; %d file(s) in the baseline.",
		len(changed), set.read, len(changed)-set.read, base.whole.files)
	if set.total() == 0 {
		return out.refuse(refusal("no changed source file could be read, so this run says nothing about the change set"))
	}
	return out.reportBar(base, set)
}

// Exit 1 means over the bar, and the report says how many lines: a share tells nobody what to delete. At
// most maxShown of the per-file lines are printed and the rest announced, for the reason at maxShown;
// every one of them is a finding.
func (c console) reportBar(base baselines, set changeSet) int {
	fmt.Fprintf(c.stdout, "measured by: comment-density build %s, tree %s\n", toolBuild(), toolTree())
	// Note lines, not comment lines: the first prose line of a block that sits on a declaration is its
	// summary, and the rule requires one where a name leaves something to say and forbids it paying
	// the bar. Both sides are measured the same way, so what the percentages compare is like with like.
	fmt.Fprintf(c.stdout, "host repo: %.1f%% note lines, %.1f-line mean block, %.0f%% of blocks over %d lines (%d file(s) in the baseline)\n",
		base.whole.stats.noteRatio()*100, base.whole.stats.meanBlock(), base.whole.stats.longShare()*100, longBlockLines, base.whole.files)
	fmt.Fprintf(c.stdout, "change set: %.1f%% note lines (%d note / %d code), %.1f-line mean block, %.0f%% of blocks over %d lines\n",
		set.noteRatio()*100, set.notes, set.code, set.meanBlock(), set.longShare()*100, longBlockLines)
	if set.untouched > 0 {
		fmt.Fprintf(c.stdout, "not counted: %d changed file(s) this change added no comment line to\n", set.untouched)
	}
	// One line per class the change touches, so a reader can see which population each part was held
	// to. Printed only where the change spans more than one, since otherwise it restates the line above.
	for _, class := range set.classes() {
		own := base.forClass(class)
		holds := set.byClass[class]
		if len(set.byClass) > 1 {
			fmt.Fprintf(c.stdout, "  %s/: %.1f%% against %.1f%% (%d file(s) in that part of the baseline)\n",
				shell.CutBytesMarked(shell.Oneline(class), maxPathBytes), holds.noteRatio()*100, own.stats.noteRatio()*100, own.files)
		}
	}

	findings := len(set.over)
	cut := cutAcrossClasses(set, base)
	if cut > 0 {
		findings++
		fmt.Fprintf(c.stdout, "over on lines: cut %d note line(s)\n", cut)
		if owed := cutAcrossClasses(changeSet{byClass: chargeableByClass(set)}, base); owed > 0 {
			fmt.Fprintf(c.stdout, "chargeable: %d note line(s), in the files this change wrote\n", owed)
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
	if allowed := (rate{numerator: base.whole.stats.longBlocks, denominator: base.whole.stats.blocks}).allowance(set.blocks); set.longBlocks > allowed {
		findings++
		fmt.Fprintf(c.stdout, "over on blocks: %d block(s) over %d lines against %d allowed\n", set.longBlocks, longBlockLines, allowed)
	}
	for i, file := range set.over {
		if i == maxShown {
			fmt.Fprintf(c.stdout, "… and %d further file(s) over the ceiling, not shown\n", len(set.over)-maxShown)
			break
		}
		// Its own test, and named as one: a file can sit under its class's rate and still be the
		// densest file in it. "nothing chargeable" beside this line otherwise reads as a contradiction.
		fmt.Fprintf(c.stdout, "%s: %.0f%% against a %.0f%% per-file ceiling\n",
			shell.CutBytesMarked(shell.Oneline(file.rel), maxPathBytes), file.ratio*100, file.ceiling*100)
	}
	if findings == 0 {
		return exitClean
	}
	return exitFound
}

// classes is the directory classes this change touches, in a fixed order so two runs over one tree
// print one report.
func (c changeSet) classes() []string {
	names := make([]string, 0, len(c.byClass))
	for class := range c.byClass {
		names = append(names, class)
	}
	sort.Strings(names)
	return names
}

// chargeableByClass weights each class's mass by how much of it this change wrote, so the figure a
// reader acts on is held to the same per-class rates as the overage it explains.
func chargeableByClass(set changeSet) map[string]stats {
	weighted := map[string]stats{}
	for _, file := range set.mass {
		holds := weighted[classOf(file.rel)]
		holds.add(stats{notes: int(float64(file.notes)*file.written + 0.5), code: int(float64(file.code)*file.written + 0.5)})
		weighted[classOf(file.rel)] = holds
	}
	return weighted
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
