// Cases for the ratchet script that holds each instruction file to a recorded count. The script under
// test and the record it reads are the two paths the const block names.
//
// What must hold is that it refuses in both directions. A file over its line has risen, and a file
// under its line has left slack a later change would spend. A suite proving only that a clean tree
// passes goes green against a script with no comparison in it, so every row drives both.
//
// The subject is what bash did on a tree shaped a particular way, which only a launch can answer.
// Every fixture is built in process: instruction files, a baseline, a stub checker answering from a
// table the case writes. The script is copied in by this process, so the test cache keys on its bytes.
package voicebaseline

import (
	"configs/ai/tools/runtest"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The script under test, and the paths a checkout holds it and everything it reads at. The fixture
// carries the script at its shipped depth, so the offset it walks up to find a root still points at one.
const (
	script          = "../../kk-flavor/skills/kk-ecosystem/scripts/voice-baseline.sh"
	scriptInRoot    = "ai/kk-flavor/skills/kk-ecosystem/scripts/voice-baseline.sh"
	checkerInRoot   = "ai/kk-flavor/skills/kk-edit/scripts/voice-check.sh"
	baselineInRoot  = "ai/kk-flavor/voice-baseline.txt"
	standardsInRoot = "ai/kk-flavor/standards"
)

// The two instruction files most rows need. A name here does one job: it tells the stub's branches
// apart.
const (
	alpha = "alpha.md"
	beta  = "beta.md"
)

// The first line of every fixture baseline, which the file keeps across a regeneration.
const fixtureHeader = "# fixture\n"

// A count that makes the stub exit 2 instead of reporting. The real checker exits 2 where a
// measurement failed, and that is a different fact from a file with no findings.
const refuses = -1

// What the stub checker answers for one instruction file.
type measurement struct {
	name  string
	count int
}

func TestEveryWayATreeStandsAgainstItsBaseline(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// What the stub answers for each of the two instruction files.
		alpha, beta int
		// The lines under the header, so a row can leave a file out of the baseline entirely.
		baseline []string
		code     int
		says     string
	}{
		{
			name:     "every file on its line passes",
			alpha:    3,
			beta:     1,
			baseline: []string{baselineLine(3, alpha), baselineLine(1, beta)},
			code:     0,
			says:     "every one on its line",
		},
		{
			// The change that raised the count is the change that repairs it, so a rise cannot be left
			// for a later change to find.
			name:     "a file over its line is refused",
			alpha:    5,
			beta:     1,
			baseline: []string{baselineLine(3, alpha), baselineLine(1, beta)},
			code:     1,
			says:     "over its baseline of 3",
		},
		{
			// A fall is refused too: the baseline still records slack the tree has given up, and a
			// later change spends it without ever reaching the floor it already stood on.
			name:     "a file under its line is refused",
			alpha:    1,
			beta:     1,
			baseline: []string{baselineLine(3, alpha), baselineLine(1, beta)},
			code:     1,
			says:     "under its baseline of 3",
		},
		{
			name:     "a new file with findings and no baseline line is refused",
			alpha:    2,
			beta:     0,
			baseline: []string{baselineLine(0, beta)},
			code:     1,
			says:     "no baseline line",
		},
		{
			name:     "a new file measuring zero and having no baseline line is allowed",
			alpha:    0,
			beta:     0,
			baseline: []string{baselineLine(0, beta)},
			code:     0,
			says:     "every one on its line",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			root := newRoot(t, measurement{alpha, scenario.alpha}, measurement{beta, scenario.beta})
			writeBaseline(t, root, scenario.baseline...)
			if stood := runOver(t, root); stood.Code != scenario.code || !stood.Said(scenario.says) {
				t.Errorf("wanted exit %d and %q. A comparison answering the same in both directions is "+
					"one that is not being made\n%v", scenario.code, scenario.says, stood)
			}
		})
	}
}

// Exit 2 says the check did not run. A caller may never read that as a pass. Each row names its own
// cause, because the code alone says one of these happened and leaves the cause open.
func TestEveryWayTheRatchetDidNotRunExitsTwoAndNamesIt(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		// The fixture this row breaks, answering with the root to run over.
		arrange func(t *testing.T) string
		says    string
	}{
		{
			name: "a missing baseline file did not run",
			arrange: func(t *testing.T) string {
				return newRoot(t, measurement{alpha, 1}, measurement{beta, 1})
			},
			says: "is missing",
		},
		{
			name: "a checker it cannot execute did not run",
			arrange: func(t *testing.T) string {
				requireExecuteBitDenies(t)
				root := newRoot(t, measurement{alpha, 1}, measurement{beta, 1})
				writeBaseline(t, root, baselineLine(1, alpha), baselineLine(1, beta))
				if err := os.Chmod(filepath.Join(root, checkerInRoot), 0o644); err != nil {
					t.Fatalf("clearing the checker's execute bit: %v — nothing was measured", err)
				}
				return root
			},
			says: "not executable",
		},
		{
			// A run that exits 2 measured no file. Its empty summary otherwise reads as a file with no
			// findings. That is a count under its baseline, which --regenerate then writes in as the new
			// floor. A check that did not run becomes an improvement on the record.
			name: "a checker that refuses to measure a file did not run",
			arrange: func(t *testing.T) string {
				root := newRoot(t, measurement{alpha, refuses}, measurement{beta, 1})
				writeBaseline(t, root, baselineLine(3, alpha), baselineLine(1, beta))
				return root
			},
			says: "did not measure " + standardsInRoot + "/" + alpha,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			refused := runOver(t, scenario.arrange(t))
			if refused.Code != 2 {
				t.Errorf("wanted exit 2 and the refusal %q. Anything else here claims a measurement that "+
					"was never taken\n%v", scenario.says, refused)
				return
			}
			if !refused.Said(scenario.says) {
				t.Errorf("the refusal does not say %q, so a caller cannot tell this cause from the others "+
					"that also exit 2\n%v", scenario.says, refused)
			}
		})
	}
}

// Regeneration belongs in the same change that lowered a count, so what it writes has to be what the
// tree measures now. The tree has to stand against it afterwards, or the floor was written from
// something other than the counts just taken. The file's own header survives it: dropped, the ratchet
// is a bare list of numbers with no line saying what it is for.
func TestRegenerateRewritesTheBaselineFromWhatTheTreeMeasures(t *testing.T) {
	t.Parallel()
	root := newRoot(t, measurement{alpha, 4}, measurement{beta, 2})
	writeBaseline(t, root, baselineLine(9, alpha), baselineLine(9, beta))

	if regenerated := runOver(t, root, "--regenerate"); regenerated.Code != 0 {
		t.Fatalf("--regenerate did not run, so the file read below is still the fixture's own\n%v", regenerated)
	}
	written := runtest.ReadFile(t, filepath.Join(root, baselineInRoot))
	if !strings.Contains(written, baselineLine(4, alpha)+"\n") {
		t.Errorf("alpha.md measures 4 and is not recorded at 4, so the new floor is not the count the "+
			"tree just reported\n%s", written)
	}
	if !strings.HasPrefix(written, fixtureHeader) {
		t.Errorf("the header the fixture wrote is gone, and the ratchet is now a bare list of numbers "+
			"nobody can read\n%s", written)
	}
	if held := runOver(t, root); held.Code != 0 {
		t.Errorf("the tree is off a baseline written from that same tree a moment earlier\n%v", held)
	}
}

// One run reports every file, and the script pairs each line with the file it asked for at that
// position. Every file here reports a count no other file reports. A line read back against the wrong
// file lands on a number that file is not recorded at, and the run turns from a pass into a refusal.

// The second half is the control. A tree standing against its baseline is also what a script that
// paired no file at all would produce. The same tree is run again against a baseline whose counts have
// been moved one file along. That is what a mispairing looks like from outside, and every file has to
// come back off its line.
func TestEachCountIsHeldAgainstTheFileItWasMeasuredFrom(t *testing.T) {
	t.Parallel()
	measured := spread()

	paired := newRoot(t, measured...)
	writeBaseline(t, paired, baselineLines(measured)...)
	if held := runOver(t, paired); held.Code != 0 || !held.Said("every one on its line") {
		t.Errorf("%d files carrying %d different counts did not stand against a baseline recording each "+
			"of them, so a count is reaching a file it was not measured from\n%v",
			len(measured), len(measured), held)
	}

	mispaired := newRoot(t, measured...)
	writeBaseline(t, mispaired, baselineLines(shifted(measured))...)
	offTheirLine := fmt.Sprintf("%d file(s) off their line", len(measured))
	if shown := runOver(t, mispaired); shown.Code != 1 || !shown.Said(offTheirLine) {
		t.Errorf("every file was given its neighbour's count and the run wanted exit 1 with %q. A suite "+
			"that cannot see this cannot see a mispairing either, and the half above passes on nothing"+
			"\n%v", offTheirLine, shown)
	}
}

// A dozen instruction files, each measuring something no other one measures. A count paired with the
// wrong file then lands on a number that file is not recorded at, and the comparison says so.
func spread() []measurement {
	measured := make([]measurement, 12)
	for i := range measured {
		// Zero padded, so that the order `sort` puts the files in is the order the counts were built in.
		measured[i] = measurement{name: fmt.Sprintf("file%03d.md", i), count: i + 1}
	}
	return measured
}

// The same files with every count moved to the file after it. That is what a batch read back out of
// order does to a tree where no two files measure alike.
func shifted(measured []measurement) []measurement {
	moved := make([]measurement, len(measured))
	for i, one := range measured {
		moved[i] = measurement{name: one.name, count: measured[(i+1)%len(measured)].count}
	}
	return moved
}

// A fixture checkout: an instruction file per measurement, a stub checker answering for each of them,
// and the script itself at the depth it ships at. No baseline, because one case is about its absence.
//
// The script is read through this process. `go test`'s cache then keys on the bytes of the thing under
// test, and a cached green is a real run.
// The script takes its root as the last argument, and this suite's cases name the rest. Building the
// command is local for that reason. Running it is runtest's.
func runOver(t *testing.T, root string, arguments ...string) runtest.Run {
	t.Helper()
	command := exec.Command(runtest.Bash(t),
		append([]string{filepath.Join(root, scriptInRoot)}, append(arguments, root)...)...)
	command.Dir = t.TempDir()
	return runtest.Launch(t, command)
}

func newRoot(t *testing.T, measured ...measurement) string {
	t.Helper()
	root := t.TempDir()
	for _, one := range measured {
		runtest.WriteFile(t, filepath.Join(root, standardsInRoot, one.name), "# "+one.name+"\n", 0o644)
	}
	runtest.WriteFile(t, filepath.Join(root, checkerInRoot), newStub(measured), 0o755)
	runtest.WriteFile(t, filepath.Join(root, scriptInRoot), runtest.ReadFile(t, runtest.Runnable(t, script)), 0o755)
	return root
}

// A checker answering from the table its case wrote, and reading no file. What a case measures is the
// comparison, and never the real checker's opinion of a fixture file. It walks its arguments and
// prints a count and a path for each, which is what `--per-file` does.

// A file it refuses exits 2 there and then, leaving the files after it with no line. The real checker
// stops at the path it could not read, and the script has to name that path.

// Each branch matches on the path's last component. A file whose name ends in another's would
// otherwise take that other one's answer.
func newStub(measured []measurement) string {
	stub := strings.Builder{}
	stub.WriteString("#!/usr/bin/env bash\nfor one in \"$@\"; do\n  case \"$one\" in\n")
	for _, one := range measured {
		answer := fmt.Sprintf("echo \"%d $one\"", one.count)
		if one.count == refuses {
			answer = "exit 2"
		}
		stub.WriteString(fmt.Sprintf("    */%s) %s ;;\n", one.name, answer))
	}
	stub.WriteString("  esac\ndone\n")
	return stub.String()
}

func writeBaseline(t *testing.T, root string, lines ...string) {
	t.Helper()
	body := fixtureHeader
	if len(lines) > 0 {
		body += strings.Join(lines, "\n") + "\n"
	}
	runtest.WriteFile(t, filepath.Join(root, baselineInRoot), body, 0o644)
}

func baselineLines(measured []measurement) []string {
	lines := make([]string, len(measured))
	for i, one := range measured {
		lines[i] = baselineLine(one.count, one.name)
	}
	return lines
}

// One recorded line, in the shape the script parses: a count, a space, and the file's path from the root.
func baselineLine(count int, name string) string {
	return fmt.Sprintf("%d %s/%s", count, standardsInRoot, name)
}

// Whether a cleared execute bit really denies this process. Root ignores mode bits, and the behaviour
// under test is the executability check itself. None of the paths that deny every user stands in for
// it, because they are refused a limb earlier and never reach the check. Where the bit does not deny,
// the case is declined by name.
func requireExecuteBitDenies(t *testing.T) {
	t.Helper()
	denied := filepath.Join(t.TempDir(), "denied.sh")
	runtest.WriteFile(t, denied, "#!/bin/sh\nexit 0\n", 0o644)
	if exec.Command(denied).Run() == nil {
		t.Skip("a file with no execute bit still runs as this user, so a cleared bit builds no refusal here")
	}
}
