package projectsetup_test

import (
	"slices"
	"testing"

	"configs/ai/tools/installertest"
	projectsetup "configs/ai/tools/project-setup"
)

// There is no default client. An install that guessed would configure the client the human does not
// use, and an uninstall that guessed would leave the other one mounted. The check sits after the flag
// switch and ahead of every mode, so one run reaches it for all of them.
func TestARunWithoutAnAgentIsRefused(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run(f.project), 2)

	f.ExpectSaid("--agent=claude|codex is required")
	f.ExpectAbsent(f.project + "/.claude")
}

func TestAnUnknownAgentNamesTheTwoThereAre(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.install("--agent=unknown"), 2)

	f.ExpectSaid("codex|claude")
}

func TestNamingNoProjectIsRefused(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--agent=claude"), 2)

	f.ExpectSaid("name the project directory")
}

// A missing project is refused, and no directory is created: this installs into a repository someone
// already has. The refusal names the project as it was typed, and leaves out the flag beside it.
func TestAProjectThatIsNotThereIsNamedInTheRefusal(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--agent=claude", "--dry-run", f.base+"/nowhere"), 2)

	f.ExpectSaid(f.base + "/nowhere is not a directory")
	f.ExpectSaid("nothing was written")
}

func TestAnUnknownOptionIsNamedAndNothingIsWritten(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--agent=claude", f.project, "--not-a-flag"), 2)

	f.ExpectSaid("unknown option --not-a-flag")
	f.ExpectAbsent(f.project + "/.claude")
}

func TestTwoProjectsAtOnceAreRefused(t *testing.T) {
	f := newFixture(t)

	f.ExpectCode(f.run("--agent=claude", f.project, f.base), 2)

	f.ExpectSaid("one project at a time")
}

// Every flag the parser accepts is one the usage line names, and no other. A flag added to the parser
// and left out of the line is one a reader can never find. A flag in the line the parser refuses sends
// a reader to a run that exits 2. The case drives the parser, since a case that scanned the source for
// flag literals would agree with the code however wrong the printed line is.
func TestTheUsageLineNamesEveryFlagTheParserAccepts(t *testing.T) {
	documented := installertest.FlagsIn(projectsetup.Usage())
	if len(documented) == 0 {
		t.Fatal("the usage line names no flag at all, so this case would pass against any parser")
	}
	for _, flag := range documented {
		t.Run(flag, func(t *testing.T) {
			f := newFixture(t)
			if code := f.run("--agent=claude", "--help", flag, f.project); code != 0 {
				t.Errorf("the usage line names %s and the parser exits %d on it, so a reader following the "+
					"documented grammar is refused. It said:\n%s", flag, code, f.Said())
			}
		})
	}
	for _, flag := range acceptedFlags {
		if !slices.Contains(documented, flag) {
			t.Errorf("the parser accepts %s and the usage line does not name it, so nothing a reader can "+
				"see documents it: %q", flag, projectsetup.Usage())
		}
	}
}

// The flags this package's own parser has an arm for. This list is written by hand, and
// TestTheUsageLineNamesEveryFlagTheParserAccepts drives each entry to hold it honest.
var acceptedFlags = []string{
	"--agent=claude", "--agent=codex", "--dry-run", "--relocate", "--maintainer", "--uninstall",
}
