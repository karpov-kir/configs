// Cases for the reaper. The case that must not be weakened is "a waiter whose session is still
// writing is held": killing it ends a tool call a live session is inside, and the process is gone
// before anything can say it was ours. Every kill case therefore carries that control beside it.
//
// The listing and the clock are injected, so no case spawns a process. Every suite built out of
// process spawns runs two orders of magnitude slower on the maintainer's laptop than in CI.
package waitreap

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const projectDir = "-Users-someone-Documents-WP-configs"

const sessionID = "8913ca1e-d52a-42d8-a090-b140a9fbad67"

func scratchPath(session string) string {
	return "/private/tmp/claude-501/" + projectDir + "/" + session + "/tasks/busmee2t4.output"
}

// What the harness's own wrapper looks like: it sources a shell snapshot before the command it was
// given, and that is what marks the process as one it started.
const harnessPrefix = "/bin/zsh -c source /Users/someone/.claude/shell-snapshots/snapshot-zsh-17894.sh &&"

func waiterRow(pid int, etime string, session string) string {
	return fmt.Sprintf("%5d %11s %s eval 'until [ -s %s ]; do sleep 30; done'", pid, etime, harnessPrefix, scratchPath(session))
}

// A park on a sentinel that never arrives: the shape carrying no session anywhere in its command.
func parkRow(pid int, etime string) string {
	return fmt.Sprintf("%5d %11s %s eval 'until [ -f /tmp/143-hold ]; do sleep 120; done'", pid, etime, harnessPrefix)
}

func TestParseListingRefusesARowItCannotRead(t *testing.T) {
	good := " 1234       02:08:53 /bin/zsh -c eval 'until [ -f /tmp/x ]; do sleep 30; done'"
	if _, err := parseListing(good); err != nil {
		t.Fatalf("a well-formed listing was refused (%v), so the refusal below proves nothing", err)
	}

	truncated := good + "\n 5678       01:0"
	_, err := parseListing(truncated)
	if err == nil {
		t.Fatal("a row with no command was accepted — a truncated listing would silently shrink the table")
	}
	if !strings.Contains(err.Error(), "01:0") {
		t.Errorf("the refusal does not quote the row it could not read: %v", err)
	}
}

// Wraps a command the way the harness's shell does, which is what `onlyWaits` reads through.
func wrapped(command string) string {
	return "/bin/zsh -c source /Users/someone/.claude/shell-snapshots/snapshot-zsh-17894.sh 2>/dev/null || true && eval '" +
		command + "' < /dev/null && pwd -P >| /tmp/claude-fd67-cwd"
}

func TestEveryPollingLoopIsReported(t *testing.T) {
	// One row per way a body can wait. A condition polls a file, a process, a log or another command's
	// output. isWaitLoop, the loop detector, reads none of that text, so a row per condition is the
	// same row under another name.
	polling := map[string]string{
		"sleeps between passes": "until [ -f /tmp/143-hold ]; do sleep 120; done",
		"spins with no sleep":   "until [ ! -e /proc/self ] && false; do :; done 2>/dev/null",
		// A monitor does work on every pass and still strands forever. It is reported for that reason,
		// and never ended for the same one.
		"watches and reports":  `until [ -f "$x" ]; do cur=$(ls); echo "$cur"; sleep 20; done`,
		"written over lines":   `/bin/zsh -c eval 'until [ -f /tmp/x ]\012do\012sleep 5\012done'`,
		"wrapped by the shell": waiterRow(1, "02:08:53", sessionID),
	}
	for name, command := range polling {
		if !isWaitLoop(command) {
			t.Errorf("%s: not reported, so nobody ever sees it: %s", name, command)
		}
	}

	others := map[string]string{
		"a build":              "go test -timeout 900s ./...",
		"a loop with no sleep": "until [ -f /tmp/done ]; do make build; done",
		"the word in a string": "echo 'wait until the gate finishes'",
		// Shaped like a loop, but it consumes its input and ends when that input does.
		"reads a pipeline": "while read -r f; do :; done",
	}
	for name, command := range others {
		if isWaitLoop(command) {
			t.Errorf("%s: reported as a polling loop: %s", name, command)
		}
	}
}

func TestOnlyACommandThatIsNothingButAWaitMayBeEnded(t *testing.T) {
	endable := map[string]string{
		"a park":             wrapped("until [ -f /tmp/143-hold ]; do sleep 120; done"),
		"a counted retry":    wrapped("n=0; until [ -f /tmp/x ] || [ $n -ge 55 ]; do sleep 5; n=$((n+1)); done"),
		"with a redirection": wrapped("until ! pgrep -f 'ai/gate.sh' >/dev/null 2>&1; do sleep 3; done 2>/dev/null"),
	}
	for name, command := range endable {
		if !onlyWaits(command) {
			t.Errorf("%s: not endable, so the pile it belongs to never clears: %s", name, command)
		}
	}

	keep := map[string]string{
		// The work is after the loop, not inside it. Ending the shell loses the run the wait was for.
		"waits, then works":  wrapped("until curl -sf localhost:3000; do sleep 2; done; npm run e2e"),
		"works each pass":    wrapped(`until [ -f "$x" ]; do cur=$(ls); echo "$cur"; sleep 20; done`),
		"two loops and work": wrapped(`until [ ! -e /proc/self ] && false; do :; done 2>/dev/null; until ! pgrep -f "report.sh root"; do sleep 3; done; echo done; cat /tmp/out`),
		// A command the harness did not wrap is never ended, whatever its shape.
		"not wrapped": "until [ -f /tmp/143-hold ]; do sleep 120; done",
	}
	for name, command := range keep {
		if onlyWaits(command) {
			t.Errorf("%s: read as endable, and --kill would take the work with it: %s", name, command)
		}
	}
}

// One session directory holding one transcript, last written `idleFor` ago.
func projectsRoot(t *testing.T, idleFor time.Duration, now time.Time) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, projectDir), 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(root, projectDir, sessionID+".jsonl")
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	written := now.Add(-idleFor)
	if err := os.Chtimes(transcript, written, written); err != nil {
		t.Fatal(err)
	}
	return root
}

type recorder struct {
	killed   []int
	commands map[int]string
}

func (r *recorder) kill(pid int) error {
	r.killed = append(r.killed, pid)
	return nil
}

func (r *recorder) recheck(pid int) (string, error) {
	return r.commands[pid], nil
}

func drive(t *testing.T, args []string, listing string, idleFor time.Duration) (code int, out string, killed []int) {
	t.Helper()
	now := time.Now()
	rows := map[int]string{}
	parsed, err := parseListing(listing)
	if err != nil {
		t.Fatalf("the fixture listing does not parse: %v", err)
	}
	for _, row := range parsed {
		rows[row.pid] = row.command
	}
	record := &recorder{commands: rows}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := environment{
		now:          now,
		listing:      func() (string, error) { return listing, nil },
		projectsRoot: projectsRoot(t, idleFor, now),
		recheck:      record.recheck,
		kill:         record.kill,
	}
	code = run(args, env, stdout, stderr)
	return code, stdout.String() + stderr.String(), record.killed
}

func TestAWaiterOutlivingASilentSessionIsStrayAndIsKilled(t *testing.T) {
	listing := waiterRow(24672, "02:08:53", sessionID)

	code, out, killed := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if code != exitOK {
		t.Fatalf("exit %d, want %d: %s", code, exitOK, out)
	}
	if len(killed) != 1 || killed[0] != 24672 {
		t.Fatalf("killed %v, want just 24672: %s", killed, out)
	}
	if !strings.Contains(out, "stray") {
		t.Errorf("the report does not name the verdict it acted on: %s", out)
	}
}

func TestAWaiterWhoseSessionIsStillWritingIsHeld(t *testing.T) {
	listing := waiterRow(24672, "02:08:53", sessionID)

	code, out, killed := drive(t, []string{"--kill"}, listing, time.Minute)

	if code != exitOK {
		t.Fatalf("exit %d, want %d: %s", code, exitOK, out)
	}
	if len(killed) != 0 {
		t.Fatalf("killed %v — that ends a tool call a live session is inside", killed)
	}
	if !strings.Contains(out, "held") {
		t.Errorf("the report does not say why it left the waiter alone: %s", out)
	}
}

func TestAWaiterYoungerThanTheForegroundCapIsLeftAlone(t *testing.T) {
	listing := waiterRow(24672, "09:40", sessionID)

	_, out, killed := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if len(killed) != 0 {
		t.Fatalf("killed %v — under the cap it may be the session's own foreground wait: %s", killed, out)
	}
}

func TestAParkedWaiterIsEndedOnceItIsPastTheStaleWindow(t *testing.T) {
	listing := parkRow(24672, "02:19:00")

	_, out, killed := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if len(killed) != 1 || killed[0] != 24672 {
		t.Fatalf("killed %v, want just 24672 — a park names no session, so nothing else can ever collect it: %s", killed, out)
	}
}

func TestALoopInTheHumansOwnShellIsNeverEnded(t *testing.T) {
	listing := "24672    09:19:00 -zsh -c until [ -f /tmp/mine ]; do sleep 60; done"

	_, out, killed := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if len(killed) != 0 {
		t.Fatalf("killed %v — that is the human's own shell, not a session's leftovers: %s", killed, out)
	}
	if !strings.Contains(out, "foreign") {
		t.Errorf("the report does not mark the loop as none of the harness's: %s", out)
	}
}

func TestWithoutKillNothingIsEnded(t *testing.T) {
	listing := waiterRow(24672, "02:08:53", sessionID)

	_, out, killed := drive(t, nil, listing, 3*time.Hour)

	if len(killed) != 0 {
		t.Fatalf("killed %v without --kill", killed)
	}
	if !strings.Contains(out, "stray") {
		t.Errorf("the dry run does not say what --kill would end: %s", out)
	}
}

func TestAPidWhoseCommandChangedSinceTheListingIsNotKilled(t *testing.T) {
	listing := waiterRow(24672, "02:08:53", sessionID)
	now := time.Now()
	record := &recorder{commands: map[int]string{24672: "vim /etc/hosts"}}
	stdout := &bytes.Buffer{}
	env := environment{
		now:          now,
		listing:      func() (string, error) { return listing, nil },
		projectsRoot: projectsRoot(t, 3*time.Hour, now),
		recheck:      record.recheck,
		kill:         record.kill,
	}

	run([]string{"--kill"}, env, stdout, stdout)

	if len(record.killed) != 0 {
		t.Fatalf("killed %v after the pid was reused — that kills a stranger's process", record.killed)
	}
}

func TestAnUnknownFlagRefusesWithTheUsageLine(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := run([]string{"--nope"}, environment{now: time.Now()}, stdout, stderr)

	if code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if !strings.Contains(stderr.String(), "usage: wait-reap.sh") {
		t.Errorf("the refusal carries no usage line: %s", stderr.String())
	}
}

// The only cases here that spawn a process, and the only ones that reach the real machine. Every
// other case ends a pid a recorder wrote down, so a `terminate` that signalled no process and a
// `commandOf` that always answered "gone" would pass the whole suite while the tool ended no waiter.
func TestTerminateEndsARealProcess(t *testing.T) {
	spawned := exec.Command("sleep", "30")
	if err := spawned.Start(); err != nil {
		t.Fatalf("could not start the process this case is about: %v", err)
	}
	defer func() { _ = spawned.Process.Kill() }()

	if err := spawned.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("the process was not alive before it was signalled (%v), so its death proves nothing", err)
	}
	if err := terminate(spawned.Process.Pid); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	ended := make(chan struct{})
	go func() {
		_ = spawned.Wait()
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the process outlived the signal, so the reaper cannot end a waiter at all")
	}
}

func TestCommandOfSeparatesAGoneProcessFromAFailedRead(t *testing.T) {
	mine, err := commandOf(os.Getpid())
	if err != nil {
		t.Fatalf("reading this test's own command: %v", err)
	}
	if strings.TrimSpace(mine) == "" {
		t.Fatal("this test's own process read as gone, so no kill would ever proceed")
	}

	// A pid no process can hold, so `ps` exits 1 and the answer is the empty string.
	gone, err := commandOf(1 << 30)
	if err != nil {
		t.Fatalf("a pid that does not exist was reported as a failure to read: %v", err)
	}
	if gone != "" {
		t.Errorf("a pid that does not exist came back running %q", gone)
	}
}

// The other end of the same mapping: exit 1 is how `ps` says the pid is gone, and
// TestCommandOfSeparatesAGoneProcessFromAFailedRead reads that end through a real `ps`. Every other
// status is a read that failed, and reading one as a gone process is reading a live waiter as one
// that ended on its own.
func TestAFailedReadIsNotReadAsAGoneProcess(t *testing.T) {
	_, refused := exec.Command("sh", "-c", "exit 3").Output()
	if refused == nil {
		t.Fatal("the fixture command succeeded, so nothing below proves anything")
	}
	if _, err := commandFromPs(nil, refused); err == nil {
		t.Error("a `ps` that failed for any reason but a missing pid read as a gone process, and a gone " +
			"process is a waiter that ended on its own")
	}
}

func TestTheTwoWindowsAreTheOnesTheFlagsName(t *testing.T) {
	silent := waiterRow(24672, "02:08:53", sessionID)
	parked := parkRow(30001, "02:08:53")

	// The session has been silent 90 minutes and both waiters are two hours old, so each verdict here
	// turns on the flag named and on no other input.
	_, out, killed := drive(t, []string{"--kill", "--idle-for", "2h"}, silent, 90*time.Minute)
	if len(killed) != 0 || !strings.Contains(out, "held") {
		t.Errorf("--idle-for 2h did not hold a session silent for 90m: killed %v, %s", killed, out)
	}

	_, out, killed = drive(t, []string{"--kill", "--idle-for", "1h"}, silent, 90*time.Minute)
	if len(killed) != 1 {
		t.Errorf("--idle-for 1h did not reach a session silent for 90m: killed %v, %s", killed, out)
	}

	_, out, killed = drive(t, []string{"--kill", "--stale-after", "24h"}, parked, time.Minute)
	if len(killed) != 0 || !strings.Contains(out, "unowned") {
		t.Errorf("--stale-after 24h did not spare a park two hours old: killed %v, %s", killed, out)
	}

	_, out, killed = drive(t, []string{"--kill", "--stale-after", "1h"}, parked, time.Minute)
	if len(killed) != 1 {
		t.Errorf("--stale-after 1h did not reach a park two hours old: killed %v, %s", killed, out)
	}
}

func TestASessionWithNoTranscriptIsCollectedByAgeAndNotByTheCap(t *testing.T) {
	// The session is named and its project directory is there, but no transcript is: no evidence of an
	// owner at all, which is what the age rule is for.
	listing := waiterRow(24672, "00:20:00", "0000ffff-0000-4000-8000-00000000beef")

	_, out, killed := drive(t, []string{"--kill"}, listing, time.Hour)

	if len(killed) != 0 {
		t.Fatalf("killed %v at twenty minutes, where the prose promises the stale window: %s", killed, out)
	}
	if !strings.Contains(out, "has no transcript") {
		t.Errorf("the report does not say the evidence was missing: %s", out)
	}
}

func TestSilenceAloneDoesNotEndAWaiter(t *testing.T) {
	// The session has been silent three hours, but the waiter itself is twenty minutes old: a session
	// sitting at its prompt writes no line, and this is the wait it may be sitting in.
	listing := waiterRow(24672, "00:20:00", sessionID)

	_, out, killed := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if len(killed) != 0 {
		t.Fatalf("killed %v on the session's silence alone: %s", killed, out)
	}
	if !strings.Contains(out, "held") {
		t.Errorf("the report does not say the waiter was held: %s", out)
	}
}

func TestAMonitorIsReportedAndNeverEnded(t *testing.T) {
	listing := fmt.Sprintf("%5d %11s %s eval 'until [ -f %s ]; do cur=$(ls); echo \"$cur\"; sleep 20; done'",
		24672, "03:00:00", harnessPrefix, scratchPath(sessionID))

	_, out, killed := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if len(killed) != 0 {
		t.Fatalf("killed %v, and whatever that loop was reporting is gone with it: %s", killed, out)
	}
	if !strings.Contains(out, "monitor") || !strings.Contains(out, "does more than wait") {
		t.Errorf("a monitor was not reported as one: %s", out)
	}
}

func TestACommandNamingTwoSessionsIsAttributedToNeither(t *testing.T) {
	// The first path in a command need not be the waiter's own, and judging it by the wrong session's
	// transcript is how the wrong waiter is ended.
	command := fmt.Sprintf("%s eval 'until [ -s %s ] && [ -s %s ]; do sleep 30; done'",
		harnessPrefix, scratchPath(sessionID), scratchPath("11111111-2222-4333-8444-555555555555"))
	listing := fmt.Sprintf("%5d %11s %s", 24672, "03:00:00", command)

	_, out, _ := drive(t, []string{"--kill"}, listing, 3*time.Hour)

	if !strings.Contains(out, "names no session") {
		t.Errorf("a command naming two sessions was judged by one of them: %s", out)
	}
}

// The condition is the whole of what a reader gets about a hundred-character command line, so it has
// to be the condition of the loop that was judged. A scan for `until ` and then `while ` finds
// neither: it matches the tail of a longer word, and it prefers an `until` later in the line to a
// `while` earlier in it.
func TestTheReportNamesTheLoopTheRulingWasMadeAbout(t *testing.T) {
	cases := map[string]struct{ command, want string }{
		"the keyword is a whole word": {
			command: "echo runtil ok; until [ -f /tmp/x ]; do sleep 5; done",
			want:    "[ -f /tmp/x ]",
		},
		"the first loop wins, whichever keyword opens it": {
			command: "while [ -f /tmp/a ]; do sleep 1; done; until [ -f /tmp/b ]; do sleep 2; done",
			want:    "[ -f /tmp/a ]",
		},
		"a lone loop": {
			command: "until [ -f /tmp/x ]; do sleep 5; done",
			want:    "[ -f /tmp/x ]",
		},
		"no loop at all": {
			command: "go test -timeout 900s ./...",
			want:    "",
		},
	}
	for name, want := range cases {
		if got := waitedOn(want.command); got != want.want {
			t.Errorf("%s: waits on %q, want %q", name, got, want.want)
		}
	}
}

func TestARowWhoseFirstColumnIsNoPidRefusesTheListing(t *testing.T) {
	// A negative number reaching `kill` signals a whole process group.
	for _, first := range []string{"-1234", "0", "notapid"} {
		if _, err := parseListing(first + "   02:08:53 /bin/zsh -c eval 'until [ -f /tmp/x ]; do sleep 5; done'"); err == nil {
			t.Errorf("%q was accepted as a pid", first)
		}
	}
}

func TestTerminateRefusesAPidThatIsNoPid(t *testing.T) {
	for _, pid := range []int{0, -1, -4321} {
		if err := terminate(pid); err == nil {
			t.Errorf("terminate(%d) signalled rather than refusing", pid)
		}
	}
}
