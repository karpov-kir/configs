package aibootstrap_test

import (
	"slices"
	"testing"
)

// rtk compresses this machine's shell output for the agent, and it is the only formula.
func TestTheInstallInstallsRtk(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	if !slices.Contains(f.machine.Installs, "formula rtk") {
		t.Errorf("the install did not install rtk: %v", f.machine.Installs)
	}
}

// Installed-first, which is what keeps a finished machine quiet: an unconditional install is slow,
// noisy, and answers non-zero on an already-installed formula.
func TestAFormulaAlreadyOnTheMachineIsNotInstalledAgain(t *testing.T) {
	f := newFixture(t)
	f.machine.Installed["formula rtk"] = true

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude"), 0)

	f.ExpectSaid("  ok       rtk")
	if len(f.machine.Installs) > 0 {
		t.Errorf("the run reinstalled %v", f.machine.Installs)
	}
}

func TestADryRunNamesWhatItWouldInstallAndInstallsNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude", "--dry-run"), 0)

	f.ExpectSaid("would install rtk")
	if len(f.machine.Installs) > 0 {
		t.Errorf("a dry run installed %v", f.machine.Installs)
	}
}

// A machine without brew still gets every link. The refusal says which half did not happen. A run
// that mounted everything and installed no formula has to read as unfinished.
func TestAMachineWithoutBrewIsStillMountedAndSaysWhatItMissed(t *testing.T) {
	f := newFixture(t)
	f.machine.Without("brew")

	f.ExpectCode(f.runStep("--skip-brew", "--agent=claude"), 1)

	f.ExpectSaid("brew is not installed, so no formula was installed")
	f.ExpectLinkTo(f.home+"/.kk-flavor", f.repo+"/kk-flavor")
}
