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

// defaultRollDeadline bounds one roll of the model. The vote's rolls run in sequence, so a judge run
// is bounded at three times this and can no longer block forever.
//
// Read from measurement, not chosen. Timed on the machine and the load that produced the stall — five
// ship sessions and their audits at once — a `return` run took 30 to 49 seconds over a two-line text,
// 66 to 117 over a five-line one and 36 to 52 over a forty-line one, five samples each. Three rolls to
// a run puts the worst roll measured at about 39 seconds, so a bound of 60 would cut off work that
// finished honestly and 120 leaves three times over. That upper sample also reproduces the run the
// report described: slow, past a caller's 120-second patience, and not hung at all.
//
// Flat rather than scaled by the text, because the forty-line run was the *faster* one: what a roll
// costs tracks how loaded the machine is, not how much it was given to read.
//
// The number is the weaker half of the fix. What matters is that a roll ends and says it did: a
// deadline set too tight refuses loudly, at exit 2, where no deadline at all left a mandatory gate
// skipped in silence.
const defaultRollDeadline = 120 * time.Second

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

// runBounded runs one model call and kills it when the deadline passes.
//
// The process *group*, not the process: `claude` starts children of its own, and killing only the one
// we started leaves them running on a machine whose stalls already track its load. WaitDelay covers
// what is left — a surviving grandchild holding the output pipe would keep this blocked long after
// the child is gone, which is the hang the deadline exists to remove.
func runBounded(deadline time.Duration, name string, args []string, stdin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
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
