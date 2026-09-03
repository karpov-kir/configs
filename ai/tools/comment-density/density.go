// Comment-density detector — flags changed source files whose ADDED lines are comment-heavy.
//
//	usage: comment-density.sh [<git-diff revisions>]   # defaults to HEAD (all uncommitted changes);
//	       a path argument is refused with exit 2, never scanned
//	env:   COMMENT_MAX_RATIO — flag above this comments/(comments+code) share of added lines (default 0.3)
//	       COMMENT_MIN_LINES — ignore files with fewer added comment lines than this (default 5)
//	       DENSITY_MAX_FILE_BYTES — skip untracked files larger than this (default 262144)
//
// Prose/data files (md, txt, json, lockfiles) don't count. With no diff args, untracked text files are
// scanned too; the index is never touched.
//
// Every run ends with its denominator on stderr — files reached, files with countable added lines,
// outliers, untracked files skipped unread. Read it: an empty report at exit 0 means "nothing was
// comment-heavy" only when that first number is above zero, and "nothing was read" when it is not.
//
// A targeting aid, not a bar: it counts ADDED lines, so rewording a comment the base already carried
// moves it into the added set, and the ratio can rise across a pass that cut comments.
//
// The shell this replaces built one text stream — a real `git diff` with a hand-made pseudo-diff for
// untracked files appended — because awk could only be handed one. Here the two sources feed one
// counter directly. That removes the pseudo-diff's whole failure surface: the shell had to defend a
// forged `diff --git` header, a newline in a path writing a second line into the stream, and a missing
// final newline fusing two files together. None of those can be expressed now.
package density

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Defaults, and the one place each is written down.
const (
	defaultMaxRatio     = 0.3
	defaultMinLines     = 5
	defaultMaxFileBytes = 262144
)

// Both the path and the outlier count are bounded, because under kk-pr-review they come from a branch
// somebody else wrote. A suppressed outlier is announced, never dropped — which holds only while this
// cap and the one in the announcement stay the same number.
const (
	maxShown    = 200
	maxPathRune = 200
)

// Binary is a NUL in the first 8KB — the same probe the shell used, and the same window.
const binaryProbeBytes = 8192

const (
	exitClean     = 0
	exitFound     = 1
	exitDidNotRun = 2
)

// Config is what the environment can move. Held as a struct rather than read from os.Getenv inside the
// counter so the suite drives every boundary without touching process state.
type Config struct {
	MaxRatio     float64
	MinLines     int
	MaxFileBytes int64
}

// ConfigFromEnv reads the three overrides. A value that does not parse is not silently replaced by the
// default: a caller who set COMMENT_MAX_RATIO=0..3 asked for something, and answering with 0.3 reports
// a scan against a threshold they did not choose.
func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MaxRatio: defaultMaxRatio, MinLines: defaultMinLines, MaxFileBytes: defaultMaxFileBytes}
	if raw, ok := lookup("COMMENT_MAX_RATIO"); ok && raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return cfg, fmt.Errorf("COMMENT_MAX_RATIO is %q, which is no number — the scan did NOT run", raw)
		}
		cfg.MaxRatio = value
	}
	if raw, ok := lookup("COMMENT_MIN_LINES"); ok && raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return cfg, fmt.Errorf("COMMENT_MIN_LINES is %q, which is no whole number — the scan did NOT run", raw)
		}
		cfg.MinLines = value
	}
	if raw, ok := lookup("DENSITY_MAX_FILE_BYTES"); ok && raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("DENSITY_MAX_FILE_BYTES is %q, which is no whole number — the scan did NOT run", raw)
		}
		cfg.MaxFileBytes = value
	}
	return cfg, nil
}

// tally is the denominator every run ends with. A report with no tally cannot say whether an empty
// result means "nothing was comment-heavy" or "nothing was read", and those are opposite facts.
type tally struct {
	reached   int // files the scan opened, tracked and untracked
	countable int // of those, the ones with at least one added line it could classify
	outliers  int
	skipped   int // untracked files declined unread — too big, or binary
}

type counts struct {
	comments int
	code     int
}

// Run executes one invocation and returns its exit code.
func Run(self string, args []string, cwd string, cfg Config, stdout, stderr io.Writer) int {
	if code, refused := refuseNonRevisions(self, args, cwd, stderr); refused {
		return code
	}

	files := map[string]*counts{}
	var t tally

	// The tracked half. Its failure is exit 2 and says so: a scan that never ran must never reach a
	// caller looking like a clean tree.
	diff, err := gitDiff(cwd, args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: git rejected these arguments — exit 2, the scan did NOT run. Not a clean result.\n", self)
		return exitDidNotRun
	}
	scanDiff(diff, cfg, files, &t)

	// The untracked half runs only with no revisions: with revisions the caller named two commits, and
	// a file in neither of them is not part of what they asked about.
	if len(args) == 0 {
		if err := scanUntracked(cwd, cfg, files, &t, stderr); err != nil {
			fmt.Fprintf(stderr, "%s: could not list untracked files — exit 2, the scan did NOT run over them.\n", self)
			return exitDidNotRun
		}
	}

	return report(self, cfg, files, t, stdout, stderr)
}

// Arguments are git-diff *revisions*, never paths — `git diff <path>` is legal and diffs against the
// index, so a path silently scans the wrong change set and exits 0, indistinguishable from clean.
func refuseNonRevisions(self string, args []string, cwd string, stderr io.Writer) (int, bool) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		// Refused, not skipped: `--output=` alone drains the pipe, so the scan exits 0 over a real
		// outlier.
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(stderr, "%s: '%s' is an option, not a git-diff revision — the scan did NOT run.\n", self, arg)
			fmt.Fprintf(stderr, "  this script takes revisions only (HEAD, origin/main, a..b); paths go after '--'.\n")
			return exitDidNotRun, true
		}
		if _, err := os.Stat(path.Join(cwd, arg)); err == nil && !resolvesAsRevision(cwd, arg) {
			fmt.Fprintf(stderr, "%s: '%s' is a path, not a git-diff revision — the scan did NOT run.\n", self, arg)
			fmt.Fprintf(stderr, "  pass a revision (HEAD, origin/main, a..b); paths, if you must, go after '--'.\n")
			return exitDidNotRun, true
		}
	}
	return exitClean, false
}

func resolvesAsRevision(cwd, arg string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", arg+"^{}")
	cmd.Dir = cwd
	return cmd.Run() == nil
}

// The flags pin the shape this parses. `--src-prefix`/`--dst-prefix` survive a `diff.noprefix` in the
// caller's config, `--no-color` survives `color.diff=always`, `--no-ext-diff` and `--no-textconv`
// survive an external diff driver, and `core.quotePath=false` stops a non-ASCII path arriving C-quoted.
// `--text` is deliberate: without it one NUL byte, or a `* -diff` written by whoever wrote the branch,
// collapses the body to "Binary files … differ" and the scan exits 0 over a real hit.
func gitDiff(cwd string, revisions []string) ([]byte, error) {
	args := []string{
		"-c", "core.quotePath=false",
		"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--text",
		"--src-prefix=a/", "--dst-prefix=b/",
	}
	if len(revisions) == 0 {
		args = append(args, "HEAD")
	} else {
		args = append(args, revisions...)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	return cmd.Output()
}

// `diff --git` is the anchor for a new file, never the `+++` line alone: every line in a diff *body*
// carries a `+`, `-` or space prefix, so no file content can forge a `diff --git`. Anchored on `+++`
// alone, an added line reading `+++ b/x.txt` reassigns the file and every added line after it is
// counted against the wrong one.
func scanDiff(diff []byte, cfg Config, files map[string]*counts, t *tally) {
	scanner := bufio.NewScanner(bytes.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	file := ""
	pending := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file = ""
			pending = true
			t.reached++
		case strings.HasPrefix(line, "+++ "):
			if pending {
				pending = false
				if strings.HasPrefix(line, "+++ b/") {
					file = line[len("+++ b/"):]
				}
			}
		case strings.HasPrefix(line, "+"):
			if file != "" {
				count(file, line[1:], cfg, files, t)
			}
		}
	}
}

// scanUntracked reads what no diff mentions. It never touches the index.
func scanUntracked(cwd string, cfg Config, files map[string]*counts, t *tally, stderr io.Writer) error {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		full := path.Join(cwd, name)
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			// A file that vanished mid-scan, or was never a regular file, contributes nothing — but it
			// was still never read, so it reaches the tally rather than disappearing from it.
			t.skipped++
			continue
		}
		if info.Size() > cfg.MaxFileBytes {
			t.skipped++
			continue
		}
		body, err := os.ReadFile(full)
		if err != nil {
			t.skipped++
			continue
		}
		if isBinary(body) {
			t.skipped++
			continue
		}
		t.reached++
		for _, line := range strings.Split(string(body), "\n") {
			count(name, line, cfg, files, t)
		}
	}
	return nil
}

func isBinary(body []byte) bool {
	window := body
	if len(window) > binaryProbeBytes {
		window = window[:binaryProbeBytes]
	}
	return bytes.IndexByte(window, 0) >= 0
}

// count classifies one added line against one file.
func count(file, raw string, cfg Config, files map[string]*counts, t *tally) {
	line := strings.TrimLeft(raw, " \t\r\n\v\f")
	if line == "" || isProseOrData(file) {
		return
	}
	entry, seen := files[file]
	if !seen {
		entry = &counts{}
		files[file] = entry
		t.countable++
	}
	if isComment(line) {
		entry.comments++
	} else {
		entry.code++
	}
}

// Prose and data carry no code to be dense against, so a ratio over them measures nothing. Lockfiles
// are matched by name as well as extension: they are generated, enormous, and never anybody's comments.
func isProseOrData(file string) bool {
	base := path.Base(file)
	switch strings.ToLower(path.Ext(base)) {
	case ".md", ".markdown", ".txt", ".json", ".lock":
		return true
	case ".yaml", ".yml":
		return strings.Contains(strings.ToLower(base), "lock")
	}
	return false
}

// The comment openers, on a line already stripped of leading whitespace: `//`, `/*`, and `#`, plus a
// continuation `*` or a closing `*/` where what follows is a space or the end of the line. That last
// condition is what keeps `*ptr = 1` and `*/2` counted as code — a bare `*` opening a multiplication or
// a dereference is not a comment, and counting it as one flags dense arithmetic as dense prose.
func isComment(line string) bool {
	switch {
	case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "/*"), strings.HasPrefix(line, "#"):
		return true
	}
	rest := ""
	switch {
	case strings.HasPrefix(line, "*/"):
		rest = line[2:]
	case strings.HasPrefix(line, "*"):
		rest = line[1:]
	default:
		return false
	}
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

func report(self string, cfg Config, files map[string]*counts, t tally, stdout, stderr io.Writer) int {
	// Sorted, so two runs over one tree print one report. The shell iterated an awk hash, whose order
	// is unspecified — which made a diff of two reports unreadable and a suppressed-outlier cap pick a
	// different 200 each time.
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	shown := 0
	for _, name := range names {
		entry := files[name]
		total := entry.comments + entry.code
		if total == 0 {
			continue
		}
		ratio := float64(entry.comments) / float64(total)
		if entry.comments < cfg.MinLines || ratio <= cfg.MaxRatio {
			continue
		}
		t.outliers++
		if shown < maxShown {
			shown++
			fmt.Fprintf(stdout, "%s: %d comment / %d code added lines (%.2f)\n",
				truncateRunes(name, maxPathRune), entry.comments, entry.code, ratio)
		}
	}
	if t.outliers > maxShown {
		fmt.Fprintf(stdout, "… and %d further outlier(s), not shown\n", t.outliers-maxShown)
	}

	// The denominator goes on stderr so the report on stdout stays exactly the outliers.
	fmt.Fprintf(stderr, "%s: %d file(s) reached the scan, %d with countable added lines, %d outlier(s), %d untracked file(s) skipped unread.\n",
		self, t.reached, t.countable, t.outliers, t.skipped)
	if t.reached == 0 {
		fmt.Fprintf(stderr, "%s: nothing reached the scan, so this run says nothing about the change set.\n", self)
	}
	if t.outliers > 0 {
		return exitFound
	}
	return exitClean
}

// Bounded by runes rather than bytes, so a path of multi-byte characters is not cut mid-character into
// a report that then holds invalid UTF-8.
func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
