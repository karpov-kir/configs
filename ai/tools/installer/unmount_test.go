package installer_test

// Taking the mounts back out, over the same table the caller declared. Same table because a second
// pass re-deriving what to remove drifts from what was installed, and drifts silently in the one
// direction nobody notices: leaving things behind and reporting ok.
//
// The proof a target was written here is the whole subject: a symlink whose value resolves under this
// checkout, and nothing else.

import (
	"testing"

	"kk-flavor/tools/installer"
)

func (f *fixture) unmount(options installer.RunOptions) *installer.Run {
	f.t.Helper()
	run := f.newRun(options)
	f.declareMounts(run, f.repo)
	run.Unmount()
	f.expectContained(run)
	return run
}

func TestUnmountRemovesWhatThisCheckoutWrote(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.mount(installer.RunOptions{})

	run := f.unmount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("uninstalling an installed machine reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectAbsent(f.home + "/.zshrc")
	f.expectAbsent(f.home + "/.config/nvim")
}

// Declared above the mount table, an uninstall path that links every mount and then removes it leaves
// the machine installed by the command that exists to uninstall it if anything interrupts between the
// two — and a run over a clean machine reads like an install.
func TestUnmountOverAMachineHoldingNothingLinksNothingOnTheWay(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	run := f.unmount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("uninstalling a clean machine reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectNotSaid("  linked   ")
	f.expectSaid("is already gone")
	f.expectAbsent(f.home + "/.zshrc")
}

func TestUnmountRefusesEveryTargetItCannotProveItWrote(t *testing.T) {
	t.Parallel()
	// A real file at the target: not this run's to remove, and the refusal says who should.
	t.Run("a target that is not a symlink is left for the human", func(t *testing.T) {
		f := newFixture(t)
		f.write(f.home+"/.zshrc", "hand-written")

		run := f.unmount(installer.RunOptions{})

		f.expectSaid("is not a symlink, so this did not write it")
		f.expectFileBody(f.home+"/.zshrc", "hand-written")
		if code := run.Report(); code != 1 {
			t.Errorf("a real file at a target reported %d, wanted 1", code)
		}
	})

	// A relative value resolves against THIS process's working directory, not the link's own, so
	// ownership would be judged from somewhere the link never named — and a link reading `notmine`
	// would resolve under the checkout and be deleted.
	t.Run("and a relative link, which this never writes, is left alone", func(t *testing.T) {
		f := newFixture(t)
		f.symlink("zsh/.zshrc", f.home+"/.zshrc")

		run := f.unmount(installer.RunOptions{})

		f.expectSaid("points at the relative path")
		f.expectSymlink(f.home + "/.zshrc")
		f.expectRefusals(run, 1)
	})

	// A mount resolving into another checkout is not this run's to delete any more than it is this
	// run's to repoint.
	t.Run("and a link into another checkout is left where it is", func(t *testing.T) {
		f := newFixture(t)
		other := f.newCheckout(f.base + "/other-repo")
		f.symlink(other+"/zsh/.zshrc", f.home+"/.zshrc")

		run := f.unmount(installer.RunOptions{})

		f.expectSaid("which is not in this checkout — left alone")
		f.expectLinkTo(f.home+"/.zshrc", other+"/zsh/.zshrc")
		f.expectRefusals(run, 1)
	})

	// A link resolving nowhere resolves to empty and falls through to the same refusal, which is right:
	// its target is unknown, so its ownership is too.
	t.Run("and one resolving nowhere is left alone too", func(t *testing.T) {
		f := newFixture(t)
		f.symlink(f.base+"/gone/zsh/.zshrc", f.home+"/.zshrc")

		run := f.unmount(installer.RunOptions{})

		f.expectSaid("which is not in this checkout — left alone")
		f.expectSymlink(f.home + "/.zshrc")
		f.expectRefusals(run, 1)
	})
}

func TestADryRunUninstallRemovesNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.mount(installer.RunOptions{})

	run := f.unmount(installer.RunOptions{DryRun: true})

	if code := run.Report(); code != 0 {
		t.Errorf("a dry-run uninstall reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("would remove " + f.home + "/.zshrc")
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}
