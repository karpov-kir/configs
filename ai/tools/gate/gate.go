// The pre-commit gate: every check this repository gates on, run from cold, every time.
//
//	usage: gate.sh [--full]
//	       (no flag)  run every check, letting Go's own test cache answer where it can
//	       --full     defeat that cache too, which is what the time budget is measured against
//
// It used to be a content-keyed skip machine: 60-odd units, each keyed on a declared set of input
// files, with verdict records in the clone's git dir and a list of paths outside the Go module that
// Go's test cache could not see. That existed because the suite it guarded took about thirty minutes,
// and skipping was the only way to make a pre-commit hook bearable.
//
// The suite does not take thirty minutes any more, so none of it is needed. What replaced it is
// `ai/kk-flavor/standards/testing.md` rule 6: the whole suite runs cold in under 100 seconds, and the
// gate fails a run over that. A cache that exists to hide a slow suite hides a slow suite from the
// one check that would have forced it to be fixed.
//
// Five checks, in this order, because each is cheaper than the one after it and a failure in an
// earlier one makes a later one's output hard to read. They run concurrently all the same — the
// ordering is what gets PRINTED, and the machine has cores to spare while `go test` waits on I/O.
//
// What it may never do:
//   - Report a pass for a check it did not run. There is no cache to answer out of.
//   - Finish over budget and exit 0. A run past budgetSeconds fails and names what took the time.
//   - Skip something quietly. Every run prints one line per check.
package gate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kk-flavor/tools/shell"
)

// Env is what a caller supplies that this cannot work out for itself.
type Env struct {
	// Root is the repository the gate runs over. GATE_ROOT.
	Root string
	// Budget replaces budgetSeconds, so the suite can drive the over-budget refusal without spending
	// a hundred seconds to reach it. GATE_BUDGET_SECONDS.
	Budget int
	// Checks replaces the five real ones with a table read from a file — id, command, one per line,
	// tab-separated. GATE_CHECKS_FILE. It is how the suite reaches the run loop, the report and every
	// refusal in milliseconds rather than by running the real checks, which is the work this exists
	// not to do twice.
	Checks string
}

// The whole suite, cold, on the slowest machine that gates on it. testing.md rule 6 states the number
// and this enforces it; the two have to move together. `go test` is given the same bound, so a suite
// that hangs is reported as the budget rather than as Go's ten-minute default.
const budgetSeconds = 100

// A check the gate runs, and what it cost.
type check struct {
	id  string
	cmd string
	// out is the command's combined output, held back unless it fails: a passing check that printed
	// something is noise in a report read on every commit.
	out    string
	status int
	took   time.Duration
}

type gate struct {
	root        string
	budget      time.Duration
	out, errOut io.Writer
}

// Run executes one invocation and returns its exit code. 0 is a clean gate, 1 is a finding, and 2 is
// "this did not run" — never a result.
func Run(args []string, env Env, out, errOut io.Writer) int {
	g := &gate{out: out, errOut: errOut, budget: time.Duration(budgetSeconds) * time.Second}
	if env.Budget > 0 {
		g.budget = time.Duration(env.Budget) * time.Second
	}
	return g.run(args, env)
}

const usageLine = "usage: gate.sh [--full]"

func refuse(errOut io.Writer, reason string) int {
	fmt.Fprintf(errOut, "gate.sh: %s\n", shell.Oneline(reason))
	return 2
}

func (g *gate) fail(format string, a ...any) int {
	return refuse(g.errOut, fmt.Sprintf(format, a...))
}

func (g *gate) run(args []string, env Env) int {
	started := time.Now()

	full := false
	for _, arg := range args {
		switch arg {
		case "--full":
			full = true
		case "-h", "--help":
			fmt.Fprintln(g.out, usageLine)
			return 0
		default:
			refuse(g.errOut, fmt.Sprintf("unknown argument '%s'", arg))
			return refuse(g.errOut, usageLine)
		}
	}

	if code := g.resolveRoot(env.Root); code != 0 {
		return code
	}

	checks, code := g.plan(env, full)
	if code != 0 {
		return code
	}
	return g.runChecks(checks, started)
}

func (g *gate) resolveRoot(root string) int {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return g.fail("could not resolve the root '%s' — nothing ran", root)
	}
	// Physically, because /var is a symlink to /private/var on macOS and a command that cd's would
	// otherwise be handed a path spelt differently from the one every child reports back.
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return g.fail("could not resolve the root '%s' — nothing ran", root)
	}
	g.root = physical
	return 0
}

// The five, or a table a suite handed over. `go test` carries the budget as its own timeout: a suite
// that hangs then fails as the thing it is, rather than after Go's ten-minute default with a
// goroutine dump that reads like a deadlock.
//
// `--full` is `-count=1`. Go's test cache is keyed on the module and is correct now that nothing here
// reads outside it, so an ordinary run may answer out of it — but the budget is a claim about a COLD
// run, so the run that measures it must not.
func (g *gate) plan(env Env, full bool) ([]check, int) {
	if env.Checks != "" {
		return g.checksFromFile(env.Checks)
	}
	if _, err := exec.LookPath("go"); err != nil {
		return nil, g.fail("no go on this machine, so nothing here can be built or run — nothing ran")
	}
	cold := ""
	if full {
		cold = " -count=1"
	}
	bound := fmt.Sprintf("%ds", int(g.budget.Seconds()))
	return []check{
		{id: "gofmt", cmd: "cd ai/tools && test -z \"$(gofmt -l .)\" || { gofmt -l . >&2; exit 1; }"},
		{id: "vet", cmd: "cd ai/tools && go vet ./..."},
		{id: "gotest", cmd: "cd ai/tools && go test" + cold + " -timeout " + bound + " ./..."},
		{id: "wiring", cmd: "ECO_TOOLS_BUILD=1 ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent=claude --gate && " +
			"ECO_TOOLS_BUILD=1 ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh --agent=codex --gate"},
		{id: "guide", cmd: "ECO_TOOLS_BUILD=1 ai/guide.sh --check"},
	}, 0
}

func (g *gate) checksFromFile(path string) ([]check, int) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, g.fail("GATE_CHECKS_FILE names %s, which is not a file — nothing ran", path)
	}
	var checks []check
	for _, line := range strings.Split(string(body), "\n") {
		id, cmd, found := strings.Cut(line, "\t")
		if !found || id == "" {
			continue
		}
		checks = append(checks, check{id: id, cmd: cmd})
	}
	if len(checks) == 0 {
		return nil, g.fail("GATE_CHECKS_FILE named no check at all — read this as the gate broken, never as a clean run")
	}
	return checks, 0
}

// Every check at once, printed in declared order. The printer blocks on each in turn, so the report
// reads the same whatever order they finish in.
func (g *gate) runChecks(checks []check, started time.Time) int {
	fmt.Fprintf(g.out, "%d check(s)\n\n", len(checks))

	var wait sync.WaitGroup
	for i := range checks {
		wait.Add(1)
		go func(c *check) {
			defer wait.Done()
			at := time.Now()
			c.out, c.status = g.execute(c.cmd)
			c.took = time.Since(at)
		}(&checks[i])
	}
	wait.Wait()

	failed, unmeasured := 0, 0
	for _, c := range checks {
		took := fmt.Sprintf("%ds", int(c.took.Round(time.Second).Seconds()))
		switch c.status {
		case 0:
			g.line("ran ok", c.id, took)
		case 2:
			// "It did not run" — a fixture that could not be built, a tool this machine does not have.
			// Held apart from a failure: calling it one names the code for something the machine did.
			g.line("NO MEASURE", c.id, took+"  it exited 2 — it did not run, so nothing is known")
			g.tail(c.out, 10)
			unmeasured++
		default:
			g.line("FAILED", c.id, took)
			g.tail(c.out, 40)
			failed++
		}
	}
	return g.report(checks, started, failed, unmeasured)
}

func (g *gate) report(checks []check, started time.Time, failed, unmeasured int) int {
	wall := time.Since(started)
	fmt.Fprintf(g.out, "\n%d check(s): %d failed, %d that never measured, %ds wall clock\n",
		len(checks), failed, unmeasured, int(wall.Round(time.Second).Seconds()))

	if failed > 0 {
		return 1
	}
	if unmeasured > 0 {
		fmt.Fprintf(g.errOut, "%d check(s) exited 2 without measuring — nothing is known about them, and this is not a pass.\n", unmeasured)
		return 2
	}
	// The budget is checked last and only over a clean run: a red gate already has a reason, and
	// adding "and it was slow" on top of it buries the reason under the symptom.
	if wall > g.budget {
		g.overBudget(checks, wall)
		return 1
	}
	return 0
}

// What a run over budget says. The wall clock alone tells nobody what to do, so the checks come out
// slowest-first: the whole point of the bound is that it names the thing to fix.
func (g *gate) overBudget(checks []check, wall time.Duration) {
	slowest := append([]check(nil), checks...)
	sort.SliceStable(slowest, func(i, j int) bool { return slowest[i].took > slowest[j].took })
	fmt.Fprintf(g.errOut, "\nthe gate took %ds, and the budget is %ds — this is a failure, not a slow pass.\n",
		int(wall.Round(time.Second).Seconds()), int(g.budget.Seconds()))
	fmt.Fprintln(g.errOut, "`ai/kk-flavor/standards/testing.md` rule 6 states the bound and why no other rule buys time against it.")
	fmt.Fprintln(g.errOut, "slowest first:")
	for _, c := range slowest {
		fmt.Fprintf(g.errOut, "    %-8s %ds\n", c.id, int(c.took.Round(time.Second).Seconds()))
	}
}

// One check's command. Through a shell, because the commands are written as shell and several of them
// cd. Both streams into one buffer: a check that refuses on stderr with an empty stdout would
// otherwise print FAILED and not one word about why.
func (g *gate) execute(cmd string) (string, int) {
	run := exec.Command("sh", "-c", cmd)
	run.Dir = g.root
	out, err := run.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	// 127 is what a shell reports for a command it could not find, and that is what this is: the
	// command did not run, so its exit status is not a verdict about anything.
	return string(out), 127
}

func (g *gate) line(state, id, detail string) {
	fmt.Fprintf(g.out, "  %-11s %-10s %s\n", state, id, detail)
}

func (g *gate) tail(output string, n int) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, line := range lines {
		fmt.Fprintf(g.out, "              %s\n", line)
	}
}
