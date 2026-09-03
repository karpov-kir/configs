package ecocheck

import (
	"regexp"
	"strconv"
	"strings"

	"kk-flavor/tools/shell"
)

// The head this scan's findings lead with, which report.go's rankTable ranks them on.
const bareRuleIDCitation = "bare rule-ID citation: "

var (
	// A rule named by its number, and the `##` heading that number opens. A literal space between the
	// words, never `[[:space:]]`: this is a phrase written in prose, and a tab or a line break between
	// them is not that phrase.
	bareRuleIDPattern   = regexp.MustCompilePOSIX(`[Cc]ore [Pp]rinciples? +#?[0-9]+`)
	numberedHeadingLine = regexp.MustCompilePOSIX(`^#+[[:space:]]+[0-9]+\.`)
	digitsRun           = regexp.MustCompilePOSIX(`[0-9]+`)
)

// The file whose rules those numbers belong to, named the way a citation names it.
const principlesRef = "core-principles.md"

// The form a finding falls back to when the tree holds no heading of the cited number. The citation
// is still dangling there, and a finding that cannot name the heading must still name the form.
const unresolvedHeading = "<the numbered heading>"

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
	for file, lines := range c.filesWithLines(c.root.Named(), "*.md") {
		safeFile := shell.Oneline(file)
		for _, hit := range grepNumbered(lines, bareRuleIDPattern) {
			resolving := headings[digitsRun.FindString(hit.match)]
			if resolving == "" {
				resolving = unresolvedHeading
			}
			c.add(bareRuleIDCitation + safeFile + ":" + strconv.Itoa(hit.line) + " — " + shell.Oneline(hit.match) +
				" resolves in no file; cite it as " + principlesRef + " → **" + shell.Oneline(resolving) +
				"** (writing.md → **Readability floor**)")
		}
	}
}

// How many numbered headings the lookup holds. The reviewed tree chose this file, and one committed
// 8 MB of numbered headings is otherwise carried in memory to answer a lookup that has a handful of
// answers.
const numberedHeadingCap = 64

// The numbered `##` headings of the principles file, by the number each one opens — the resolving
// form a finding names. The first heading of a number wins.
//
// Read only when it is a regular file: resolveRef tests with a stat that follows symlinks, so a
// committed `core-principles.md -> /dev/zero` would make the read never return. That is the trap
// reportCitation refuses its own target on.
func (c *checker) numberedHeadings() map[string]string {
	principles := c.resolveRef("", principlesRef)
	if !shell.IsRegularFile(principles) {
		return nil
	}
	headings := map[string]string{}
	for _, line := range c.unfencedLines(principles) {
		if !numberedHeadingLine.MatchString(line) || len(headings) >= numberedHeadingCap {
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
