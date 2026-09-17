package aibootstrap_test

import (
	"slices"
	"testing"
)

// rtk is personal tooling and the owner tier's, which leaves the default tier no formula to install at
// all. A tier that quietly left one out would be indistinguishable from a step that never ran, so the
// skip says whose it is.
func TestTheDefaultTierInstallsNothingAndSaysRtkIsTheOwnersAlone(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	if len(f.machine.installs) > 0 {
		t.Errorf("a default tier installed %v", f.machine.installs)
	}
	f.expectSaid("  skipped  rtk is the owner tier's")
}

func TestTheOwnerTierInstallsRtk(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-brew", "--agent=claude", "--owner"), 0)

	if !slices.Contains(f.machine.installs, "rtk") {
		t.Errorf("the owner tier did not install rtk: %v", f.machine.installs)
	}
}

// Installed-first, which is what keeps a finished machine quiet: an unconditional install is slow,
// noisy, and answers non-zero on an already-installed formula.
func TestAFormulaAlreadyOnTheMachineIsNotInstalledAgain(t *testing.T) {
	f := newFixture(t)
	f.machine.installed["rtk"] = true

	f.expectCode(f.runStep("--skip-brew", "--agent=claude", "--owner"), 0)

	f.expectSaid("  ok       rtk")
	if len(f.machine.installs) > 0 {
		t.Errorf("the run reinstalled %v", f.machine.installs)
	}
}

func TestADryRunNamesWhatItWouldInstallAndInstallsNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-brew", "--agent=claude", "--owner", "--dry-run"), 0)

	f.expectSaid("would install rtk")
	if len(f.machine.installs) > 0 {
		t.Errorf("a dry run installed %v", f.machine.installs)
	}
}

// A machine without brew still gets every link. The refusal says which half did not happen, because a
// run that mounted everything and installed nothing must not read as a finished machine. Driven at the
// owner tier, the only one with a formula to miss.
func TestAMachineWithoutBrewIsStillMountedAndSaysWhatItMissed(t *testing.T) {
	f := newFixture(t)
	f.machine.without("brew")

	f.expectCode(f.runStep("--skip-brew", "--agent=claude", "--owner"), 1)

	f.expectSaid("brew is not installed, so no formula was installed")
	f.expectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
}

// A machine with no brew still installs, where the tier asks brew for nothing. Since jq went, that is
// every tier but the owner's, and refusing there failed a whole install over a prerequisite nothing in
// it needed. The skip line still prints, so the run says what this tier does not take rather than
// going quiet about it.
func TestATierThatInstallsNoFormulaDoesNotNeedBrew(t *testing.T) {
	f := newFixture(t)
	f.machine.without("brew")

	f.expectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	f.expectNotSaid("brew is not installed")
	f.expectSaid("  skipped  rtk is the owner tier's")
	if len(f.machine.installs) > 0 {
		t.Errorf("a tier with no formula installed %v", f.machine.installs)
	}
}
