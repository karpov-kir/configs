package readerjudge

import (
	"context"
	"encoding/json"
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

	"configs/ai/tools/flavorconfig"
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

const rollTimeoutKey = "roll-timeout"

const configName = "reader-judge.conf"

const minRollSeconds = 1

// The ceiling keeps `time.Duration(seconds)` far from its overflow near 9.2e9 seconds, where the
// deadline turns negative and every roll is cancelled before it starts.
const maxRollSeconds = 24 * 60 * 60

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

// rollDeadline answers how long one roll gets, plus the line to announce when this machine's override
// decided it. The line prints every run, so a tuned machine never looks like an untuned one.
func rollDeadline(configHome, home string) (time.Duration, string, error) {
	shipped := defaultRollDeadline
	seconds, err := secondsIn(flavorconfig.Path(home, configName), shipped)
	if err != nil {
		return 0, "", err
	}
	if seconds > 0 {
		shipped = time.Duration(seconds) * time.Second
	}
	path := overridePath(configHome, home)
	seconds, err = secondsIn(path, shipped)
	if err != nil {
		return 0, "", err
	}
	if seconds == 0 {
		return shipped, "", nil
	}
	deadline := time.Duration(seconds) * time.Second
	return deadline, fmt.Sprintf("a roll of the model gets %s, set by %s, in place of the default %s",
		deadline, path, shipped), nil
}

// secondsIn reads the roll-timeout the config at path carries, or zero when there is no such file.
// `fallback` is the number the refusal offers when it asks for the file to be removed. It is the
// number removing the file restores.
func secondsIn(path string, fallback time.Duration) (int, error) {
	settings, err := flavorconfig.Read(path, []string{rollTimeoutKey})
	if err != nil {
		return 0, fmt.Errorf("%w, so how long a roll of the model gets is unknown", err)
	}
	if settings == nil {
		return 0, nil
	}
	raw, set := settings[rollTimeoutKey]
	if !set {
		return 0, fmt.Errorf("%s sets no %s — add a `%s <seconds>` line, or remove the file to use the default of %s",
			path, rollTimeoutKey, rollTimeoutKey, fallback)
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < minRollSeconds || seconds > maxRollSeconds {
		return 0, fmt.Errorf("%s sets %s to %s, which is not a whole number of seconds between %d and %d",
			path, rollTimeoutKey, shell.Echoable(raw), minRollSeconds, maxRollSeconds)
	}
	return seconds, nil
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
// A reaped roll leaves its children to cmd.WaitDelay. They are reparented to launchd holding the
// roll's output pipe, and five seconds later os/exec closes it and returns.

// A sweep of the group stood here and has gone. Its claim was that a leader's pid being free says the
// group behind it is this roll's. A group outlives its leader, so a free pid says only that the pid is
// free. Any process that took that pid, made a group and exited leaves the same shape, which is the
// shape TestAReapedRollsChildrenAreLeftToTheWaitDelay, the case for this, builds.

// What the sweep bought was closing the pipe now instead of five seconds from now. What it risked was
// SIGKILL to a process group belonging to another session. The second is worse than the first.
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
		// A limit arrives at a failing exit too: `claude -p --output-format json` exits 1 with the
		// apology in its result. A check on success only left a table of 404 failed calls on
		// 2026-09-25 reporting "exit status 1" with no reason.
		var failed *exec.ExitError
		said := out
		if errors.As(err, &failed) {
			said = append(append([]byte(nil), out...), failed.Stderr...)
		}
		if line := exhausted(command.name, command.stdin, said); line != "" {
			return "", &ProviderExhausted{Client: command.name, Said: line}
		}
		return "", fmt.Errorf("the model did not answer (%v): %s", err, whatItSaid(said))
	}
	// Asked of a call that SUCCEEDED, which is the whole reason it is here: a provider out of
	// capacity answers at exit 0 with its apology where the verdict goes. The judge survives that
	// anyway, since an apology is not a list of unit numbers — but model-check's probe asks for a
	// single character and takes any answer, so a rate-limited machine measured every name in
	// models.json as running. Read as an answer, this is a green over a question nobody reached.
	if line := exhausted(command.name, command.stdin, out); line != "" {
		return "", &ProviderExhausted{Client: command.name, Said: line}
	}
	return string(out), nil
}

// whatItSaid is the CLI's own account of a failed call, cut to a message's length. It is the JSON
// result where the CLI printed one, or else the output as it came.
func whatItSaid(out []byte) string {
	var reply struct {
		Result string `json:"result"`
	}
	text := string(out)
	if json.Unmarshal(out, &reply) == nil && reply.Result != "" {
		text = reply.Result
	}
	if strings.TrimSpace(text) == "" {
		return "it printed nothing"
	}
	return shell.CutBytesMarked(shell.Oneline(text), 300)
}
