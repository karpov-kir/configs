package installer_test

// The mounts a run wrote whose source has gone. A skill renamed or deleted takes its source
// directory with it, and the mount table stops naming the old target. Link never sees it. The link
// left under the mount directory resolves into a directory no checkout has.

// It is the single deletion this package makes, so every case here is about the signature staying
// narrow. That means what it takes, and the four shapes beside it that it must leave exactly where
// they are.

import (
	"strings"
	"testing"

	"configs/ai/tools/installer"
)

func TestAMountWhoseSourceIsGoneIsDropped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.newSkill("kk-build")
	f.newSkill("kk-was-renamed")
	f.mountSkills([]string{"kk-build", "kk-was-renamed"}, installer.RunOptions{})
	f.expectLinkTo(f.skillsMount()+"/kk-was-renamed", f.repo+"/skills/kk-was-renamed")

	f.RemoveAll(f.repo + "/skills/kk-was-renamed")
	run := f.mountSkills([]string{"kk-build"}, installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("dropping a stale mount reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("removed  " + f.skillsMount() + "/kk-was-renamed")
	f.expectAbsent(f.skillsMount() + "/kk-was-renamed")

	// The control: a skill this checkout still has keeps its mount. Without it, a sweep that took
	// everything would pass every assertion above.
	f.expectLinkTo(f.skillsMount()+"/kk-build", f.repo+"/skills/kk-build")
}

// The summary claims only what it checked, so each of these has to be left alone AND left out of the
// count. A run that removed one of them still prints the same line.
func TestTheSweepLeavesEverythingItCannotProveItWrote(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.newSkill("kk-build")
	f.MkdirAll(f.skillsMount())
	// A relative link, which this package never writes. It resolves against this process's working
	// directory, and the link that holds it goes unconsulted. `skills/kk-relative` would then name some
	// other tree's skills and be swept with the mount beside it.
	f.Symlink("skills/kk-relative", f.skillsMount()+"/kk-relative")
	// A dangling link the human made themselves, pointing nowhere near the source root.
	f.Symlink(f.base+"/a-skill-of-my-own", f.skillsMount()+"/hand-made")
	// A dangling mount from another checkout, which is theirs to sweep.
	f.MkdirAll(f.base + "/another-checkout/skills")
	f.Symlink(f.base+"/another-checkout/skills/kk-gone", f.skillsMount()+"/kk-gone")
	// And a real directory somebody copied in.
	f.MkdirAll(f.skillsMount() + "/copied-in-by-hand")

	run := f.mountSkills([]string{"kk-build"}, installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("a home holding mounts from elsewhere reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSymlink(f.skillsMount() + "/kk-relative")
	f.expectSymlink(f.skillsMount() + "/hand-made")
	f.expectSymlink(f.skillsMount() + "/kk-gone")
	f.expectDir(f.skillsMount() + "/copied-in-by-hand")
	f.expectSaid("ok       every mount under " + f.skillsMount() + " this checkout wrote still resolves")
}

func TestADryRunOverAStaleMountRemovesNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.newSkill("kk-build")
	f.MkdirAll(f.skillsMount())
	f.Symlink(f.repo+"/skills/kk-was-renamed", f.skillsMount()+"/kk-was-renamed")

	run := f.mountSkills([]string{"kk-build"}, installer.RunOptions{DryRun: true})

	if code := run.Report(); code != 0 {
		t.Errorf("a dry run over a stale mount reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("would remove " + f.skillsMount() + "/kk-was-renamed")
	f.expectSymlink(f.skillsMount() + "/kk-was-renamed")
}

// A source root the run cannot read, and one that resolves and holds no source. Both leave every
// mount of this checkout's dangling at once, so an ungated loop reads the machine's whole set as
// deleted and takes it. It then reports an empty mount directory.

// The second is the case the loop can act on. An unreadable root leaves every mount's own source
// unreadable too, so that arm is safe by accident. A root emptied by a half-finished checkout is a
// set of deletions no human made.
func TestASourceRootThatSaysNothingStopsTheSweep(t *testing.T) {
	t.Parallel()
	newHomeWithAStaleMount := func(t *testing.T) *fixture {
		f := newFixture(t)
		f.MkdirAll(f.skillsMount())
		f.Symlink(f.repo+"/skills/kk-was-renamed", f.skillsMount()+"/kk-was-renamed")
		return f
	}

	t.Run("a source root that cannot be read leaves every mount alone", func(t *testing.T) {
		f := newHomeWithAStaleMount(t)

		f.mountSkills(nil, installer.RunOptions{})

		f.expectSymlink(f.skillsMount() + "/kk-was-renamed")
		f.expectSaid(f.repo + "/skills cannot be read, so no mount under " + f.skillsMount() + " was checked")
	})

	t.Run("and one that resolves and holds nothing does too", func(t *testing.T) {
		f := newHomeWithAStaleMount(t)
		f.MkdirAll(f.repo + "/skills")

		f.mountSkills(nil, installer.RunOptions{})

		f.expectSymlink(f.skillsMount() + "/kk-was-renamed")
		f.expectSaid(f.repo + "/skills holds no source, so no mount under " + f.skillsMount() + " was checked")
	})

	// The control, and the load-bearing half: the same root holding one source sweeps the stale mount.
	// Without it, a sweep that never ran at all would pass both cases above.
	t.Run("while the same root holding one source sweeps it", func(t *testing.T) {
		f := newHomeWithAStaleMount(t)
		f.newSkill("kk-still-here")

		f.mountSkills([]string{"kk-still-here"}, installer.RunOptions{})

		f.expectAbsent(f.skillsMount() + "/kk-was-renamed")
	})
}

// A skill directory name is text a branch chose, and the removal message quotes it straight back to
// the terminal. `ESC[2K` erases the line it lands in and `ESC[1A` moves the cursor up one line. A
// name carrying either can wipe the sole record that a deletion happened, or the refusal beside it.

// ESC alone is exercised through a NAME. APFS refuses a filename that is invalid UTF-8, so a raw
// 0x9b cannot be spelled as a directory here. That byte is the CSI an 8-bit terminal acts on, and
// the byte the shell could never reach.

// A raw 0x9b reaches a message through file content instead, in the audience case next door.
func TestAMountWhoseNameCarriesAControlByteStillReportsAsOneLine(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.newSkill("kk-stays")
	const escape = "\x1b"
	gone := "idsd" + escape + "[2Kgone"
	f.newSkill(gone)
	f.mountSkills([]string{"kk-stays", gone}, installer.RunOptions{})

	// The control, and the half that carries the rest. Every assertion that follows is otherwise
	// equally satisfied by a run that never mounted the name, and the case would measure that instead.
	f.expectSymlink(f.skillsMount() + "/" + gone)

	f.RemoveAll(f.repo + "/skills/" + gone)
	run := f.mountSkills([]string{"kk-stays"}, installer.RunOptions{})

	if code := run.Report(); code != 0 {
		t.Errorf("sweeping a mount named with an ESC reported %d, wanted 0: %v", code, run.Refusals())
	}
	f.expectSaid("removed  " + f.skillsMount() + "/idsd")
	f.expectAbsent(f.skillsMount() + "/" + gone)
	if strings.Contains(f.said(), escape) {
		t.Errorf("an ESC out of the mount's own name reached the terminal:\n%q", f.said())
	}
}
