// Cases for the pre-commit gate. Three must not be weakened, and each is the gate reporting a pass it
// did not earn:
//
//   - a check that FAILED must exit 1, because a gate whose only signal is its own exit status is
//     read by a hook that reads nothing else;
//   - a check that exited 2 must exit 2 and not 0, because "it did not run" is not "it passed";
//   - a run over the time budget must exit 1, because the budget is the one check that forces every
//     other one to stay fast, and a gate that reports it as a warning has no budget.
//
// No case here runs a real check. The cases about the run loop, the report and the refusals drive the
// gate through its checks-file seam, which reaches all three in milliseconds; the cases about the six
// themselves read the commands `plan` builds and never execute them. Running the real ones means
// running the suite this file is part of.
package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

// A file a check can print back, written into the fixture root, which is where every command runs.
func (f *fixture) file(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, name), []byte(body), 0o644); err != nil {
		f.t.Fatalf("writing %s: %v", name, err)
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

// The real six, as plan builds them for this machine. The cases below read a command out of these and
// never run one.
func (f *fixture) planned(full bool) []check {
	f.t.Helper()
	g := &gate{root: f.root, budget: time.Duration(budgetSeconds) * time.Second, out: &f.out, errOut: &f.errOut}
	checks, code := g.plan(Env{}, full)
	if code != 0 {
		f.t.Fatalf("plan refused on this machine, so these cases held nothing to account: %s", f.errOut.String())
	}
	return checks
}

// One of the real six by id, for the cases that read a command and never run it.
func (f *fixture) plannedCheck(id string) check {
	f.t.Helper()
	for _, c := range f.planned(false) {
		if c.id == id {
			return c
		}
	}
	f.t.Fatalf("the plan builds no %s check, so this case held nothing to account", id)
	return check{}
}

// A PATH holding the named tools and nothing else, so a case can take one binary away from the plan
// without taking the rest. Symlinked rather than copied, and to the real binary this machine resolves,
// so `go` still finds its own toolchain through GOROOT.
func (f *fixture) onlyOnPath(tools ...string) {
	f.t.Helper()
	dir := f.t.TempDir()
	for _, tool := range tools {
		real, err := exec.LookPath(tool)
		if err != nil {
			f.t.Skipf("no %s on this machine, so the case that takes one away cannot be arranged", tool)
		}
		if err := os.Symlink(real, filepath.Join(dir, tool)); err != nil {
			f.t.Fatalf("linking %s into a PATH of this case's own: %v", tool, err)
		}
	}
	f.t.Setenv("PATH", dir)
}

// gofmt missing is a gate that did not run, and no shape of the check itself says so: the listing one
// this started as, `test -z "$(gofmt -l .)"`, came back green, because the shell's complaint goes to
// stderr and the substitution comes back empty, and the pipeline it carries now comes back with a
// format finding against a machine that measured nothing. The pair is the control — the same plan with
// gofmt on PATH builds its checks.
func TestThePlanRefusesWhereGofmtIsMissing(t *testing.T) {
	f := newFixture(t)
	f.onlyOnPath("go")

	g := &gate{root: f.root, out: &f.out, errOut: &f.errOut}
	checks, code := g.plan(Env{}, false)
	if code != 2 {
		t.Errorf("plan answered %d with no gofmt on PATH and built %d check(s) — a missing tool is a gate "+
			"that did not run, never a clean one", code, len(checks))
	}
	f.expectSaid("no gofmt")
}

func TestThePlanBuildsItsChecksWhereGofmtIsThere(t *testing.T) {
	f := newFixture(t)
	f.onlyOnPath("go", "gofmt")

	if checks := f.planned(false); len(checks) == 0 {
		t.Errorf("plan built no check with both tools on PATH")
	}
}

// Two things about the bound every `go test` in the plan carries, and each has cost this repository a
// red gate that was not one. It has to be there at all, or that run hangs for Go's ten-minute default.
// And it has to sit ABOVE the budget, or no gotest check can ever reach the gate's own over-budget
// report: Go kills the package at the budget and prints a goroutine dump, so the run is reported as a
// failure rather than as a suite that has to get faster. Driven at GATE_BUDGET_SECONDS=5, the gotest
// check died that way and the slowest-first report never ran.
func TestEveryGoCommandIsBoundedAboveTheBudget(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(map[bool]string{false: "an ordinary run", true: "--full"}[full], func(t *testing.T) {
			f := newFixture(t)
			checked := 0
			for _, c := range f.planned(full) {
				for _, bound := range goTestBounds(c.cmd) {
					checked++
					switch {
					case bound == 0:
						t.Errorf("a `go test` in the %s check carries no -timeout, so it hangs for Go's "+
							"ten-minute default: %s", c.id, c.cmd)
					case bound <= budgetSeconds:
						t.Errorf("the %s check bounds `go test` at %ds and the budget is %ds, so Go kills the "+
							"suite before the gate's wall clock can report it: %s", c.id, bound, budgetSeconds, c.cmd)
					}
				}
			}
			if checked == 0 {
				t.Fatalf("the plan runs no `go test` at all, so this case held nothing to account and the " +
					"Go suite is not what this gate gates")
			}
		})
	}
}

// One entry per `go test` in a command, holding the seconds its own -timeout names and zero where it
// carries none. Per invocation and not per flag: a command running `go test` twice with a bound on only
// one of them would otherwise be vouched for by the half that carries it.
func goTestBounds(cmd string) []int {
	var bounds []int
	fields := strings.Fields(cmd)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "go" && fields[i+1] == "test" {
			bounds = append(bounds, boundIn(fields[i+2:]))
		}
	}
	return bounds
}

// The seconds one invocation's own flags name, or zero. Reading stops at the shell operator that ends
// the invocation, so a later command's bound cannot stand in for this one's.
func boundIn(flags []string) int {
	for i, field := range flags {
		switch field {
		case "&&", "||", ";", "|":
			return 0
		case "-timeout":
			if i+1 == len(flags) {
				return 0
			}
			seconds, err := strconv.Atoi(strings.TrimSuffix(flags[i+1], "s"))
			if err != nil {
				return 0
			}
			return seconds
		}
	}
	return 0
}

// The format check reads the files git holds, and a walk is the one thing it may not do: this machine
// keeps whole checkouts inside this one, and `gofmt -l .` hands gofmt every .go file in all of them.
// `--others` is half of it — a .go file written and not staged yet is work the gate has to read, and a
// check narrowed to `--cached` passes work it never looked at.
func TestTheFormatCheckReadsTheFilesGitHolds(t *testing.T) {
	f := newFixture(t)
	cmd := f.plannedCheck("gofmt").cmd

	for _, word := range []string{"git ls-files", "--cached", "--others", "--exclude-standard"} {
		if !strings.Contains(cmd, word) {
			t.Errorf("the gofmt check does not name %s, so the files it reads are not the ones this "+
				"repository holds: %s", word, cmd)
		}
	}
	if strings.Contains(cmd, "gofmt -l .") {
		t.Errorf("the gofmt check walks the tree, which on this machine is several checkouts: %s", cmd)
	}
}

// The wiring check is one binary run twice, once per agent, and it builds that binary once. Each run
// carrying ECO_TOOLS_BUILD=1 rebuilt and re-stamped the same source for itself, which is the floor
// under every warm gate. The flag itself stays: without it what gets measured can be a downloaded
// release binary rather than this tree.
func TestTheWiringCheckBuildsItsBinaryOnce(t *testing.T) {
	f := newFixture(t)
	cmd := f.plannedCheck("wiring").cmd

	if builds := strings.Count(cmd, "ECO_TOOLS_BUILD=1"); builds != 1 {
		t.Errorf("the wiring check forces %d build(s) of eco-check, and one is what makes it this "+
			"tree's binary: %s", builds, cmd)
	}
	for _, agent := range []string{"--agent=claude", "--agent=codex"} {
		if !strings.Contains(cmd, agent) {
			t.Errorf("the wiring check does not run %s, so one agent's tree goes unchecked: %s", agent, cmd)
		}
	}
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

// A failure under the successes that follow it, which is what one `go test ./...` prints: an `ok` line
// per package, so a break in an early package is pushed out of any positional tail by the packages
// after it and the report shows forty lines of passes and nothing to act on. What a reader needs is
// the package, the case and the reason, and the report says what it dropped rather than quietly
// cutting it.
func TestAFailureUnderTheSuccessesAfterItIsStillShown(t *testing.T) {
	f := newFixture(t)
	var output strings.Builder
	output.WriteString("ok  \tconfigs/ai/tools/before\t0.11s\n")
	output.WriteString("--- FAIL: TestTheOneThatBroke (0.00s)\n")
	output.WriteString("    broke_test.go:9: the-reason-it-failed\n")
	output.WriteString("FAIL\nFAIL\tconfigs/ai/tools/broke\t0.20s\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&output, "ok  \tconfigs/ai/tools/after%d\t0.01s\n", i)
	}
	f.file("gotest.log", output.String())
	f.table("gotest\tcat gotest.log; exit 1")

	f.run()
	f.expectCode(1)
	f.expectSaid("--- FAIL: TestTheOneThatBroke")
	f.expectSaid("the-reason-it-failed")
	f.expectSaid("configs/ai/tools/broke")
	f.expectSaid("not shown")
	f.expectSilentAbout("configs/ai/tools/after199")
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
		{"an unknown argument is refused with the usage line", []string{"--sideways"}, 2, usageLine},
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
