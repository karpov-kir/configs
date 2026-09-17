package installer_test

// The second-checkout guard. A run derives its checkout from where the installer lives, so run from a
// scratch clone it repoints every mount at the clone and reports "repointed" six times for an act
// nobody authorised. Delete the clone afterwards — the entire point of a scratch clone — and the
// human's next login has no shell config and no git config. link is right that a symlink carries no
// data of its own; the damage is to the MOUNT, which is why the guard sits above link rather than
// inside it, and why the stale-symlink and trailing-slash cases next door must stay green alongside
// these. This repository is cloned routinely to verify published state, so this is a live hazard.
//
// The fixture is a second checkout mounted onto a home by running ITS table, so the mounts under test
// are the ones the package really writes rather than a hand-made imitation of them.

import (
	"testing"

	"kk-flavor/tools/installer"
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

	// The load-bearing half. A guard that refuses after repointing has still moved the machine, so the
	// mounts are read back rather than the message being taken at its word.
	f.expectLinkTo(f.home+"/.zshrc", other+"/zsh/.zshrc")
	f.expectLinkTo(f.home+"/.config/nvim", other+"/nvim")
}

// A dry run has to refuse as well. Unguarded it reports "would repoint" for all of them and exits 0,
// which is the same lie one step earlier: someone checks with a dry run, reads ok, and runs it for
// real.
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

// The escape hatch, which has to exist or someone genuinely relocating their configs cannot. A flag of
// its own rather than a member of the skip family every caller passes as a block, so it is not
// something that rides along by habit — every case here reaches the guard without it.
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
// The guard reports per root, and a reader told about one of them and not the other moves that
// checkout, re-runs, and is refused again by a root nobody named — so the case that matters is the
// second one appearing, not the first.
func TestTwoForeignCheckoutsAreBothNamed(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)
	third := f.newCheckout(f.base + "/third-repo")
	f.removeLink(f.home + "/.gitconfig")
	f.symlink(third+"/git/.gitconfig", f.home+"/.gitconfig")

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 1 {
		t.Errorf("two foreign checkouts reported %d, wanted 1", code)
	}
	f.expectSaid(other)
	f.expectSaid(third)
	f.expectRefusals(run, 2)
	f.expectLinkTo(f.home+"/.gitconfig", third+"/git/.gitconfig")
}

// A checkout that no longer resolves is the aftermath of this very bug, or of a directory moved on
// purpose. Repointing a dangling mount is the repair, so the guard must not stand in front of it — one
// that refused here would leave the human's shell broken with no way to fix it from the repository.
func TestAMountFromACheckoutThatIsGoneIsRepairedRatherThanRefused(t *testing.T) {
	t.Parallel()
	f, other := newHomeMountedElsewhere(t)
	f.removeAll(other)

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a mount from a deleted checkout reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// The three shapes a stale mount takes that are NOT a second checkout. Each is a link the stale-symlink
// rule is right to repoint, and each would be refused by a guard that dropped one of its conditions —
// refusing every machine that holds one unrelated config symlink, with nothing going red.
func TestAStaleMountIsNotMistakenForASecondCheckout(t *testing.T) {
	t.Parallel()
	// What a machine with an older dotfiles layout holds: a real directory that is not a checkout of
	// this repository. The root resolves and the tail matches; only the missing installer holds them
	// apart. The stale-symlink case next door uses a dangling link, so it is turned away a limb earlier
	// and never reaches this test.
	t.Run("a mount into an unrelated real directory is repointed", func(t *testing.T) {
		f := newFixture(t)
		f.mkdirAll(f.home + "/.config")
		f.mkdirAll(f.base + "/old-dotfiles/nvim")
		f.symlink(f.base+"/old-dotfiles/nvim", f.home+"/.config/nvim")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 0)
		f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	})

	// A mount naming a checkout's root rather than the config inside it. A run writes
	// `<checkout>/nvim` and never `<checkout>`, so such a link is stale whatever the root turns out to
	// be — and only the relative-path comparison holds the two apart, since the root here really is a
	// second checkout and really does hold a copy of the installer.
	t.Run("and one naming a checkout directory rather than a file in it is repointed too", func(t *testing.T) {
		f := newFixture(t)
		other := f.newCheckout(f.base + "/other-repo")
		f.mkdirAll(f.home + "/.config")
		f.symlink(other, f.home+"/.config/nvim")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 0)
		f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	})

	// A relative link value is not one this package wrote — every link it makes is absolute — and
	// resolving a root out of one would resolve it against this process's working directory rather than
	// the link's own. The link itself dangles, which is what makes it merely stale.
	t.Run("and a relative link is repointed like any other stale link", func(t *testing.T) {
		f := newFixture(t)
		f.mkdirAll(f.home + "/.config")
		f.symlink("other-repo/nvim", f.home+"/.config/nvim")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 0)
		f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	})
}

// The same checkout named through a symlinked path. A temp directory is handed back as /var/folders/…
// while /var is a symlink to /private/var, so this is the shape a real macOS mount takes — and a guard
// comparing an unresolved root against a resolved checkout calls that checkout a stranger to itself and
// refuses a machine that is mounted correctly. Both sides are resolved physically for that reason.
func TestAMountNamingTheRunningCheckoutThroughASymlinkedPathIsNotForeign(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.symlink(f.repo, f.base+"/alias-to-checkout")
	f.symlink(f.base+"/alias-to-checkout/zsh/.zshrc", f.home+"/.zshrc")

	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a mount through a symlinked path reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// A reader told their two configs would move, and not that every skill moves with them, has not been
// told the scale of what the run would do. Without this the bulk set could drop out of the count and
// nothing would redden.
func TestABulkMountTakesPartInTheCountTheGuardReports(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	other := f.newCheckout(f.base + "/other-repo")
	for _, checkout := range []string{f.repo, other} {
		f.mkdirAll(checkout + "/skills/kk-build")
		f.mkdirAll(checkout + "/skills/kk-ship")
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
	// The load-bearing half again: read back rather than taken at the message's word.
	f.expectLinkTo(f.home+"/.claude/skills/kk-build", other+"/skills/kk-build")

	relocated := mountedFrom(f.repo, installer.RunOptions{Relocate: true})
	if code := relocated.Report(); code != 0 {
		t.Errorf("--relocate over a bulk set reported %d, wanted 0: %v", code, relocated.Refusals())
	}
	f.expectSaid("moving 6 mount(s)")
	f.expectLinkTo(f.home+"/.claude/skills/kk-build", f.repo+"/skills/kk-build")
}
