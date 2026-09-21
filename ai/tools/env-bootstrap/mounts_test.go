package envbootstrap_test

import "testing"

// The table, driven end to end. A test comparing the declared sources against a list written here
// would agree with itself. This case asserts that each config lands where env/README.md says a human
// finds it.
func TestAFreshMachineGetsEveryConfigInTheTable(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew"), 0)

	f.expectSaid("env bootstrap: ok")
	// A fresh machine gives the second-checkout guard no target, so it passes straight through. The
	// assertion reads the wording, because the exit code alone looks the same when the guard was never
	// reached.
	f.expectSaid("no mount on this machine comes from another checkout")
	f.expectLinkTo(f.home+"/.zpreztorc", f.repo+"/zsh/.zpreztorc")
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
	f.expectLinkTo(f.home+"/.gitconfig", f.repo+"/git/.gitconfig")
	f.expectLinkTo(f.home+"/.config/ghostty", f.repo+"/ghostty")
	f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	f.expectLinkTo(f.home+"/.config/starship.toml", f.repo+"/starship/starship.toml")
}

// The guard stops before the first write, and the packages step goes with it. In the shell this
// replaced, the packages step ran after a function that ended the process. Here a caller's own branch
// holds the two together, and this case is what asserts that branch.
func TestAMachineMountedFromAnotherCheckoutInstallsNothingEither(t *testing.T) {
	f := newFixture(t)
	stranger := f.base + "/stranger/env"
	f.newCheckout(stranger)
	f.Symlink(stranger+"/zsh/.zshrc", f.home+"/.zshrc")

	f.expectCode(f.run(), 1)

	f.expectSaid("not to this checkout")
	f.expectLinkTo(f.home+"/.zshrc", stranger+"/zsh/.zshrc")
	if len(f.brew.installs) > 0 {
		t.Errorf("the run installed %v from a checkout it refused to mount from", f.brew.installs)
	}
}
