package commentpass

import "strings"

// A prose line past its allowance is wrapped at word boundaries, as a formatter would, and the gate
// then checks width and length as before. The wrap keeps the line's own marker: `//`, `#` and a
// block's `*` lines go on with the same marker, and a one-line `/** … */` or `/* … */` opens into a
// block. A word longer than the allowance stays whole, and the gate refuses its line.

// rewrap wraps each written comment's over-wide lines to the columns its id may take.
func rewrap(m material, decisions []decision, width int) []decision {
	out := make([]decision, len(decisions))
	for i, d := range decisions {
		out[i] = d
		if d.verb != "rewrite" && d.verb != "add" {
			continue
		}
		var text []string
		for _, line := range d.text {
			text = append(text, wrapLine(line, width-m.indents[d.id])...)
		}
		out[i].text = text
	}
	return out
}

// wrapLine is the line as it fits in allowance columns. A line that fits, or carries no marker the
// wrap knows, comes back as it was.
func wrapLine(line string, allowance int) []string {
	if placedColumns(line) <= allowance {
		return []string{line}
	}
	trimmed := strings.TrimLeft(line, " \t")
	for _, single := range []struct{ open, close string }{{"/** ", " */"}, {"/* ", " */"}} {
		if strings.HasPrefix(trimmed, single.open) && strings.HasSuffix(trimmed, single.close) {
			body := strings.TrimSuffix(strings.TrimPrefix(trimmed, single.open), single.close)
			lines := []string{strings.TrimSpace(single.open)}
			lines = append(lines, fill("* ", body, allowance)...)
			return append(lines, "*/")
		}
	}
	for _, marker := range []string{"// ", "# ", "* "} {
		if strings.HasPrefix(trimmed, marker) {
			return fill(marker, strings.TrimPrefix(trimmed, marker), allowance)
		}
	}
	return []string{line}
}

// fill lays the words out after the marker, as many to a line as fit in allowance columns.
func fill(marker, body string, allowance int) []string {
	var out []string
	current := ""
	for _, word := range strings.Fields(body) {
		next := word
		if current != "" {
			next = current + " " + word
		}
		if current != "" && placedColumns(marker+next) > allowance {
			out = append(out, marker+current)
			next = word
		}
		current = next
	}
	return append(out, marker+current)
}

// placedColumns is how wide a comment line prints past its indent. A block's `*` line is set one
// column in, under the opening `/**`, as indent places it.
func placedColumns(line string) int {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "*") {
		return columns(trimmed) + 1
	}
	return columns(trimmed)
}
