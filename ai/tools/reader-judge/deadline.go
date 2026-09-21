package readerjudge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"configs/ai/tools/shell"
)

// defaultRollDeadline bounds one roll of the model. A vote rolls every roll at once, so a judge run is
// bounded at one of these whatever its roll count, and can no longer block forever.
//
// Flat, not scaled by the text: what is judged barely moves the clock. Measured 2026-09-16 over
// twenty runs of the shipped path — codex, gpt-5.6-luna at low effort, three rolls a run — against
// 9KB to 53KB of this repo's own standards, memo defeated each time, on a machine already carrying
// three other gate runs at load 5 to 8. Eighteen finished in 19 to 45 seconds, six times the text
// buying about twice the clock.
//
// The other two are why this is 900 and not 45. Two consecutive runs, different payloads, took 344
// and 342 seconds — silent throughout, and each went on to answer correctly. That is an honest roll,
// which is the one thing a bound may not clip, since clipping one costs the whole gate at exit 2.
//
// 420 was 2.8 times the slowest roll then known; against 343 it had become 1.22 while still calling
// itself generous. 900 restores the ratio. Concurrency is what makes it affordable, every roll of a
// vote waiting at once — one at a time it would bound a run at 45 minutes.
const defaultRollDeadline = 900 * time.Second

const overrideKey = "roll-timeout"

// overridePath is where this machine tunes the deadline — the one place ecosystem.md → **Conventions
// a new file joins** puts a machine-local value. Never in the tree: `~/.kk-flavor` is a symlink into
// the checkout, so a value tuned there would travel to everyone on the next commit.
//
// A config home that is not absolute is treated as unset, which is what the XDG spec says to do with
// one. A relative config home taken as given would let a checkout shipping
// `cfg/kk-flavor/reader-judge.conf` set the bound for every run made from inside it. The tree under
// review does not get to decide how long its own judge waits. Empty when both paths are relative: an
// override has nowhere to sit, so a run has nowhere to look.
func overridePath(configHome, home string) string {
	if !filepath.IsAbs(configHome) {
		if !filepath.IsAbs(home) {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "kk-flavor", "reader-judge.conf")
}

// The path is returned alongside the bound, and not only when an override set it: a roll that times
// out is the one failure whose repair is a line in that file, and the file is worth naming whether it
// exists yet or not. Empty when there is nowhere for one to sit, which overridePath explains.
func ResolveRollDeadline(self, configHome, home string, stderr io.Writer) (time.Duration, string, bool) {
	path := overridePath(configHome, home)
	deadline, announcement, err := rollDeadline(configHome, home)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v — the judge did NOT run\n", self, err)
		return 0, "", false
	}
	if announcement != "" {
		fmt.Fprintf(stderr, "%s: %s\n", self, announcement)
	}
	return deadline, path, true
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

// rollEnvironment is every variable a roll is handed. A provider CLI needs the machine's own shape
// and its login and nothing else, so the child gets this named list rather than whatever the calling
// process carries. `ANTHROPIC_BASE_URL` set by a devcontainer, by a `.envrc` in the very checkout
// under review, or by a CI step would otherwise point every roll at an endpoint of someone else's
// choosing — which both reads the judged text and dictates the verdict, with nothing in the output
// saying so.
//
// A credential passes and a destination does not. A key says who is asking; a base URL says where the
// question goes, and only the second moves the answer somewhere nobody here chose. `LC_` is a prefix
// rather than a name because the locale variables are a family and the CLIs read whichever the
// machine sets.
var rollEnvironment = []string{
	"HOME", "PATH", "USER", "LOGNAME", "SHELL", "TMPDIR", "TERM", "LANG", "LC_",
	"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "SSL_CERT_FILE", "SSL_CERT_DIR",
	"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "OPENAI_API_KEY", "CODEX_HOME",
}

// rollEnv is the calling process's environment cut down to rollEnvironment. Built from the caller's
// own slice rather than read from the process, so the suite can drive it without setting anything.
func rollEnv(environ []string) []string {
	var kept []string
	for _, entry := range environ {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		for _, allowed := range rollEnvironment {
			if name == allowed || strings.HasSuffix(allowed, "_") && strings.HasPrefix(name, allowed) {
				kept = append(kept, entry)
				break
			}
		}
	}
	return kept
}

// `name` is both the binary and the client, since each CLI is named after its client. `model`
// duplicates a string already inside `args`, so a failure can name it without parsing argv back.
type modelCommand struct {
	name  string
	args  []string
	stdin string
	dir   string
	model string
	// env is added on top of the allow-list above, for a caller measuring a variable the allow-list
	// exists to drop. The eval is the only one: a variant that set `MAX_THINKING_TOKENS` in its own
	// process would now be dropped at the child and would quietly measure the baseline under another
	// name, which is the one failure an eval may not have.
	env []string
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
//
// A reaped child is not an empty group, which is why ErrProcessDone leaves work here to do. A provider
// that exits leaving children behind has them reparented to launchd, still holding the roll's output
// pipe. The roll then waits on them for the whole of cmd.WaitDelay, which is the very hang the group
// kill exists to end.
func killRollGroup(p *os.Process) error {
	if err := p.Signal(syscall.Signal(0)); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			sweepTheAbandonedGroup(p.Pid)
		}
		return err
	}
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}

// Making a group takes being the pid it is named after. A pid held by no process cannot name a group
// another process made, so an unused pid is what says the group behind it is still this roll's. ESRCH
// from the sweep is the group having emptied itself, which is the outcome asked for.

// sweepTheAbandonedGroup kills what a reaped roll left running, and stays silent once the leader's pid
// belongs to another process.
func sweepTheAbandonedGroup(leader int) {
	if !errors.Is(syscall.Kill(leader, syscall.Signal(0)), syscall.ESRCH) {
		return
	}
	_ = syscall.Kill(-leader, syscall.SIGKILL)
}

func runBounded(deadline time.Duration, command modelCommand) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.name, command.args...)
	cmd.Stdin = strings.NewReader(command.stdin)
	cmd.Dir = command.dir
	cmd.Env = append(rollEnv(os.Environ()), command.env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killRollGroup(cmd.Process) }
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		// Asked only once the call failed, so a call that answered just as the clock ran out is
		// reported as the answer it is.
		if ctx.Err() != nil {
			return "", &RollTimedOut{Deadline: deadline}
		}
		// Ahead of the catch-all below, which is what used to swallow this: a name the provider will
		// not run came back as "did not answer".
		if refusedTheModel(command.name, command.stdin, out, err) {
			return "", &ModelRefused{Client: command.name, Model: command.model}
		}
		return "", fmt.Errorf("the model did not answer (%v)", err)
	}
	// Asked of a call that SUCCEEDED, which is the whole reason it is here: a provider out of
	// capacity answers at exit 0 with its apology where the verdict goes. The judge survives that
	// anyway, since an apology is not a list of unit numbers — but model-check's probe asks for a
	// single character and takes any answer, so a rate-limited machine measured every name in
	// models.json as running. Read as an answer, this is a green over a question nobody reached.
	if exhausted(command.name, command.stdin, out) {
		return "", &ProviderExhausted{Client: command.name}
	}
	return string(out), nil
}
