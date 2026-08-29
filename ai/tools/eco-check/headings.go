package ecocheck

import (
	"regexp"
	"strings"

	"kk-flavor/tools/shell"
)

var (
	headingLine   = regexp.MustCompilePOSIX(`^#+[[:space:]]`)
	headingMarker = regexp.MustCompilePOSIX(`^#+[[:space:]]*`)
	// The number a heading is written under, `## 7. What a suite reports`. Matched against a heading
	// already in comparison form, where a whitespace run is one space, so a single space is exact.
	leadingHeadingNumber = regexp.MustCompilePOSIX(`^[0-9]+\. `)
)

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
