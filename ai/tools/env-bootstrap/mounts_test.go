package envbootstrap_test

import "testing"

// The table, driven end to end rather than read out of the source. A test that compared the declared
// sources with a list written here would agree with itself; this one asserts that each config actually
// lands where env/README.md says a human would find it.
func TestAFreshMachineGetsEveryConfigInTheTable(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew"), 0)

	f.expectSaid("env bootstrap: ok")
	// A machine with nothing mounted yet has nothing for the second-checkout guard to protect, so it
	// passes straight through. Asserted on the wording rather than on the exit, because a guard that
	// printed nothing when it passes is indistinguishable from one that was never reached.
	f.expectSaid("no mount on this machine comes from another checkout")
	f.expectLinkTo(f.home+"/.zpreztorc", f.repo+"/zsh/.zpreztorc")
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
	f.expectLinkTo(f.home+"/.gitconfig", f.repo+"/git/.gitconfig")
	f.expectLinkTo(f.home+"/.config/ghostty", f.repo+"/ghostty")
	f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	f.expectLinkTo(f.home+"/.config/starship.toml", f.repo+"/starship/starship.toml")
}

// The guard stops before the first write, and the packages step goes with it. It is a separate
// statement from "nothing was linked": brew is reached through a caller's own branch here, where the
// shell reached it after a function that ended the process, so nothing but this case says the two
// still travel together.
func TestAMachineMountedFromAnotherCheckoutInstallsNothingEither(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/env"
	f.newCheckout(stranger)
	f.symlink(stranger+"/zsh/.zshrc", f.home+"/.zshrc")

	f.expectCode(f.run(), 1)

	f.expectSaid("not to this checkout")
	f.expectLinkTo(f.home+"/.zshrc", stranger+"/zsh/.zshrc")
	if len(f.brew.installs) > 0 {
		t.Errorf("the run installed %v from a checkout it refused to mount from", f.brew.installs)
	}
}
