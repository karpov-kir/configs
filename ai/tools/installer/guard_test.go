package installer_test

// The second-checkout guard. A run derives its checkout from where the installer lives. Run from a
// scratch clone, it repoints every mount at the clone and reports "repointed" six times for an act
// no human authorised.

// Delete the clone afterwards, which is the entire point of a scratch clone, and the human's next
// login is missing its shell config and its git config.

// Link is right that a symlink carries no data of its own. The damage is to the MOUNT, and that is
// why the guard sits in front of Link instead of inside it. It is also why the stale-symlink and
// trailing-slash cases next door must stay green alongside these.

// This repository is cloned routinely to verify published state, so this is a live hazard.

// The fixture is a second checkout mounted onto a home by running ITS table, and the mounts under
// test are the ones the package really writes. A hand-made imitation would test itself.

import (
	"testing"

	"configs/ai/tools/installer"
)

// A home already mounted from a second checkout, and where that checkout is.
func newHomeMountedElsewhere(t *testing.T) (*fixture, string) {
	t.Helper()
	f := newFixture(t)
	other := f.newCheckout(f.base + "/other-repo")
	f.mountFrom(other, installer.RunOptions{})
	f.expectLinkTo(f.home+"/.zshrc", other+"/zsh/.zshrc")
	return f, other
}

func TestARunFromASecondCheckoutWritesNothing(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 1 {
		t.Errorf("a run from a second checkout reported %d, wanted 1", code)
	}
	f.expectSaid("not to this checkout")
	f.expectSaid("4 mounts")
	f.expectSaid(other)
	f.expectSaid("nothing was written")
	f.expectSaid("--relocate")
	f.expectNotSaid(label + ": ok")

	// The half that carries the rest. A guard that refuses after repointing has still moved the
	// machine, so the mounts are read back instead of the message being taken at its word.
	f.expectLinkTo(f.home+"/.zshrc", other+"/zsh/.zshrc")
	f.expectLinkTo(f.home+"/.config/nvim", other+"/nvim")
}

// A dry run has to refuse as well. With the guard gone it reports "would repoint" for all of them
// and exits 0. That is the same lie one step earlier: someone checks with a dry run, reads ok, and
// runs it for real.
func TestADryRunFromASecondCheckoutRefusesRatherThanPreviewingTheMove(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)

	run := f.mount(installer.RunOptions{DryRun: true})

	if code := run.Report(); code != 1 {
		t.Errorf("a dry run from a second checkout reported %d, wanted 1", code)
	}
	f.expectNotSaid(label + ": ok")
	f.expectNotSaid("would repoint")
	f.expectLinkTo(f.home+"/.zshrc", other+"/zsh/.zshrc")
}

// The escape hatch, which has to exist or someone genuinely relocating their configs cannot. It is a
// flag of its own, apart from the skip family every caller passes as a block, so habit alone never
// carries it in. Every case here reaches the guard without it.
func TestRelocateMovesTheMountsToThisCheckout(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)

	run := f.mount(installer.RunOptions{Relocate: true})

	if code := run.Report(); code != 0 {
		t.Errorf("--relocate reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("moving 4 mount(s)")
	f.expectSaid(other)
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// A machine mounted from two other checkouts at once, which is what half-moving one by hand leaves.
// The guard reports per root. A reader told about one root and left ignorant of the other moves that
// checkout, re-runs, and meets a refusal from a root the first report never named. The case that
// matters is the second root appearing.
func TestTwoForeignCheckoutsAreBothNamed(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)
	third := f.newCheckout(f.base + "/third-repo")
	f.removeLink(f.home + "/.gitconfig")
	f.Symlink(third+"/git/.gitconfig", f.home+"/.gitconfig")

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 1 {
		t.Errorf("two foreign checkouts reported %d, wanted 1", code)
	}
	f.expectSaid(other)
	f.expectSaid(third)
	f.expectRefusals(run, 2)
	f.expectLinkTo(f.home+"/.gitconfig", third+"/git/.gitconfig")
}

// A checkout that no longer resolves is the aftermath of this bug, or of a directory moved on
// purpose. A dangling mount is repaired by repointing it, so the guard stands aside. A guard that
// refused here would leave the human's shell broken, with no way to fix it from the repository.
func TestAMountFromACheckoutThatIsGoneIsRepairedRatherThanRefused(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)
	f.RemoveAll(other)

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a mount from a deleted checkout reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// The three shapes a stale mount takes that are NOT a second checkout. Each is a link the
// stale-symlink rule is right to repoint, and a guard that dropped one of its conditions would
// refuse each of them. It would refuse every machine holding one unrelated config symlink, with no
// case going red.
func TestAStaleMountIsNotMistakenForASecondCheckout(t *testing.T) {
	t.Parallel()
	// What a machine with an older dotfiles layout holds: a real directory that is no checkout of this
	// repository. The root resolves and the tail matches, and only the missing installer holds them
	// apart. The stale-symlink case next door uses a dangling link, so it is turned away a step
	// earlier and never reaches this test.
	t.Run("a mount into an unrelated real directory is repointed", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.home + "/.config")
		f.MkdirAll(f.base + "/old-dotfiles/nvim")
		f.Symlink(f.base+"/old-dotfiles/nvim", f.home+"/.config/nvim")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 0)
		f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	})

	// A mount naming a checkout's root instead of the config inside it. A run writes
	// `<checkout>/nvim` and never a bare `<checkout>`, so such a link is stale whatever the root turns
	// out to be. Only the relative-path comparison holds the two apart, since the root here really is
	// a second checkout and really does hold a copy of the installer.
	t.Run("and one naming a checkout directory rather than a file in it is repointed too", func(t *testing.T) {
		f := newFixture(t)
		other := f.newCheckout(f.base + "/other-repo")
		f.MkdirAll(f.home + "/.config")
		f.Symlink(other, f.home+"/.config/nvim")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 0)
		f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	})

	// A relative link value is one this package never wrote, since every link it makes is absolute. A
	// root resolved out of one lands against this process's working directory, and the link's own
	// directory goes unused. The link itself dangles, which is what makes it merely stale.
	t.Run("and a relative link is repointed like any other stale link", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.home + "/.config")
		f.Symlink("other-repo/nvim", f.home+"/.config/nvim")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 0)
		f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	})
}

// The same checkout named through a symlinked path. A temp directory is handed back as
// /var/folders/…, and /var is a symlink to /private/var, so a real macOS mount takes this shape.

// A guard comparing an unresolved root against a resolved checkout calls that checkout a stranger to
// itself and refuses a machine that is mounted correctly. Both sides are resolved physically for
// that reason.
func TestAMountNamingTheRunningCheckoutThroughASymlinkedPathIsNotForeign(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.Symlink(f.repo, f.base+"/alias-to-checkout")
	f.Symlink(f.base+"/alias-to-checkout/zsh/.zshrc", f.home+"/.zshrc")

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a mount through a symlinked path reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// A reader told their two configs would move, and left to discover that every skill moves with them,
// has missed the scale of what the run would do. The bulk set could otherwise drop out of the count
// with no case going red.
func TestABulkMountTakesPartInTheCountTheGuardReports(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	other := f.newCheckout(f.base + "/other-repo")
	for _, checkout := range []string{f.repo, other} {
		f.MkdirAll(checkout + "/skills/kk-build")
		f.MkdirAll(checkout + "/skills/kk-ship")
	}
	mountedFrom := func(repo string, options installer.RunOptions) *installer.Run {
		options.Repo = repo
		options.BulkLabel = "skills"
		run := f.newRun(options)
		f.declareMounts(run, repo)
		run.AddBulk(repo+"/skills/kk-build", f.home+"/.claude/skills/kk-build")
		run.AddBulk(repo+"/skills/kk-ship", f.home+"/.claude/skills/kk-ship")
		run.Mount()
		f.expectContained(run)
		return run
	}

	mountedFrom(other, installer.RunOptions{})
	f.expectLinkTo(f.home+"/.claude/skills/kk-build", other+"/skills/kk-build")

	run := mountedFrom(f.repo, installer.RunOptions{})

	if code := run.Report(); code != 1 {
		t.Errorf("a run from a second checkout reported %d, wanted 1", code)
	}
	f.expectSaid("6 mounts (4 configs and 2 skills)")
	f.expectSaid("...and all 2 skills")
	// The carrying half again: the mounts are read back, and the message's word is left aside.
	f.expectLinkTo(f.home+"/.claude/skills/kk-build", other+"/skills/kk-build")

	relocated := mountedFrom(f.repo, installer.RunOptions{Relocate: true})
	if code := relocated.Report(); code != 0 {
		t.Errorf("--relocate over a bulk set reported %d, wanted 0: %v", code, relocated.Refusals())
	}
	f.expectSaid("moving 6 mount(s)")
	f.expectLinkTo(f.home+"/.claude/skills/kk-build", f.repo+"/skills/kk-build")
}
