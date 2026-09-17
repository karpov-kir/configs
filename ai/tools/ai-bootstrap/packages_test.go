package aibootstrap_test

import (
	"slices"
	"testing"
)

// jq is everyone's; rtk is personal tooling and the owner tier's. A tier that quietly left one out
// would be indistinguishable from a step that never ran, so the skip says whose it is.
func TestTheDefaultTierInstallsJqAndSaysRtkIsTheOwnersAlone(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	if !slices.Contains(f.machine.installs, "jq") {
		t.Errorf("jq was not installed: %v", f.machine.installs)
	}
	if slices.Contains(f.machine.installs, "rtk") {
		t.Errorf("a default tier installed rtk: %v", f.machine.installs)
	}
	f.expectSaid("  skipped  rtk is the owner tier's")
}

func TestTheOwnerTierInstallsRtkAsWell(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-brew", "--agent=claude", "--owner", "--skip-rtk"), 0)

	if !slices.Contains(f.machine.installs, "rtk") {
		t.Errorf("the owner tier did not install rtk: %v", f.machine.installs)
	}
}

// Installed-first, which is what keeps a finished machine quiet: an unconditional install is slow,
// noisy, and answers non-zero on an already-installed formula.
func TestAFormulaAlreadyOnTheMachineIsNotInstalledAgain(t *testing.T) {
	f := newFixture(t)
	f.machine.installed["jq"] = true

	f.expectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	f.expectSaid("  ok       jq")
	if len(f.machine.installs) > 0 {
		t.Errorf("the run reinstalled %v", f.machine.installs)
	}
}

func TestADryRunNamesWhatItWouldInstallAndInstallsNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.runStep("--skip-brew", "--agent=claude", "--dry-run"), 0)

	f.expectSaid("would install jq")
	if len(f.machine.installs) > 0 {
		t.Errorf("a dry run installed %v", f.machine.installs)
	}
}

// A machine without brew still gets every link. The refusal says which half did not happen, because a
// run that mounted everything and installed nothing must not read as a finished machine.
func TestAMachineWithoutBrewIsStillMountedAndSaysWhatItMissed(t *testing.T) {
	f := newFixture(t)
	f.machine.without("brew")

	f.expectCode(f.runStep("--skip-brew", "--agent=claude"), 1)

	f.expectSaid("brew is not installed, so no formula was installed")
	f.expectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
}
