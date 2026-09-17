// Turning a text into the units a vote may delete, and applying the verdict back onto it.
//
// A source file's units are its comment blocks; prose's are its markdown blocks. What is shown but
// never offered lives here too — the code around a comment, a commit's subject and trailers, and
// everything the diff did not touch.
package bloatjudge

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"configs/ai/tools/shell"
)

// Unit is one thing the model may delete, by the 1-based line it starts on and how many lines it spans.
type Unit struct {
	Line int
	Span int
}

// Split turns candidates into units and builds the view the model reads. A source file's candidates are its
// comment blocks and its code is shown unnumbered; prose's are its markdown blocks, so a fenced block
// is held whole and the model drops a pasted repro or not at all. offer says which candidates become
// units, the rest are shown as context, and a unit's continuation lines are marked `.` in the margin.
func Split(lines []string, candidates []Unit, offer func(Unit) bool) ([]Unit, string) {
	var units []Unit
	numberAt := map[int]int{}
	for _, c := range candidates {
		if offer(c) {
			units = append(units, c)
			numberAt[c.Line] = len(units)
		}
	}
	var view strings.Builder
	continuing := 0
	for i, raw := range lines {
		at := i + 1
		switch {
		case numberAt[at] > 0:
			fmt.Fprintf(&view, "%4d| %s\n", numberAt[at], raw)
			continuing = units[numberAt[at]-1].Span - 1
		case continuing > 0:
			fmt.Fprintf(&view, "   .| %s\n", raw)
			continuing--
		default:
			fmt.Fprintf(&view, "    | %s\n", raw)
		}
	}
	return units, view.String()
}

func commentBlocks(lines []string) []Unit {
	var found []Unit
	inBlock, inStar := false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		// A script's interpreter directive is a `#` line, so it reads as a comment and would join the
		// header below it — and a unit is a thing the model may delete, with Apply dropping every line
		// of it. A vote against that header would take `#!/usr/bin/env bash` with it and leave a file
		// the kernel will not run. It is never offered, on the same reasoning Kind.Subject withholds a
		// commit's subject line.
		if i == 0 && strings.HasPrefix(line, "#!") {
			inBlock, inStar = false, false
			continue
		}
		// Inside a `/*` block every line belongs to it until one carries `*/`, whatever it starts
		// with: a continuation without a leading `*` is still the same comment, and ending the block
		// there would delete its first line alone and leave the tail to break the file.
		switch {
		case inStar:
			found[len(found)-1].Span++
			if strings.Contains(line, "*/") {
				inStar, inBlock = false, false
			}
		case !isComment(line):
			inBlock = false
		case inBlock:
			found[len(found)-1].Span++
			inStar = opensStar(line)
		default:
			found = append(found, Unit{Line: i + 1, Span: 1})
			inBlock = true
			inStar = opensStar(line)
		}
	}
	return found
}

// proseBlocks makes the markdown block the unit. A line in the middle of a hard-wrapped paragraph is
// not a unit any reader ever sees, and offered as one it lets a majority delete half a sentence.
// Prose written a paragraph to a line is unaffected; a commit message at 72 columns, and the four
// standards that wrap, are not.
func proseBlocks(lines []string) []Unit {
	var found []Unit
	inFence, inParagraph, inOrdered := false, false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		switch {
		case inFence:
			found[len(found)-1].Span++
			if shell.IsFenceDelimiter(line) {
				inFence = false
			}
		case shell.IsFenceDelimiter(line):
			found = append(found, Unit{Line: i + 1, Span: 1})
			inFence, inParagraph, inOrdered = true, false, false
		case line == "":
			inParagraph, inOrdered = false, false
		case inParagraph && !opensBlock(line, inOrdered):
			found[len(found)-1].Span++
		default:
			found = append(found, Unit{Line: i + 1, Span: 1})
			number, isItem := listMarker(line)
			inParagraph, inOrdered = wraps(line), isItem && number != ""
		}
	}
	return found
}

// opensBlock says a line begins a markdown block of its own instead of continuing the paragraph
// above it. inOrdered says an ordered list is already open, which is what keeps this from becoming
// the defect it removes: CommonMark lets an ordered list interrupt a paragraph only where it numbers
// from one, and a message wrapping onto `163. Three wordings were tried` is otherwise cut in half.
func opensBlock(line string, inOrdered bool) bool {
	switch {
	case strings.HasPrefix(line, "#"), strings.HasPrefix(line, ">"), strings.HasPrefix(line, "|"):
		return true
	}
	number, marked := listMarker(line)
	return marked && (number == "" || number == "1" || inOrdered)
}

// wraps says a plain line below this one belongs to the same unit. A paragraph, a list item and a
// quote all wrap — the quote by markdown's own lazy continuation. A heading and a table row do not:
// a heading that swallowed the paragraph under it would make deleting the paragraph delete the
// heading too, and a table row's neighbour is another row or the end of the table.
func wraps(line string) bool {
	return !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "|")
}

// listMarker reads a line's list marker: the empty string for a bullet, the digits for an ordered
// item, and false where there is no marker or no space after one. A marker needs that space —
// `**Bold**` opening a paragraph and `--- a comparison` are not list items, and reading them as ones
// would split a paragraph the writer did not split.
func listMarker(line string) (string, bool) {
	number, rest := "", ""
	switch {
	case strings.HasPrefix(line, "-"), strings.HasPrefix(line, "*"), strings.HasPrefix(line, "+"):
		rest = line[1:]
	default:
		digits := 0
		for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits+1 > len(line) || (line[digits] != '.' && line[digits] != ')') {
			return "", false
		}
		number, rest = line[:digits], line[digits+1:]
	}
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	return number, true
}

// offerFor is which candidates a kind puts to the vote — `Kind.Trailers` for what a commit withholds.
// One function, so a run and anything measuring a run offer the same units over the same text.
func offerFor(lines []string, kind Kind) func(Unit) bool {
	withheld := map[int]bool{}
	if kind.Trailers {
		maps.Copy(withheld, trailerLines(lines))
	}
	if kind.Subject {
		maps.Copy(withheld, subjectLines(lines))
	}
	if len(withheld) == 0 {
		return func(Unit) bool { return true }
	}
	// Every line of the unit, not its first. A unit can reach further than the block it started as —
	// an unclosed fence earlier in the message runs one unit to the end of the text — and a unit
	// whose first line is not withheld would carry the structure into a vote.
	return func(u Unit) bool {
		for at := u.Line; at < u.Line+u.Span; at++ {
			if withheld[at] {
				return false
			}
		}
		return true
	}
}

// subjectLines is the first block of a commit message. Empty where that block is the whole message,
// which is git's own shape for a subject-only commit: withholding it would leave nothing to judge and
// report a clean run over text no roll ever read.
func subjectLines(lines []string) map[int]bool {
	first := 0
	for first < len(lines) && strings.TrimSpace(lines[first]) == "" {
		first++
	}
	last := first
	for last < len(lines) && strings.TrimSpace(lines[last]) != "" {
		last++
	}
	if !hasContent(lines[min(last, len(lines)):]) {
		return nil
	}
	withheld := map[int]bool{}
	for at := first + 1; at <= last; at++ {
		withheld[at] = true
	}
	return withheld
}

func narrowToDiff(offer func(Unit) bool, added map[int]bool) func(Unit) bool {
	return func(u Unit) bool {
		if !offer(u) {
			return false
		}
		for line := u.Line; line < u.Line+u.Span; line++ {
			if added[line] {
				return true
			}
		}
		return false
	}
}

// trailerLines is the git trailer block of a commit message: the last block, once any of its lines
// reads `Token: value`, and only where a block stands above it — git's own rule, so `Fix: the thing`
// alone is a subject. One line is enough because git writes `(cherry picked from commit <sha>)` and
// bare issue refs in there too, and a block handed back is attribution the prompt tells a vote to cut.
func trailerLines(lines []string) map[int]bool {
	last := len(lines)
	for last > 0 && strings.TrimSpace(lines[last-1]) == "" {
		last--
	}
	first := last
	for first > 1 && strings.TrimSpace(lines[first-2]) != "" {
		first--
	}
	if !hasContent(lines[:max(first-1, 0)]) {
		return nil
	}
	trailered := false
	withheld := map[int]bool{}
	for at := first; at <= last; at++ {
		trailered = trailered || isTrailer(lines[at-1])
		withheld[at] = true
	}
	if !trailered {
		return nil
	}
	return withheld
}

func hasContent(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// isTrailer is git's own shape for one: a token of letters, digits and dashes, a colon, then a space.
func isTrailer(line string) bool {
	token, rest, found := strings.Cut(line, ":")
	if !found || token == "" || rest == "" || rest[0] != ' ' {
		return false
	}
	for _, r := range token {
		if !(r == '-' || shell.IsAlnumRune(r)) {
			return false
		}
	}
	return true
}

func offeredKey(units []Unit) string {
	parts := make([]string, len(units))
	for i, u := range units {
		parts[i] = strconv.Itoa(u.Line) + "+" + strconv.Itoa(u.Span)
	}
	return strings.Join(parts, ",")
}

func opensStar(line string) bool {
	return strings.HasPrefix(line, "/*") && !strings.Contains(line[2:], "*/")
}

// isComment mirrors comment-density's: `//`, `/*`, `#`, and a continuation `*` or closing `*/` followed
// by a space or the end of the line, so `*ptr = 1` stays code.
func isComment(line string) bool {
	switch {
	case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "/*"), strings.HasPrefix(line, "#"):
		return true
	}
	rest := ""
	switch {
	case strings.HasPrefix(line, "*/"):
		rest = line[2:]
	case strings.HasPrefix(line, "*"):
		rest = line[1:]
	default:
		return false
	}
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// Apply deletes the chosen units' lines and returns what is left, always ending in one newline. Text
// that ended in one and lost no unit comes back byte-identical; text that did not gains one.
// A block cut from the middle leaves both its blank lines, so the seam doubles. Left alone: git's
// `--cleanup` collapses them and markdown renders one and two alike, while closing the seam would
// delete the blank between two functions in a source file, where a blank is structure.
func Apply(lines []string, units []Unit, gone []int) string {
	drop := map[int]bool{}
	for _, index := range gone {
		unit := units[index-1]
		for offset := 0; offset < unit.Span; offset++ {
			drop[unit.Line+offset] = true
		}
	}
	var kept []string
	for i, line := range lines {
		if !drop[i+1] {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n") + "\n"
}
