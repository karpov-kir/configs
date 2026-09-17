// Package waitreap finds the wait loops sessions leave running and ends the abandoned ones. It
// reports only, unless `--kill` is passed.
//
//	usage: wait-reap.sh [--kill] [--idle-for <duration>] [--stale-after <duration>]
//
// A background task is parented to a daemon that outlives the session, so a loop polling for a file
// no session will write keeps running until the machine reboots. `ai/README.md` → Wait loops sessions
// leave behind states which waiters `--kill` ends; verdictOf decides each of those tests.
//
// tested by: the Go suite beside this file.
package waitreap

import (
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
)

// Exit codes, on the tools' shared vocabulary: 2 is "did not run", 3 is "ran and refuses a result".
// A listing that cannot be read is exit 3: the process table was reached, and what came back is what
// cannot be trusted.
const (
	exitOK        = 0
	exitDidNotRun = 2
	exitRefused   = 3
)

// A foreground call runs for at most ten minutes, and this cap adds five minutes of margin: the ten
// are counted on the call, and the process carrying it can have started before the call did. Under
// this age a waiter may be a call a live session is sitting inside, and ending it fails that call.
const foregroundCap = 15 * time.Minute

const defaultIdleFor = 30 * time.Minute

// How long a waiter has to have been running before anything ends it, whatever its session says. For
// a waiter no session claims it is the only evidence there is, because its own command names none.
const defaultStaleAfter = 2 * time.Hour

// The name every message carries. A caller reaches this tool through the stub, so a message names the
// stub and its usage line matches the line the stub's own header documents.
const stubName = "wait-reap.sh"

// By absolute path, because this one listing both supplies the kill list and confirms it: a `ps`
// earlier in PATH could name any pid as an abandoned waiter and then agree with itself when asked
// again.
const psCommand = "/bin/ps"

type verdict string

const (
	// `stray` is reached three ways, all of them past the stale window — a named session gone quiet, a
	// named session with no transcript at all, and a waiter naming no session. They are one verdict
	// because the act is the same; the evidence column is what separates them.
	verdictStray   verdict = "stray"
	verdictHeld    verdict = "held"
	verdictYoung   verdict = "young"
	verdictUnowned verdict = "unowned"
	verdictForeign verdict = "foreign"
	verdictMonitor verdict = "monitor"
)

// environment is what the reaper reads the machine through, so the suite drives it without spawning
// a process or ending one.
type environment struct {
	now          time.Time
	listing      func() (string, error)
	projectsRoot string
	recheck      func(pid int) (string, error)
	kill         func(pid int) error
}

// Run judges the machine's own wait loops and returns an exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "%s: no home directory, so no session could be attributed: %s\n", stubName, err)
		return exitDidNotRun
	}
	env := environment{
		now:          time.Now(),
		listing:      readListing,
		projectsRoot: filepath.Join(home, ".claude", "projects"),
		recheck:      commandOf,
		kill:         terminate,
	}
	return run(args, env, stdout, stderr)
}

// settings is one invocation's arguments: whether anything is to be ended, and the two ages a kill is
// gated on. The two ages travel as one thing because neither decides a kill on its own.
type settings struct {
	ending     bool
	idleFor    time.Duration
	staleAfter time.Duration
}

// parseArgs reads the arguments, or names its refusal and returns the exit code that goes with it.
func parseArgs(args []string, stderr io.Writer) (settings, int) {
	parsed := settings{idleFor: defaultIdleFor, staleAfter: defaultStaleAfter}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--kill":
			parsed.ending = true
		case "--idle-for", "--stale-after":
			flag := args[index]
			index++
			if index >= len(args) {
				return parsed, usage(stderr, flag+" names no duration")
			}
			window, err := time.ParseDuration(args[index])
			if err != nil || window <= 0 {
				return parsed, usage(stderr, fmt.Sprintf("%q is no duration", args[index]))
			}
			if flag == "--idle-for" {
				parsed.idleFor = window
			} else {
				parsed.staleAfter = window
			}
		default:
			return parsed, usage(stderr, fmt.Sprintf("%q is no argument of mine", args[index]))
		}
	}
	return parsed, exitOK
}

func run(args []string, env environment, stdout, stderr io.Writer) int {
	asked, code := parseArgs(args, stderr)
	if code != exitOK {
		return code
	}

	text, err := env.listing()
	if err != nil {
		fmt.Fprintf(stderr, "%s: the process listing could not be read, so nothing was judged: %s\n", stubName, err)
		return exitDidNotRun
	}
	rows, err := parseListing(text)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", stubName, err)
		return exitRefused
	}

	counts := map[verdict]int{}
	ended := 0
	for _, row := range rows {
		if !isWaitLoop(row.command) {
			continue
		}
		reached := verdictOf(row, env, asked)
		counts[reached.verdict]++
		fmt.Fprintln(stdout, reached.line(row))
		if !asked.ending || reached.verdict != verdictStray {
			continue
		}
		if err := end(row, env); err != nil {
			fmt.Fprintf(stdout, "  not ended: %s\n", err)
			continue
		}
		ended++
	}

	fmt.Fprintln(stdout, summary(counts, ended, asked.ending))
	return exitOK
}

// judgment is one waiter's verdict and the evidence behind it, so the report can show a reader what
// it read as well as what it decided.
type judgment struct {
	verdict verdict
	session string
	idle    time.Duration
	// True where a session was named but no transcript stands under it, which is a different fact from
	// a session that has one and has gone quiet.
	noTranscript bool
}

// The verdicts whose name is the whole reason they were reached. The rest are read off a session, and
// evidence builds those.
var standingEvidence = map[verdict]string{
	verdictYoung:   "under the foreground cap",
	verdictForeign: "not started by the harness",
	verdictMonitor: "does more than wait",
}

// evidence says what the verdict was read off, which is the column a reader checks the ruling against.
func (j judgment) evidence() string {
	if standing, found := standingEvidence[j.verdict]; found {
		return standing
	}
	switch {
	case j.session == "":
		return "names no session"
	case j.noTranscript:
		return fmt.Sprintf("%s has no transcript", j.session)
	default:
		return fmt.Sprintf("%s silent %s", j.session, round(j.idle))
	}
}

func (j judgment) line(row process) string {
	return fmt.Sprintf("%-13s pid %-7d age %-8s %-52s waits on: %s",
		j.verdict, row.pid, round(row.age), j.evidence(), waitedOn(row.command))
}

func verdictOf(row process, env environment, asked settings) judgment {
	if !isHarnessOwned(row.command) {
		return judgment{verdict: verdictForeign}
	}
	if row.age < foregroundCap {
		return judgment{verdict: verdictYoung}
	}
	if !onlyWaits(row.command) {
		return judgment{verdict: verdictMonitor}
	}
	reached, gone := sessionEvidence(row.command, env, asked.idleFor)
	// The waiter's own age gates every kill, whichever way the session evidence ran. A transcript
	// records turns and tool events, so a session sitting at its prompt writes no line at all: 5% of
	// them go quiet for over half an hour and then resume, and silence alone would end a live wait.
	if gone && row.age >= asked.staleAfter {
		reached.verdict = verdictStray
	}
	return reached
}

// sessionEvidence attributes a waiter to the session that started it and reads how long that session
// has been silent. `gone` is the session half of the kill test on its own — the age rule in verdictOf
// is what turns it into a stray.
func sessionEvidence(command string, env environment, idleFor time.Duration) (reached judgment, gone bool) {
	project, session, found := sessionOf(command)
	if !found {
		// A session parks a turn on a sentinel that never arrives, and that command carries no path to
		// attribute it by. Age is the only evidence there is.
		return judgment{verdict: verdictUnowned}, true
	}
	written, err := os.Stat(filepath.Join(env.projectsRoot, project, session+".jsonl"))
	if err != nil {
		// A session with no transcript is not a session known to have gone quiet: it may never have
		// existed under this root at all. That is the same evidence a waiter naming no session gives, so
		// it takes the same rule.
		return judgment{verdict: verdictUnowned, session: session, noTranscript: true}, true
	}
	idle := env.now.Sub(written.ModTime())
	return judgment{verdict: verdictHeld, session: session, idle: idle}, idle >= idleFor
}

// end re-reads the command behind the pid before signalling it. Between the listing and here the
// process may have exited and the pid been handed to something else, and that something else is a
// stranger's work.
func end(row process, env environment) error {
	current, err := env.recheck(row.pid)
	if err != nil {
		return fmt.Errorf("could not re-read pid %d: %s", row.pid, err)
	}
	if strings.TrimSpace(current) != strings.TrimSpace(row.command) {
		return fmt.Errorf("pid %d is now running something else", row.pid)
	}
	if err := env.kill(row.pid); err != nil {
		return fmt.Errorf("could not end pid %d: %s", row.pid, err)
	}
	return nil
}

func summary(counts map[verdict]int, ended int, ending bool) string {
	total := 0
	for _, count := range counts {
		total += count
	}
	if total == 0 {
		return "no wait loops found."
	}
	parts := []string{}
	for _, reached := range []verdict{verdictStray, verdictHeld, verdictYoung, verdictUnowned, verdictMonitor, verdictForeign} {
		if counts[reached] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[reached], reached))
		}
	}
	line := fmt.Sprintf("%d wait loops: %s.", total, strings.Join(parts, ", "))
	if ending {
		return fmt.Sprintf("%s Ended %d.", line, ended)
	}
	if counts[verdictStray] > 0 {
		return fmt.Sprintf("%s Nothing was ended — pass --kill to end the stray ones.", line)
	}
	return line
}

func usage(stderr io.Writer, reason string) int {
	fmt.Fprintf(stderr, "%s: %s\n", stubName, reason)
	fmt.Fprintf(stderr, "usage: %s [--kill] [--idle-for <duration>] [--stale-after <duration>]\n", stubName)
	return exitDidNotRun
}

func round(age time.Duration) string {
	if age >= time.Hour {
		return age.Round(time.Minute).String()
	}
	return age.Round(time.Second).String()
}

func readListing() (string, error) {
	out, err := exec.Command(psCommand, "-Ao", "pid=,etime=,args=").Output()
	return string(out), err
}

// commandOf returns what a pid is running now, or the empty string when it has exited. `ps` exits 1
// for a pid it cannot find, which is an answer; every other failure is reported, or a `ps` that could
// not run would read as a machine where every waiter had just gone.
func commandOf(pid int) (string, error) {
	out, err := exec.Command(psCommand, "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	return commandFromPs(out, err)
}

// commandFromPs reads what `ps` answered about one pid.
func commandFromPs(out []byte, err error) (string, error) {
	if err == nil {
		return string(out), nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	return "", err
}

func terminate(pid int) error {
	if pid <= 0 {
		// `kill` reads a negative pid as a process group and 0 as this process's own group, so a pid
		// that never came from a listing row stops here, before the signal.
		return fmt.Errorf("%d is no pid", pid)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
