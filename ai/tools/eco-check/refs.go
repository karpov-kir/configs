package ecocheck

import (
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

var (
	// Wider than the shell.LinkTargets budget.go uses: that one excludes `#`, because a budget target
	// with a fragment names no file to count, while a citation with one names a file *and* a section,
	// both checked here. So this admits the fragment and cuts it off itself below.
	markdownLinkPattern = regexp.MustCompilePOSIX(`\]\([^)]+\)`)
	homeRefPattern      = regexp.MustCompilePOSIX(`~/\.(kk-flavor|claude/skills)/[A-Za-z0-9._/-]+`)
	backtickedPathToken = regexp.MustCompilePOSIX(`^([A-Za-z0-9][A-Za-z0-9._/-]*/[A-Za-z0-9._-]+\.(sh|md)|[A-Za-z0-9][A-Za-z0-9._-]*\.sh|[A-Z][A-Z0-9]*(-[A-Z0-9]+)+\.md)$`)
	headingLine         = regexp.MustCompilePOSIX(`^#+[[:space:]]`)
	headingMarker       = regexp.MustCompilePOSIX(`^#+[[:space:]]*`)
	skillFamilyToken    = regexp.MustCompile(`\b(kk|idsd)-[a-z0-9-]+`)
	// The cited file at the head of a `<file> → <Section>`, in its three written forms.
	trailingLinkTarget = regexp.MustCompilePOSIX(`\]\([^()]*\)$`)
	trailingBarePath   = regexp.MustCompilePOSIX(`[A-Za-z0-9._/-]+$`)
	markdownFileTail   = regexp.MustCompilePOSIX(`[A-Za-z0-9]\.md$`)
	sectionCutPoint    = regexp.MustCompilePOSIX(`[():;,.!?"]`)
	// A rule named by its number, and the `##` heading that number opens. A literal space between the
	// words, never `[[:space:]]`: this is a phrase written in prose, and a tab or a line break between
	// them is not that phrase.
	bareRuleIDPattern   = regexp.MustCompilePOSIX(`[Cc]ore [Pp]rinciples? +#?[0-9]+`)
	numberedHeadingLine = regexp.MustCompilePOSIX(`^#+[[:space:]]+[0-9]+\.`)
	digitsRun           = regexp.MustCompilePOSIX(`[0-9]+`)
	// The number a heading is written under, `## 7. What a suite reports`. Matched against a heading
	// already in comparison form, where a whitespace run is one space, so a single space is exact.
	leadingHeadingNumber = regexp.MustCompilePOSIX(`^[0-9]+\. `)
	// A markdown file named in prose, whichever of the three forms wrote it: the `](…)` and the
	// backticks fall outside the character class, so one pattern reads all three. Anchored at the end,
	// because it is matched against a window that ends on the `.md` being tested.
	//
	// The leading run is optional, and that is load-bearing rather than tidy: required, the shortest
	// name it can match is `aa.md`, so `a.md` matches nothing and every probe past it fails.
	markdownFileTokenAtEnd = regexp.MustCompilePOSIX(`([A-Za-z0-9~][A-Za-z0-9._/~-]*)?[A-Za-z0-9]\.md$`)
)

// The file whose rules those numbers belong to, named the way a citation names it.
const principlesRef = "core-principles.md"

// The form a finding falls back to when the tree holds no heading of the cited number. The citation
// is still dangling there, and a finding that cannot name the heading must still name the form.
const unresolvedHeading = "<the numbered heading>"

// What separates a cited file from the section it names.
const sectionArrow = "→"

// Relative markdown links, resolved against the linking file's own directory. A template's links
// resolve where it is emitted (a project's `.idsd/`), so a bare sibling name is unverifiable and
// passes; only a traversal out of the emitted directory is checkable there.
func (c *checker) scanDanglingLinks() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md") {
		isTemplate := strings.Contains(file, "/templates/")
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		dir := shell.DirName(file)
		for _, line := range lines {
			for _, match := range markdownLinkPattern.FindAllString(line, -1) {
				link := strings.TrimSuffix(strings.TrimPrefix(match, "]("), ")")
				if strings.HasPrefix(link, "http") || strings.HasPrefix(link, "mailto:") ||
					strings.HasPrefix(link, "#") || strings.HasPrefix(link, "~") {
					continue
				}
				target, _, _ := strings.Cut(link, "#")
				if shell.PathExists(shell.Join(dir, target)) {
					continue
				}
				if isTemplate && !isTraversal(target) {
					continue
				}
				c.add("dangling link: " + shell.Oneline(file) + " -> " + shell.Oneline(link))
			}
		}
	}
}

func isTraversal(target string) bool {
	return strings.HasPrefix(target, "/") || strings.HasPrefix(target, "../") ||
		strings.Contains(target, "/../") || target == ".."
}

// `~/.kk-flavor/...` and `~/.claude/skills/...` — how a skill reaches outside its own directory.
func (c *checker) scanHomeRefs() {
	var refs []string
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		for _, line := range lines {
			for _, match := range homeRefPattern.FindAllString(line, -1) {
				// The trailing run of sentence punctuation is where the prose ended, not part of
				// the path.
				refs = append(refs, strings.TrimRight(match, ".,;:"))
			}
		}
	}
	for _, ref := range shell.SortUnique(refs) {
		if c.resolveRef("", ref) == "" {
			c.add("dangling home ref: " + shell.Oneline(ref))
		}
	}
}

// Backticked in-repo paths — `scripts/report.sh`, `templates/ice-template.md`, `AGENT-BRIEF.md`.
// Fenced blocks are skipped. The shapes stay narrow on purpose: a bare lowercase `*.md` is as often
// a file a project owns (`charter.md`, `roadmap.md`), so only SHOUTY-with-a-hyphen is matched.
func (c *checker) scanPathRefs() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		dir := shell.DirName(file)
		// A skill cites its own tooling from the skill root (`scripts/report.sh`) even in a file
		// that sits under `scripts/`, so resolve from both.
		skillRoot := dir
		if rest, ok := strings.CutPrefix(file, c.root.Skills()+"/"); ok {
			first, _, _ := strings.Cut(rest, "/")
			skillRoot = shell.Join(c.root.Skills(), first)
		}
		var tokens []string
		for _, span := range backtickedSpans(lines) {
			if backtickedPathToken.MatchString(span) {
				tokens = append(tokens, span)
			}
		}
		for _, token := range shell.SortUnique(tokens) {
			if c.refExists(dir, token) {
				continue
			}
			if skillRoot != dir && c.refExists(skillRoot, token) {
				continue
			}
			c.add("dangling path ref: " + shell.Oneline(file) + " -> " + shell.Oneline(token))
		}
	}
}

// The text inside each pair of backticks, outside fenced blocks.
func backtickedSpans(lines []string) []string {
	var spans []string
	inFence := false
	for _, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		spans = append(spans, delimitedSpans(line, "`")...)
	}
	return spans
}

// The text inside each pair of the given delimiter on one line. Split on the delimiter rather than
// shrinking the line: rebuilding the tail on every hit is quadratic in a line length the tree
// chooses, and one committed multi-megabyte line would stall the whole check.
func delimitedSpans(line, delimiter string) []string {
	var spans []string
	parts := strings.Split(line, delimiter)
	for k := 0; k <= len(parts)-3; {
		if parts[k+1] != "" {
			spans = append(spans, parts[k+1])
			k += 2
			continue
		}
		k++
	}
	return spans
}

type citation struct {
	src  string
	line int
	// The cited file. Empty when nothing before the arrow named one and nothing earlier in the block
	// could stand in, which is the citation reportCitation refuses rather than resolves.
	path string
	// What stood immediately before the arrow, for the finding that has no path to name.
	head        string
	section     string
	isDelimited bool
}

// The other half of a `<file> → <Section>` citation: the heading it names must still be there. An
// arrow counts only when the text before it resolves to a real markdown file, which keeps prose
// arrows ("intent → build") out.
func (c *checker) scanCitations() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.sh") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		for _, cited := range citationsIn(file, lines) {
			c.reportCitation(cited)
		}
	}
}

// What a citation finding against a test harness ends on. A suite covering anything citation-shaped
// has to put a citation in a fixture, and a fixture written as a literal line is content this scan
// reads: the suite then reports its own test data against the checkout, from a case that passed. Four
// suites have paid for that diagnosis.
//
// The rule is that such a fixture is assembled at run time, and this is where it is said — in the
// finding its author is already looking at, rather than in a convention they had to know first.
//
// An exemption for these files was the alternative, and the tree refused it: no suite here carries a
// citation at all, so the exemption would buy nothing, while `shell-mutate.sh` carries a real one in
// its body that the exemption would silently stop checking. A per-line marker was the other, and it
// cannot reach the shape that causes this — a fixture inside a heredoc has no room for a marker, and
// a citation in a heredoc is exactly what a markdown fixture is. So the cost taken is that a harness
// may carry no citation literal at all, ever, even where a heredoc would have been the plain way to
// write one.
const harnessCitationNote = " — this file is a test harness: if that citation is fixture text, assemble it at run time instead of writing it out"

// Findings from this scan go through here so a harness pays the note once, wherever it was reported.
func (c *checker) addCitationFinding(cited citation, finding string) {
	if isTestHarness(cited.src) {
		finding += harnessCitationNote
	}
	c.add(finding)
}

func (c *checker) reportCitation(cited citation) {
	position := shell.Oneline(cited.src) + ":" + strconv.Itoa(cited.line)
	// A citation the scanner cannot see is worse than one it reports wrong: nothing reads the section,
	// so the rename that breaks it leaves this gate green. The head is a backticked run naming no
	// markdown file — `kk-qualify` → **The residue** — and the section arrived in the `**` form
	// ecosystem.md → **Conventions a new file joins** mandates, which together is the shape that reads
	// as a citation to a person and resolves as nothing here.
	if cited.path == "" {
		c.addCitationFinding(cited, "uncheckable citation: "+position+" -> `"+shell.CutBytes(shell.Oneline(cited.head), 60)+
			"` → "+shell.Oneline(cited.section)+
			" — the text before the arrow names no markdown file, so nothing checks that section; write the file's path")
		return
	}
	target := c.resolveRef(shell.DirName(cited.src), cited.path)
	if target == "" {
		c.addCitationFinding(cited, "unresolvable citation path: "+position+" -> "+shell.Oneline(cited.path))
		return
	}
	// Reported even when the section resolves today: undelimited is how it stops resolving in
	// silence. An undelimited name truncates at the first comma, so half a heading satisfies the
	// citation and a rename then breaks it without a word.
	if !cited.isDelimited {
		c.addCitationFinding(cited, "undelimited section citation: "+position+" -> "+shell.Oneline(cited.path)+
			" → "+shell.Oneline(cited.section)+" is not wrapped in ** or backticks")
	}
	// The cited path resolved through whatever the reviewed tree pointed it at, and pathExists
	// follows a symlink: `evil.md -> /dev/zero` or a committed FIFO makes the read below never
	// return. Reported rather than skipped: a target nothing read must not be indistinguishable from
	// a checked one.
	if !shell.IsRegularFile(target) {
		c.addCitationFinding(cited, "citation target is not a regular file: "+position+" -> "+shell.Oneline(cited.path)+
			" — it was NOT read")
		return
	}
	headings := c.markdownHeadings(target)
	// Prose runs on past the heading it names, so accept the longest leading run that is a heading.
	named := plainText(cited.section)
	for want := named; want != ""; {
		if _, isHeading := headings[want]; isHeading {
			return
		}
		cut := strings.LastIndexByte(want, ' ')
		if cut < 0 {
			break
		}
		want = want[:cut]
	}
	c.addCitationFinding(cited, "dangling section ref: "+position+" -> "+shell.Oneline(cited.path)+" → "+
		shell.Oneline(cited.section)+" — "+c.danglingVariant(cited, target, named, headings))
}

// Which of the four ways a section citation dangles this one is. All four read as correct to a human
// and only this scan finds them, so a message that says "dangling" and stops leaves the reader to
// hunt through the cited file for a difference they have already failed to see once.
//
// Each variant is decided from what is in hand where the citation failed — the name as written and
// the cited file's own sections — and the numbered one quotes the citation that resolves, because
// both strings are right here and a form to paste beats a form to work out.
func (c *checker) danglingVariant(cited citation, target, named string, headings map[string]string) string {
	if numberless := withoutLeadingNumber(named); numberless != "" {
		if resolving, isHeading := headings[numberless]; isHeading {
			return "that heading is numbered differently — cite it as " + shell.Oneline(cited.path) +
				" → **" + shell.Oneline(resolving) + "**"
		}
	}
	if written, isBolded := c.boldedRuns(target)[named]; isBolded {
		return "**" + shell.Oneline(written) + "** is bolded text there, not a heading — cite the heading above it, or make it one"
	}
	if elsewhere := c.fileWithHeading(named); elsewhere != "" {
		return "that heading is in " + shell.Oneline(elsewhere) + " — the section did not move, the citation did"
	}
	return "that file carries no heading and no bolded run of that name — it reads like a paraphrase of one"
}

// The bounds the tree-wide heading index is built under. It only ever enriches a finding already
// being printed, so hitting either bound costs a variant name and never a finding: the lookup
// answers with nothing and danglingVariant falls through to a sentence that claims nothing about
// the rest of the tree. A committed file of 8 MiB of headings is why they are here at all.
const (
	headingIndexCap = 4096
	headingKeyCap   = 200
)

// The first markdown file under the root that holds a heading of this name. This is the variant
// nothing else can name: the section is real, the file it used to be in is real, and the citation
// reads as correct in every form.
//
// The cited file needs no excluding here. Reaching this point means the citation matched none of that
// file's heading keys, and the index holds those same keys, so the file the citation named is the one
// answer this lookup cannot give.
//
// Built on the first dangling citation and never for a clean tree, then held: the alternative is a
// walk of every markdown file per failing citation.
func (c *checker) fileWithHeading(named string) string {
	if c.headingOwners == nil {
		c.headingOwners = map[string]string{}
		c.indexHeadingOwners()
	}
	return c.headingOwners[named]
}

func (c *checker) indexHeadingOwners() {
	for _, file := range c.filesNamed(c.root.Named(), "*.md") {
		for heading, written := range c.markdownHeadings(file) {
			if len(c.headingOwners) >= headingIndexCap {
				return
			}
			if _, seen := c.headingOwners[heading]; !seen && len(written) <= headingKeyCap {
				c.headingOwners[heading] = file
			}
		}
	}
}

// The lines of a markdown file a citation can resolve against: everything outside its fenced blocks,
// because a heading inside one is a code sample rather than a section.
func (c *checker) unfencedLines(path string) []string {
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}
	var unfenced []string
	inFence := false
	for _, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if !inFence {
			unfenced = append(unfenced, line)
		}
	}
	return unfenced
}

// The `**bolded**` runs on the lines that are not headings, keyed by comparison form and holding the
// text a finding quotes back. This is the near miss a citation makes most often, and the one a reader
// re-reading the file will not see, because a bolded lead-in and a heading look alike on the page.
//
// Read only where a citation has already failed. Every resolving citation would otherwise pay for a
// pass over its target's every line, to answer a question the resolving ones never ask.
func (c *checker) boldedRuns(path string) map[string]string {
	bolded := map[string]string{}
	for _, line := range c.unfencedLines(path) {
		if headingLine.MatchString(line) {
			continue
		}
		for _, span := range delimitedSpans(line, "**") {
			bolded[plainText(span)] = span
		}
	}
	return bolded
}

// Every `#` heading in a markdown file, keyed by comparison form and holding the text a finding
// quotes back — the only thing a citation resolves against.
func (c *checker) markdownHeadings(path string) map[string]string {
	headings := map[string]string{}
	for _, line := range c.unfencedLines(path) {
		if !headingLine.MatchString(line) {
			continue
		}
		written := strings.Trim(headingMarker.ReplaceAllString(line, ""), shell.SpaceBytes)
		heading := plainText(written)
		// A heading may carry a subtitle after an em dash and a citation names only the run before
		// it, so accept that run too. Cut at the em dash and nowhere else: a trailing run, or a
		// word-by-word prefix, would let half a heading satisfy a citation.
		forms := []string{heading}
		if i := strings.Index(heading, " — "); i >= 0 {
			forms = append(forms, heading[:i])
		}
		// The second carve-out, and the last: a heading numbered `## 7. What a suite reports` is cited
		// by its text. Registered here rather than matched at the citation, because the matcher trims
		// *trailing* words off a citation to find a heading, and a leading token is the one thing
		// trimming from the right walks away from. The shape was not one the checker disliked, it was
		// one nothing could resolve, so the two files in this tree that number their headings read as
		// hard dangling refs while every em-dash heading passed.
		//
		// The em-dash comment's warning is about a *run*: a rule cutting at any trailing word, or at
		// any word boundary, admits every prefix of a heading, and a citation naming three words of a
		// nine-word heading then resolves. This rule cuts a fixed affix — the digits, the dot and the
		// space that open the line — and nothing else, so the set it adds per heading is one string,
		// not a prefix family. Naming part of a numbered heading's text still resolves nowhere, which
		// TestNumberedHeadingCitations proves against this same heading.
		for _, form := range forms {
			headings[form] = written
			if numberless := withoutLeadingNumber(form); numberless != "" {
				headings[numberless] = written
			}
		}
	}
	return headings
}

// A numbered heading's text without the `7. ` that opens it, and empty when the heading opens with no
// number — an empty key would answer a citation that named nothing.
func withoutLeadingNumber(heading string) string {
	numberless := leadingHeadingNumber.ReplaceAllString(heading, "")
	if numberless == heading {
		return ""
	}
	return numberless
}

func citationsIn(file string, lines []string) []citation {
	var found []citation
	for _, block := range citationBlocks(lines) {
		found = append(found, block.citations(file)...)
	}
	return found
}

// A run of adjacent lines one citation may be spread across, joined by the newline between them, and
// the 1-based number of the first of them.
type citationBlock struct {
	firstLine int
	text      string
}

// The blocks of a file. A citation wraps: a hard line break between the cited file and its section,
// or inside the section name, is what a formatter leaves behind, and reading one line at a time
// cannot see across it — the citation resolves against nothing and the scan goes quiet, which is the
// one outcome a dangling-reference scan must not have.
//
// A blank line ends a block, and so does a fence. A paragraph break is not a wrap, and joining across
// one would let a file named in one paragraph answer an arrow in the next: a citation nobody wrote.
func citationBlocks(lines []string) []citationBlock {
	var blocks []citationBlock
	var current []string
	firstLine := 0
	inFence := false
	closeBlock := func() {
		if len(current) > 0 {
			blocks = append(blocks, citationBlock{firstLine: firstLine, text: strings.Join(current, "\n")})
			current = nil
		}
	}
	for i, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			closeBlock()
			continue
		}
		if inFence || strings.Trim(line, shell.SpaceBytes) == "" {
			closeBlock()
			continue
		}
		if len(current) == 0 {
			firstLine = i + 1
			current = append(current, line)
			continue
		}
		current = append(current, continuationText(line))
	}
	closeBlock()
	return blocks
}

// What a continued line contributes to its block. Its indentation is layout, and a leading `#` is the
// marker of the comment or the heading the citation was written inside — a script cites in a comment,
// so that marker rides onto the continuation line while belonging to neither the path nor the section
// name. citedSection strips a heading marker off a section for the same reason.
func continuationText(line string) string {
	indented := strings.TrimLeft(line, shell.SpaceBytes)
	unmarked := strings.TrimLeft(indented, "#")
	if len(unmarked) == len(indented) {
		return indented
	}
	return strings.TrimLeft(unmarked, shell.SpaceBytes)
}

// The citations in one block, each reported at the line its arrow sits on — where a reader looks for
// it, and the line the unwrapped form has always been reported at.
func (b citationBlock) citations(file string) []citation {
	var found []citation
	segments := strings.Split(b.text, sectionArrow)
	line := b.firstLine
	carried := ""
	for i := 1; i < len(segments); i++ {
		// Both of these are carried along the split rather than recounted from the head of the block:
		// a file choosing to carry thousands of arrows would otherwise cost a pass over the whole
		// block for each one.
		line += strings.Count(segments[i-1], "\n")
		if named := lastMarkdownFileIn(segments[i-1]); named != "" {
			carried = named
		}
		section, isDelimited, isBold := citedSection(segments[i])
		if section == "" {
			continue
		}
		head := headBefore(segments[i-1])
		cited := citation{
			src:         file,
			line:        line,
			path:        head.path,
			head:        head.token,
			section:     section,
			isDelimited: isDelimited,
		}
		if head.path == "" {
			// Nothing immediately before the arrow named a file, and only the mandated `**` form is
			// read as a citation from here on: `intent → **build**` is prose, but so is every other
			// arrow in this tree whose right side is bare or backticked, and admitting those turns
			// every prose arrow near a filename into a finding.
			if !isBold {
				continue
			}
			// The file named earlier in the same block, which is how this tree writes a second
			// citation into a file it has already named: `You run under `x.md` … (→ **Section**)`.
			// Without this the arrow resolves against nothing and the scan says nothing — the silence
			// a dangling-reference scan may not have. Bounded to the block for the reason the block
			// exists: a file named in one paragraph must not answer an arrow in the next.
			cited.path = carried
			// Still nothing, and the head was backticked: that is the uncheckable citation, and
			// reportCitation names it. A bare head with no file in the block is prose that happened to
			// bold its right side, so it stays out.
			if cited.path == "" && !head.isBackticked {
				continue
			}
		}
		found = append(found, cited)
	}
	return found
}

// The two bounds the search below runs under, both because the reviewed tree writes the text. The
// first is the longest path read back as a cited file: past it the window is cut, so a committed path
// longer than any anyone writes yields a shorter name that resolves nowhere rather than a scan
// reading megabytes backwards. The second is how many `.md` occurrences are tried before the answer
// is taken to be none — a paragraph of a million bare `.md` tokens forms no filename at all, and each
// one costs a match against the window it sits in.
const (
	maxNamedPathBytes = 1024
	maxNameProbes     = 64
)

// The last markdown file named anywhere in a run of text, in any of the three written forms.
//
// Searched backwards from the end inside a bounded window, never by collecting every match: the text
// is a whole block, the reviewed tree chooses how long a block is, and one committed paragraph of a
// million `.md` tokens would otherwise be held in memory as a million matched strings.
func lastMarkdownFileIn(text string) string {
	for at, probes := len(text), 0; at > 0 && probes < maxNameProbes; probes++ {
		hit := strings.LastIndex(text[:at], ".md")
		if hit < 0 {
			return ""
		}
		at = hit
		end := hit + len(".md")
		// `a.mdx` ends on no markdown file: the suffix has to be where the name ends.
		if end < len(text) && isPathByte(text[end]) {
			continue
		}
		start := max(hit-maxNamedPathBytes, 0)
		if named := markdownFileTokenAtEnd.FindString(text[start:end]); named != "" {
			return named
		}
	}
	return ""
}

func isPathByte(b byte) bool {
	return isAlnumByte(b) || b == '.' || b == '_' || b == '-' || b == '/' || b == '~'
}

// What stood immediately before an arrow.
type citedHead struct {
	// The cited file: a markdown link, a backticked path, or a bare filename. Empty unless the token
	// is a markdown filename, which is what keeps a prose arrow out.
	path string
	// That token whichever way the test went, so a finding with no path to name has something to
	// quote back.
	token string
	// Whether the token arrived inside backticks. A backticked run that is not a markdown file is
	// written as a reference and read as none, which is the one head shape worth a finding of its own.
	isBackticked bool
}

func headBefore(before string) citedHead {
	before = strings.TrimRight(before, shell.SpaceBytes)
	// The cited file sits immediately before the arrow, so only the line the text ends on can carry
	// it. Cut to that line rather than let the patterns below reach back over a wrap: `$` under
	// MustCompilePOSIX is the end of a *line*, so `[A-Za-z0-9._/-]+$` over several of them answers
	// with the leftmost line-final token — `bash` out of a script's `#!` line, never the cited path.
	if wrap := strings.LastIndexByte(before, '\n'); wrap >= 0 {
		before = before[wrap+1:]
	}
	head := citedHead{}
	switch {
	case trailingLinkTarget.MatchString(before):
		open := strings.LastIndex(before, "](")
		head.token = strings.TrimSuffix(before[open+2:], ")")
	case strings.HasSuffix(before, "`"):
		head.isBackticked = true
		unticked := before[:len(before)-1]
		if tick := strings.LastIndexByte(unticked, '`'); tick >= 0 {
			head.token = unticked[tick+1:]
		}
	default:
		head.token = trailingBarePath.FindString(before)
	}
	head.path, _, _ = strings.Cut(head.token, "#")
	if !markdownFileTail.MatchString(head.path) {
		head.path = ""
	}
	return head
}

// Whether the name arrived inside `**` or backticks is the whole decision here: read exactly, or
// guessed at by the fallback. ecosystem.md → **Conventions a new file joins** requires the
// delimited form for that reason.
//
// The `**` half is told from the backticked half as well, because it is the form that file mandates
// and the only one a head naming no file is read through: a bare or backticked right side after a
// prose arrow is how this tree writes prose, and reading those as citations would report the tree
// against itself.
func citedSection(after string) (section string, isDelimited, isBold bool) {
	after = strings.TrimLeft(after, shell.SpaceBytes)
	if rest, ok := strings.CutPrefix(after, "**"); ok {
		if end := strings.Index(rest, "**"); end > 0 {
			section = rest[:end]
			isBold = true
		}
	} else if rest, ok := strings.CutPrefix(after, "`"); ok {
		if end := strings.IndexByte(rest, '`'); end > 0 {
			section = rest[:end]
		}
	}
	isDelimited = section != ""
	if section == "" {
		section = after
		// An undelimited name carries no end marker, so the line it starts on is the only bound it
		// has. The delimited forms above carry their own and may wrap; this one may not.
		if newline := strings.IndexByte(section, '\n'); newline >= 0 {
			section = section[:newline]
		}
		if cut := sectionCutPoint.FindStringIndex(section); cut != nil {
			section = section[:cut[0]]
		}
		if dash := strings.Index(section, "—"); dash > 0 {
			section = section[:dash]
		}
	}
	section = strings.NewReplacer("`", "", "*", "").Replace(section)
	section = headingMarker.ReplaceAllString(section, "")
	return strings.Trim(section, shell.SpaceBytes), isDelimited, isBold
}

// A rule cited by its number, `Core Principle 3`. Each rule in that list has a `##` heading, so a
// citation to it has the form scanCitations resolves. The numbered form resolves in no file: nothing
// checks it, a renumbering silently repoints it, and a reader who follows it finds no heading of that
// name (writing.md → **Readability floor**). So the finding names the heading to write instead,
// because a finding a reader has to research first is one they park.
//
// One phrase, deliberately, and only in `.md`. `Phase 3`, `step 2` and `rule 4` are how skills cite
// their own headings and list items, all legitimate, so any shape wide enough to catch a bare number
// reports them; here a false positive costs more than the citations a wider net would catch.
//
// Fences are not skipped, as in the direction scan: the wrong form steers its reader from inside one
// too. The headings read *out of* the principles file do skip them, because a heading inside a fence
// is not one scanCitations resolves, and the form this finding names has to resolve.
func (c *checker) scanBareRuleIDs() {
	headings := c.numberedHeadings()
	for _, file := range c.filesNamed(c.root.Named(), "*.md") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		safeFile := shell.Oneline(file)
		for _, hit := range grepNumbered(lines, bareRuleIDPattern) {
			lineNumber, matched, _ := strings.Cut(hit, ":")
			resolving := headings[digitsRun.FindString(matched)]
			if resolving == "" {
				resolving = unresolvedHeading
			}
			c.add("bare rule-ID citation: " + safeFile + ":" + lineNumber + " — " + shell.Oneline(matched) +
				" resolves in no file; cite it as " + principlesRef + " → **" + shell.Oneline(resolving) +
				"** (writing.md → **Readability floor**)")
		}
	}
}

// The numbered `##` headings of the principles file, by the number each one opens — the resolving
// form a finding names. The first heading of a number wins, and at most 64 are held: the reviewed
// tree chose this file, and one committed 8 MB of numbered headings is otherwise carried in memory to
// answer a lookup that has a handful of answers.
//
// Read only when it is a regular file: resolveRef tests with a stat that follows symlinks, so a
// committed `core-principles.md -> /dev/zero` would make the read never return. That is the trap
// reportCitation refuses its own target on.
//
// The variable is not named `target`, because the mutation harness anchors `refs: citation target
// read with no regular-file test` on reportCitation's otherwise-identical line, and an anchor
// matching twice stops that harness running at all.
func (c *checker) numberedHeadings() map[string]string {
	principles := c.resolveRef("", principlesRef)
	if !shell.IsRegularFile(principles) {
		return nil
	}
	lines, err := c.readLines(principles)
	if err != nil {
		return nil
	}
	headings := map[string]string{}
	inFence := false
	for _, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if inFence || !numberedHeadingLine.MatchString(line) || len(headings) >= 64 {
			continue
		}
		text := headingMarker.ReplaceAllString(line, "")
		number, _, _ := strings.Cut(text, ".")
		if _, seen := headings[number]; !seen {
			headings[number] = text
		}
	}
	return headings
}

// Our own skill namespaces: a name in prose must be a skill that exists.
func (c *checker) scanUnknownSkills() {
	var names []string
	for _, file := range c.filesNamed(c.root.Named(), "*.md", "*.yaml") {
		lines, err := c.readLines(file)
		if err != nil {
			continue
		}
		for _, line := range lines {
			for _, match := range skillFamilyToken.FindAllString(line, -1) {
				names = append(names, strings.TrimSuffix(match, "-"))
			}
		}
	}
	for _, name := range shell.SortUnique(names) {
		// `kk-flavor` is the shared layer, not a skill.
		if name == "kk-flavor" || shell.IsRegularFile(shell.Join(shell.Join(c.root.Skills(), name), "SKILL.md")) {
			continue
		}
		// Two readings, and the scan cannot tell them apart: a misspelled skill, or prose that
		// happens to wear the family's shape (`kk-drive-verified`). Suppressing a token whose
		// prefix is a real skill would mask the first — `kk-drives` is exactly that shape — so the
		// message carries both readings instead.
		c.add("unknown skill referenced: " + shell.Oneline(name) + " — no skills/" + shell.Oneline(name) +
			"/SKILL.md. If this is prose rather than a skill, reword it so it does not read as one")
	}
}
