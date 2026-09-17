package projectsetup_test

import (
	"testing"

	projectsetup "configs/ai/tools/project-setup"
)

// There is no default client, in either direction: an install that guessed would configure the one the
// human does not use, and an uninstall that guessed would leave the other one mounted.
func TestEveryModeRefusesWithoutAnAgent(t *testing.T) {
	for _, mode := range []string{"", "--uninstall", "--dry-run"} {
		t.Run("with "+mode, func(t *testing.T) {
			f := newFixture(t)
			args := []string{f.project}
			if mode != "" {
				args = append(args, mode)
			}

			f.expectCode(f.run(args...), 2)

			f.expectSaid("--agent=claude|codex is required")
			f.expectAbsent(f.project + "/.claude")
		})
	}
}

func TestAnUnknownAgentNamesTheTwoThereAre(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=unknown"), 2)

	f.expectSaid("codex|claude")
}

func TestNamingNoProjectIsRefused(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude"), 2)

	f.expectSaid("name the project directory")
}

// A project that is not there is refused rather than created: this installs into a repository someone
// already has. The refusal names the project as it was typed, not the flag beside it.
func TestAProjectThatIsNotThereIsNamedInTheRefusal(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude", "--dry-run", f.base+"/nowhere"), 2)

	f.expectSaid(f.base + "/nowhere is not a directory")
	f.expectSaid("nothing was written")
}

func TestAnUnknownOptionIsNamedAndNothingIsWritten(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude", f.project, "--not-a-flag"), 2)

	f.expectSaid("unknown option --not-a-flag")
	f.expectAbsent(f.project + "/.claude")
}

func TestTwoProjectsAtOnceAreRefused(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude", f.project, f.base), 2)

	f.expectSaid("one project at a time")
}

// Every flag the parser accepts is one the usage line names, and no other. A flag added to the parser
// and never written into the line is one no reader can find; one in the line the parser refuses sends a
// reader to a run that exits 2.
//
// The parser is driven rather than read: a case that scanned the source for flag literals would agree
// with the code however wrong the printed line is.
func TestTheUsageLineNamesEveryFlagTheParserAccepts(t *testing.T) {
	documented := flagsIn(projectsetup.Usage())
	if len(documented) == 0 {
		t.Fatal("the usage line names no flag at all, so this case would pass against any parser")
	}
	for _, flag := range documented {
		t.Run(flag, func(t *testing.T) {
			f := newFixture(t)
			if code := f.run("--agent=claude", "--help", flag, f.project); code != 0 {
				t.Errorf("the usage line names %s and the parser exits %d on it, so a reader following the "+
					"documented grammar is refused. It said:\n%s", flag, code, f.said())
			}
		})
	}
	for _, flag := range acceptedFlags {
		if !contains(documented, flag) {
			t.Errorf("the parser accepts %s and the usage line does not name it, so nothing a reader can "+
				"see documents it: %q", flag, projectsetup.Usage())
		}
	}
}

// The flags this package's own parser has an arm for. Written here rather than scanned out of the
// source, and held honest by the loop above driving each one.
var acceptedFlags = []string{
	"--agent=claude", "--agent=codex", "--dry-run", "--relocate", "--maintainer", "--uninstall",
}
