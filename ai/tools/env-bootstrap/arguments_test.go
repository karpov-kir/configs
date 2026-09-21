package envbootstrap_test

import "testing"

// An unknown option must stop the run. This tool writes into $HOME, so a typo carried past the parser
// changes the machine in a way its human did not mean.
func TestAnUnknownOptionIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew", "--not-a-flag"), 2)

	f.expectSaid("--not-a-flag")
	f.expectSaid("usage: bootstrap.sh")
	f.expectAbsent(f.home + "/.zshrc")
}

// The stub's own basename, which is the string eco-check's scans anchor on to read a stub's grammar
// and find the dispatch behind it. ai/tools/stub_usage_test.go holds every stub to it.
func TestHelpPrintsTheUsageLineAndChangesNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--help"), 0)

	f.expectSaid("usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew]")
	f.expectAbsent(f.home + "/.zshrc")
	f.expectAbsent(f.home + "/.config")
}

// A dry run that wrote anything would be worse than having no flag at all. Someone checks with it, and
// the check is the run that changes their machine.
func TestADryRunSaysWhatItWouldDoAndWritesNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--skip-brew", "--dry-run"), 0)

	f.expectSaid("would link")
	f.expectAbsent(f.home + "/.zshrc")
	f.expectAbsent(f.home + "/.config")
}
