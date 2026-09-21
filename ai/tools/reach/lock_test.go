// Cases for the thing resolve.sh does that another copy of resolve.sh can be doing at the same moment:
// building a tool. Every stub execs this script and the gate runs its checks concurrently, so two
// builds of one tool inside a single checkout are reachable today. One run of the root package's suite
// rebuilt `ai/tools/bin/rule-echo` and `ai/tools/bin/model-policy` while other cases were launching
// the stubs that exec the resolver.

// What must hold: the binary in bin/ and the stamp beside it describe the same build. The move is
// atomic within the directory, while the stamp write beside it is not. Two builds over source that
// changed between them can then leave the binary of one beside the stamp of the other.

// The run after reads that stamp against the source, finds it current, and serves the older binary at
// exit 0 in silence. That is the failure the stamp exists to prevent.

// The interleavings here are forced, and no case waits for one to happen. The PATH fixture's own `go`,
// `mv` and `sleep` write and wait on marker files, so each case hands the script an ordering and
// measures what it did with it. A case that cannot get the ordering it needs says so and reports no
// measurement. Whatever order the machine happened to produce never passes as a result.
package reach

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The source the second build compiles. The case writes it into the tree between the two builds. A
// served binary names its own generation in its bytes, because the fixture toolchain copies the source
// it compiled into what it writes.
const laterSource = `package main

func main() { _ = "the later source" }
`

const laterMark = `"the later source"`

// What every fixture binary here carries, whichever source built it. A case reads it before comparing
// bytes: a binary carrying no source at all would make that comparison vacuous.
const anySourceMark = "func main()"

// The files a case and the shims on its PATH signal each other with.
const (
	buildStarted = "build-started"
	buildRelease = "build-release"
	moveDone     = "move-done"
	moveRelease  = "move-release"
	runQueued    = "run-queued"
)

// How long a shim waits for its release, and how long a case waits for an ordering to arrive. Both are
// bounds on a hang. Every wait here ends when the marker it names is written, microseconds after the
// writer writes it, so the ordinary run pays neither bound.

// They are this wide because what waits is a process on a machine already running the rest of this
// package in parallel. A bound that expires under load reports a script that no longer takes the
// ordering, and that report would be a lie.
const (
	shimPoll         = "0.02"
	shimPolls        = 1500
	orderingDeadline = 30 * time.Second
)

// A toolchain that copies the source it compiled into the binary it writes. A case can then read the
// generation of the source that the binary beside a stamp came from. `go build -o <staging> ./<tool>/`
// names the package last, and every fixture keeps one source file inside it.
//
// The hook at the end is where a case orders the build from.
const recordingToolchain = `#!/bin/sh
out=""
package=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "-o" ]; then out="$argument"; fi
  package="$argument"
  previous="$argument"
done
[ -n "$out" ] || exit 1
printf '#!/bin/sh\nexit 0\n' >"$out"
cat "${package}main.go" >>"$out"
chmod 755 "$out"
%s`

// A binary and a stamp that name the same build, or the wording for the pair that does not. Every case
// reads it from here, because it is the single claim this file exists for.
const tornPair = "the stamp beside %s names the source in the tree while the binary beside it was built " +
	"from the source that came before the edit, so every run after this one reads that pair as current and " +
	"serves the older binary at exit 0 in silence"

// Two builds of one tool, the second landing entirely inside the first. The first compiles the source
// the tree held when it started, and is let out only after the second has moved its own binary into
// bin/. That ordering pairs one build's bytes with the other's stamp.
func TestABuildLandingInsideAnotherLeavesNoBinaryBesideTheOtherBuildsStamp(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "interleaved")
	resolver := filepath.Join(tools, "resolve.sh")
	binary := filepath.Join(tools, "bin", tool)
	signals := newSignals(t, sandbox)

	// The first build stops inside its compile, holding whatever a build of this tool holds.
	first := newBuildPath(t, sandbox, "first-build", pausesInsideTheBuild(t, signals))
	// The second stops between its move and its stamp write, the point where the pair can be torn. Its
	// `sleep` says instead when it is queued behind the first build, since a build that never reaches its
	// move is what waiting for a lock looks like from here.
	second := newBuildPath(t, sandbox, "second-build", fmt.Sprintf(recordingToolchain, ""))
	placeShim(t, sandbox, second, "mv", pausesAfterTheMove(t, signals))
	placeShim(t, sandbox, second, "sleep", reportsWaiting(t, signals))

	firstRun := start(t, newLaunch(t, resolver, first, tool))
	awaitAny(t, firstRun, filepath.Join(signals, buildStarted))

	// The edit both builds straddle. From here the tree holds source the first build never read.
	writeFile(t, filepath.Join(tools, tool, "main.go"), laterSource, 0o644)

	secondRun := start(t, newLaunch(t, resolver, second, tool))
	awaitAny(t, secondRun, filepath.Join(signals, moveDone), filepath.Join(signals, runQueued))

	mark(t, filepath.Join(signals, buildRelease))
	expectServed(t, firstRun.outcome(t), binary)
	mark(t, filepath.Join(signals, moveRelease))
	expectServed(t, secondRun.outcome(t), binary)

	body := read(t, binary)
	if !strings.Contains(body, anySourceMark) {
		t.Fatalf("%s carries none of the source it was built from, so nothing below can tell one build's "+
			"bytes from the other's", binary)
	}
	held := strings.TrimSpace(read(t, binary+".stamp"))
	if len(held) != stampLength {
		t.Fatalf("the two builds left %q beside %s rather than a stamp, so nothing below compares a binary "+
			"with the source its stamp names", held, binary)
	}
	if held == stampOf(t, moduleIn(tools), tool) && !strings.Contains(body, laterMark) {
		t.Errorf(tornPair, binary)
	}
}

// A build killed while it holds the lock must not wedge the tool for every session after it. Two kinds
// of kill, because the lock is given up two different ways. A signal the shell can catch runs the trap
// that removes it. A signal it cannot catch leaves the lock behind for a later run to break on its
// age.

// The hour is written onto the lock, and no case waits it out. What a waiter needs is a lock old enough
// that no build could still be inside it. A case that measured that by waiting would be the slowest in
// this package by two orders of magnitude.
func TestABuildKilledWhileItHoldsTheLockLeavesTheToolReachable(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "killed")
	resolver := filepath.Join(tools, "resolve.sh")
	binary := filepath.Join(tools, "bin", tool)
	lock := binary + ".lock"

	caught := newSignals(t, sandbox)
	stopped := start(t, inOwnGroup(newLaunch(t, resolver,
		newBuildPath(t, sandbox, "caught-build", pausesInsideTheBuild(t, caught)), tool)))
	awaitAny(t, stopped, filepath.Join(caught, buildStarted))
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("no build of %s held %s while its compile ran (%v), so neither half of this case says "+
			"anything about what a killed build leaves behind", tool, lock, err)
	}
	signalGroup(t, stopped, syscall.SIGTERM)
	stopped.outcome(t)
	if _, err := os.Stat(lock); err == nil {
		t.Errorf("a build stopped by SIGTERM left %s behind, so every build of %s on this machine now waits "+
			"the lock out before it can start", lock, tool)
	}

	uncaught := newSignals(t, sandbox)
	wedged := start(t, inOwnGroup(newLaunch(t, resolver,
		newBuildPath(t, sandbox, "uncaught-build", pausesInsideTheBuild(t, uncaught)), tool)))
	awaitAny(t, wedged, filepath.Join(uncaught, buildStarted))
	signalGroup(t, wedged, syscall.SIGKILL)
	wedged.outcome(t)
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("SIGKILL left no lock behind (%v), so what is broken below is a lock nothing here holds",
			err)
	}
	abandoned := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lock, abandoned, abandoned); err != nil {
		t.Fatalf("ageing %s: %v — the abandoned lock this case is about was never set up", lock, err)
	}

	served := launch(t, newLaunch(t, resolver,
		newBuildPath(t, sandbox, "after-kill", fmt.Sprintf(recordingToolchain, "")), tool))
	expectServed(t, served, binary)
}

// A run that finds the lock held waits for it and then serves what the build it waited for wrote. It
// compiles no source of its own. The toolchain on this PATH refuses, so a run that built anything at
// all refuses with it.
func TestARunThatWaitedForALockServesThatBuildAndMakesNoneOfItsOwn(t *testing.T) {
	t.Parallel()
	sandbox := newSandbox(t)
	tools := newToolsDir(t, sandbox, "queued")
	resolver := filepath.Join(tools, "resolve.sh")
	binary := filepath.Join(tools, "bin", tool)
	lock := binary + ".lock"
	signals := newSignals(t, sandbox)
	if err := os.MkdirAll(lock, 0o755); err != nil {
		t.Fatalf("taking %s the way a build in flight holds it: %v — nothing was measured", lock, err)
	}

	path := newBuildPath(t, sandbox, "refusing-build", failingToolchain)
	placeShim(t, sandbox, path, "sleep", reportsWaiting(t, signals))
	queued := start(t, newLaunch(t, resolver, path, tool))
	awaitAny(t, queued, filepath.Join(signals, runQueued))
	if _, err := os.Stat(binary); err == nil {
		t.Fatalf("a run that found %s held built %s beside the build holding it, which is the pair this "+
			"file is about", lock, binary)
	}

	// The build that held the lock, finishing: its bytes, the stamp naming the source it built, and the
	// lock given up.
	placeBinary(t, tools, tool, foreignBinary, 0o755)
	writeFile(t, binary+".stamp", stampOf(t, moduleIn(tools), tool)+"\n", 0o644)
	if err := os.Remove(lock); err != nil {
		t.Fatalf("giving %s up on behalf of the build that held it: %v", lock, err)
	}

	served := queued.outcome(t)
	expectServed(t, served, binary)
	if served.said("does not compile") {
		t.Errorf("the run compiled source the build it waited for had already compiled, so every tool the "+
			"gate launches at once builds itself once per launch\n%v", served)
	}
}

// The first build's hook: it says it is inside its compile and stays there until the case lets it out.
// That is what puts the second build's whole run inside this one's.
func pausesInsideTheBuild(t *testing.T, signals string) string {
	t.Helper()
	return fmt.Sprintf(recordingToolchain,
		writesMarker(filepath.Join(signals, buildStarted))+awaitsMarker(t, filepath.Join(signals, buildRelease)))
}

// An `mv` that moves and then stops, so a case can put another build's whole move and stamp write between
// this build's two. It calls the real mv by absolute path, because the only `mv` on this PATH is this shim.
func pausesAfterTheMove(t *testing.T, signals string) string {
	t.Helper()
	return fmt.Sprintf(`#!/bin/sh
%s "$@" || exit $?
%s%s`, realCommand(t, "mv"), writesMarker(filepath.Join(signals, moveDone)),
		awaitsMarker(t, filepath.Join(signals, moveRelease)))
}

// A `sleep` that says a run is waiting before it sleeps. resolve.sh sleeps only when it is queued
// behind another build of the same tool, so this marker is what tells "waited its turn" from "built
// anyway".
func reportsWaiting(t *testing.T, signals string) string {
	t.Helper()
	return fmt.Sprintf("#!/bin/sh\n%sexec %s \"$@\"\n",
		writesMarker(filepath.Join(signals, runQueued)), realCommand(t, "sleep"))
}

// A marker written with the shell's own redirection, because the PATH a shim runs on is narrow. It
// holds only the commands the script under test calls, and no command a shim might want.
func writesMarker(marker string) string {
	return fmt.Sprintf(": >\"%s\"\n", marker)
}

// A bounded wait for a marker, for use inside a shim. A case has its own waiter. The bound is there
// because a shim left waiting for a marker no process will write would outlive the case that launched
// it. Its non-zero exit reaches the case as a build that failed.
func awaitsMarker(t *testing.T, marker string) string {
	t.Helper()
	return fmt.Sprintf(`waited=0
while [ ! -f "%s" ]; do
  [ "$waited" -lt %d ] || exit 1
  waited=$((waited + 1))
  %s %s
done
`, marker, shimPolls, realCommand(t, "sleep"), shimPoll)
}

// Waits for the first of these markers, or for the launch to end without writing any of them. A case
// that failed to get its ordering has measured no interleaving. Naming the missing marker is the
// difference between a case somebody can fix and one that only hangs.
func awaitAny(t *testing.T, launched *pending, markers ...string) {
	t.Helper()
	deadline := time.Now().Add(orderingDeadline)
	for {
		for _, marker := range markers {
			if _, err := os.Stat(marker); err == nil {
				return
			}
		}
		select {
		case <-launched.done:
			t.Fatalf("the launch ended before it wrote any of %v, so the ordering this case measures never "+
				"happened and nothing was measured\n%v", markers, launched.got)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("none of %v appeared within %v, so the script no longer takes the ordering this case "+
				"drives and nothing was measured", markers, orderingDeadline)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// A marker the case writes itself.
func mark(t *testing.T, marker string) {
	t.Helper()
	writeFile(t, marker, "", 0o644)
}

// A directory a case and the shims on its PATH signal each other through. One per ordering, because a
// marker outlives the run that wrote it and a second run reading the first run's markers would be released
// before it ever started.
func newSignals(t *testing.T, sandbox string) string {
	t.Helper()
	dir, err := os.MkdirTemp(sandbox, "signals-")
	if err != nil {
		t.Fatalf("building the signal directory under %s: %v — nothing was measured", sandbox, err)
	}
	return sandboxed(t, sandbox, dir)
}

// A shim standing where newPathDir, the PATH fixture helper, linked a real command. The link goes first:
// writing through it would follow it to the real binary outside the sandbox. That is the accident
// newSandbox, the sandbox helper, records in its own header.
func placeShim(t *testing.T, sandbox, path, name, body string) {
	t.Helper()
	shim := sandboxed(t, sandbox, filepath.Join(path, name))
	if err := os.Remove(shim); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clearing the %s linked into %s: %v — nothing was measured", name, path, err)
	}
	writeFile(t, shim, body, 0o755)
}

func realCommand(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("this machine has no %s, and the shims here call it — nothing was measured", name)
	}
	return path
}

// A launch under way. Every case here runs two scripts at once, and a `go test` fixture cannot report
// from the goroutine that waits on one. t.Fatalf outside the test's own goroutine does not stop the
// test.
type pending struct {
	done chan struct{}
	// The fixture sets these before done closes and reads them after it, so the channel carries them across.
	command *exec.Cmd
	got     outcome
	err     error
}

// A launch started here and waited on elsewhere. It starts on the caller's goroutine, so the process is
// there to be signalled the moment this returns.
func start(t *testing.T, command *exec.Cmd) *pending {
	t.Helper()
	var out, err strings.Builder
	command.Stdout, command.Stderr = &out, &err
	launched := &pending{done: make(chan struct{}), command: command}
	if startErr := command.Start(); startErr != nil {
		t.Fatalf("could not start %s: %v — nothing was measured", command.Path, startErr)
	}
	go func() {
		defer close(launched.done)
		var exit *exec.ExitError
		switch waitErr := command.Wait(); {
		case waitErr == nil:
		case errors.As(waitErr, &exit):
			launched.got.code = exit.ExitCode()
		default:
			launched.err = waitErr
		}
		launched.got.stdout, launched.got.stderr = out.String(), err.String()
	}()
	return launched
}

// What the launch came back with, once it is over.
func (p *pending) outcome(t *testing.T) outcome {
	t.Helper()
	<-p.done
	if p.err != nil {
		t.Fatalf("could not run %s: %v — nothing was measured", p.command.Path, p.err)
	}
	return p.got
}

// A launch in a process group of its own, so that a case signalling it reaches the shim it is waiting
// on as well. bash holds a caught signal until the command in front of it returns. The command in
// front of a build here is a shim that would otherwise wait out its own bound first.
func inOwnGroup(command *exec.Cmd) *exec.Cmd {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command
}

func signalGroup(t *testing.T, launched *pending, signal syscall.Signal) {
	t.Helper()
	if err := syscall.Kill(-launched.command.Process.Pid, signal); err != nil {
		t.Fatalf("sending %v to the build: %v — the kill this case is about never happened", signal, err)
	}
}
