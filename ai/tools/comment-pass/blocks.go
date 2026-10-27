package commentpass

import (
	"regexp"
	"strings"

	"configs/ai/tools/shell"
)

// block is a run of comment lines, by its first line, counted from 1, and how many lines it spans.
type block struct {
	Line int
	Span int
}

// commentBlocks is each comment block of a source file: `//` and `/* */` blocks, or `#` blocks in a
// file that comments with `#`, with a `*` continuation or a closing `*/` read as part of the block. A
// script's `#!` line is code.
func commentBlocks(lines []string, hash bool) []block {
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
		// Outside a `/*` block a line opening on `*` continues an expression, as `* b` under `a +`. A
		// directive is read by a tool, so no comment block takes it.
		case !isComment(line, hash) || strings.HasPrefix(line, "*") || isDirective(line):
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

// isComment reads only `#` where hash is set, so a Python `//` line stays floor division. Elsewhere it
// reads `//` and `/*`, and a continuation `*` or closing `*/` followed by a space or the line's end, so
// `*ptr = 1` stays code, and a `#` line is code: a Rust attribute, a Swift `#if`, a private field.
func isComment(line string, hash bool) bool {
	switch {
	case hash:
		return strings.HasPrefix(line, "#")
	case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "/*"):
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

// hashCommented says the file comments with `#`.
func hashCommented(path string) bool {
	f := familyOf(path)
	return f == familyPy || f == familySh
}

// isDecorator says the line opens a decorator, an attribute or a directive on the line below it: `@`
// and a directive in any file, and `#[` where `#` opens no comment. A directive a comment never goes
// above opens nothing.
func isDecorator(text string, hash bool) bool {
	switch {
	case isDirective(text):
		return !reStayingDirective.MatchString(commentBody(text))
	// A line closing a block it did not open is that block's last line, as ` @return the x */`.
	case strings.HasSuffix(text, "*/") && !strings.Contains(text, "/*"):
		return false
	}
	return strings.HasPrefix(text, "@") || !hash && strings.HasPrefix(text, "#[")
}

// reStayingDirective is a directive a comment never goes above: one closing a range a directive above
// it opened, which belongs with the code above it, or one a tool reads only at the file's top.
var reStayingDirective = regexp.MustCompile(`^(?:#?endregion\b|eslint-enable|fmt:\s*on\b|isort:\s*on\b|swiftlint:enable|` +
	`pylint:\s*enable|-\*-.*coding[:=]|vim?:.*fileencoding=|@(?:jest|vitest)-environment\b)`)

// maxDecoratorLines bounds how far a decorator split over lines is followed to its close.
const maxDecoratorLines = 20

// reDecoratorName is a decorator's name, which its arguments, if any, follow directly.
var reDecoratorName = regexp.MustCompile(`^@[\w$.:\[\]]*`)

// decoratorEnd is the last line of the decorator opening on line n: its name and the arguments that
// follow it, or a `#[` attribute's brackets. It is 0 where no decorator stands on its own there: none
// opens, or the line it ends on goes on to the declaration, as `@objc func tapped() {`.
func decoratorEnd(lines []string, n int, hash bool) int {
	text := ""
	if n >= 1 && n <= len(lines) {
		text = strings.TrimSpace(lines[n-1])
	}
	switch {
	case !isDecorator(text, hash):
		return 0
	case isDirective(text):
		return n
	}
	// A decorator ends its line alone, or beside another decorator or a comment.
	alone := func(after string, end int) int {
		rest := strings.TrimSpace(after)
		if before, closed, found := strings.Cut(rest, "*/"); found && strings.HasPrefix(before, "/*") {
			rest = strings.TrimSpace(closed)
		}
		switch {
		case rest == "" || isComment(rest, hash):
			return end
		case strings.HasPrefix(rest, "@"):
			if decoratorEnd([]string{rest}, 1, hash) == 0 {
				return 0
			}
			return end
		}
		return 0
	}
	rest := withoutStrings(text, false)
	if strings.HasPrefix(rest, "@") {
		rest = rest[len(reDecoratorName.FindString(rest)):]
		if !strings.HasPrefix(rest, "(") {
			return alone(rest, n)
		}
	}
	depth := 0
	for end := n; end <= len(lines) && end < n+maxDecoratorLines; end++ {
		if end > n {
			rest = withoutStrings(lines[end-1], false)
		}
		for i, r := range rest {
			switch r {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth--; depth == 0 {
					return alone(rest[i+1:], end)
				}
			}
		}
	}
	return n
}

// isAttribute says the line has an attribute's or a decorator's shape, which is code: `@` in any
// file, and `#[` or `#![` where `#` opens no comment.
func isAttribute(text string, hash bool) bool {
	return strings.HasPrefix(text, "@") || !hash && (strings.HasPrefix(text, "#[") || strings.HasPrefix(text, "#!["))
}

// reBareDirective is a directive written with no space after its marker: a Go compiler or build
// directive, or a `#region` fold marker.
var reBareDirective = regexp.MustCompile(`^(?://(?:go:|line |export |extern |\+build|nolint)|#(?:end)?region\b)`)

// reToolDirective is a comment's body that a linter, type checker, formatter or coverage tool reads, in
// the forms those tools write, so prose opening on the same word stays prose.
var reToolDirective = regexp.MustCompile(`^(?:eslint-(?:disable|enable|env)|@ts-(?:ignore|expect-error|nocheck|check)\b|` +
	`prettier-ignore|(?:istanbul|c8|v8) ignore|tslint:|biome-ignore|deno-lint-ignore|@jsx\b|@flow\b|#(?:end)?region\b|` +
	`NOSONAR|nosec\b|type:\s*ignore|noqa\b|pylint:\s*(?:disable|enable|skip-file)|mypy:\s*\w|pyright:\s*\w|` +
	`fmt:\s*(?:off|on|skip)\b|isort:\s*(?:skip|off|on)\b|shellcheck\s+(?:disable|enable|source|shell)=|` +
	`pragma:\s*no\s*(?:cover|branch)\b|swiftlint:(?:disable|enable)|#nosec\b|\+build\b|@(?:jest|vitest)-environment\b|` +
	`-\*-.*coding[:=]|vim?:.*fileencoding=)`)

// reStarDirective is a directive a tool reads only from a `/* */` comment, whose word opens prose too.
var reStarDirective = regexp.MustCompile(`^/\*\s*(?:eslint|jshint|global|globals)\s+\S`)

// isDirective says the comment line is an instruction a compiler or a tool reads, which is code.
func isDirective(text string) bool {
	return reBareDirective.MatchString(text) || reStarDirective.MatchString(text) || reToolDirective.MatchString(commentBody(text))
}
