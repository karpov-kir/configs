package bloatjudge

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kk-flavor/tools/shell"
)

// defaultRollDeadline bounds one roll of the model. The vote rolls a wave at a time, so a judge run is
// bounded at two of these rather than three, and can no longer block forever — and at one whenever the
// quorum settles, which is the common case.
//
// Read from measurement. Seven rolls timed here — 68, 85, 95, 98, 104, 119 and 150 seconds — over
// texts from 13KB up to the 80KB a decision record at its 100-entry cap presents, beside the 30, 36,
// 49, 52, 66 and 117 the earlier reading quotes over texts of two to forty lines.
//
// Nothing in that spread is the text: 13KB cost 104 seconds where 53KB cost 85. Nor is it the machine
// — the 119-second roll held 7% of a CPU, and three rolls run at once finished in the time one of them
// took. A roll is spent waiting on the API, so the bound scales with neither the text nor the load.
//
// 120 came from this same distribution read as though its worst roll were 39 seconds, and landed
// inside it: one of the thirteen rolls above exceeds it, and two more land within three seconds
// of it. That is the reported stall — a `record-entry`
// judge over a full record, refused at exit 2 twice running, cut off mid-answer rather than hung.
//
// 420 is 2.8 times the slowest roll seen. Generous deliberately: this exists so a run ends, not so it
// ends soon, and a bound that clips an honest roll costs the caller the whole gate. Concurrency is
// what makes it affordable — one roll at a time, this figure would bound a run at 21 minutes.
const defaultRollDeadline = 420 * time.Second

// The only line the override file may carry.
const overrideKey = "roll-timeout"

// overridePath is where this machine tunes the deadline — the one place ecosystem.md → **Conventions
// a new file joins** puts a machine-local value. Never in the tree: `~/.kk-flavor` is a symlink into
// the checkout, so a value tuned there would travel to everyone on the next commit.
//
// A config home that is not absolute is treated as unset, which is what the XDG spec says to do with
// one. Taken as given, a checkout shipping `cfg/kk-flavor/bloat-judge.conf` would set the bound for
// every run made from inside it, and the tree under review does not get to decide how long its own
// judge waits. Empty when neither path is absolute: there is then no location an override could sit
// at, so there is none to miss.
func overridePath(configHome, home string) string {
	if !filepath.IsAbs(configHome) {
		if !filepath.IsAbs(home) {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "kk-flavor", "bloat-judge.conf")
}

// ResolveRollDeadline is how a command asks: it reports the deadline and says whether the judge may
// run at all, having already put the announcement or the refusal on stderr. A refusal is worded like
// every other way this tool declines to run, so a caller reading stderr meets one vocabulary.
func ResolveRollDeadline(self, configHome, home string, stderr io.Writer) (time.Duration, bool) {
	deadline, announcement, err := rollDeadline(configHome, home)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
		return 0, false
	}
	if announcement != "" {
		fmt.Fprintf(stderr, "%s: %s\n", self, announcement)
	}
	return deadline, true
}

// rollDeadline answers how long one roll gets, plus the line to announce when an override decided it —
// printed every run, so a tuned machine never looks like an untuned one.
//
// A file that is present but unusable is refused rather than ignored, and no path here falls back to
// the default: a default quietly restored is indistinguishable from the override working. Absent is a
// different thing from broken, and only absent is quiet.
func rollDeadline(configHome, home string) (time.Duration, string, error) {
	path := overridePath(configHome, home)
	// `IsSymlink` as well, so a dangling link refuses instead of reading as absent — an existence test
	// alone cannot see one.
	if path == "" || (!shell.PathExists(path) && !shell.IsSymlink(path)) {
		return defaultRollDeadline, "", nil
	}
	if !shell.IsRegularFile(path) {
		return 0, "", fmt.Errorf("%s is not a readable regular file, so how long a roll of the model gets is unknown", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", fmt.Errorf("could not read %s (%v), so how long a roll of the model gets is unknown", path, err)
	}
	seconds := 0
	for _, line := range shell.SplitLines(string(raw)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Refused rather than skipped: a line the human meant as a setting, silently ignored, is a
		// bound they think they changed and did not. The echo is one-lined, this file being written
		// by hand and its content reaching a terminal.
		fields := shell.SplitFields(trimmed)
		if len(fields) != 2 || fields[0] != overrideKey {
			return 0, "", fmt.Errorf("%s has a line this does not understand: %s — the only supported line is `%s <seconds>`",
				path, echoable(trimmed), overrideKey)
		}
		if seconds != 0 {
			return 0, "", fmt.Errorf("%s sets %s more than once — which one wins is not this tool's guess to make", path, overrideKey)
		}
		if seconds, err = strconv.Atoi(fields[1]); err != nil || seconds < 1 {
			return 0, "", fmt.Errorf("%s sets %s to %s, which is not a whole number of seconds above zero",
				path, overrideKey, echoable(fields[1]))
		}
	}
	if seconds == 0 {
		return 0, "", fmt.Errorf("%s sets no %s — add a `%s <seconds>` line, or remove the file to use the default of %s",
			path, overrideKey, overrideKey, defaultRollDeadline)
	}
	deadline := time.Duration(seconds) * time.Second
	return deadline, fmt.Sprintf("a roll of the model gets %s, set by %s, in place of the default %s",
		deadline, path, defaultRollDeadline), nil
}

type modelCommand struct {
	name  string
	args  []string
	stdin string
	dir   string
}

func runBounded(deadline time.Duration, command modelCommand) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.name, command.args...)
	cmd.Stdin = strings.NewReader(command.stdin)
	cmd.Dir = command.dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		// Asked only once the call failed, so a call that answered just as the clock ran out is
		// reported as the answer it is.
		if ctx.Err() != nil {
			return "", fmt.Errorf("the model did not answer within %s", deadline)
		}
		return "", fmt.Errorf("the model did not answer (%v)", err)
	}
	return string(out), nil
}
