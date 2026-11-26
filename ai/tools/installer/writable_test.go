package installer_test

// The states of an existing file that make FileWritable refuse, and the one it accepts.

import (
	"errors"
	"testing"

	"configs/ai/tools/installer"
)

// A file with the human's own words in it, and a run pointed at it.
func newWritableFixture(t *testing.T, body string) (*fixture, string) {
	t.Helper()
	f := newBareFixture(t)
	file := f.base + "/RTK.md"
	f.Write(file, body)
	return f, file
}

func TestTheStatesOfATargetFileThatRefuseAWrite(t *testing.T) {
	t.Parallel()
	// The control: a plain file passes, so each refusal below is the state's and not the guard's.
	t.Run("a plain regular file is writable", func(t *testing.T) {
		f, file := newWritableFixture(t, "real\n")
		run := f.newRun(installer.RunOptions{})

		if !run.FileWritable(file) {
			t.Errorf("a plain file was refused:\n%s", f.Said())
		}
		f.expectRefusals(run, 0)
	})

	t.Run("a symlinked target refuses", func(t *testing.T) {
		f, real := newWritableFixture(t, "real\n")
		link := f.base + "/link.md"
		f.Symlink(real, link)
		run := f.newRun(installer.RunOptions{})

		if run.FileWritable(link) {
			t.Error("a symlink was accepted")
		}
		f.ExpectSaid("is a symlink")
	})

	t.Run("and a missing file refuses", func(t *testing.T) {
		f, _ := newWritableFixture(t, "unused\n")
		run := f.newRun(installer.RunOptions{})

		if run.FileWritable(f.base + "/nope.md") {
			t.Error("a missing file was accepted")
		}
		f.ExpectSaid("never creates one")
	})

	t.Run("and a directory at the path refuses", func(t *testing.T) {
		f, _ := newWritableFixture(t, "unused\n")
		f.MkdirAll(f.base + "/a-directory")
		run := f.newRun(installer.RunOptions{})

		if run.FileWritable(f.base + "/a-directory") {
			t.Error("a directory was accepted")
		}
		f.ExpectSaid("is not a regular file")
	})

	// A hardlink passes the symlink check and the existence check alike, so no earlier guard catches it.
	t.Run("and a hardlinked target refuses", func(t *testing.T) {
		f, private := newWritableFixture(t, "secret\n")
		hard := f.base + "/hard.md"
		f.Hardlink(private, hard)
		run := f.newRun(installer.RunOptions{})

		if run.FileWritable(hard) {
			t.Error("a hardlinked file was accepted")
		}
		f.ExpectSaid("hard links")
	})

	// An unestablished link count is the case where writing might share someone's file. Go asks the
	// kernel, so the failure is stubbed.
	t.Run("and a link count that cannot be read refuses", func(t *testing.T) {
		f, file := newWritableFixture(t, "secret\n")
		run := f.newRun(installer.RunOptions{})
		installer.StubLinkCount(run, func(string) (uint64, error) {
			return 0, errors.New("this filesystem has no link count")
		})

		if run.FileWritable(file) {
			t.Error("an unreadable link count was accepted")
		}
		f.ExpectSaid("no link count could be read")
	})
}
