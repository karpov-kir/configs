package ecoreport

import (
	"os"
	"strings"
)

// What counts as indentation on a markdown line here. The awk scan this replaced ran under LC_ALL=C,
// where `[[:space:]]` covers carriage return, vertical tab and form feed too. A fence or an item
// pushed right by one of those is still found.
const markdownIndent = " \t\r\v\f"

// Returns the open `- [ ]` items a markdown file carries, each under the heading it sits beneath, or
// an error when the file could not be read. An empty result and a read failure look the same here, so
// a caller that ignores the error lets a report it never read pass the merge gate. Checkboxes inside a
// fence or an HTML comment are skipped, so an example in a report does not block a merge.
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

// Returns the lines a reader of this markdown sees, and whether a fence or an HTML comment was still
// open at the end. The open-item scan reports what it could see, treating an unclosed fence as a typo
// too small to block every merge. The stage-result reader refuses such a file, because a line hidden
// there is a finding someone submitted.
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

// Reports whether the line is `^#{1,6} `, on the line as written. Markdown stops at six hashes, so a
// run of seven opens no section and its items keep the heading already in force. An indented `#` does
// not open one either.
func isMarkdownHeading(line string) bool {
	hashes := 0
	for hashes < len(line) && line[hashes] == '#' {
		hashes++
	}
	return hashes > 0 && hashes <= 6 && hashes < len(line) && line[hashes] == ' '
}
