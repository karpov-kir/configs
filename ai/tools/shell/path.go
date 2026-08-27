package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// Join builds paths by concatenation, never filepath.Join: a root arrives as a literal argument and
// every message echoes it back, so a `./ai` root has to stay `./ai` rather than be cleaned to `ai`.
func Join(dir, name string) string {
	return dir + "/" + name
}

// DirName and BaseName are dirname(1) and basename(1), for the same reason Join exists:
// filepath.Dir cleans its result.
func DirName(path string) string {
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

func BaseName(path string) string {
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

// The `[ -e ]`, `[ -d ]`, `[ -f ]` and `[ -r ]` of the shell version, which follow a symlink, and the
// `[ -L ]` that does not.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func IsRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func IsReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// --- shared:canonical-dir ---
// The directory a real path resolves to, symlinks followed — the `cd -P … && pwd -P` the shell
// version used. Empty when the path is not a directory or cannot be resolved, and every caller reads
// that emptiness as "not there".
func CanonicalDir(path string) string {
	if !IsDir(path) {
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

// --- end shared:canonical-dir ---

// --- shared:contained-in-root ---
// True when a path's directory sits at or under rootCanon, which is CanonicalDir of the root.
//
// A symlink is refused rather than resolved: CanonicalDir canonicalises a *directory*, so it never
// sees the final component, and a link at a budget path would walk through a check that only tested
// its parent. A regular file, or nothing: existence alone admits a FIFO or a device, which a read
// then blocks on forever, and a dangling symlink fails existence entirely — so callers enter on
// exists-or-is-a-symlink and both become a refusal, not a silent drop. Readable too, not just
// regular: a mode-000 file passes every type test, and the read behind the figure then fails,
// leaving a file counted whose words are not.
func ContainedInRoot(rootCanon, path string) bool {
	if IsSymlink(path) || !IsRegularFile(path) || !IsReadable(path) {
		return false
	}
	dir := CanonicalDir(DirName(path))
	if rootCanon == "" || dir == "" {
		return false
	}
	return dir == rootCanon || strings.HasPrefix(dir, rootCanon+"/")
}

// --- end shared:contained-in-root ---

// Fnmatch is fnmatch as find(1)'s -name and -path use it: no FNM_PATHNAME, so `*` spans `/` too, and
// no FNM_PERIOD, so it matches a leading dot.
func Fnmatch(pattern, text string) bool {
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

// A bracket expression, returning the bytes it spans. An unterminated `[` is a literal `[`, which is
// what fnmatch does with one.
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
