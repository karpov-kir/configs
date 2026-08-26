package ecocheck

import (
	"os"
	"path/filepath"
	"strings"
)

// The primitives the shell version reached for through `tr`, `sed`, `basename` and `wc`. They are
// here rather than inline because their exact edges — which bytes count as a control byte, which
// space `s/^ //` removes — are the contract several scans read a finding out of.

// Anything attacker-chosen that reaches a finding goes through oneline first: one physical line per
// finding is what makes the ranking mean anything. Every control byte goes, not only the two that
// split a line — an ESC sequence erases the real finding printed above it.
func oneline(text string) string {
	out := []byte(text)
	for i, b := range out {
		if b < 0x20 || b == 0x7f {
			out[i] = ' '
		}
	}
	return string(out)
}

// `cut -c1-n` under LC_ALL=C, which is bytes. Used where a finding quotes a name the tree chose, so
// the bound has to be on what is printed, not on what a locale calls a character.
func cutBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	return text[:n]
}

// Comparison form for a heading or a cited section name: lowercased, markdown emphasis dropped,
// whitespace runs flattened, one leading and one trailing space removed.
func plainText(text string) string {
	out := make([]byte, 0, len(text))
	wasSpace := false
	for i := 0; i < len(text); i++ {
		b := text[i]
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if b == '`' || b == '*' || b == '_' {
			continue
		}
		if isSpaceByte(b) {
			if !wasSpace {
				out = append(out, ' ')
			}
			wasSpace = true
			continue
		}
		wasSpace = false
		out = append(out, b)
	}
	return strings.TrimSuffix(strings.TrimPrefix(string(out), " "), " ")
}

// The C-locale `[[:space:]]`, which is what every scan was compared against.
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\v' || b == '\f' || b == '\r'
}

func isAlnumByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// Paths are built by concatenation, never filepath.Join: the root arrives as a literal argument and
// every finding echoes it back, so a `./ai` root has to stay `./ai` rather than be cleaned to `ai`.
func join(dir, name string) string {
	return dir + "/" + name
}

// dirname(1) and basename(1), for the same reason join exists: filepath.Dir cleans its result.
func dirName(path string) string {
	i := strings.LastIndexByte(path, '/')
	switch {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	default:
		return path[:i]
	}
}

func baseName(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		if path == "" {
			return ""
		}
		return "/"
	}
	if i := strings.LastIndexByte(trimmed, '/'); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// One file as the line-oriented tools saw it: split on \n, with no empty record for a trailing
// newline. Bytes come back untouched — Go has no notion of a binary file, so the `-a` the shell
// version needed on every grep is gone with the hazard it guarded: one committed NUL byte could
// make a scan read no violation out of a file it had been handed.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return splitLines(string(data)), nil
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

func countNonEmptyLines(text string) int {
	count := 0
	for _, line := range splitLines(text) {
		if line != "" {
			count++
		}
	}
	return count
}

// `wc -l` counts newline bytes, `wc -w` counts runs of non-whitespace. Both figures ride the
// exit-0 census line, so they are counted per file and summed by the caller: concatenating first
// would glue the last word of a file with no final newline onto the first word of the next.
func countLinesAndWords(path string) (lines, words int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	inWord := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == '\n' {
			lines++
		}
		if isSpaceByte(b) {
			inWord = false
			continue
		}
		if !inWord {
			words++
			inWord = true
		}
	}
	return lines, words
}

// awk's `split($0, f, /[[:space:]]+/)` minus the empty fields it produces at either end, which no
// caller could have used: an empty field carries no token.
func splitFields(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool {
		return r < 0x80 && isSpaceByte(byte(r))
	})
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// The directory a real path resolves to, symlinks followed — the `cd -P … && pwd -P` the shell
// version used. Empty when the path is not a directory or cannot be resolved, and every caller
// reads that emptiness as "not there".
func canonicalDir(path string) string {
	if !isDir(path) {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return ""
	}
	return resolved
}

// fnmatch as find(1)'s -path uses it: no FNM_PATHNAME, so `*` spans `/` too. Only reached for a
// cited token carrying a glob metacharacter; the common case is a suffix lookup.
func fnmatch(pattern, text string) bool {
	patternIndex, textIndex := 0, 0
	starPattern, starText := -1, -1
	for textIndex < len(text) {
		if matched, width := matchOne(pattern, patternIndex, text[textIndex]); matched {
			if pattern[patternIndex] == '*' {
				starPattern, starText = patternIndex, textIndex
				patternIndex++
				continue
			}
			patternIndex += width
			textIndex++
			continue
		}
		if starPattern < 0 {
			return false
		}
		starText++
		textIndex = starText
		patternIndex = starPattern + 1
	}
	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}

// Reports whether the pattern element at index matches one byte, and how many pattern bytes that
// element spans. A `*` reports a match of width 0 so the caller can record its backtrack point.
func matchOne(pattern string, index int, b byte) (matched bool, width int) {
	if index >= len(pattern) {
		return false, 0
	}
	switch pattern[index] {
	case '*':
		return true, 0
	case '?':
		return true, 1
	case '[':
		return matchClass(pattern[index:], b)
	case '\\':
		if index+1 < len(pattern) {
			return pattern[index+1] == b, 2
		}
		return b == '\\', 1
	default:
		return pattern[index] == b, 1
	}
}

// A bracket expression, returning the bytes it spans. An unterminated `[` is a literal `[`, which
// is what fnmatch does with one.
func matchClass(pattern string, b byte) (matched bool, width int) {
	i := 1
	negated := false
	if i < len(pattern) && (pattern[i] == '!' || pattern[i] == '^') {
		negated = true
		i++
	}
	found := false
	first := true
	for i < len(pattern) {
		if pattern[i] == ']' && !first {
			if found != negated {
				return true, i + 1
			}
			return false, i + 1
		}
		first = false
		low := pattern[i]
		i++
		if i+1 < len(pattern) && pattern[i] == '-' && pattern[i+1] != ']' {
			high := pattern[i+1]
			i += 2
			if b >= low && b <= high {
				found = true
			}
			continue
		}
		if b == low {
			found = true
		}
	}
	return b == '[', 1
}
