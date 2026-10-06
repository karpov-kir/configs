package commentpass

import (
	"sort"
	"strings"
)

// edit replaces the lines from first to last with text. Lines count from 1, and both ends are
// included. An edit whose first line comes after its last inserts the text before the first.
type edit struct {
	first, last int
	text        []string
}

// applyDecisions turns the decisions into edits and applies them, last line first, so an earlier
// edit's lines keep their numbers. A comment's text takes the indent of the line it sits on.
func applyDecisions(lines []string, m material, decisions []decision) []string {
	byID := map[string]decision{}
	for _, d := range decisions {
		byID[d.id] = d
	}
	var edits []edit
	for _, c := range m.candidates {
		d := byID[c.id]
		switch d.verb {
		case "remove":
			edits = append(edits, edit{first: c.first, last: c.last})
		case "rewrite":
			edits = append(edits, edit{first: c.first, last: c.last, text: indent(d.text, indentOf(lines, c.first))})
		}
	}
	for _, p := range m.places {
		if d := byID[p.id]; d.verb == "add" {
			at := aboveDecorators(lines, p.line)
			edits = append(edits, edit{first: at, last: at - 1, text: indent(d.text, indentOf(lines, p.line))})
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].first > edits[j].first })
	out := append([]string{}, lines...)
	for _, e := range edits {
		tail := append(append([]string{}, e.text...), out[e.last:]...)
		out = append(out[:e.first-1], tail...)
	}
	return out
}

// indentOf is the leading whitespace of the line.
func indentOf(lines []string, line int) string {
	if line < 1 || line > len(lines) {
		return ""
	}
	text := lines[line-1]
	return text[:len(text)-len(strings.TrimLeft(text, " \t"))]
}

// indent sets each line of the comment at the declaration's indent.
func indent(text []string, prefix string) []string {
	out := make([]string, len(text))
	for i, line := range text {
		out[i] = prefix + strings.TrimLeft(line, " \t")
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "*") {
			out[i] = prefix + " " + strings.TrimLeft(line, " \t")
		}
	}
	return out
}

// aboveDecorators is the first line of the decorators a declaration carries, where its comment goes.
func aboveDecorators(lines []string, line int) int {
	for line > 1 && strings.HasPrefix(strings.TrimSpace(lines[line-2]), "@") {
		line--
	}
	return line
}
