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

// defaultRollDeadline bounds one roll of the model. A vote rolls every roll at once, so a judge run is
// bounded at one of these whatever its roll count, and can no longer block forever.
//
// Flat, not scaled by the text or by the load: a roll is spent waiting on the API, and neither
// predicts it. Over thirteen timed rolls, 13KB cost 104 seconds where 53KB cost 85, and a roll that
// took 119 seconds held 7% of a CPU.
//
// 420 is 2.8 times the slowest of those rolls, 150 seconds. Generous deliberately: this exists so a
// run ends, not so it ends soon, and a bound that clips an honest roll costs the whole gate. The 120
// it replaces sat inside the distribution and refused honest rolls at exit 2. Concurrency is what
// makes 420 affordable: one roll at a time, it would bound a run at 21 minutes.
const defaultRollDeadline = 420 * time.Second

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

// `name` is both the binary and the client, since each CLI is named after its client. `model`
// duplicates a string already inside `args`, so a failure can name it without parsing argv back.
type modelCommand struct {
	name  string
	args  []string
	stdin string
	dir   string
	model string
}

// The whole process group, because a roll spawns children and the point of Setpgid above is to reach
// them. Asked of os.Process first rather than killing the pid outright: Cancel runs on the deadline
// goroutine while Wait runs on this one, and between Wait reaping the child and receiving the
// watchCtx result the pid is already freed. A freed pid that the kernel has recycled is, because
// every roll sets Setpgid, immediately a live group leader belonging to somebody else — another
// session, a dev server — and the negated pid would deliver SIGKILL to all of it.
//
// os.Process.Signal holds the process's own lock and answers ErrProcessDone once reaped, which
// os/exec already reads as "finished, not a failure to cancel". This narrows the window from the
// whole Wait-to-channel gap down to the two adjacent syscalls below; it does not close it. Closing it
// needs pidfd or process handles, which darwin does not have.
func killRollGroup(p *os.Process) error {
	if err := p.Signal(syscall.Signal(0)); err != nil {
		return err
	}
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}

func runBounded(deadline time.Duration, command modelCommand) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.name, command.args...)
	cmd.Stdin = strings.NewReader(command.stdin)
	cmd.Dir = command.dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killRollGroup(cmd.Process) }
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		// Asked only once the call failed, so a call that answered just as the clock ran out is
		// reported as the answer it is.
		if ctx.Err() != nil {
			return "", fmt.Errorf("the model did not answer within %s", deadline)
		}
		// Ahead of the catch-all below, which is what used to swallow this: a name the provider will
		// not run came back as "did not answer".
		if refusedTheModel(command.name, command.stdin, out, err) {
			return "", &ModelRefused{Client: command.name, Model: command.model}
		}
		return "", fmt.Errorf("the model did not answer (%v)", err)
	}
	return string(out), nil
}
