package envbootstrap_test

import (
	"slices"
	"testing"
)

// Installed-first, which is what keeps a finished machine quiet: an unconditional install is slow,
// noisy, and answers non-zero on an already-installed cask.
func TestAPackageAlreadyOnTheMachineIsNotInstalledAgain(t *testing.T) {
	f := newFixture(t)
	f.brew.Installed["formula mise"] = true
	f.brew.Installed["cask ghostty"] = true

	f.ExpectCode(f.run(), 0)

	f.ExpectSaid("  ok       mise")
	f.ExpectSaid("  ok       ghostty")
	if slices.Contains(f.brew.Installs, "formula mise") || slices.Contains(f.brew.Installs, "cask ghostty") {
		t.Errorf("the run reinstalled something this machine already had: %v", f.brew.Installs)
	}
}

// A cask is asked for with its own flag on both halves. `brew list --formula ghostty` exits non-zero
// for an installed cask, and a run without the flag reinstalls it on every pass.
func TestACaskIsInstalledAsACaskAndNotAsAFormula(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run(), 0)

	if !slices.Contains(f.brew.Installs, "cask ghostty") {
		t.Errorf("ghostty was not installed as a cask: %v", f.brew.Installs)
	}
}

func TestADryRunNamesWhatItWouldInstallAndInstallsNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--dry-run"), 0)

	f.ExpectSaid("would install mise")
	f.ExpectSaid("would install --cask ghostty")
	if len(f.brew.Installs) > 0 {
		t.Errorf("a dry run installed %v", f.brew.Installs)
	}
}

// One failure must not abort the rest: a machine missing one cask should still get every other package
// and every link, or fixing it becomes one run per problem.
func TestAFailedInstallIsCollectedRatherThanEndingTheRun(t *testing.T) {
	f := newFixture(t)
	f.brew.Failing["formula mise"] = true

	f.ExpectCode(f.run(), 1)

	f.ExpectSaid("brew install mise failed")
	if !slices.Contains(f.brew.Installs, "formula starship") {
		t.Errorf("the run stopped at the first failure and never reached starship: %v", f.brew.Installs)
	}
	f.ExpectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// A machine without brew still gets every link. The refusal is what keeps a half-done machine from
// reading as a finished one.
func TestAMachineWithoutBrewIsStillMountedAndSaysWhatItMissed(t *testing.T) {
	f := newFixture(t)
	f.brew.Without("brew")

	f.ExpectCode(f.run(), 1)

	f.ExpectSaid("brew is not installed, so no formula or cask was installed")
	f.ExpectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// A silent skip reads exactly like a step that was never reached, so the skip is said out loud. Most
// of this package's cases pass the flag.
func TestSkipBrewSaysItWasSkipped(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--skip-brew"), 0)

	f.ExpectSaid("brew (skipped)")
	f.ExpectNotSaid("  ok       mise")
	if len(f.brew.Installs) > 0 {
		t.Errorf("--skip-brew installed %v", f.brew.Installs)
	}
}
