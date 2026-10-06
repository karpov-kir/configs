package commentpass

import (
	"strings"

	"configs/ai/tools/shell"
)

// block is a run of comment lines, by its first line, counted from 1, and how many lines it spans.
type block struct {
	Line int
	Span int
}

// commentBlocks is each comment block of a source file: `//`, `#` and `/* */` blocks, with a `*`
// continuation or a closing `*/` read as part of the block. A script's `#!` line is code.
func commentBlocks(lines []string) []block {
	var found []block
	inBlock, inStar := false, false
	for i, raw := range lines {
		line := strings.TrimLeft(raw, shell.SpaceBytes)
		if i == 0 && strings.HasPrefix(line, "#!") {
			continue
		}
		// Inside a `/*` block every line belongs to it until one carries `*/`, whatever it opens on.
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
			found = append(found, block{Line: i + 1, Span: 1})
			inBlock = true
			inStar = opensStar(line)
		}
	}
	return found
}

func opensStar(line string) bool {
	return strings.HasPrefix(line, "/*") && !strings.Contains(line[2:], "*/")
}

// isComment reads `//`, `/*` and `#`, and a continuation `*` or closing `*/` followed by a space or the
// line's end, so `*ptr = 1` stays code.
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
