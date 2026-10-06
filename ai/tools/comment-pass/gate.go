package commentpass

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

var reMarkdown = regexp.MustCompile(`\*\*[^*]+\*\*|^\s*(?:[-*]|\d+\.)\s+\S|^\s*#{1,6}\s+\S`)

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
			if len([]rune(line))+indentWidth(m, d.id) > width {
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

// printWidth is the repository's prettier printWidth, or prettier's default where it sets none.
func printWidth(root string) int {
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
