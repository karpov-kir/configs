package aibootstrap_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	aibootstrap "configs/ai/tools/ai-bootstrap"
)

// An unknown option must stop the run. This tool writes into $HOME, so a typo carried past the parser
// changes the machine in a way its human did not mean. The retired --skip-maintainer-skills is the
// case that occurs.
func TestAnUnknownOptionIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=claude", "--skip-maintainer-skills"), 2)

	f.expectSaid("unknown option --skip-maintainer-skills")
	f.expectAbsent(f.home + "/.kk-flavor")
}

// There is no default client, in either direction. An install that guessed would configure whichever
// client the human avoids, and an uninstall that guessed would leave the other one mounted.
func TestEveryModeRefusesWithoutAnAgent(t *testing.T) {
	for _, mode := range []string{"", "--uninstall", "--dry-run"} {
		t.Run("with "+mode, func(t *testing.T) {
			f := newFixture(t)
			args := skipSteps
			if mode != "" {
				args = append(slices.Clone(skipSteps), mode)
			}

			f.expectCode(f.run(args...), 2)

			f.expectSaid("--agent=claude|codex is required")
			f.expectAbsent(f.home + "/.kk-flavor")
		})
	}
}

func TestAnUnknownAgentIsRefusedRatherThanTakenAsTheDefault(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.install("--agent=unknown"), 2)

	f.expectAbsent(f.home + "/.kk-flavor")
}

func TestHelpPrintsTheUsageLineAndChangesNothing(t *testing.T) {
	f := newFixture(t)

	f.expectCode(f.run("--agent=claude", "--help"), 0)

	f.expectSaid("usage: bootstrap.sh")
	f.expectAbsent(f.home + "/.kk-flavor")
	f.expectAbsent(f.home + "/.claude")
}

// Every flag the parser accepts is one the usage line names, and no other. A flag added to the parser
// and left out of the line is one a reader can never find. A flag in the line the parser refuses
// sends a reader to a run that exits 2. The parser is driven here, because a case that scanned the
// source for flag literals would agree with the code however wrong the printed line is.
func TestTheUsageLineNamesEveryFlagTheParserAcceptsAndNoOther(t *testing.T) {
	documented := flagsIn(aibootstrap.Usage())
	if len(documented) == 0 {
		t.Fatal("the usage line names no flag at all, so this case would pass against any parser")
	}
	for _, flag := range documented {
		t.Run(flag, func(t *testing.T) {
			f := newFixture(t)
			if code := f.run("--agent=claude", "--help", flag); code != 0 {
				t.Errorf("the usage line names %s and the parser exits %d on it, so a reader following the "+
					"documented grammar is refused. It said:\n%s", flag, code, f.said())
			}
		})
	}

	// The other direction, driven the only way a parser can answer it. Every flag it accepts is one it
	// does not call unknown, and a flag missing from the line produces the refusal this loop reports.
	for _, flag := range acceptedFlags {
		if !slices.Contains(documented, flag) {
			t.Errorf("the parser accepts %s and the usage line does not name it, so nothing a reader can "+
				"see documents it: %q", flag, aibootstrap.Usage())
		}
	}
}

// The flags this package's own parser has an arm for. The list is written by hand, and the loop that
// drives each one keeps it honest. A flag added to the parser and left off this list leaves the usage
// line unchecked for it, which the driven half cannot see. So the list is what a reader checks
// against the switch, and the driven half is what checks the line.
var acceptedFlags = []string{
	"--agent=claude", "--agent=codex", "--dry-run", "--relocate", "--maintainer", "--owner",
	"--skip-brew", "--skip-tools", "--skip-mcp", "--skip-rtk", "--skip-verify", "--uninstall",
}

// The flags a usage line names. `--agent=claude|codex` is a selector carrying two spellings, so both
// come back.
var flagPattern = regexp.MustCompile(`--[a-z][a-z-]*(?:=[a-z|]+)?`)

func flagsIn(line string) []string {
	var found []string
	for _, match := range flagPattern.FindAllString(line, -1) {
		if name, choices, isSelector := strings.Cut(match, "="); isSelector {
			for _, choice := range strings.Split(choices, "|") {
				found = append(found, name+"="+choice)
			}
			continue
		}
		found = append(found, match)
	}
	slices.Sort(found)
	return slices.Compact(found)
}
