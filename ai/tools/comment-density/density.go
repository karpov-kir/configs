// Comment-density detector. By default Run flags changed source files whose ADDED lines are
// comment-heavy. With `--bar` it holds the whole change set to the host repo's own comment rate
// (bar.go). With `--voice` it reads the comments instead of counting them (voice.go). The
// command-line contract (arguments, environment, exit codes) is the stub's:
// ai/kk-flavor/skills/kk-edit/scripts/comment-density.sh.
//
// The default mode states its own standing in its report: a targeting aid, not a bar.
package density

import (
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"kk-flavor/tools/diffscan"
	gitrepo "kk-flavor/tools/repo"
	"kk-flavor/tools/shell"
)

const (
	defaultMaxRatio     = 0.3
	defaultMinLines     = 5
	defaultMaxFileBytes = 262144
)

// Both the path and the outlier count are bounded, because under kk-pr they come from a branch
// somebody else wrote. A suppressed outlier is announced, never dropped, and that holds only while this
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

// The stub this command runs behind, written out rather than read from argv[0]. `stub_usage_test.go`
// compares the usage line below against the one the stub's own header documents, and a name that
// changes with how the binary was reached leaves it nothing stable to compare.
const stubName = "comment-density.sh"

// Every form the binary takes, in the order it takes them. The pathspec half is real: a bare path is
// refused where a revision belongs, and one after `--` narrows the scan to it.
const usage = "usage: " + stubName + " [--bar | --voice [--profile=comment|prose|instruction]] [<git-diff revisions>] [-- <paths>]"

// console is the tool's name and its two streams. Findings go to stdout bare; a note on stderr opens
// with the name, and nothing else in the package writes there. The default mode's denominator is a
// note too, so its stdout is exactly the outliers; the bar prints its two shape lines on stdout.
type console struct {
	self   string
	stdout io.Writer
	stderr io.Writer
}

func (c console) note(format string, args ...any) {
	fmt.Fprintf(c.stderr, "%s: %s\n", c.self, fmt.Sprintf(format, args...))
}

func (c console) refuse(err error) int {
	c.note("%v", err)
	return exitDidNotRun
}

// An argument this tool will not take, answered with the grammar as well as the complaint. Held apart
// from refuse because every other exit 2 here is a sound invocation the tool could not carry out — a
// repository with no baseline, a revision git would not resolve — and printing the grammar there sends
// the caller to fix an argument that was already right.
func (c console) refuseArguments(err error) int {
	c.note("%v", err)
	c.note("%s", usage)
	return exitDidNotRun
}

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

// The default mode counts a file's added lines alone, so its blocks stay at zero; the bar counts whole
// files, blocks included.
type stats struct {
	comments int
	// prose is the comment lines carrying words. A `/**`, a `*/` and a doc tag line are comment lines
	// a reader pays for, so they count in comments; they carry no sentence, so a block's LENGTH is
	// measured without them.
	prose      int
	code       int
	blocks     int
	longBlocks int
}

func (s stats) total() int { return s.comments + s.code }

func (s stats) ratio() float64 {
	if s.total() == 0 {
		return 0
	}
	return float64(s.comments) / float64(s.total())
}

func (s stats) meanBlock() float64 {
	if s.blocks == 0 {
		return 0
	}
	return float64(s.comments) / float64(s.blocks)
}

func (s stats) longShare() float64 {
	if s.blocks == 0 {
		return 0
	}
	return float64(s.longBlocks) / float64(s.blocks)
}

func (s *stats) add(other stats) {
	s.comments += other.comments
	s.prose += other.prose
	s.code += other.code
	s.blocks += other.blocks
	s.longBlocks += other.longBlocks
}

// scan is one run's accumulating state. Held together because every arm reads the config and writes
// the counts, and a file that reached `files` without reaching `countable` is the one inconsistency
// they must not be able to express.
type scan struct {
	cfg    Config
	files  map[string]*stats
	result diffscan.Result
	// Density's own two: how many files held a line this tool counts, and how many were over the bar.
	// Reached and SkippedUnread belong to the shared scan and live on result.
	countable int
	outliers  int
}

// `--bar` and `--voice` select a mode only as the first argument. Later in the arguments either is an
// option like any other, and refused as one. The two are exclusive: the bar says how many comment
// lines a change set may carry, the voice check says whether the lines it carries can be read.
//
// One port for the whole run, handed down to every seam rather than built at each: two adapters can
// answer about two repositories, and the change set would then be weighed against one clone while the
// baseline and the authorship came from another.
func Run(self string, args []string, cwd string, git gitrepo.Git, cfg Config, stdout, stderr io.Writer) int {
	out := console{self: self, stdout: stdout, stderr: stderr}
	if len(args) > 0 && args[0] == "--bar" {
		return bar(out, git, args[1:], cwd, cfg)
	}
	if len(args) > 0 && args[0] == "--voice" {
		return voice(out, git, args[1:], cwd, cfg)
	}
	return scanAddedLines(out, git, args, cwd, cfg)
}

func scanAddedLines(out console, git gitrepo.Git, args []string, cwd string, cfg Config) int {
	if err := diffscan.RefuseNonRevisions(git, args, cwd); err != nil {
		return out.refuseArguments(err)
	}
	s := &scan{cfg: cfg, files: map[string]*stats{}}

	diff, err := diffscan.Diff(git, cwd, args)
	if err != nil {
		return out.refuse(err)
	}
	if err := s.result.WalkDiff(diff, func(added diffscan.AddedLine) { s.count(added.File, added.Text) }); err != nil {
		return out.refuse(fmt.Errorf("the diff could not be read to the end (%v) — exit 2, the scan did NOT run over all of it. Not a clean result.", err))
	}

	// The untracked half runs only with no revisions: with revisions the caller named two commits, and
	// a file in neither of them is not part of what they asked about. SkipSecretNamed is off here and on
	// in dup-literals: this reports PATHS and counts, where that one echoes 60 bytes of every finding.
	named, _ := diffscan.RevisionsNamed(args)
	if len(named) == 0 {
		opts := diffscan.Options{MaxFileBytes: cfg.MaxFileBytes}
		if err := s.result.WalkUntracked(git, cwd, opts, func(added diffscan.AddedLine) { s.count(added.File, added.Text) }); err != nil {
			return out.refuse(errors.New("could not list untracked files — exit 2, the scan did NOT run over them."))
		}
	}

	return s.report(out)
}

func (s *scan) count(file, raw string) {
	line := strings.TrimLeft(raw, shell.SpaceBytes)
	if line == "" || notThisRepositorysSource(file) {
		return
	}
	entry, seen := s.files[file]
	if !seen {
		entry = &stats{}
		s.files[file] = entry
		s.countable++
	}
	if isComment(line) {
		entry.comments++
	} else {
		entry.code++
	}
}

// notThisRepositorysSource is every reason a discovered file is none of this repository's business.
// One function, so the diff scan, the bar's baseline and the voice check cannot disagree about what a
// run covers. A file NAMED on the command line is not put through it: naming a fixture is asking for it.
func notThisRepositorysSource(file string) bool {
	return isProseOrData(file) || isFixture(file)
}

// isFixture is Go's own reserved directory for a test's material. A file under it is written to be
// read by a test rather than to be this repository's source, and it is routinely in another language
// — so counted, it measures the fixture through the repository. A handful of TypeScript fixtures is
// enough to hold a Go and shell repository to a foreign language's comment rate. Matched as a path
// segment, so `testdata/` at any depth is skipped.
func isFixture(file string) bool {
	return file == "testdata" || strings.HasPrefix(file, "testdata/") || strings.Contains(file, "/testdata/")
}

// Lockfiles are matched by name as well as extension: the yaml ones are generated, and nobody's comments.
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

// A continuation `*` or a closing `*/` counts only where a space or the end of the line follows. That is
// what keeps `*ptr = 1` and `*/2` counted as code: a bare `*` opening a dereference or a multiplication
// is not a comment, and counting it as one flags dense arithmetic as dense prose.
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

func (s *scan) report(out console) int {
	// Sorted, so two runs over one tree print one report.
	names := make([]string, 0, len(s.files))
	for name := range s.files {
		names = append(names, name)
	}
	sort.Strings(names)

	shown := 0
	for _, name := range names {
		entry := s.files[name]
		ratio := entry.ratio()
		if entry.comments < s.cfg.MinLines || ratio <= s.cfg.MaxRatio {
			continue
		}
		s.outliers++
		if shown < maxShown {
			shown++
			fmt.Fprintf(out.stdout, "%s: %d comment / %d code added lines (%.2f)\n",
				shell.CutBytesMarked(shell.Oneline(name), maxPathBytes), entry.comments, entry.code, ratio)
		}
	}
	if s.outliers > maxShown {
		fmt.Fprintf(out.stdout, "… and %d further outlier(s), not shown\n", s.outliers-maxShown)
	}

	out.note("%d file(s) reached the scan, %d with countable added lines, %d outlier(s), %d untracked file(s) skipped unread.",
		s.result.Reached, s.countable, s.outliers, s.result.SkippedUnread)
	if s.result.Reached == 0 {
		out.note("nothing reached the scan, so this run says nothing about the change set.")
	} else if s.countable == 0 {
		out.note("no file reached carried a countable added line, so this run ranks nothing.")
	} else {
		out.note("a targeting aid, not a bar: this ranks files to read. It counts ADDED lines only, so rewording an old comment raises a ratio. --bar holds the change set to the repo's own rate.")
	}
	if s.outliers > 0 {
		return exitFound
	}
	return exitClean
}
