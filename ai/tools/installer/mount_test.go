package installer_test

// Linking, and the two rules that must not be weakened: a real file at a target is refused rather than
// deleted, and a machine mounted from another checkout is left alone rather than moved here. The
// hand-run form of the first is `rm -rf ~/.config/nvim && ln -s …`, and this package exists to not be
// that — so a regression there is silent data loss on somebody's machine rather than a failing check.

import (
	"strings"
	"testing"

	"configs/ai/tools/installer"
)

func TestAFreshMachineGetsEveryLink(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a fresh home reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid(label + ": ok")
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
	f.expectLinkTo(f.home+"/.gitconfig", f.repo+"/git/.gitconfig")
	f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	f.expectLinkTo(f.home+"/.config/starship.toml", f.repo+"/starship/starship.toml")

	// A parent the README creates by hand: ~/.config does not exist on a fresh machine, and a link into
	// a missing directory fails rather than creating it.
	t.Run("and a missing parent directory is created", func(t *testing.T) {
		f.expectDir(f.home + "/.config")
	})

	// A machine with nothing mounted yet has nothing for the second-checkout guard to protect, so it
	// must pass straight through. Asserted on the wording rather than on the exit, because a guard that
	// printed nothing when it passes is indistinguishable from one that was never reached — and this
	// one runs on every machine that is already set up.
	t.Run("and the second-checkout guard passes rather than staying silent", func(t *testing.T) {
		f.expectSaid("no mount on this machine comes from another checkout")
	})
}

// Idempotence is the property that makes this safe to run on a working machine, and the only evidence
// for it is a second run over the first run's output.
func TestASecondRunOverAFinishedHomeRelinksNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.mount(installer.RunOptions{})
	run := f.mount(installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a second run reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("  ok       " + f.home + "/.zshrc")
	f.expectNotSaid("linked   " + f.home + "/.zshrc")
}

// A link differing from the computed source only by a trailing slash is the same directory spelled two
// ways. Compared raw it looks stale and gets rewritten on every run — idempotence lost to a cosmetic
// difference, and the noise hides a genuinely stale link. Every machine set up from ai/README.md's old
// skills loop holds links of this shape.
func TestALinkDifferingOnlyByATrailingSlashIsLeftAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.MkdirAll(f.home + "/.config")
	f.Symlink(f.repo+"/nvim/", f.home+"/.config/nvim")

	f.mount(installer.RunOptions{})

	f.expectSaid("  ok       " + f.home + "/.config/nvim")
	f.expectNotSaid("repointed " + f.home + "/.config/nvim")
}

// A symlink carries no data of its own, so repointing one loses nothing — it is the only target a link
// may be written over, and a stale one is what an older layout leaves behind.
func TestAStaleSymlinkIsRepointedRatherThanRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.MkdirAll(f.home + "/.config")
	f.Symlink(f.base+"/somewhere-else", f.home+"/.config/nvim")

	run := f.mount(installer.RunOptions{})

	f.expectRefusals(run, 0)
	f.expectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	f.expectSaid("repointed " + f.home + "/.config/nvim")
}

func TestARealTargetIsRefusedRatherThanDeleted(t *testing.T) {
	t.Parallel()
	// The README's hand-run form is `rm -rf ~/.config/nvim && ln -s …`. Doing that unattended destroys
	// a real config. The body is asserted afterwards, so a version that refused AND deleted would still
	// fail here.
	t.Run("a real directory at a target is refused and its contents survive", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.home + "/.config/nvim")
		f.Write(f.home+"/.config/nvim/init.lua", "my real config")

		run := f.mount(installer.RunOptions{})

		if code := run.Report(); code != 1 {
			t.Errorf("a real directory at a target reported %d, wanted 1", code)
		}
		f.expectSaid("exists and is not a symlink")
		f.expectFileBody(f.home+"/.config/nvim/init.lua", "my real config")
		f.expectNotSymlink(f.home + "/.config/nvim")
	})

	// A real file is the same hazard in the other shape, and takes the other branch of the existence
	// test.
	t.Run("and a real file at a target survives too", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.home + "/.config")
		f.Write(f.home+"/.config/starship.toml", "hand-written prompt")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 1)
		f.expectFileBody(f.home+"/.config/starship.toml", "hand-written prompt")

		// One refusal must not abort the rest: a machine with one stray file should still get every
		// other link, or fixing them becomes one run per problem.
		f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
	})
}

// The vacuity case. Without it, a run over an incomplete checkout reports success having linked
// nothing.
func TestASourceMissingFromTheCheckoutIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.RemoveAll(f.repo + "/nvim")

	run := f.mount(installer.RunOptions{})

	f.expectRefusals(run, 1)
	f.expectSaid("is missing from the repository")
	f.expectAbsent(f.home + "/.config/nvim")
	// And the rest is still linked, for the reason one refusal does not abort the run.
	f.expectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// The flag has to write nothing, or it is worse than not having it: someone checks with a dry run and
// it is the run that changed their machine.
func TestADryRunWritesNothingAtAll(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	run := f.mount(installer.RunOptions{DryRun: true})

	if code := run.Report(); code != 0 {
		t.Errorf("a dry run reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("would link")
	f.expectAbsent(f.home + "/.zshrc")
	f.expectAbsent(f.home + "/.config")
}

// `${target%/*}` on `/.zshrc` is the empty string, not `/`, and computing nothing makes a run refuse
// naming a parent it cannot print — sending the reader after a directory that was never the problem.
// Both spellings refuse, so only the wording tells them apart.
//
// An empty $HOME is the only way a target lands at the filesystem root. The write never happens: the
// run is bounded to the case's own tree, so the guard turns it away and names the parent it resolved,
// which is the whole of what this case is about. Letting it through would leave a machine that can
// write to / holding links at its filesystem root pointing into a temp directory.
func TestARootLevelTargetNamesTheRootAsItsParent(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	run := f.newRun(installer.RunOptions{})
	run.AddConfig(f.repo+"/zsh/.zshrc", "/.zshrc")

	run.Mount()

	breaches := run.Breaches()
	if len(breaches) != 1 {
		t.Fatalf("the run made %d attempt(s) at a root-level target, wanted 1: %v", len(breaches), breaches)
	}
	if !strings.Contains(breaches[0], "resolves to /;") {
		t.Errorf("the parent came back as something other than /: %s", breaches[0])
	}
	f.expectNotSaid("could not create :")
	f.expectAbsent("/.zshrc")
}
