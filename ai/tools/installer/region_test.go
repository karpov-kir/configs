package installer_test

// The region a run owns inside a file it does not. What is asserted here is the half no installer's
// own suite can reach cheaply — the states of a target file, and the promise that nothing outside the
// fences moves. A caller's suite covers which fences it passes and what body it writes.

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"kk-flavor/tools/installer"
)

const (
	openFence  = "<!-- kk:begin -->"
	closeFence = "<!-- kk:end -->"
)

// A file with the human's own words in it, and a run pointed at it.
func newRegionFixture(t *testing.T, body string) (*fixture, string) {
	t.Helper()
	f := newBareFixture(t)
	file := f.base + "/CLAUDE.md"
	f.Write(file, body)
	return f, file
}

func TestAnAbsentRegionIsAppendedWithoutDisturbingTheFile(t *testing.T) {
	t.Parallel()
	f, file := newRegionFixture(t, "Their own words.\n")
	run := f.newRun(installer.RunOptions{})

	if !run.WriteRegion(file, openFence, closeFence, "BODY") {
		t.Fatalf("writing an absent region failed: %v", run.Refusals())
	}
	f.expectContained(run)
	f.expectSaid("added")
	f.expectFileBody(file, "Their own words.\n\n"+openFence+"\nBODY\n"+closeFence+"\n")
}

func TestASecondWriteOfTheSameBodyChangesNothing(t *testing.T) {
	t.Parallel()
	f, file := newRegionFixture(t, "Their own words.\n")
	run := f.newRun(installer.RunOptions{})
	run.WriteRegion(file, openFence, closeFence, "BODY")
	before, _ := os.ReadFile(file)

	run.WriteRegion(file, openFence, closeFence, "BODY")

	f.expectSaid("already carries")
	f.expectFileBody(file, string(before))
}

func TestAChangedBodyRewritesOnlyWhatIsBetweenTheFences(t *testing.T) {
	t.Parallel()
	f, file := newRegionFixture(t, "Their own words.\n")
	run := f.newRun(installer.RunOptions{})
	run.WriteRegion(file, openFence, closeFence, "BODY")

	run.WriteRegion(file, openFence, closeFence, "NEWBODY")

	f.expectSaid("rewrote")
	f.expectFileBody(file, "Their own words.\n\n"+openFence+"\nNEWBODY\n"+closeFence+"\n")
}

func TestRemovingTheRegionLeavesTheFileAsItWasFound(t *testing.T) {
	t.Parallel()
	// The blank line the writer added ahead of the region goes with it, so install-then-uninstall does
	// not grow a blank line per cycle.
	t.Run("a one-line file comes back byte for byte", func(t *testing.T) {
		f, file := newRegionFixture(t, "Their own words.\n")
		run := f.newRun(installer.RunOptions{})
		run.WriteRegion(file, openFence, closeFence, "BODY")

		run.RemoveRegion(file, openFence, closeFence)

		f.expectSaid("removed")
		f.expectFileBody(file, "Their own words.\n")
	})

	// The one-line file above cannot see this: removal holds back the blank line it added ahead of the
	// fence, and holding it as the line itself is indistinguishable from holding nothing — so every
	// blank line in a real, paragraphed file goes with the region. The whole file is compared, not
	// grepped for its words: a grep for the paragraphs passes over a file whose paragraph breaks are
	// gone.
	t.Run("and so does a paragraphed one", func(t *testing.T) {
		const paragraphs = "# Project\n\nHow this works.\n\nAnd a second paragraph.\n"
		f, file := newRegionFixture(t, paragraphs)
		run := f.newRun(installer.RunOptions{})
		run.WriteRegion(file, openFence, closeFence, "BODY")

		run.RemoveRegion(file, openFence, closeFence)

		f.expectFileBody(file, paragraphs)
	})

	// Absent is success, not a refusal — an uninstall run twice is a thing people do, and the second
	// run has nothing to say beyond "already gone".
	t.Run("and removing an absent region refuses nothing", func(t *testing.T) {
		f, file := newRegionFixture(t, "Their own words.\n")
		run := f.newRun(installer.RunOptions{})

		run.RemoveRegion(file, openFence, closeFence)

		f.expectSaid("carries no")
		f.expectRefusals(run, 0)
	})
}

// Half a fence means something edited inside the region or truncated the file, and either way the span
// a write would rewrite is no longer the span that was written. Guessing at its extent is how an
// installer eats a paragraph the human wrote.
func TestHalfAFenceRefusesRatherThanGuessing(t *testing.T) {
	t.Parallel()
	const half = "Theirs.\n" + openFence + "\nstray\n"

	t.Run("a write refuses and touches nothing", func(t *testing.T) {
		f, file := newRegionFixture(t, half)
		run := f.newRun(installer.RunOptions{})

		run.WriteRegion(file, openFence, closeFence, "BODY")

		f.expectSaid("one half of")
		f.expectRefusals(run, 1)
		f.expectFileBody(file, half)
	})

	t.Run("and removal refuses on it too", func(t *testing.T) {
		f, file := newRegionFixture(t, half)
		run := f.newRun(installer.RunOptions{})

		run.RemoveRegion(file, openFence, closeFence)

		f.expectSaid("not ours to guess")
		f.expectFileBody(file, half)
	})

	// A second open before the close is the same damage as a missing one: two regions, and no way to
	// say which is this run's.
	t.Run("and so does a second open fence before the close", func(t *testing.T) {
		doubled := openFence + "\nA\n" + openFence + "\nB\n" + closeFence + "\n"
		f, file := newRegionFixture(t, doubled)
		run := f.newRun(installer.RunOptions{})

		run.WriteRegion(file, openFence, closeFence, "BODY")

		f.expectRefusals(run, 1)
		f.expectFileBody(file, doubled)
	})
}

func TestTheStatesOfATargetFileThatRefuseAWrite(t *testing.T) {
	t.Parallel()
	// Writing through a symlink edits a file in a place the caller never named, which for an
	// instruction file symlinked into a checkout means editing the checkout.
	t.Run("a symlinked target refuses and the file behind it is untouched", func(t *testing.T) {
		f, real := newRegionFixture(t, "real\n")
		link := f.base + "/link.md"
		f.Symlink(real, link)
		run := f.newRun(installer.RunOptions{})

		run.WriteRegion(link, openFence, closeFence, "BODY")

		f.expectSaid("is a symlink")
		f.expectFileBody(real, "real\n")
	})

	// A missing file is refused rather than created: this is for regions inside files that already
	// exist, and creating one here would let a typo in a path produce a plausible-looking new file in
	// someone's repository.
	t.Run("and a missing file refuses rather than being created", func(t *testing.T) {
		f, _ := newRegionFixture(t, "unused\n")
		run := f.newRun(installer.RunOptions{})

		run.WriteRegion(f.base+"/nope.md", openFence, closeFence, "BODY")

		f.expectSaid("never creates one")
		f.expectAbsent(f.base + "/nope.md")
	})

	t.Run("and a directory at the path refuses", func(t *testing.T) {
		f, _ := newRegionFixture(t, "unused\n")
		f.MkdirAll(f.base + "/a-directory")
		run := f.newRun(installer.RunOptions{})

		run.WriteRegion(f.base+"/a-directory", openFence, closeFence, "BODY")

		f.expectSaid("is not a regular file")
	})

	// A hardlink is neither a symlink nor a missing file, so nothing above catches it — and the append
	// path copies the file's existing contents into the replacement, which for a link to somebody's
	// private file copies that file into the project. The rename breaks the link so the original is
	// never modified, but the read has already happened.
	t.Run("and a hardlinked target refuses before reading it", func(t *testing.T) {
		f, private := newRegionFixture(t, "secret\n")
		hard := f.base + "/hard.md"
		if err := os.Link(private, hard); err != nil {
			t.Fatalf("the fixture could not hardlink %s: %v", private, err)
		}
		run := f.newRun(installer.RunOptions{})

		run.WriteRegion(hard, openFence, closeFence, "BODY")

		f.expectSaid("hard links")
		f.expectFileBody(hard, "secret\n")
		f.expectFileBody(private, "secret\n")
	})

	// A link count nobody established is the case where writing might share someone's file, so it is
	// the wrong place to assume the safe answer. The shell reached this state routinely, through
	// `stat`'s two incompatible format flags; here it is stubbed, because Go asks the kernel.
	t.Run("and a link count that cannot be read refuses", func(t *testing.T) {
		f, file := newRegionFixture(t, "secret\n")
		run := f.newRun(installer.RunOptions{})
		installer.StubLinkCount(run, func(string) (uint64, error) {
			return 0, errors.New("this filesystem has no link count")
		})

		run.WriteRegion(file, openFence, closeFence, "BODY")

		f.expectSaid("no link count could be read")
		f.expectFileBody(file, "secret\n")
	})
}

// A write that cannot land has to be counted as a refusal, or the run exits 0 reporting ok having
// failed to write. The directory is stripped of write permission while the file inside it stays
// writable, so the file itself passes every guard and the temp file beside it is what fails.
func TestAReplacementThatCannotLandIsCountedAsARefusal(t *testing.T) {
	t.Parallel()
	f, file := newRegionFixture(t, "Theirs.\n\n"+openFence+"\nOLD\n"+closeFence+"\n")
	locked := f.base + "/locked"
	f.MkdirAll(locked)
	moved := locked + "/CLAUDE.md"
	if err := os.Rename(file, moved); err != nil {
		t.Fatalf("the fixture could not move the file into the locked directory: %v", err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatalf("the fixture could not lock %s: %v", locked, err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	// Probed rather than assumed. Root ignores mode bits, and so does CAP_DAC_OVERRIDE without root and
	// a filesystem that drops them — and there a 555 directory builds no refusal at all, leaving the
	// case asserting against a write that happily succeeds.
	if syscall.Access(locked, 0x2) == nil {
		t.Skip("this process can write into a mode-555 directory, so the replacement here cannot be made to fail")
	}
	run := f.newRun(installer.RunOptions{})

	run.WriteRegion(moved, openFence, closeFence, "NEW")

	f.expectRefusals(run, 1)
	f.expectSaid("could not create a temporary file beside")
	f.expectFileBody(moved, "Theirs.\n\n"+openFence+"\nOLD\n"+closeFence+"\n")
}

func TestADryRunOverARegionWritesNothing(t *testing.T) {
	t.Parallel()
	f, file := newRegionFixture(t, "Theirs.\n")
	run := f.newRun(installer.RunOptions{DryRun: true})

	run.WriteRegion(file, openFence, closeFence, "BODY")

	f.expectSaid("would add")
	f.expectFileBody(file, "Theirs.\n")
}
