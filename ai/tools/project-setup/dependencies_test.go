package projectsetup_test

import (
	"testing"

	"configs/ai/tools/machine"
)

// A mise on PATH is asked for `--version` before it counts: one that cannot answer is a broken
// executable, and every step after this would fail somewhere less obvious.
func TestAWorkingMiseSatisfiesThePrerequisiteWithoutTouchingBrew(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude"), 0)

	f.expectSaid("ok       mise is available on PATH")
	if !f.machine.Ran("mise", "--version") {
		t.Errorf("mise was never verified: %v", f.machine.Spelled("mise"))
	}
	if f.machine.RanAny("brew") {
		t.Errorf("a working mise still reached brew: %v", f.machine.Spelled("brew"))
	}
}

func TestABrokenMiseFailsReadinessWithoutChangingTheMachine(t *testing.T) {
	f := newFixture(t)
	f.machine.Answering("mise", func(machine.Command) int { return 1 })

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("mise --version failed")
	if f.machine.RanAny("brew") {
		t.Errorf("a broken mise triggered a machine change: %v", f.machine.Spelled("brew"))
	}
}

// mise installed through an existing Homebrew, and only mise. The auto-update is suppressed because an
// install of one formula that first updates every tap turns a project setup into a wait the human
// never asked for.
func TestAMissingMiseIsInstalledThroughAnExistingBrew(t *testing.T) {
	f := newFixture(t)
	f.machine.Present["mise"] = false
	f.machine.Answering("brew", func(command machine.Command) int {
		f.machine.Present["mise"] = true
		return 0
	})

	f.expectCode(f.install("--agent=claude"), 0)

	if !f.machine.Ran("brew", "install", "mise") {
		t.Errorf("brew was not asked for mise: %v", f.machine.Spelled("brew"))
	}
	if environment := f.machine.Calls[len(f.machine.Calls)-2].Env; len(environment) != 1 ||
		environment[0] != "HOMEBREW_NO_AUTO_UPDATE=1" {
		t.Errorf("brew was handed %v, so the install would first update every tap", environment)
	}
	f.expectSaid("ok       mise is available on PATH")
}

// With neither, it says what to do and stops: putting a package manager on somebody's machine to
// reach a runtime is their decision.
func TestNeitherMiseNorBrewStopsWithTheInstructions(t *testing.T) {
	f := newFixture(t)
	f.machine.Present["mise"] = false
	f.machine.Present["brew"] = false

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("https://mise.jdx.dev/installing-mise.html")
	f.expectSaid("PATH")
	f.expectAbsent(f.skillsMount("claude"))
}

func TestAFailedBrewInstallIsNamedRatherThanPassedOver(t *testing.T) {
	f := newFixture(t)
	f.machine.Present["mise"] = false
	f.machine.Answering("brew", func(machine.Command) int { return 1 })

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("brew install mise failed")
}

// brew can exit 0 having put mise somewhere this shell cannot reach. This run reports the PATH
// problem, because the install itself did not fail.
func TestBrewSucceedingWithoutAReachableMiseGivesThePathRecovery(t *testing.T) {
	f := newFixture(t)
	f.machine.Present["mise"] = false
	f.machine.Answering("brew", func(machine.Command) int { return 0 })

	f.expectCode(f.install("--agent=claude"), 1)

	f.expectSaid("Add the Homebrew bin directory to PATH")
}

func TestADryRunNamesTheBoundedInstallAndRunsNothing(t *testing.T) {
	f := newFixture(t)
	f.machine.Present["mise"] = false

	f.expectCode(f.install("--agent=claude", "--dry-run"), 0)

	f.expectSaid("would run HOMEBREW_NO_AUTO_UPDATE=1 brew install mise")
	if f.machine.RanAny("brew") {
		t.Errorf("a dry run invoked brew: %v", f.machine.Spelled("brew"))
	}
}
