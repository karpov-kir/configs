package voicecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// defaultWidth is the line width a comment keeps where the repository's formatter sets no width.
// Prettier wraps code and leaves a comment line as written, so a comment line has no other bound.
const defaultWidth = 120

// prettierFormats is the extensions prettier formats. Its width applies to those files.
var prettierFormats = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true,
	".cjs": true, ".mts": true, ".cts": true, ".vue": true}

var prettierConfigs = []string{".prettierrc", ".prettierrc.json", ".prettierrc.js", ".prettierrc.cjs",
	"prettier.config.js", "prettier.config.cjs", "prettier.config.mjs"}

var (
	rePrintWidth = regexp.MustCompile(`"?printWidth"?\s*:\s*(\d+)`)
	reRequire    = regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
)

// prettierWidth is the print width the repository at root formats to. A config extending another
// through `require` is followed one step into node_modules. A shared config sets its width there.
func prettierWidth(root string) int {
	var texts []string
	for _, name := range prettierConfigs {
		if body, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			texts = append(texts, string(body))
		}
	}
	if body, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Prettier json.RawMessage `json:"prettier"`
		}
		if json.Unmarshal(body, &pkg) == nil && len(pkg.Prettier) > 0 {
			var module string
			if json.Unmarshal(pkg.Prettier, &module) == nil {
				texts = append(texts, "require('"+module+"')")
			} else {
				texts = append(texts, string(pkg.Prettier))
			}
		}
	}
	for _, text := range texts {
		if m := rePrintWidth.FindStringSubmatch(text); m != nil {
			if width, err := strconv.Atoi(m[1]); err == nil && width > 0 {
				return width
			}
		}
	}
	for _, text := range texts {
		for _, m := range reRequire.FindAllStringSubmatch(text, -1) {
			for _, candidate := range []string{m[1], m[1] + ".js", filepath.Join(m[1], "index.js")} {
				body, err := os.ReadFile(filepath.Join(root, "node_modules", candidate))
				if err != nil {
					continue
				}
				if w := rePrintWidth.FindStringSubmatch(string(body)); w != nil {
					if width, err := strconv.Atoi(w[1]); err == nil && width > 0 {
						return width
					}
				}
			}
		}
	}
	return defaultWidth
}

// longLines finds a block's comment lines wider than the repository formats code to. A writer's line
// of 139 characters stood in a repository whose code wraps at 120, and no gate read it.
func (s scanner) longLines(file string, b block, lines []string) []Finding {
	// The width is prettier's, and prettier formats only these files. A piped block is a writer's, and
	// the writer checks it before it lands in one of them.
	if file != "-" && !prettierFormats[strings.ToLower(filepath.Ext(file))] {
		return nil
	}
	width := s.width
	if width == 0 {
		width = defaultWidth
	}
	var found []Finding
	for at := b.start; at <= b.end && at <= len(lines); at++ {
		line := strings.TrimRight(lines[at-1], " \t")
		if n := utf8.RuneCountInString(line); n > width {
			found = append(found, Finding{File: file, Line: at, Check: checkLongLine,
				Text: strconv.Itoa(n) + " characters, over " + strconv.Itoa(width)})
		}
	}
	return found
}
