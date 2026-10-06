// Package commentpass decides and writes the comments of a pull request's changed files, one model call
// per file. The call returns, for each comment the change touched and each declaration it added or
// changed, a decision with its reason, and the comment text where the decision writes one. The tool
// applies the decisions and runs the mechanical gates. A comment the change did not touch is context.
package commentpass

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	readerjudge "configs/ai/tools/reader-judge"
)

// candidate is a comment block the change touched: a line of the block or of the declaration under it
// is in the changed ranges.
type candidate struct {
	id       string
	first    int // the block's first line
	last     int // the block's last line
	decl     int // the line the block sits on, or 0 where none follows
	text     string
	isHeader bool
}

// place is a declaration the change added or changed that carries no comment.
type place struct {
	id   string
	line int
}

// material is what one file's call decides on.
type material struct {
	candidates []candidate
	places     []place
	// indents is the width of the indent each id's comment takes.
	indents map[string]int
}

// changedLines is each line of the file at head that the change added or changed, from base.
func changedLines(dir, base, path string) (map[int]bool, error) {
	out, err := exec.Command("git", "-C", dir, "diff", "--no-color", "--unified=0", base, "--", path).Output()
	if err != nil {
		return nil, fmt.Errorf("git cannot diff %s from %s", path, base)
	}
	changed := map[int]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		m := reHunk.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for n := start; n < start+count; n++ {
			changed[n] = true
		}
	}
	return changed, nil
}

var reHunk = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

// reDeclaration opens a declaration: a top-level one, a class member, a typed field, or a row of a
// table that opens on its key.
var reDeclaration = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?` +
	`(?:function\*?|class|const|let|var|type|interface|enum|func)\s+[A-Za-z_$]` +
	`|^\s+(?:(?:public|private|protected|static|readonly|async|get|set|override)\s+)*[A-Za-z_$][\w$]*\??\s*(?:<[^>]*>)?\s*\(.*\{\s*$` +
	`|^\s+(?:(?:public|private|protected|static|readonly)\s+)+[A-Za-z_$][\w$]*\??\s*[:=]` +
	`|^\s+\[?[A-Za-z_$][\w$.]*\]?\??\s*:\s*[{\[]\s*$` +
	`|^\s+(?:readonly\s+)?[A-Za-z_$][\w$]*\??\s*:\s*[^=;]+;\s*$`)

// notDeclaration is a statement keyword a member pattern would read as a call.
var notDeclaration = regexp.MustCompile(`^\s*(?:if|for|while|switch|return|catch|await|throw|else|do|try|new|typeof|case|` +
	`it|describe|test|beforeEach|afterEach|beforeAll|afterAll|expect)\b`)

// reUnitTest is a unit test's file. Its comments the change touched are material. A test is the wrong
// place for a fact, so the pass offers a test file only those comments.
var reUnitTest = regexp.MustCompile(`\.(?:test|spec)\.[a-z]+$|_test\.go$`)

// findMaterial reads the file's comment blocks and declarations against the changed lines.
func findMaterial(path string, lines []string, changed map[int]bool) material {
	m := material{indents: map[string]int{}}
	commented := map[int]bool{}
	for n, u := range readerjudge.CommentBlocks(lines) {
		last := u.Line + u.Span - 1
		decl := declarationAfter(lines, last)
		commented[decl] = true
		touched := false
		for _, line := range declarationSpan(lines, decl) {
			touched = touched || changed[line]
		}
		for line := u.Line; line <= last; line++ {
			touched = touched || changed[line]
		}
		if !touched {
			continue
		}
		block := strings.Join(lines[u.Line-1:last], "\n")
		m.indents["c"+strconv.Itoa(n+1)] = len(indentOf(lines, u.Line))
		m.candidates = append(m.candidates, candidate{id: "c" + strconv.Itoa(n+1), first: u.Line, last: last,
			decl: decl, text: block, isHeader: u.Line == firstContentLine(lines) && last < len(lines) && strings.TrimSpace(lines[last]) == ""})
	}
	n := 0
	if reUnitTest.MatchString(path) {
		return m
	}
	for line := 1; line <= len(lines); line++ {
		text := lines[line-1]
		if !changed[line] || commented[line] || !reDeclaration.MatchString(text) || notDeclaration.MatchString(text) {
			continue
		}
		n++
		m.indents["p"+strconv.Itoa(n)] = len(indentOf(lines, line))
		m.places = append(m.places, place{id: "p" + strconv.Itoa(n), line: line})
	}
	return m
}

// declarationAfter is the first code line after the block's last line, or 0 where the file ends first.
func declarationAfter(lines []string, last int) int {
	for n := last + 1; n <= len(lines); n++ {
		if strings.TrimSpace(lines[n-1]) != "" {
			return n
		}
	}
	return 0
}

// firstContentLine is the file's first line holding anything. A file header opens on it and has a
// blank line after it, where a block on the first declaration sits directly on that declaration.
func firstContentLine(lines []string) int {
	for n, line := range lines {
		if strings.TrimSpace(line) != "" {
			return n + 1
		}
	}
	return 0
}

// declarationSpan is the lines of the declaration opening on line `at`, up to the bracket that closes
// what it opens. A declaration that opens no bracket is its own line. A change to a body changes the
// declaration, and the comment on it is the pass's material.
func declarationSpan(lines []string, at int) []int {
	if at < 1 || at > len(lines) {
		return nil
	}
	depth, opened := 0, false
	var out []int
	for n := at; n <= len(lines); n++ {
		out = append(out, n)
		for _, r := range lines[n-1] {
			switch r {
			case '{', '(', '[':
				depth++
				opened = true
			case '}', ')', ']':
				depth--
			}
		}
		if !opened || depth <= 0 {
			return out
		}
	}
	return out
}
