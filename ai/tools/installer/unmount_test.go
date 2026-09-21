package installer_test

// Taking the mounts back out, over the same table the caller declared. Same table, because a second
// pass re-deriving what to remove goes out of step with what was installed. It goes out of step in
// the direction hardest to spot, leaving things behind and reporting ok.

// The proof a target was written here is the whole subject: a symlink whose value resolves under this
// checkout, and no other shape.

import (
	"testing"

	"configs/ai/tools/installer"
)

func (f *fixture) unmount(options installer.RunOptions) *installer.Run {
	f.t.Helper()
	run := f.newRun(options)
	f.declareMounts(run, f.repo)
	run.Unmount()
	f.ExpectNoBreach(run.Breaches())
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
	f.ExpectAbsent(f.home + "/.zshrc")
	f.ExpectAbsent(f.home + "/.config/nvim")
}

// An uninstall path that links every mount and then removes it leaves the machine installed by the
// command that exists to uninstall it, whenever anything interrupts between the two. A run over a
// clean machine also reads like an install.
func TestUnmountOverAMachineHoldingNothingLinksNothingOnTheWay(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	run := f.unmount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("uninstalling a clean machine reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.ExpectNotSaid("  linked   ")
	f.ExpectSaid("is already gone")
	f.ExpectAbsent(f.home + "/.zshrc")
}

func TestUnmountRefusesEveryTargetItCannotProveItWrote(t *testing.T) {
	t.Parallel()
	// A real file at the target: not this run's to remove, and the refusal says who should.
	t.Run("a target that is not a symlink is left for the human", func(t *testing.T) {
		f := newFixture(t)
		f.Write(f.home+"/.zshrc", "hand-written")

		run := f.unmount(installer.RunOptions{})

		f.ExpectSaid("is not a symlink, so this did not write it")
		f.ExpectFileBody(f.home+"/.zshrc", "hand-written")
		if code := run.Report(); code != 1 {
			t.Errorf("a real file at a target reported %d, wanted 1", code)
		}
	})

	// A relative value resolves against THIS process's working directory instead of the link's own,
	// and ownership then gets judged from somewhere the link never named. A link reading `notmine`
	// resolves under the checkout and is deleted.
	t.Run("and a relative link, which this never writes, is left alone", func(t *testing.T) {
		f := newFixture(t)
		f.Symlink("zsh/.zshrc", f.home+"/.zshrc")

		run := f.unmount(installer.RunOptions{})

		f.ExpectSaid("points at the relative path")
		f.ExpectSymlink(f.home + "/.zshrc")
		f.expectRefusals(run, 1)
	})

	// A mount resolving into another checkout is not this run's to delete any more than it is this
	// run's to repoint.
	t.Run("and a link into another checkout is left where it is", func(t *testing.T) {
		f := newFixture(t)
		other := f.newCheckout(f.base + "/other-repo")
		f.Symlink(other+"/zsh/.zshrc", f.home+"/.zshrc")

		run := f.unmount(installer.RunOptions{})

		f.ExpectSaid("which is not in this checkout — left alone")
		f.ExpectLinkTo(f.home+"/.zshrc", other+"/zsh/.zshrc")
		f.expectRefusals(run, 1)
	})

	// A link resolving nowhere resolves to empty and falls through to the same refusal. Its target is
	// unknown, so its ownership is unknown too.
	t.Run("and one resolving nowhere is left alone too", func(t *testing.T) {
		f := newFixture(t)
		f.Symlink(f.base+"/gone/zsh/.zshrc", f.home+"/.zshrc")

		run := f.unmount(installer.RunOptions{})

		f.ExpectSaid("which is not in this checkout — left alone")
		f.ExpectSymlink(f.home + "/.zshrc")
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
	f.ExpectSaid("would remove " + f.home + "/.zshrc")
	f.ExpectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}
