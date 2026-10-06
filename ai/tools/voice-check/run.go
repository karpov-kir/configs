package voicecheck

import (
	"fmt"
	"io"
	"strconv"

	"configs/ai/tools/flavorconfig"
	"configs/ai/tools/repo"
	"configs/ai/tools/shell"
)

const (
	defaultMaxFileBytes = 262144
)

// A path echoed in a finding is bounded, because under kk-pr it can come from a branch somebody else
// wrote.
const maxPathBytes = 200
const (
	exitClean     = 0
	exitFound     = 1
	exitDidNotRun = 2
)

// The stub this command runs behind. The name is spelled here instead of read from argv[0].
// `stub_usage_test.go` compares this file's usage line against the stub's own header. A name that
// changed with how the binary was reached would leave that test no fixed text to compare.
const stubName = "voice-check.sh"

// Every form a caller writes by hand, in the order the binary takes them. A path that starts with `-`
// goes after `--`, where no flag is read.
//
// The stub's header states this line word for word, and a case holds the two together, so a flag added
// here is added there in the same edit.
const usage = "usage: " + stubName + " [--profile=prose|instruction] [--kind=pr-body|ticket] [--per-file] [--] <path>|- ..."

// console is the tool's name and its two streams. A finding goes to stdout bare. A note goes to
// stderr under the tool's name, and the rest of the package writes there through this type alone.
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
// carry out, such as a path that cannot be read. A grammar printed there would point the caller at an
// argument that was already right.
func (c console) refuseArguments(err error) int {
	c.note("%v", err)
	c.note("%s", usage)
	return exitDidNotRun
}

type Config struct {
	MaxFileBytes int64
}

const configName = "voice-check.conf"

// LoadConfig resolves the byte cap: the tracked default this tool ships in configs, then this
// run's environment on top. A value that does not parse refuses the run. A caller who set one asked
// for a bound, and falling back to the default would report a scan against a bound they did not pick.
func LoadConfig(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{MaxFileBytes: defaultMaxFileBytes}
	home, _ := lookup("HOME")
	path := flavorconfig.Path(home, configName)
	shipped, err := flavorconfig.Read(path, []string{"max-file-bytes"})
	if err != nil {
		return cfg, fmt.Errorf("%w — the scan did NOT run", err)
	}
	raw, source := shipped["max-file-bytes"], path+"'s `max-file-bytes`"
	if set, ok := lookup("DENSITY_MAX_FILE_BYTES"); ok && set != "" {
		raw, source = set, "DENSITY_MAX_FILE_BYTES"
	}
	if raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return cfg, fmt.Errorf("%s is %q, which is no whole number of bytes — the scan did NOT run", source, shell.Echoable(raw))
		}
		cfg.MaxFileBytes = value
	}
	return cfg, nil
}

// Run is the command: the flags, then the paths the scan reads. It exits 1 on findings, 0 when clean
// and 2 when the scan did not run.
func Run(self string, args []string, cwd string, git repo.Git, cfg Config, stdout, stderr io.Writer) int {
	return voice(console{self: self, stdout: stdout, stderr: stderr}, args, cwd, git, cfg)
}
