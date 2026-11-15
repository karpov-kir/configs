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
			// A block removed from the file's top, or from between two blank lines, takes the blank
			// line after it, so the file opens on code and no gap doubles.
			last := c.last
			if last < len(lines) && strings.TrimSpace(lines[last]) == "" && (c.first == 1 || strings.TrimSpace(lines[c.first-2]) == "") {
				last++
			}
			edits = append(edits, edit{first: c.first, last: last})
		case "rewrite":
			// A rewritten block that a blank line parts from its declaration goes directly above it, so it
			// no longer reads as a file header by position. A file header keeps its blank line.
			last := c.last
			if !c.isHeader && c.decl > c.last {
				for last+1 < aboveDecorators(m.code, c.decl, m.hashComments) && strings.TrimSpace(lines[last]) == "" {
					last++
				}
			}
			edits = append(edits, edit{first: c.first, last: last, text: indent(d.text, indentOf(lines, c.first))})
		}
	}
	for _, p := range m.places {
		if d := byID[p.id]; d.verb == "add" {
			at := aboveDecorators(m.code, p.line, m.hashComments)
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

// aboveDecorators is the first line of the decorators and attributes a declaration carries, where its
// comment goes.
func aboveDecorators(lines []string, line int, hash bool) int {
	for opened := true; opened; {
		opened = false
		for at := line - 1; at >= 1 && at > line-1-maxDecoratorLines; at-- {
			if decoratorEnd(lines, at, hash) == line-1 {
				line, opened = at, true
				break
			}
		}
	}
	return line
}

// unchangedAsKept turns a rewrite that says the same words in the same comment form into a keep. A
// rewrite that only breaks the lines at other columns changes nothing a reader gets, and writing it
// would put a diff on every line of a comment the change never meant to touch. A comment over the
// bound of lines stays a rewrite, so the gate refuses the same words given back.
func unchangedAsKept(m material, decisions []decision) []decision {
	byID := map[string]candidate{}
	for _, c := range m.candidates {
		byID[c.id] = c
	}
	out := make([]decision, len(decisions))
	for i, d := range decisions {
		out[i] = d
		if c := byID[d.id]; d.verb == "rewrite" && !c.isLong && sameComment(strings.Split(c.text, "\n"), d.text) {
			out[i].verb, out[i].text = "keep", nil
		}
	}
	return out
}

// sameComment reports whether two comments carry the same words and open on the same marker.
// A doc block and a plain block differ, and so do `///` and `//`. Line breaks do not count.
func sameComment(was, now []string) bool {
	form := func(lines []string) string {
		if len(lines) == 0 {
			return ""
		}
		trimmed := strings.TrimLeft(lines[0], " \t")
		for _, open := range []string{"/**", "/*", "///", "//", "#"} {
			if strings.HasPrefix(trimmed, open) {
				return open
			}
		}
		return ""
	}
	words := func(lines []string) string {
		var all []string
		for _, line := range lines {
			all = append(all, strings.Fields(commentBody(line))...)
		}
		return strings.Join(all, " ")
	}
	return form(was) == form(now) && words(was) == words(now)
}
