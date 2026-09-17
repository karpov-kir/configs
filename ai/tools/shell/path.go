package shell

import (
	"os"
	"path/filepath"
	"strings"
)

// MaxFileBytes bounds every whole-file read in these tools. The tree under review chooses how large a
// committed file is, and a whole-file read is not the streaming one it replaced: 64 MiB of newlines
// packs to a few hundred KB on disk and took ~2.5 GB resident, one slice header per line, which
// OOM-kills the review stage. The bound sits far above any real instruction file — the largest here is
// under 60 KB — so reaching it says something about the branch, not about the tree growing.
//
// A file over the bound is always reported and never truncated: an unread file must not look like one
// that held nothing. What else it costs differs per caller, so each states its own.
const MaxFileBytes = 8 << 20

// Join builds paths by concatenation, never filepath.Join: a root arrives as a literal argument and
// every message echoes it back, so a `./ai` root has to stay `./ai` rather than be cleaned to `ai`.
func Join(dir, name string) string {
	return dir + "/" + name
}

// DirName and BaseName are dirname(1) and basename(1), for the same reason Join exists:
// filepath.Dir cleans its result.
func DirName(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		if path == "" {
			return "."
		}
		return "/"
	}
	i := strings.LastIndexByte(trimmed, '/')
	if i < 0 {
		return "."
	}
	if parent := strings.TrimRight(trimmed[:i], "/"); parent != "" {
		return parent
	}
	return "/"
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

// There is deliberately no IsReadable alongside these. `[ -r ]` has two answers, open(2)'s and
// access(2)'s, and they differ for root and for a mode-000 file, so each caller keeps the one it
// means: ecoroot's containment test opens the file, ecoreport asks access(2) because its suite skips
// its permission cases on exactly that answer. A shared one here would be picked by import order.
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

func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

// The directory a real path resolves to, symlinks followed. Empty when the path is not a directory or
// cannot be resolved, and every caller reads that emptiness as "not there".
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

// RealPath is realpath(1): the absolute path with every symlink followed, and the error the resolution
// failed with when something along the way is not there.
//
// The working directory is prepended by concatenation rather than by filepath.Abs, and that is the
// whole reason this exists. Abs cleans `..` lexically, against the name the path was reached by
// instead of the directory it really names — so `$HOME/.kk-flavor/../project-skills.sh` becomes
// `$HOME/project-skills.sh`, and `.kk-flavor` is a link into a checkout whose real parent is somewhere
// else entirely. Left in the path, each `..` is popped by EvalSymlinks against the directory it has
// already resolved, which is what the shell's own `cd -P` does.
func RealPath(path string) (string, error) {
	absolute := path
	if !filepath.IsAbs(absolute) {
		working, err := os.Getwd()
		if err != nil {
			return "", err
		}
		absolute = Join(working, absolute)
	}
	return filepath.EvalSymlinks(absolute)
}

// OwnDirectory is the real directory a program is running from, taken from the argv[0] the stub
// preserved with `exec -a`. Every tool here reads its skills, declarations and sibling scripts from
// beside that stub, so this is what names them — never the process's own working directory, which is
// wherever the human happened to be standing.
//
// One implementation for all six entry points. filepath.Dir is safe on what RealPath answers, which is
// absolute and already clean.
func OwnDirectory(invocation string) (string, error) {
	real, err := RealPath(invocation)
	if err != nil {
		return "", err
	}
	return filepath.Dir(real), nil
}

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

// Width is how many pattern bytes the element at index spans. A `*` matches with width 0, so the
// caller can record its backtrack point there.
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

// An unterminated `[` is a literal `[`, which is what fnmatch does with one.
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
