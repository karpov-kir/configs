package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

var (
	headingLine   = regexp.MustCompilePOSIX(`^#+[[:space:]]`)
	headingMarker = regexp.MustCompilePOSIX(`^#+[[:space:]]*`)
)

// The lines of a markdown file a citation can resolve against: everything outside its fenced blocks,
// because a heading inside one is a code sample rather than a section.
func (c *checker) unfencedLines(path string) []string {
	lines, err := c.readLines(path)
	if err != nil {
		return nil
	}
	return unfenced(lines)
}

// The same, for a reader that already holds the lines. Whether a fence is skipped at all is each
// scan's own call — the direction scan reads inside one on purpose.
func unfenced(lines []string) []string {
	var outside []string
	inFence := false
	for _, line := range lines {
		if shell.IsFenceDelimiter(line) {
			inFence = !inFence
			continue
		}
		if !inFence {
			outside = append(outside, line)
		}
	}
	return outside
}

// The `**bolded**` runs on the lines that are not headings, keyed by comparison form and holding the
// text a finding quotes back. This is the near miss a citation makes most often, and the one a reader
// re-reading the file will not see, because a bolded lead-in and a heading look alike on the page.
//
// Read only where a citation has already failed. Every resolving citation would otherwise pay for a
// pass over its target's every line, to answer a question the resolving ones never ask.
func (c *checker) boldedRuns(path string) map[string]string {
	if cached, ok := c.bolded[path]; ok {
		return cached
	}
	bolded := map[string]string{}
	for _, line := range c.unfencedLines(path) {
		if headingLine.MatchString(line) {
			continue
		}
		for _, span := range delimitedSpans(line, "**") {
			bolded[plainText(span)] = span
		}
	}
	if c.bolded == nil {
		c.bolded = map[string]map[string]string{}
	}
	c.bolded[path] = bolded
	return bolded
}

// Every `#` heading in a markdown file, keyed by comparison form and holding the text a finding
// quotes back — the only thing a citation resolves against.
// Memoised per path, like the tree walks are, because the citation scan asks this once per citation
// and the reviewed tree writes both sides: 200 citations into a 6.3 MB target took 64 seconds and 3000
// never finished, off a 114 KB file and a 6 MB one, each far under the read bound on its own. The maps
// are strictly smaller than the bytes they were parsed from, and every caller only reads them.
func (c *checker) markdownHeadings(path string) map[string]string {
	if cached, ok := c.headings[path]; ok {
		return cached
	}
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
		if subtitled := shell.BeforeEmDash(heading); subtitled != "" {
			forms = append(forms, subtitled)
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
			if numberless := shell.WithoutLeadingNumber(form); numberless != "" {
				headings[numberless] = written
			}
		}
	}
	if c.headings == nil {
		c.headings = map[string]map[string]string{}
	}
	c.headings[path] = headings
	return headings
}
