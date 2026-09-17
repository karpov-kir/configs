package envbootstrap_test

import "testing"

// An option nobody understood must stop the run rather than ride along: this writes into $HOME, so a
// typo carried past the parser is a machine changed by a command its human did not mean.
func TestAnUnknownOptionIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew", "--not-a-flag"), 2)

	f.expectSaid("--not-a-flag")
	f.expectSaid("usage: bootstrap.sh")
	f.expectAbsent(f.home + "/.zshrc")
}

// The stub's own basename, which is the string eco-check's scans anchor on to read a stub's grammar
// and find the dispatch behind it. ai/tools/tool-stub-test.sh holds every stub to it.
func TestHelpPrintsTheUsageLineAndChangesNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--help"), 0)

	f.expectSaid("usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew]")
	f.expectAbsent(f.home + "/.zshrc")
	f.expectAbsent(f.home + "/.config")
}

// The flag has to write nothing, or it is worse than not having it: someone checks with --dry-run and
// it is the run that changed their machine.
func TestADryRunSaysWhatItWouldDoAndWritesNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew", "--dry-run"), 0)

	f.expectSaid("would link")
	f.expectAbsent(f.home + "/.zshrc")
	f.expectAbsent(f.home + "/.config")
}
