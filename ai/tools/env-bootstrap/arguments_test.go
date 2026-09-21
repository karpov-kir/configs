package envbootstrap_test

import "testing"

// An unknown option must stop the run. This tool writes into $HOME, so a typo carried past the parser
// changes the machine in a way its human did not mean.
func TestAnUnknownOptionIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--skip-brew", "--not-a-flag"), 2)

	f.ExpectSaid("--not-a-flag")
	f.ExpectSaid("usage: bootstrap.sh")
	f.ExpectAbsent(f.home + "/.zshrc")
}

// The stub's own basename, which is the string eco-check's scans anchor on to read a stub's grammar
// and find the dispatch behind it. ai/tools/stub_usage_test.go holds every stub to it.
func TestHelpPrintsTheUsageLineAndChangesNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--help"), 0)

	f.ExpectSaid("usage: bootstrap.sh [--dry-run] [--relocate] [--skip-brew]")
	f.ExpectAbsent(f.home + "/.zshrc")
	f.ExpectAbsent(f.home + "/.config")
}

// A dry run that wrote anything would be worse than having no flag at all. Someone checks with it, and
// the check is the run that changes their machine.
func TestADryRunSaysWhatItWouldDoAndWritesNothing(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--skip-brew", "--dry-run"), 0)

	f.ExpectSaid("would link")
	f.ExpectAbsent(f.home + "/.zshrc")
	f.ExpectAbsent(f.home + "/.config")
}
