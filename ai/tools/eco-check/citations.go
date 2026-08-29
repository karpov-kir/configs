package ecocheck

import (
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

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
