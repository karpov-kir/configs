package aibootstrap_test

import (
	"testing"

	"kk-flavor/tools/machine"
)

// The flags every step case but its own passes, so what an exit status reports is one step alone.
func stepsExcept(step string) []string {
	var flags []string
	for _, flag := range skipSteps {
		if flag != step {
			flags = append(flags, flag)
		}
	}
	return flags
}

func (f *fixture) runStep(step string, args ...string) int {
	f.t.Helper()
	return f.run(append(append([]string{}, stepsExcept(step)...), args...)...)
}

// --- the tools step -----------------------------------------------------------------------------

func TestTheToolsStepRunsTheInstallerAndReportsItsSuccess(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-tools", "--agent=claude"), 0)

	if !f.machine.RanAny(f.repo + "/tools/install.sh") {
		t.Errorf("the tools installer was never invoked: %v", f.machine.Calls)
	}
}

// 3 is the repository having cut no release. Nothing is broken and nobody at this machine can cut one,
// so it must not become a refusal — that would fail every fresh clone until the first release exists.
func TestNoReleaseToInstallFromIsNotAFailureWhenThisMachineHasGo(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering(f.repo+"/tools/install.sh", func(machine.Command) int { return 3 })

	f.expectCode(f.runStep("--skip-tools", "--agent=claude"), 0)

	f.expectSaid("build from source on first use")
	f.expectSaid("needs Go")
	f.expectNotSaid("failed")
}

// Go is checked here rather than left to verify, because --skip-verify turns that off and without this
// a run would end green having installed no tools onto a machine that cannot build them either.
func TestNoReleaseAndNoGoIsARefusal(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering(f.repo+"/tools/install.sh", func(machine.Command) int { return 3 })
	f.machine.without("go")

	f.expectCode(f.runStep("--skip-tools", "--agent=claude"), 1)

	f.expectSaid("neither downloaded nor built")
}

// Every other outcome the installer has still fails the run. Without this the arm above could be a
// blanket "the installer's exit code is ignored" and read exactly the same.
func TestAnInstallerThatRefusedFailsTheRun(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering(f.repo+"/tools/install.sh", func(machine.Command) int { return 2 })

	f.expectCode(f.runStep("--skip-tools", "--agent=claude"), 1)

	f.expectSaid("install.sh failed")
	f.expectNotSaid("build from source on first use")
}

func TestAMachineWithoutGhCannotFetchTheToolBinaries(t *testing.T) {
	f := newFixture(t)
	f.machine.without("gh")

	f.expectCode(f.runStep("--skip-tools", "--agent=claude"), 1)

	f.expectSaid("gh is not installed")
	if f.machine.RanAny(f.repo + "/tools/install.sh") {
		t.Errorf("the installer ran on a machine with no gh: %v", f.machine.Calls)
	}
}

// --- the MCP registry ---------------------------------------------------------------------------

func TestTheMcpStepSyncsThroughTheClientItWasAskedAbout(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-mcp", "--agent=codex"), 0)

	if !f.machine.Ran(f.repo+"/mcp-sync.sh", "--agent=codex") {
		t.Errorf("the MCP sync did not run for codex: %v", f.machine.Calls)
	}
}

func TestAMachineWithoutTheClientCliRegistersNothing(t *testing.T) {
	f := newFixture(t)
	f.machine.without("claude")

	f.expectCode(f.runStep("--skip-mcp", "--agent=claude"), 1)

	f.expectSaid("the claude CLI is not on PATH")
	if f.machine.RanAny(f.repo + "/mcp-sync.sh") {
		t.Errorf("the sync ran with no client CLI: %v", f.machine.Calls)
	}
}

// --- verify ---------------------------------------------------------------------------------------

// A setup that reports success without checking anything has reported nothing, so the last step is the
// repository's own gate over what was just linked.
func TestVerifyRunsTheGateFromTheCheckoutRoot(t *testing.T) {
	f := newFixture(t)
	var ranIn string
	f.machine.Answering(f.repo+"/gate.sh", func(command machine.Command) int {
		ranIn = command.Directory
		return 0
	})

	f.expectCode(f.runStep("--skip-verify", "--agent=claude"), 0)

	// The checkout root, because the gate scopes every check against the repository it is standing in
	// and ai/ is not one.
	if want := f.base + "/checkout"; ranIn != want {
		t.Errorf("the gate ran in %q, wanted %q — anywhere else it scopes the wrong tree", ranIn, want)
	}
}

// The re-entry marker. The gate discovers every `*-test.sh` and runs it, so a suite that drove this
// installer would reach verify again, and verify would run the gate again. Nothing does that today;
// the marker is what stops the one written tomorrow from hanging a machine setup instead of failing it.
func TestVerifyTellsTheRunItStartsNotToVerifyAgain(t *testing.T) {
	f := newFixture(t)
	var handed []string
	f.machine.Answering(f.repo+"/gate.sh", func(command machine.Command) int {
		handed = command.Env
		return 0
	})

	f.expectCode(f.runStep("--skip-verify", "--agent=claude"), 0)

	if len(handed) != 1 || handed[0] != "BOOTSTRAP_VERIFYING=1" {
		t.Errorf("the gate was handed %v, so a nested run would verify again and never finish", handed)
	}
}

func TestARunInsideAVerifyDoesNotVerifyAgain(t *testing.T) {
	f := newFixture(t)
	f.isInsideVerify = true

	f.expectCode(f.runStep("--skip-verify", "--agent=claude"), 0)

	f.expectSaid("already inside a verify run")
	if f.machine.RanAny(f.repo + "/gate.sh") {
		t.Errorf("a nested run re-entered the gate: %v", f.machine.Calls)
	}
}

// The gate's 1 and 2 send a reader to different places — the code, or this machine and whatever else
// was writing in the checkout. Folding them into one refusal blames the checks for something they did
// not do, and the exit code alone cannot tell the two apart.
func TestAFailingGateAndAnUnmeasuredOneAreDifferentRefusals(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering(f.repo+"/gate.sh", func(machine.Command) int { return 1 })

	f.expectCode(f.runStep("--skip-verify", "--agent=claude"), 1)

	f.expectSaid("reported a failing check")
	f.expectNotSaid("could not measure every check")
}

// The refusal points at the gate's own output. It once told the reader to re-run "if it says a check
// refused its own result" — an exit 3 the shell runner this replaced produced when the checkout moved
// while the suites ran. The gate has no such code: 0, 1 and 2, where 2 is a check that never measured
// and printed its own reason. Advice for a state nothing can reach sends a reader hunting for a line
// no tool prints.
func TestAGateThatCouldNotMeasureIsNotBlamedOnTheCode(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering(f.repo+"/gate.sh", func(machine.Command) int { return 2 })

	f.expectCode(f.runStep("--skip-verify", "--agent=claude"), 1)

	f.expectSaid("could not measure every check")
	f.expectSaid("each said why above")
	f.expectNotSaid("refused its own result")
	f.expectNotSaid("reported a failing check")
}

// A missing gate must not be reported as a failing check. Without this the call exits 127 and the arm
// above blames the checks for a file that was never there — a false diagnosis pointing at code that is
// fine. Checked in the dry run too: a dry run that says ok over a checkout where the real run cannot
// work is the same lie one step earlier.
func TestACheckoutWithoutTheGateIsARefusalInBothModes(t *testing.T) {
	f := newFixture(t)
	f.RemoveAll(f.repo + "/gate.sh")

	f.expectCode(f.runStep("--skip-verify", "--agent=claude"), 1)
	f.expectSaid("is not in this checkout")
	f.expectSaid("not the same as passing")
	f.expectNotSaid("reported a failing check")

	f.expectCode(f.runStep("--skip-verify", "--agent=claude", "--dry-run"), 1)
	f.expectNotSaid("ai bootstrap: ok")
}
