// The register check for comments and prose, and the density figure beside it.
//
// Two modes, one question each. Bare arguments read the comments a change set added and report which
// sentences are written in the register the rule forbids (voice.go). `--density` reports how many
// comment lines the set carries beside the host repository's own rate (bar.go), and acts on nothing.
package voicecheck

import (
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

const (
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
const stubName = "voice-check.sh"

// Every form the binary takes, in the order it takes them. The pathspec half is real: a bare path is
// refused where a revision belongs, and one after `--` narrows the scan to it.
const usage = "usage: " + stubName + " [--density | --profile=comment|prose|instruction] [<git-diff revisions>] [-- <paths>]"

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
	MaxFileBytes int64
}

// ConfigFromEnv reads the one override. A value that does not parse is not silently replaced by the
// default: a caller who set it asked for something, and answering with the default reports a scan
// against a bound they did not choose.
func ConfigFromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MaxFileBytes: defaultMaxFileBytes}
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

// Two modes, one question each. Bare is the register check and exits 1 on findings. `--density`
// reports how many comment lines a change set carries beside the host repository's rate, and always
// exits 0, because nothing acts on that figure.
//
// `--density` selects the mode only as the first argument. Later in the arguments it is an option
// like any other, and refused as one.
func Run(self string, args []string, cwd string, cfg Config, stdout, stderr io.Writer) int {
	out := console{self: self, stdout: stdout, stderr: stderr}
	if len(args) > 0 && args[0] == "--density" {
		return bar(out, args[1:], cwd, cfg)
	}
	return voice(out, args, cwd, cfg)
}

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
