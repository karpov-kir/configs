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
package density

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"kk-flavor/tools/diffscan"
	"kk-flavor/tools/shell"
)

const (
	defaultMaxRatio     = 0.3
	defaultMinLines     = 5
	defaultMaxFileBytes = 262144
)

// Both the path and the outlier count are bounded, because under kk-pr-review they come from a branch
// somebody else wrote. A suppressed outlier is announced, never dropped — which holds only while this
// cap and the one in the announcement stay the same number.
const (
	maxShown     = 200
	maxPathBytes = 200
)
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
		if err != nil || value < 0 || value > 1 {
			return cfg, fmt.Errorf("COMMENT_MAX_RATIO is %q, which is no share between 0 and 1 — the scan did NOT run", raw)
		}
		cfg.MaxRatio = value
	}
	if raw, ok := lookup("COMMENT_MIN_LINES"); ok && raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return cfg, fmt.Errorf("COMMENT_MIN_LINES is %q, which is no positive whole number — the scan did NOT run", raw)
		}
		cfg.MinLines = value
	}
	if raw, ok := lookup("DENSITY_MAX_FILE_BYTES"); ok && raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return cfg, fmt.Errorf("DENSITY_MAX_FILE_BYTES is %q, which is no whole number of bytes — the scan did NOT run", raw)
		}
		cfg.MaxFileBytes = value
	}
	return cfg, nil
}

type counts struct {
	comments int
	code     int
}

// scan is one run's accumulating state. Held together because every arm reads the config and writes
// the counts, and a file that reached `files` without reaching `countable` is the one inconsistency
// they must not be able to express.
type scan struct {
	cfg    Config
	files  map[string]*counts
	result diffscan.Result
	// Density's own two: how many files held a line this tool counts, and how many cleared the bar.
	// Reached and SkippedUnread belong to the shared scan and live on result.
	countable int
	outliers  int
}

func Run(self string, args []string, cwd string, cfg Config, stdout, stderr io.Writer) int {
	// Reaching the added lines is `kk-flavor/tools/diffscan`'s, shared with dup-literals: which
	// arguments are refused, the git flags that pin the diff's shape, what counts as binary, and the
	// `diff --git` anchor that stops a file's own content forging a header.
	if err := diffscan.RefuseNonRevisions(args, cwd); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", self, err)
		return exitDidNotRun
	}
	s := &scan{cfg: cfg, files: map[string]*counts{}}

	diff, err := diffscan.Diff(cwd, args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", self, err)
		return exitDidNotRun
	}
	if err := s.result.WalkDiff(diff, func(added diffscan.AddedLine) { s.count(added.File, added.Text) }); err != nil {
		fmt.Fprintf(stderr, "%s: the diff could not be read to the end (%v) — exit 2, the scan did NOT run over all of it. Not a clean result.\n", self, err)
		return exitDidNotRun
	}

	// The untracked half runs only with no revisions: with revisions the caller named two commits, and
	// a file in neither of them is not part of what they asked about.
	//
	// SkipSecretNamed is off here and on in dup-literals, and the difference is not an oversight: this
	// reports PATHS and counts, where that one echoes 60 bytes of every finding.
	if len(args) == 0 {
		opts := diffscan.Options{MaxFileBytes: cfg.MaxFileBytes}
		if err := s.result.WalkUntracked(cwd, opts, func(added diffscan.AddedLine) { s.count(added.File, added.Text) }); err != nil {
			fmt.Fprintf(stderr, "%s: could not list untracked files — exit 2, the scan did NOT run over them.\n", self)
			return exitDidNotRun
		}
	}

	return s.report(self, stdout, stderr)
}

func (s *scan) count(file, raw string) {
	line := strings.TrimLeft(raw, shell.SpaceBytes)
	if line == "" || isProseOrData(file) {
		return
	}
	entry, seen := s.files[file]
	if !seen {
		entry = &counts{}
		s.files[file] = entry
		s.countable++
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

func (s *scan) report(self string, stdout, stderr io.Writer) int {
	// Sorted, so two runs over one tree print one report.
	names := make([]string, 0, len(s.files))
	for name := range s.files {
		names = append(names, name)
	}
	sort.Strings(names)

	shown := 0
	for _, name := range names {
		// count creates an entry only while incrementing one of the two counters, so total is at
		// least 1 here and the ratio below has no zero denominator to guard.
		entry := s.files[name]
		total := entry.comments + entry.code
		ratio := float64(entry.comments) / float64(total)
		if entry.comments < s.cfg.MinLines || ratio <= s.cfg.MaxRatio {
			continue
		}
		s.outliers++
		if shown < maxShown {
			shown++
			fmt.Fprintf(stdout, "%s: %d comment / %d code added lines (%.2f)\n",
				shell.CutBytesMarked(shell.Oneline(name), maxPathBytes), entry.comments, entry.code, ratio)
		}
	}
	if s.outliers > maxShown {
		fmt.Fprintf(stdout, "… and %d further outlier(s), not shown\n", s.outliers-maxShown)
	}

	// The denominator goes on stderr so the report on stdout stays exactly the outliers.
	fmt.Fprintf(stderr, "%s: %d file(s) reached the scan, %d with countable added lines, %d outlier(s), %d untracked file(s) skipped unread.\n",
		self, s.result.Reached, s.countable, s.outliers, s.result.SkippedUnread)
	if s.result.Reached == 0 {
		fmt.Fprintf(stderr, "%s: nothing reached the scan, so this run says nothing about the change set.\n", self)
	}
	if s.outliers > 0 {
		return exitFound
	}
	return exitClean
}
