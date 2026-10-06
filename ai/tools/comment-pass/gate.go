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

// The gate checks what takes no judgement. A line fits the repository's print width. A block holds at
// most four prose lines, and a file header at most eight. A comment holds no markdown. A comment that
// fails the gate fails the pass, with its text shown, and the pass retries and drops no comment.

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
		limit := maxBlockLines
		if d.id == headerID {
			limit = maxHeaderLines
		}
		prose := 0
		for _, line := range d.text {
			body := commentBody(line)
			if body != "" {
				prose++
			}
			if columns(line)+indentWidth(m, d.id) > width {
				out = append(out, fmt.Sprintf("%s: a line is wider than %d: %s", d.id, width, line))
			}
			if reMarkdown.MatchString(body) {
				out = append(out, fmt.Sprintf("%s: markdown in a comment: %s", d.id, line))
			}
		}
		if prose > limit {
			out = append(out, fmt.Sprintf("%s: %d prose lines, over %d:\n%s", d.id, prose, limit, strings.Join(d.text, "\n")))
		}
	}
	return out
}

// indentWidth is the indent the comment will take, which counts toward the line's width. The gate
// reads the text before it is placed, so the material carries each id's indent.
func indentWidth(m material, id string) int { return m.indents[id] }

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
