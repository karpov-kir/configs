package commentpass

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The gate checks what takes no judgement. A line fits the repository's print width. Each block of a
// written comment is bounded on its own, a run of line comments or a doc block alike: at most four prose
// lines, and a file header at most eight. A note above a summary is two blocks, the shape the page asks
// of a declaration that needs both. A comment holds no markdown. The edit leaves every code line as it
// was. A comment that fails the gate fails the pass, with its text shown, and the file is left as it
// was. A line past the width is wrapped before the gate reads it.
const (
	maxBlockLines  = 4
	maxHeaderLines = 8
	// defaultWidth is prettier's own default, for a repository that sets none.
	defaultWidth = 80
)

// reMarkdown is bold or a heading. A list marker is left alone: "- 1 is returned" reads as prose.
var reMarkdown = regexp.MustCompile(`(?:^|\s)\*\*[^*\s][^*]*[^*\s]\*\*(?:\s|$|[.,;:])|^#{1,6}\s+\S`)

// gateFindings is each way the written comments fail the gate, by the comment's id.
func gateFindings(m material, decisions []decision, width int) []string {
	headerID := ""
	for _, c := range m.candidates {
		if c.isHeader {
			headerID = c.id
		}
	}
	var out []string
	for _, d := range decisions {
		if d.verb != "rewrite" && d.verb != "add" {
			continue
		}
		for _, part := range commentParts(d.text) {
			limit := maxBlockLines
			if d.id == headerID {
				limit = maxHeaderLines
			}
			prose := 0
			for _, line := range part {
				if commentBody(line) != "" {
					prose++
				}
			}
			if prose > limit {
				out = append(out, fmt.Sprintf("%s: %d prose lines in one block, over %d:\n%s", d.id, prose, limit, strings.Join(part, "\n")))
			}
		}
		for _, line := range d.text {
			body := commentBody(line)
			if placedColumns(line)+indentWidth(m, d.id) > width {
				out = append(out, fmt.Sprintf("%s: a line is wider than %d: %s", d.id, width, line))
			}
			if reMarkdown.MatchString(body) {
				out = append(out, fmt.Sprintf("%s: markdown in a comment: %s", d.id, line))
			}
		}
	}
	return out
}

// commentParts splits a written comment into its blocks: each run of line comments, and each `/* */`
// block from its opening line to the line that closes it.
func commentParts(text []string) [][]string {
	var parts [][]string
	inStar, lastLine := false, false
	for _, line := range text {
		trimmed := strings.TrimLeft(line, " \t")
		switch {
		case inStar:
			parts[len(parts)-1] = append(parts[len(parts)-1], line)
			inStar = !strings.Contains(trimmed, "*/")
		case strings.HasPrefix(trimmed, "/*"):
			parts = append(parts, []string{line})
			inStar = !strings.Contains(trimmed[2:], "*/")
			lastLine = false
		case lastLine:
			parts[len(parts)-1] = append(parts[len(parts)-1], line)
		default:
			parts = append(parts, []string{line})
			lastLine = true
		}
	}
	return parts
}

// indentWidth is the indent the comment will take, which counts toward the line's width. The gate
// reads the text before it is placed, so the material carries each id's indent.
func indentWidth(m material, id string) int { return m.indents[id] }

// codeChanges reports the first code line the edit changes, drops, moves or adds. A comment edit
// may do none of these. A blank line is no code line. The check counts an attribute, a tool
// directive and a line with code after its `*/` as code even inside a comment block. A misread
// block then cannot hide an edit to one.
func codeChanges(before, after []string, path string) []string {
	hash := hashCommented(path)
	code := func(lines []string) (text []string, at []int) {
		// Inside a `/* */` block a line opening on `@` is a doc tag, as `@param`, and no attribute.
		inBlock, inStar := map[int]bool{}, map[int]bool{}
		for _, bl := range commentBlocks(firstOf(codeView(lines, path)), hash) {
			// A block may join `//` lines and `/* */` parts, so each part's inside is tracked on its own.
			isOpen := false
			for n := bl.Line; n < bl.Line+bl.Span; n++ {
				trimmed := strings.TrimSpace(lines[n-1])
				inBlock[n], inStar[n] = true, isOpen
				if !isOpen && strings.HasPrefix(trimmed, "/*") {
					isOpen = !strings.Contains(trimmed[2:], "*/")
				} else if isOpen && strings.Contains(trimmed, "*/") {
					isOpen = false
				}
			}
		}
		for n, line := range lines {
			trimmed := strings.TrimSpace(line)
			isCodeAttribute := isAttribute(trimmed, hash) && !inStar[n+1]
			if trimmed != "" && (!inBlock[n+1] || isCodeAttribute || isDirective(trimmed) || codeAfterClose(trimmed, hash)) {
				text, at = append(text, line), append(at, n+1)
			}
		}
		return text, at
	}
	was, lineOf := code(before)
	now, _ := code(after)
	for i := 0; i < len(was) || i < len(now); i++ {
		switch {
		case i >= len(now):
			return []string{fmt.Sprintf("the edit would drop the code line at line %d: %s", lineOf[i], was[i])}
		case i >= len(was):
			return []string{fmt.Sprintf("the edit would add a code line: %s", now[i])}
		case was[i] != now[i]:
			return []string{fmt.Sprintf("the edit would change the code line at line %d: %s, to: %s", lineOf[i], was[i], now[i])}
		}
	}
	return nil
}

// codeAfterClose says the line closes a `/* */` comment and holds code after it. A line comment's `*/`
// is a word of its prose.
func codeAfterClose(line string, hash bool) bool {
	if strings.HasPrefix(line, "//") || hash && strings.HasPrefix(line, "#") {
		return false
	}
	at := strings.LastIndex(line, "*/")
	return at >= 0 && strings.TrimSpace(line[at+2:]) != ""
}

var reCommentLead = regexp.MustCompile(`^\s*(?:/\*\*?|\*/|\*|//|#)\s?`)

// commentBody is a comment line's words, its markers taken off.
func commentBody(line string) string {
	body := strings.TrimSpace(reCommentLead.ReplaceAllString(line, ""))
	return strings.TrimSpace(strings.TrimSuffix(body, "*/"))
}

// printWidth is the width the repository's prettier resolves. A config can take its width from a
// shared package it requires, which no file here spells, so the repository's own prettier is asked
// first. Where node or prettier is missing, the config files are read, and prettier's default stands
// where they set none.
func printWidth(root string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "-e",
		"require('prettier').resolveConfig(process.argv[1]).then(c=>console.log((c&&c.printWidth)||''))",
		filepath.Join(root, "package.json"))
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		if width, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && width > 0 {
			return width
		}
	}
	for _, name := range []string{".prettierrc", ".prettierrc.json"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		var cfg struct {
			PrintWidth int `json:"printWidth"`
		}
		if json.Unmarshal(body, &cfg) == nil && cfg.PrintWidth > 0 {
			return cfg.PrintWidth
		}
	}
	body, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err == nil {
		var pkg struct {
			Prettier struct {
				PrintWidth int `json:"printWidth"`
			} `json:"prettier"`
		}
		if json.Unmarshal(body, &pkg) == nil && pkg.Prettier.PrintWidth > 0 {
			return pkg.Prettier.PrintWidth
		}
	}
	return defaultWidth
}

// tabColumns is how wide a tab is read.
const tabColumns = 4

// columns is how wide the line prints, a tab taken at tabColumns.
func columns(line string) int {
	n := 0
	for _, r := range line {
		if r == '\t' {
			n += tabColumns
			continue
		}
		n++
	}
	return n
}

// otherWidth bounds a line in a file prettier does not format.
const otherWidth = 100

// widthFor is the width a file's lines print within: the repository's prettier width for a file
// prettier formats, and otherWidth for the rest.
func widthFor(path string, prettier int) int {
	switch filepath.Ext(path) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return prettier
	}
	return otherWidth
}
