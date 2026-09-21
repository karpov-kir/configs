package aibootstrap_test

import (
	"slices"
	"testing"
)

// rtk is personal tooling and the owner tier's, which leaves the default tier no formula to install at
// all. A tier that quietly left one out would read like a step that never ran, so the skip line names
// the tier it belongs to.
func TestTheDefaultTierInstallsNothingAndSaysRtkIsTheOwnersAlone(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	if len(f.machine.Installs) > 0 {
		t.Errorf("a default tier installed %v", f.machine.Installs)
	}
	f.ExpectSaid("  skipped  rtk is the owner tier's")
}

func TestTheOwnerTierInstallsRtk(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude", "--owner"), 0)

	if !slices.Contains(f.machine.Installs, "formula rtk") {
		t.Errorf("the owner tier did not install rtk: %v", f.machine.Installs)
	}
}

// Installed-first, which is what keeps a finished machine quiet: an unconditional install is slow,
// noisy, and answers non-zero on an already-installed formula.
func TestAFormulaAlreadyOnTheMachineIsNotInstalledAgain(t *testing.T) {
	f := newFixture(t)
	f.machine.Installed["formula rtk"] = true

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude", "--owner"), 0)

	f.ExpectSaid("  ok       rtk")
	if len(f.machine.Installs) > 0 {
		t.Errorf("the run reinstalled %v", f.machine.Installs)
	}
}

func TestADryRunNamesWhatItWouldInstallAndInstallsNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude", "--owner", "--dry-run"), 0)

	f.ExpectSaid("would install rtk")
	if len(f.machine.Installs) > 0 {
		t.Errorf("a dry run installed %v", f.machine.Installs)
	}
}

// A machine without brew still gets every link. The refusal says which half did not happen. A run
// that mounted everything and installed no formula has to read as unfinished. This case runs at the
// owner tier, the only tier with a formula to miss.
func TestAMachineWithoutBrewIsStillMountedAndSaysWhatItMissed(t *testing.T) {
	f := newFixture(t)
	f.machine.Without("brew")

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude", "--owner"), 1)

	f.ExpectSaid("brew is not installed, so no formula was installed")
	f.ExpectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
}

// A machine lacking brew still installs, where the tier asks brew for no formula. Since jq went, that
// is every tier but the owner's, and refusing there failed a whole install over a prerequisite the
// tier never used. The skip line still prints, so the run says what this tier does not take.
func TestATierThatInstallsNoFormulaDoesNotNeedBrew(t *testing.T) {
	f := newFixture(t)
	f.machine.Without("brew")

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	f.ExpectNotSaid("brew is not installed")
	f.ExpectSaid("  skipped  rtk is the owner tier's")
	if len(f.machine.Installs) > 0 {
		t.Errorf("a tier with no formula installed %v", f.machine.Installs)
	}
}
