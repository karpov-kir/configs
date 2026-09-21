package installer_test

// Linking, and the two rules that must hold. A real file at a target is refused, and it survives. A
// machine mounted from another checkout is left alone where it is.

// The hand-run form of the first rule is `rm -rf ~/.config/nvim && ln -s …`, and this package exists
// to replace that. A regression there is silent data loss on somebody's machine, and the check here
// is what turns it into a failure instead.

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
	f.ExpectSaid(label + ": ok")
	f.ExpectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
	f.ExpectLinkTo(f.home+"/.gitconfig", f.repo+"/git/.gitconfig")
	f.ExpectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	f.ExpectLinkTo(f.home+"/.config/starship.toml", f.repo+"/starship/starship.toml")

	// A parent the README creates by hand. ~/.config is absent on a fresh machine, and a link into a
	// missing directory fails where it would have to create one.
	t.Run("and a missing parent directory is created", func(t *testing.T) {
		f.ExpectDir(f.home + "/.config")
	})

	// A machine with no mount yet leaves the second-checkout guard an empty table, so it must pass
	// straight through. The assertion reads the wording, because a silent guard is
	// indistinguishable from one that was never reached. This guard runs on every machine already set
	// up.
	t.Run("and the second-checkout guard passes rather than staying silent", func(t *testing.T) {
		f.ExpectSaid("no mount on this machine comes from another checkout")
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
	f.ExpectSaid("  ok       " + f.home + "/.zshrc")
	f.ExpectNotSaid("linked   " + f.home + "/.zshrc")
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

	f.ExpectSaid("  ok       " + f.home + "/.config/nvim")
	f.ExpectNotSaid("repointed " + f.home + "/.config/nvim")
}

// A symlink carries no data of its own, and a repoint therefore loses none. It is the single target
// a link may be written over, and a stale one is what an older layout leaves behind.
func TestAStaleSymlinkIsRepointedRatherThanRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.MkdirAll(f.home + "/.config")
	f.Symlink(f.base+"/somewhere-else", f.home+"/.config/nvim")

	run := f.mount(installer.RunOptions{})

	f.expectRefusals(run, 0)
	f.ExpectLinkTo(f.home+"/.config/nvim", f.repo+"/nvim")
	f.ExpectSaid("repointed " + f.home + "/.config/nvim")
}

func TestARealTargetIsRefusedRatherThanDeleted(t *testing.T) {
	t.Parallel()
	// The README's hand-run form is `rm -rf ~/.config/nvim && ln -s …`, and unattended it destroys a
	// real config. The body is asserted afterwards, and a version that refused AND deleted still fails
	// here.
	t.Run("a real directory at a target is refused and its contents survive", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.home + "/.config/nvim")
		f.Write(f.home+"/.config/nvim/init.lua", "my real config")

		run := f.mount(installer.RunOptions{})

		if code := run.Report(); code != 1 {
			t.Errorf("a real directory at a target reported %d, wanted 1", code)
		}
		f.ExpectSaid("exists and is not a symlink")
		f.ExpectFileBody(f.home+"/.config/nvim/init.lua", "my real config")
		f.ExpectNotSymlink(f.home + "/.config/nvim")
	})

	// A real file is the same hazard in the other shape, and takes the other branch of the existence
	// test.
	t.Run("and a real file at a target survives too", func(t *testing.T) {
		f := newFixture(t)
		f.MkdirAll(f.home + "/.config")
		f.Write(f.home+"/.config/starship.toml", "hand-written prompt")

		run := f.mount(installer.RunOptions{})

		f.expectRefusals(run, 1)
		f.ExpectFileBody(f.home+"/.config/starship.toml", "hand-written prompt")

		// One refusal must not abort the rest: a machine with one stray file should still get every
		// other link, or fixing them becomes one run per problem.
		f.ExpectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
	})
}

// The vacuity case. It is what stops a run over an incomplete checkout from reporting success with
// no link made.
func TestASourceMissingFromTheCheckoutIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.RemoveAll(f.repo + "/nvim")

	run := f.mount(installer.RunOptions{})

	f.expectRefusals(run, 1)
	f.ExpectSaid("is missing from the repository")
	f.ExpectAbsent(f.home + "/.config/nvim")
	// And the rest is still linked, for the reason one refusal does not abort the run.
	f.ExpectLinkTo(f.home+"/.zshrc", f.repo+"/zsh/.zshrc")
}

// The flag has to leave the disk alone, or it is worse than absent. Someone checks with a dry run,
// and the dry run is what changed their machine.
func TestADryRunWritesNothingAtAll(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	run := f.mount(installer.RunOptions{DryRun: true})

	if code := run.Report(); code != 0 {
		t.Errorf("a dry run reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.ExpectSaid("would link")
	f.ExpectAbsent(f.home + "/.zshrc")
	f.ExpectAbsent(f.home + "/.config")
}

// `${target%/*}` on `/.zshrc` gives the empty string where `/` is wanted. A run that computed an
// empty parent refuses while naming a parent it cannot print, sending the reader after a directory
// that was never the problem. Both spellings refuse, so the wording is what tells them apart.

// An empty $HOME is the single way a target lands at the filesystem root. The write never happens,
// because the run is bounded to the case's own tree. The guard turns it away and names the parent it
// resolved, which is the whole of what this case is about.

// A write let through here would leave a machine that can write to / holding links at its filesystem
// root pointing into a temp directory.
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
	f.ExpectNotSaid("could not create :")
	f.ExpectAbsent("/.zshrc")
}
