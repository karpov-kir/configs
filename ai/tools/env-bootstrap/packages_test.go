package envbootstrap_test

import (
	"slices"
	"testing"
)

// Installed-first, which is what keeps a finished machine quiet: an unconditional install is slow,
// noisy, and answers non-zero on an already-installed cask.
func TestAPackageAlreadyOnTheMachineIsNotInstalledAgain(t *testing.T) {
	f := newFixture(t)
	f.brew.installed["formula mise"] = true
	f.brew.installed["cask ghostty"] = true

	f.expectCode(f.run(), 0)

	f.expectSaid("  ok       mise")
	f.expectSaid("  ok       ghostty")
	if slices.Contains(f.brew.installs, "formula mise") || slices.Contains(f.brew.installs, "cask ghostty") {
		t.Errorf("the run reinstalled something this machine already had: %v", f.brew.installs)
	}
}

// A cask is asked for with its own flag on both halves. `brew list --formula ghostty` answers no for a
// cask that is installed, so a run that dropped the flag would reinstall it on every pass and report a
// finished machine as one needing work.
func TestACaskIsInstalledAsACaskAndNotAsAFormula(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run(), 0)

	if !slices.Contains(f.brew.installs, "cask ghostty") {
		t.Errorf("ghostty was not installed as a cask: %v", f.brew.installs)
	}
}

func TestADryRunNamesWhatItWouldInstallAndInstallsNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--dry-run"), 0)

	f.expectSaid("would install mise")
	f.expectSaid("would install --cask ghostty")
	if len(f.brew.installs) > 0 {
		t.Errorf("a dry run installed %v", f.brew.installs)
	}
}

// One failure must not abort the rest: a machine missing one cask should still get every other package
// and every link, or fixing it becomes one run per problem.
func TestAFailedInstallIsCollectedRatherThanEndingTheRun(t *testing.T) {
	f := newFixture(t)
	f.brew.failing["formula mise"] = true

	f.expectCode(f.run(), 1)

	f.expectSaid("brew install mise failed")
	if !slices.Contains(f.brew.installs, "formula starship") {
		t.Errorf("the run stopped at the first failure and never reached starship: %v", f.brew.installs)
	}
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// A machine without brew still gets every link. The refusal says which half did not happen, because a
// run that mounted everything and installed nothing must not read as a finished machine.
func TestAMachineWithoutBrewIsStillMountedAndSaysWhatItMissed(t *testing.T) {
	f := newFixture(t)
	f.brew.without()

	f.expectCode(f.run(), 1)

	f.expectSaid("brew is not installed, so no formula or cask was installed")
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// Said out loud rather than passed over in silence. A skipped step that prints nothing reads exactly
// like a step that was never reached, and every case in this suite but the ones above passes the flag.
func TestSkipBrewSaysItWasSkipped(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew"), 0)

	f.expectSaid("brew (skipped)")
	f.expectNotSaid("  ok       mise")
	if len(f.brew.installs) > 0 {
		t.Errorf("--skip-brew installed %v", f.brew.installs)
	}
}
