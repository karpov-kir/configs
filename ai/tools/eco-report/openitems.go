package ecoreport

import (
	"os"
	"strings"
)

// What indents a markdown line here. The scan this replaced ran awk under LC_ALL=C, whose
// `[[:space:]]` covers the carriage return, the vertical tab and the form feed as well as space and
// tab, so a fence or an item pushed right by one of those is still seen.
const markdownIndent = " \t\r\v\f"

// The open `- [ ]` a markdown file still carries, each under the heading it sits beneath, and an
// error when the file could not be read at all. A caller has to tell those two apart: no items and an
// unread file both come back with nothing to print, and read as "nothing open" a report nothing
// opened passes the merge gate still holding unrouted findings.
//
// Fence- and comment-aware, so a checkbox written as an EXAMPLE in a report never blocks a merge.
func openItemsIn(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	visible, _ := visibleMarkdownLines(string(body))
	section := ""
	var found []string
	for _, line := range visible {
		if isMarkdownHeading(line) {
			section = line
			continue
		}
		if item := strings.TrimLeft(line, markdownIndent); strings.HasPrefix(item, "- [ ]") {
			found = append(found, section+" | "+item)
		}
	}
	return strings.Join(found, "\n"), nil
}

// The lines a reader of this markdown actually sees, plus whether a fence or a comment was still open
// at the end of it. Two readers walk this and they answer that flag differently: the open-item scan
// reports what it could see, since a fence nobody closed is a typo and blocking every merge behind one
// is worse than missing an item; the stage-result reader refuses the file, because there the hidden
// line is a finding someone submitted.
func visibleMarkdownLines(body string) (visible []string, leftOpen bool) {
	inFence, inComment := false, false
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimLeft(line, markdownIndent); strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.Contains(line, "<!--") {
			inComment = true
		}
		if inComment {
			if strings.Contains(line, "-->") {
				inComment = false
			}
			continue
		}
		visible = append(visible, line)
	}
	return visible, inFence || inComment
}

// `^#{1,6} `, on the line as written. Markdown stops at six hashes, so a seventh does not open a
// section and the items under it keep the heading they were already under. An indented `#` is not a
// heading either.
func isMarkdownHeading(line string) bool {
	hashes := 0
	for hashes < len(line) && line[hashes] == '#' {
		hashes++
	}
	return hashes > 0 && hashes <= 6 && hashes < len(line) && line[hashes] == ' '
}
