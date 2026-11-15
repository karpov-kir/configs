package commentpass

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A comment line may be as wide as its file's formatter prints code, read from that formatter's own
// configuration, so a rewritten comment fits the lines around it. A file whose formatter sets no
// width, as with gofmt, takes the `.editorconfig` `max_line_length`, and otherwise has no bound.

// noWidth is the width of a file nothing bounds.
const noWidth = 0

const (
	// oxfmtWidth is oxfmt's own default, for a repository that sets none.
	oxfmtWidth = 100
	// prettierWidth is prettier's own default, for a repository that sets none.
	prettierWidth = 80
	// rustfmtWidth is rustfmt's own default `max_width`.
	rustfmtWidth = 100
)

// fileWidth is the width a file's lines print within, or noWidth. The path is relative to root.
func fileWidth(root, path string) int {
	abs := filepath.Join(root, path)
	dir := filepath.Dir(abs)
	switch familyOf(path) {
	case familyJs:
		switch kind, at := jsFormatter(root, dir); kind {
		case formatterOxfmt:
			if width := oxfmtConfigWidth(at); width > 0 {
				return width
			}
			// oxfmt reads `max_line_length` from `.editorconfig` where its own config sets no width.
			if width := editorconfigWidth(root, abs); width > 0 {
				return width
			}
			return oxfmtWidth
		case formatterPrettier:
			return prettierConfigWidth(root, abs, at)
		}
	case familyRs:
		if width := rustfmtConfigWidth(root, dir); width > 0 {
			return width
		}
		return rustfmtWidth
	}
	if width := editorconfigWidth(root, abs); width > 0 {
		return width
	}
	return noWidth
}

// formatter names the formatter that formats a script file, or formatterNone where no directory declares one.
type formatter string

const (
	formatterNone     formatter = ""
	formatterOxfmt    formatter = "oxfmt"
	formatterPrettier formatter = "prettier"
)

// jsFormatter names the formatter of a script file and the directory that declares it. The nearest
// directory from the file up to the root that holds a formatter's config file, or a package.json that
// lists one, decides.
func jsFormatter(root, dir string) (formatter, string) {
	for _, at := range dirsUp(root, dir) {
		for _, name := range []string{".oxfmtrc.json", ".oxfmtrc.jsonc"} {
			if isFile(filepath.Join(at, name)) {
				return formatterOxfmt, at
			}
		}
		for _, name := range prettierFiles {
			if isFile(filepath.Join(at, name)) {
				return formatterPrettier, at
			}
		}
		if body, err := os.ReadFile(filepath.Join(at, "package.json")); err == nil {
			var pkg struct {
				Prettier        json.RawMessage   `json:"prettier"`
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if json.Unmarshal(body, &pkg) == nil {
				lists := func(name string) bool {
					_, dep := pkg.Dependencies[name]
					_, dev := pkg.DevDependencies[name]
					return dep || dev
				}
				switch {
				case len(pkg.Prettier) > 0:
					return formatterPrettier, at
				case lists("oxfmt"):
					return formatterOxfmt, at
				case lists("prettier"):
					return formatterPrettier, at
				}
			}
		}
	}
	return formatterNone, ""
}

// dirsUp lists dir and each directory above it, nearest first, up to and including root.
func dirsUp(root, dir string) []string {
	var dirs []string
	for at := dir; ; at = filepath.Dir(at) {
		dirs = append(dirs, at)
		if at == root || filepath.Dir(at) == at || !strings.HasPrefix(at, root) {
			return dirs
		}
	}
}

// prettierFiles are the config files prettier looks for in a directory.
var prettierFiles = []string{".prettierrc", ".prettierrc.json", ".prettierrc.json5", ".prettierrc.yaml", ".prettierrc.yml",
	".prettierrc.toml", ".prettierrc.js", ".prettierrc.cjs", ".prettierrc.mjs", ".prettierrc.ts", "prettier.config.js",
	"prettier.config.cjs", "prettier.config.mjs", "prettier.config.ts"}

// oxfmtConfigWidth is the `printWidth` the directory's oxfmt config sets, or 0.
func oxfmtConfigWidth(dir string) int {
	for _, name := range []string{".oxfmtrc.json", ".oxfmtrc.jsonc"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var cfg struct {
			PrintWidth int `json:"printWidth"`
		}
		if json.Unmarshal(jsonc(body), &cfg) == nil {
			return cfg.PrintWidth
		}
	}
	return 0
}

// prettierConfigWidth is the width the repository's prettier resolves for the file. A config can take
// its width from a shared package it requires, which no file here spells, so the repository's own
// prettier is asked first. Where node or prettier is missing, the config's JSON is read, then the
// `.editorconfig` `max_line_length`, and prettier's default stands where neither sets a width.
func prettierConfigWidth(root, file, dir string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "-e",
		"require('prettier').resolveConfig(process.argv[1],{editorconfig:true}).then(c=>console.log((c&&c.printWidth)||''))", file)
	// node resolves `prettier` from the directory that declares it, which can sit below the root.
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		if width, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && width > 0 {
			return width
		}
	}
	for _, name := range []string{".prettierrc", ".prettierrc.json"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
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
	if body, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pkg struct {
			Prettier struct {
				PrintWidth int `json:"printWidth"`
			} `json:"prettier"`
		}
		if json.Unmarshal(body, &pkg) == nil && pkg.Prettier.PrintWidth > 0 {
			return pkg.Prettier.PrintWidth
		}
	}
	if width := editorconfigWidth(root, file); width > 0 {
		return width
	}
	return prettierWidth
}

// rustfmtConfigWidth is the `max_width` the nearest rustfmt config sets, or 0. rustfmt reads the
// nearest `rustfmt.toml` or `.rustfmt.toml` from the file's directory up.
func rustfmtConfigWidth(root, dir string) int {
	for _, at := range dirsUp(root, dir) {
		for _, name := range []string{"rustfmt.toml", ".rustfmt.toml"} {
			if body, err := os.ReadFile(filepath.Join(at, name)); err == nil {
				return tomlTopInt(body, "max_width")
			}
		}
	}
	return 0
}

var reTomlInt = regexp.MustCompile(`^([A-Za-z0-9_-]+)\s*=\s*(\d+)\s*(?:#.*)?$`)

// tomlTopInt is the integer a top-level TOML key holds, or 0. A key under a `[table]` is another key.
func tomlTopInt(body []byte, key string) int {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			return 0
		}
		if m := reTomlInt.FindStringSubmatch(line); m != nil && m[1] == key {
			n, _ := strconv.Atoi(m[2])
			return n
		}
	}
	return 0
}

// editorconfigWidth is the `max_line_length` `.editorconfig` sets for the file, or 0. The files are read
// from the file's directory up to the one marked `root = true` or the repository's root, and a nearer
// file's section, or a later section in one file, wins.
func editorconfigWidth(root, file string) int {
	width, set := 0, false
	for _, dir := range dirsUp(root, filepath.Dir(file)) {
		body, err := os.ReadFile(filepath.Join(dir, ".editorconfig"))
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			continue
		}
		value, found, isRoot := editorconfigValue(string(body), filepath.ToSlash(rel), "max_line_length")
		if found && !set {
			set = true
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				width = n
			}
		}
		if isRoot {
			break
		}
	}
	return width
}

// editorconfigValue is the value the last section matching rel sets for key, and whether the file is
// marked `root = true`. A value of `off` reads as found and unset.
func editorconfigValue(body, rel, key string) (value string, found, isRoot bool) {
	matching := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			matching = editorconfigMatch(line[1:len(line)-1], rel)
			continue
		}
		name, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name, val = strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(val)
		switch {
		case name == "root" && !matching && strings.EqualFold(val, "true"):
			isRoot = true
		case matching && name == key:
			value, found = strings.ToLower(val), true
		}
	}
	return value, found, isRoot
}

// editorconfigMatch says the section's glob matches the path, relative to the file's directory, under
// the EditorConfig glob rules: a glob without `/` matches the name in any directory.
func editorconfigMatch(glob, rel string) bool {
	if !strings.Contains(glob, "/") {
		glob = "**/" + glob
	}
	glob = strings.TrimPrefix(glob, "/")
	re, err := regexp.Compile("^(?:" + globRegexp(glob) + ")$")
	return err == nil && re.MatchString(rel)
}

var reNumRange = regexp.MustCompile(`^\{(-?\d+)\.\.(-?\d+)\}`)

// globRegexp turns an EditorConfig glob into a regular expression: `*` takes no `/`, `**` takes any,
// `?` one character, `[...]` a class, `{a,b}` either word and `{1..3}` a number in the range.
func globRegexp(glob string) string {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch {
		case c == '\\' && i+1 < len(glob):
			i++
			b.WriteString(regexp.QuoteMeta(string(glob[i])))
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '[':
			end := strings.IndexByte(glob[i:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			class := glob[i+1 : i+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + class + "]")
			i += end
		case c == '{':
			if m := reNumRange.FindStringSubmatch(glob[i:]); m != nil {
				lo, _ := strconv.Atoi(m[1])
				hi, _ := strconv.Atoi(m[2])
				var alts []string
				for n := min(lo, hi); n <= max(lo, hi); n++ {
					alts = append(alts, strconv.Itoa(n))
				}
				b.WriteString("(?:" + strings.Join(alts, "|") + ")")
				i += len(m[0]) - 1
				continue
			}
			depth++
			b.WriteString("(?:")
		case c == '}' && depth > 0:
			depth--
			b.WriteString(")")
		case c == ',' && depth > 0:
			b.WriteString("|")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	for ; depth > 0; depth-- {
		b.WriteString(")")
	}
	return b.String()
}

// jsonc is the JSON a JSONC body holds: its `//` and `/* */` comments and trailing commas taken off,
// strings left as they are.
func jsonc(body []byte) []byte {
	var out bytes.Buffer
	inString := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case inString:
			out.WriteByte(c)
			if c == '\\' && i+1 < len(body) {
				i++
				out.WriteByte(body[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(body) && body[i+1] == '/':
			for i < len(body) && body[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case c == '/' && i+1 < len(body) && body[i+1] == '*':
			end := bytes.Index(body[i+2:], []byte("*/"))
			if end < 0 {
				return out.Bytes()
			}
			i += end + 3
		case c == ',':
			if next := significant(body, i+1); next < len(body) && (body[next] == '}' || body[next] == ']') {
				continue
			}
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}

// significant is the index of the first byte at or past i that is no blank and no comment.
func significant(body []byte, i int) int {
	for i < len(body) {
		switch {
		case body[i] == ' ' || body[i] == '\t' || body[i] == '\r' || body[i] == '\n':
			i++
		case bytes.HasPrefix(body[i:], []byte("//")):
			for i < len(body) && body[i] != '\n' {
				i++
			}
		case bytes.HasPrefix(body[i:], []byte("/*")):
			end := bytes.Index(body[i+2:], []byte("*/"))
			if end < 0 {
				return len(body)
			}
			i += end + 4
		default:
			return i
		}
	}
	return i
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
