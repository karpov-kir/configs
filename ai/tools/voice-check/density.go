// The register check for comments and prose, and the density figure beside it.
//
// Two modes, one question each. Bare arguments read the comments a change set added and report which
// sentences are written in the register the rule forbids (voice.go). `--density` reports how many
// comment lines the set carries beside the host repository's own rate (bar.go). No lane reads it.
package voicecheck

import (
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"configs/ai/tools/repo"
)

const (
	defaultMaxFileBytes = 262144
)

// The path and the file count are bounded, because under kk-pr they come from a branch somebody else
// wrote. The tool announces each suppressed line. That holds only while this cap and the cap in the
// announcement stay the same number.
const (
	maxShown     = 200
	maxPathBytes = 200
)
const (
	exitClean     = 0
	exitFound     = 1
	exitDidNotRun = 2
)

// The stub this command runs behind. The name is spelled here instead of read from argv[0].
// `stub_usage_test.go` compares this file's usage line against the stub's own header. A name that
// changed with how the binary was reached would leave that test no fixed text to compare.
const stubName = "voice-check.sh"

// Every form a caller writes by hand, in the order the binary takes them. The pathspec half is real: a
// bare path is refused where a revision belongs, and one after `--` narrows the scan to it.
//
// The stub's header states this line word for word, and a case holds the two together, so a flag added
// here is added there in the same edit.
const usage = "usage: " + stubName + " [--density | --per-file | --profile=comment|prose|instruction] [--source] [<git-diff revisions>] [-- <paths>]"

// console is the tool's name and its two streams. A finding goes to stdout bare. A note goes to
// stderr under the tool's name, and the rest of the package writes there through this type alone. The
// density mode prints its two shape lines on stdout and its denominator as a note.
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

// An argument this tool will not take. The refusal states the grammar as well as the complaint.
//
// It is held apart from refuse. Every other exit 2 here is a sound invocation the tool failed to
// carry out, such as a repository missing a baseline or a revision git could not resolve. A grammar
// printed there would point the caller at an argument that was already right.
func (c console) refuseArguments(err error) int {
	c.note("%v", err)
	c.note("%s", usage)
	return exitDidNotRun
}

type Config struct {
	MaxFileBytes int64
}

// ConfigFromEnv reads the only override. A value that does not parse refuses the run. A caller who set
// it asked for a bound, and falling back to the default would report a scan against a bound they did
// not pick.
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

// The density mode counts whole files, blocks included.
type stats struct {
	comments int
	// prose is the comment lines carrying words. A `/**`, a `*/` and a doc tag line are comment lines a
	// reader pays for, so they count in comments. They carry no sentence, so a block's LENGTH leaves
	// them out.
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

// Two modes, one question each. Bare is the register check and exits 1 on findings. `--density`
// reports a change set's comment lines beside the host repository's rate, and always exits 0,
// because no edit turns on that figure. `--density` selects the mode only as the first argument.
// Later in the arguments it is an option like any other, and refused as one.
func Run(self string, args []string, cwd string, git repo.Git, cfg Config, stdout, stderr io.Writer) int {
	out := console{self: self, stdout: stdout, stderr: stderr}
	if len(args) > 0 && args[0] == "--density" {
		return bar(out, args[1:], cwd, git, cfg)
	}
	return voice(out, args, cwd, git, cfg)
}

func notThisRepositorysSource(file string) bool {
	return isProseOrData(file) || isFixture(file)
}

// isFixture is Go's own reserved directory for a test's material. A file under it is a test's
// material, and it is routinely in another language, so counting it measures the fixture through the
// repository. A handful of TypeScript fixtures holds a Go repository to a foreign comment rate. The
// match is on a path segment, so `testdata/` at any depth is skipped.
func isFixture(file string) bool {
	return file == "testdata" || strings.HasPrefix(file, "testdata/") || strings.Contains(file, "/testdata/")
}

// Lockfiles are matched by name as well as extension. The yaml ones are generated, so their lines
// belong to no author.
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

// A continuation `*` or a closing `*/` counts only where a space or the end of the line follows. That
// keeps `*ptr = 1` and `*/2` counted as code. A bare `*` opens a dereference or a multiplication, and
// counting it as a comment would flag dense arithmetic as dense prose.
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
