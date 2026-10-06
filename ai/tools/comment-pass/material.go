// Package commentpass decides and writes the comments of a pull request's changed files, one model call
// per file. The call returns, for each comment the change touched and each declaration it added or
// changed, a decision with its reason, and the comment text where the decision writes one. The tool
// applies the decisions and runs the mechanical gates. A comment the change did not touch is context.
package commentpass

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	// indents is the width, in columns, of the indent each id's comment takes.
	indents map[string]int
	// hashComments marks a file whose comments open on `#`, where every other file's open on `//` or `/*`.
	hashComments bool
	// lifetimes marks a file whose single quotes open no string, as Rust's lifetimes.
	lifetimes bool
}

// changedLines is, for each file at head, the lines the change added or changed since base. Renames
// are followed, so a renamed file's unchanged lines are context.
func changedLines(dir, base string) (map[string]map[int]bool, error) {
	out, err := exec.Command("git", "-C", dir, "diff", "--no-color", "--unified=0", "-M", base).Output()
	if err != nil {
		return nil, fmt.Errorf("git cannot diff the tree from %s", base)
	}
	changed := map[string]map[int]bool{}
	file := ""
	for _, line := range strings.Split(string(out), "\n") {
		if name, found := strings.CutPrefix(line, "+++ b/"); found {
			file = name
			changed[file] = map[int]bool{}
			continue
		}
		m := reHunk.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for n := start; n < start+count; n++ {
			changed[file][n] = true
		}
	}
	return changed, nil
}

var reHunk = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

// reDeclaration opens a declaration: a top-level one, a class member, a typed field, or a row of a
// table that opens on its key.
var reDeclaration = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?` +
	`(?:function\*?|class|const|let|var|type|interface|enum|func|def|fn|fun|pub\s+fn)\s+[A-Za-z_$]` +
	`|^\s+(?:(?:public|private|protected|static|readonly|async|get|set|override)\s+)*[A-Za-z_$][\w$]*\??\s*(?:<[^>]*>)?\s*\(.*\{\s*$` +
	`|^\s+(?:(?:public|private|protected|static|readonly)\s+)+[A-Za-z_$][\w$]*\??\s*[:=]` +
	`|^\s+\[?[A-Za-z_$][\w$.]*\]?\??\s*:\s*[{\[]\s*$` +
	`|^\s+(?:readonly\s+)?[A-Za-z_$][\w$]*\??\s*:\s*[^=;]+;\s*$`)

// notDeclaration is a statement keyword a member pattern would read as a call.
var notDeclaration = regexp.MustCompile(`^\s*(?:if|for|while|switch|return|catch|await|throw|else|do|try|new|typeof|case|` +
	`it|describe|test|beforeEach|afterEach|beforeAll|afterAll|expect)\b`)

// reLocal is a variable declared inside a body, which is no declaration a reader looks up.
var reLocal = regexp.MustCompile(`^\s+(?:const|let|var)\s`)

// reMemberArrow is a class member holding an arrow function. Any other line holding `=>` is a callback
// passed in a call, as `useEffect(() => {`, and no declaration.
var reMemberArrow = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s|^\s+(?:(?:public|private|protected|static|readonly)\s+)+[A-Za-z_$]`)

// reUnitTest is a unit test's file. Its comments the change touched are material. A test is the wrong
// place for a fact, so the pass offers a test file only those comments.
var reUnitTest = regexp.MustCompile(`\.(?:test|spec)\.[a-z]+$|_test\.go$`)

// findMaterial reads the file's comment blocks and declarations against the changed lines.
func findMaterial(path string, lines []string, changed map[int]bool) material {
	ext := strings.ToLower(filepath.Ext(path))
	m := material{indents: map[string]int{}, hashComments: ext == ".py" || ext == ".sh", lifetimes: ext == ".rs"}
	commented := map[int]bool{}
	for n, u := range commentBlocks(lines) {
		last := u.Line + u.Span - 1
		decl := declarationAfter(lines, last)
		commented[decl] = true
		touched := false
		for _, line := range declarationSpan(lines, decl, m.lifetimes) {
			touched = touched || changed[line]
		}
		for line := u.Line; line <= last; line++ {
			touched = touched || changed[line]
		}
		if !touched {
			continue
		}
		block := strings.Join(lines[u.Line-1:last], "\n")
		m.indents["c"+strconv.Itoa(n+1)] = columns(indentOf(lines, u.Line))
		m.candidates = append(m.candidates, candidate{id: "c" + strconv.Itoa(n+1), first: u.Line, last: last,
			decl: decl, text: block, isHeader: u.Line == firstContentLine(lines) && last < len(lines) &&
				(strings.TrimSpace(lines[last]) == "" || strings.HasPrefix(lines[last], "package "))})
	}
	n := 0
	if reUnitTest.MatchString(path) {
		return m
	}
	for line := 1; line <= len(lines); line++ {
		text := lines[line-1]
		if commented[line] || !reDeclaration.MatchString(text) || notDeclaration.MatchString(text) || reLocal.MatchString(text) ||
			strings.Contains(text, "=>") && !reMemberArrow.MatchString(text) {
			continue
		}
		touched := false
		for _, n := range declarationSpan(lines, line, m.lifetimes) {
			touched = touched || changed[n]
		}
		if !touched {
			continue
		}
		n++
		m.indents["p"+strconv.Itoa(n)] = columns(indentOf(lines, line))
		m.places = append(m.places, place{id: "p" + strconv.Itoa(n), line: line})
	}
	return m
}

// declarationAfter is the first code line after the block's last line, past any decorator, or 0 where
// the file ends first.
func declarationAfter(lines []string, last int) int {
	for n := last + 1; n <= len(lines); n++ {
		if text := strings.TrimSpace(lines[n-1]); text != "" && !strings.HasPrefix(text, "@") {
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
// what it opens. A bracket inside a string is no bracket. A brace-less body, as in Python, runs while
// its lines are indented past the declaration. The span stops before the next declaration at the
// declaration's own indent, which bounds a bracket the count misreads.
func declarationSpan(lines []string, at int, lifetimes bool) []int {
	if at < 1 || at > len(lines) {
		return nil
	}
	own := len(indentOf(lines, at))
	depth := 0
	var out []int
	for n := at; n <= len(lines); n++ {
		if n > at && reDeclaration.MatchString(lines[n-1]) && len(indentOf(lines, n)) <= own && depth <= 0 {
			return out
		}
		out = append(out, n)
		bracketed := false
		for _, r := range withoutStrings(lines[n-1], lifetimes) {
			switch r {
			case '{', '(', '[':
				depth++
				bracketed = true
			case '}', ')', ']':
				depth--
				bracketed = true
			}
		}
		if depth > 0 {
			continue
		}
		text := strings.TrimSpace(lines[n-1])
		switch {
		case strings.HasSuffix(text, "=>"):
			// An arrow's body opens on the next line.
			continue
		case strings.HasSuffix(text, ":") && (bracketed || n == at):
			// A Python signature closes on a colon, and its body is the lines indented past it.
			for body := n + 1; body <= len(lines); body++ {
				if inner := strings.TrimSpace(lines[body-1]); inner != "" && len(indentOf(lines, body)) <= own {
					break
				}
				out = append(out, body)
			}
			return out
		case bracketed || n == at:
			return out
		}
	}
	return out
}

// withoutStrings is the line with its quoted strings blanked, so their brackets go uncounted.
func withoutStrings(line string, lifetimes bool) string {
	out := []rune(line)
	var quote rune
	for i, r := range out {
		switch {
		case quote != 0 && r == quote && (i == 0 || out[i-1] != '\\'):
			quote = 0
		case quote != 0:
			out[i] = ' '
		case r == '"' || r == '`' || r == '\'' && !lifetimes:
			quote = r
		}
	}
	return string(out)
}
