// The case for the pair install.sh writes into bin/ while resolve.sh writes the same pair: a tool's
// binary and the stamp beside it. install.sh lands a release binary with the stamp the release
// recorded, resolve.sh lands a build with a stamp of the source in the tree, and one checkout reaches
// both. A skill's stub execs resolve.sh on every invocation. A person installing a release during a
// session is enough.

// What must hold: the binary in bin/ and the stamp beside it come from one writer. The move is atomic
// within the directory. The stamp write beside it is not, so an install landing inside a build leaves
// one writer's binary under the other's stamp. resolve.sh then reads that stamp against the source,
// finds it current, and execs a binary the stamp does not describe at exit 0.

// The interleaving is forced, and this case waits for none to happen. The build's `mv` shim stops it
// between its move and its stamp write, and the install's `sleep` shim says when the install is queued
// behind the build's lock. Whatever order the machine happened to produce never passes as a result.
package reach

import (
	"configs/ai/tools/runtest"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The tool the two writers meet over. install.sh installs the release's tools in the order the
// workflow lists them. The first of them is where an install reaches a build that is still running.
var contested = fixtureTools[0]

// What the fixture release ships for that tool, and the stamp it records for it. A case reads both off
// bin/ afterwards to say which writer each half of the pair came from.
var (
	releaseBody  = "fake " + contested + " binary"
	releaseStamp = "stamp-for-" + contested
)

// An install running while a build of the same tool sits between its move and its stamp write. The
// install has to wait for that build, because the pair it is about to write is the pair the build is
// half way through writing.
func TestAnInstallLandingInsideABuildLeavesNoBinaryUnderTheOtherWritersStamp(t *testing.T) {
	t.Parallel()
	sandbox := runtest.Sandbox(t)
	checkout := newContestedCheckout(t, sandbox)
	tools := filepath.Join(checkout, "ai", "tools")
	binary := filepath.Join(tools, "bin", contested)

	// The build holds the lock over its move and its stamp write. The gap between the two is where an
	// install with no lock of its own writes.
	buildSignals := newSignals(t, sandbox)
	buildPath := newBuildPath(t, sandbox, "contested-build", fmt.Sprintf(recordingToolchain, ""))
	placeShim(t, sandbox, buildPath, "mv", pausesAfterTheMove(t, buildSignals))
	build := start(t, newLaunch(t, filepath.Join(tools, "resolve.sh"), buildPath, contested))
	awaitAny(t, build, filepath.Join(buildSignals, moveDone))

	// The install, whole, inside that gap. Its `sleep` says when it is queued behind the build, since an
	// install that never reaches its own move is what waiting for a lock looks like from here.
	installSignals := newSignals(t, sandbox)
	installing := start(t, newInstall(t, sandbox, checkout, installSignals))
	awaitSettled(t, installing, filepath.Join(installSignals, runQueued))

	mark(t, filepath.Join(buildSignals, moveRelease))
	expectServed(t, build.outcome(t), binary)
	if installed := installing.outcome(t); installed.Code != 0 {
		t.Fatalf("the install refused the fixture release, so no install wrote the pair this case reads\n%v",
			installed)
	}

	body := runtest.ReadFile(t, binary)
	fromRelease := strings.Contains(body, releaseBody)
	if !fromRelease && !strings.Contains(body, anySourceMark) {
		t.Fatalf("%s carries neither writer's bytes, so the comparison below cannot tell the two writers "+
			"apart", binary)
	}
	held := strings.TrimSpace(runtest.ReadFile(t, binary+".stamp"))
	if held != releaseStamp && len(held) != stampLength {
		t.Fatalf("the two writers left %q beside %s rather than either writer's stamp, so the comparison "+
			"below has no stamp to read", held, binary)
	}
	if fromRelease != (held == releaseStamp) {
		t.Errorf("%s holds one writer's binary under the other writer's stamp. resolve.sh reads that stamp "+
			"against the source on every run afterwards, so what a stub execs is not what the stamp names",
			binary)
	}
}

// Waits until the install is through, or until it says it is queued behind the build's lock. Those are
// the two orderings the install can meet, and a case that let the build go before either would read
// whichever order the machine produced.
func awaitSettled(t *testing.T, launched *pending, queued string) {
	t.Helper()
	deadline := time.Now().Add(orderingDeadline)
	for {
		if _, err := os.Stat(queued); err == nil {
			return
		}
		select {
		case <-launched.done:
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the install neither finished nor wrote %s within %v, so the ordering this case drives "+
				"never happened and nothing was measured", queued, orderingDeadline)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// An install.sh started and waited on elsewhere, with the fake gh in front of this process's own PATH
// and a `sleep` that reports a run queued behind a lock. install.sh's other cases reach it through
// `launch`, which waits. This case has to signal the run while it is up.
func newInstall(t *testing.T, sandbox, checkout, signals string) *exec.Cmd {
	t.Helper()
	shims := newPathDir(t, sandbox, "install-shims")
	placeShim(t, sandbox, shims, "sleep", reportsWaiting(t, signals))
	command := newLaunch(t, filepath.Join(checkout, "ai", "tools", "install.sh"),
		strings.Join([]string{shims, newGhPath(t, sandbox), os.Getenv("PATH")}, ":"))
	command.Env = append(command.Env,
		"GH_FAKE_LOG="+filepath.Join(checkout, "gh-argv"),
		"GH_FAKE_TOOLS="+strings.Join(fixtureTools, " "),
		"GH_FAKE_SUFFIX="+thisSuffix,
		"GH_FAKE_DOWNLOAD=serve")
	return command
}

// A checkout both writers reach: the workflow, origin and install.sh the install cases build, plus
// resolve.sh, source-stamp.sh and Go source for one of the tools that release ships. One bin/, so the
// two writers land on one pair.

// go.mod sits at the checkout root, the offset both scripts declare from ai/tools. The fixture git
// directory puts source-stamp.sh on its `git ls-files` branch, where `--others` covers source this
// checkout never staged.
func newContestedCheckout(t *testing.T, sandbox string) string {
	t.Helper()
	checkout := newCheckout(t, sandbox, "https://github.com/pinned/target.git")
	tools := filepath.Join(checkout, "ai", "tools")
	runtest.WriteFile(t, filepath.Join(checkout, "go.mod"), "module fixture\n\ngo 1.24\n", 0o644)
	runtest.WriteFile(t, filepath.Join(tools, contested, "main.go"), "package main\n\nfunc main() {}\n", 0o644)
	copyScripts(t, tools)
	return checkout
}
