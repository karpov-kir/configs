// Cases for the pre-commit gate. Three must not be weakened, and each is the gate reporting a pass it
// did not earn:
//
//   - a check that FAILED must exit 1, because a gate whose only signal is its own exit status is
//     read by a hook that reads nothing else;
//   - a check that exited 2 must exit 2 and not 0, because "it did not run" is not "it passed";
//   - a run over the time budget must exit 1, because the budget is the one check that forces every
//     other one to stay fast, and a gate that reports it as a warning has no budget.
//
// Every case drives the gate through its checks-file seam rather than the real five: that reaches the
// run loop, the report and every refusal in milliseconds, where running the real checks means running
// the suite this file is part of.
package gate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type fixture struct {
	t      *testing.T
	root   string
	checks string
	// budget is zero for every case but the three about the bound, where spending a hundred seconds to
	// reach a refusal is not a thing a suite may do.
	budget int
	out    strings.Builder
	errOut strings.Builder
	code   int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	return &fixture{t: t, root: base, checks: filepath.Join(base, "checks")}
}

// The table the run loop reads. Tab-separated, id then command, exactly as GATE_CHECKS_FILE takes it.
func (f *fixture) table(lines ...string) {
	f.t.Helper()
	if err := os.WriteFile(f.checks, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		f.t.Fatalf("writing the checks table: %v", err)
	}
}

func (f *fixture) run(args ...string) {
	f.t.Helper()
	f.out.Reset()
	f.errOut.Reset()
	f.code = Run(args, Env{Root: f.root, Checks: f.checks, Budget: f.budget}, &f.out, &f.errOut)
}

func (f *fixture) expectCode(want int) {
	f.t.Helper()
	if f.code != want {
		f.t.Errorf("exited %d, wanted %d\nstdout:\n%s\nstderr:\n%s", f.code, want, f.out.String(), f.errOut.String())
	}
}

func (f *fixture) expectSaid(want string) {
	f.t.Helper()
	if !strings.Contains(f.out.String()+f.errOut.String(), want) {
		f.t.Errorf("nothing said %q\nstdout:\n%s\nstderr:\n%s", want, f.out.String(), f.errOut.String())
	}
}

func (f *fixture) expectSilentAbout(what string) {
	f.t.Helper()
	if strings.Contains(f.out.String()+f.errOut.String(), what) {
		f.t.Errorf("the run said %q, and nothing here should have\nstdout:\n%s\nstderr:\n%s", what, f.out.String(), f.errOut.String())
	}
}

// A command that records it ran, so a case can tell a check that was executed from one that was
// merely reported. Written into the fixture root, which is where every command runs.
func marker(name string, status int) string {
	return "printf x >> " + name + "; exit " + strconv.Itoa(status)
}

func (f *fixture) runCount(name string) int {
	f.t.Helper()
	body, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		return 0
	}
	return len(body)
}

func TestACleanRunExitsZeroAndRunsEveryCheck(t *testing.T) {
	f := newFixture(t)
	f.table("one\t"+marker("one.log", 0), "two\t"+marker("two.log", 0))

	f.run()
	f.expectCode(0)
	f.expectSaid("ran ok")
	if got := f.runCount("one.log") + f.runCount("two.log"); got != 2 {
		t.Errorf("%d of the 2 checks ran", got)
	}
}

// A failing check fails the gate, and its output reaches the report. Held back on a pass, because a
// report read on every commit that carries every passing command's chatter stops being read.
func TestAFailingCheckExitsOneAndShowsItsOutput(t *testing.T) {
	f := newFixture(t)
	f.table(
		"clean\techo nothing-to-see; exit 0",
		"broken\techo the-reason-it-failed; exit 1",
	)

	f.run()
	f.expectCode(1)
	f.expectSaid("FAILED")
	f.expectSaid("the-reason-it-failed")
	f.expectSilentAbout("nothing-to-see")
}

// Exit 2 is "it did not run", which a caller may never read as a pass. The gate's own exit status is
// the only thing a hook looks at.
func TestACheckThatDidNotRunExitsTwo(t *testing.T) {
	f := newFixture(t)
	f.table("clean\texit 0", "absent\techo the-tool-is-missing; exit 2")

	f.run()
	f.expectCode(2)
	f.expectSaid("NO MEASURE")
	f.expectSaid("the-tool-is-missing")
}

// A finding outranks a machine that could not answer: exit 2 alone means nothing was found wrong and
// something never ran, and a caller that read the pair as 2 would go looking at its own toolchain for
// a broken test.
func TestAFailureOutranksACheckThatDidNotRun(t *testing.T) {
	f := newFixture(t)
	f.table("absent\texit 2", "broken\texit 1")

	f.run()
	f.expectCode(1)
}

// The budget, which is the point of the whole file. testing.md rule 6 says the suite runs cold under
// 100 seconds and the gate fails a run over that; without this case that sentence is a wish.
func TestARunOverTheBudgetFailsAndNamesTheSlowest(t *testing.T) {
	f := newFixture(t)
	f.budget = 1
	f.table("quick\texit 0", "slow\tsleep 2")

	f.run()
	f.expectCode(1)
	f.expectSaid("the budget is 1s")
	// Named, not merely counted. A wall clock on its own tells nobody what to fix, and the bound
	// exists to point at the thing that has to get faster.
	f.expectSaid("slowest first:")
	if said := f.errOut.String(); strings.Index(said, "slow") > strings.Index(said, "quick") {
		t.Errorf("the over-budget report lists the quick check before the slow one:\n%s", said)
	}
}

// Over budget AND red is reported as red. A gate that answered "and it was slow" would bury the
// reason under the symptom, and the reason is the thing someone has to fix first.
func TestAFailingRunOverBudgetStillReportsTheFailure(t *testing.T) {
	f := newFixture(t)
	f.budget = 1
	f.table("broken\techo the-reason-it-failed; exit 1", "slow\tsleep 2")

	f.run()
	f.expectCode(1)
	f.expectSaid("the-reason-it-failed")
	f.expectSilentAbout("slowest first:")
}

// A clean run inside the budget says nothing about the budget at all.
func TestACleanRunInsideTheBudgetSaysNothingAboutIt(t *testing.T) {
	f := newFixture(t)
	f.budget = 60
	f.table("quick\texit 0")

	f.run()
	f.expectCode(0)
	f.expectSilentAbout("budget")
}

// A table naming nothing is the gate broken, never a clean sweep. It is the same refusal discovery
// used to make when it found no suites: a gate that narrowed itself to nothing looks exactly like one
// that found nothing wrong.
func TestAnEmptyTableRefuses(t *testing.T) {
	f := newFixture(t)
	f.table("")

	f.run()
	f.expectCode(2)
	f.expectSaid("named no check at all")
}

func TestTheArgumentTable(t *testing.T) {
	for _, c := range []struct {
		what   string
		args   []string
		status int
		said   string
	}{
		{"no argument runs the gate", nil, 0, "ran ok"},
		{"--full runs it too", []string{"--full"}, 0, "ran ok"},
		{"--help prints the usage line and runs nothing", []string{"--help"}, 0, usageLine},
		{"an unknown argument is refused with the usage line", []string{"--mutants"}, 2, usageLine},
	} {
		t.Run(c.what, func(t *testing.T) {
			f := newFixture(t)
			f.table("one\t" + marker("one.log", 0))
			f.run(c.args...)
			f.expectCode(c.status)
			f.expectSaid(c.said)
			if ran := f.runCount("one.log"); c.args != nil && len(c.args) > 0 && c.args[0] != "--full" && ran != 0 {
				t.Errorf("%s ran %d check(s), and it answers a question about the flags", c.what, ran)
			}
		})
	}
}

// Every command runs at the repository root, whatever directory the caller stood in. Half the real
// checks cd from there, and a root taken from the process's own cwd would scope them to a
// subdirectory and report a pass over the rest of the tree.
func TestEveryCommandRunsAtTheRoot(t *testing.T) {
	f := newFixture(t)
	f.table("where\tpwd > where.log")

	f.run()
	f.expectCode(0)
	body, err := os.ReadFile(filepath.Join(f.root, "where.log"))
	if err != nil {
		t.Fatalf("the check wrote no record of where it ran: %v", err)
	}
	said := strings.TrimSpace(string(body))
	physical, err := filepath.EvalSymlinks(f.root)
	if err != nil {
		t.Fatalf("resolving the fixture root: %v", err)
	}
	if said != physical {
		t.Errorf("the check ran in %s, and the root is %s", said, physical)
	}
}
