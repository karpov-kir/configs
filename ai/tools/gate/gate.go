// The pre-commit gate: every check this repository gates on, run from cold, every time.
//
//	usage: gate.sh [--full]
//	       (no flag)  every check, with Go's own test cache answering what it can.
//	       --full     defeats that cache, and the time budget is measured on that run.

// It used to be a content-keyed skip machine: 60-odd units, each keyed on a declared set of input
// files. Verdict records went in the clone's git dir, with a list of paths outside the Go module
// that Go's test cache could not see. The suite it guarded took about thirty minutes, and skipping
// was the only way to make a pre-commit hook bearable.

// The suite is no longer that slow, so all of that went. `ai/kk-flavor/standards/testing.md` sets
// what replaced it: the whole suite runs cold in under 100 seconds, and the gate fails a run over
// that. A cache that hides a slow suite hides it from the check that would have forced the fix.

// Six checks. They print in this order because each costs less than the check after it, and a
// failure in an earlier one makes a later one's output hard to read. They run concurrently all the
// same: the order is what gets printed, and the machine has cores to spare while `go test` waits
// on I/O.

// A pass is only ever reported for a check this run executed, because there is no cache to answer
// out of. A run past budgetSeconds, the hundred-second bound, fails and names what took the time, so
// going over budget can never end in exit 0. Every run prints one line per check, so no check is
// skipped quietly.
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

	"configs/ai/tools/shell"
)

// Env is what a caller supplies that this cannot work out for itself.
type Env struct {
	// Root is the repository the gate runs over. GATE_ROOT.
	Root string
	// Budget replaces budgetSeconds, so the suite can drive the refusal a slow run gets without
	// spending a hundred seconds to reach it. GATE_BUDGET_SECONDS.
	Budget int
	// Checks replaces the six real ones with a table read from a file: id, command, one per line,
	// tab-separated. GATE_CHECKS_FILE. The suite uses it to reach the run loop, the report and every
	// refusal in milliseconds, so it never pays for the real checks a second time.
	Checks string
}

// The whole suite, cold, on the slowest machine that gates on it. `ai/kk-flavor/standards/testing.md`
// states the same number, so the two have to move together. This gate's own wall clock is the only
// thing enforcing it, and suiteTimeoutSeconds, the backstop const, says why `go test` must not be
// handed this number.
const budgetSeconds = 100

// What `go test` carries as its own -timeout, above budgetSeconds, the gate's own bound, on purpose.
// Handed the budget itself, Go killed the package first and printed a goroutine dump. The
// slowest-first report then never ran for the check that would earn it, and that report is what names
// the thing to speed up.

// Above the budget, a merely slow suite finishes and gets reported as slow. The number is left as
// the backstop against a genuine hang, and Go's ten-minute default is what it escapes.

// Both workflows spell this number into their own `go test`, and ai/tools/workflows_test.go holds
// them to it.
const suiteTimeoutSeconds = 300

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

// Run executes one invocation and returns its exit code. 0 is a clean gate, 1 is a finding, and 2
// says the gate did not run. Exit 2 is a state of the machine, and a reader must never read it as a
// result.
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
	// Physically, because /var is a symlink to /private/var on macOS. A command that cd's would
	// otherwise be handed a path spelt differently from what every child reports back.
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return g.fail("could not resolve the root '%s' — nothing ran", root)
	}
	g.root = physical
	return 0
}

// `--full` is `-count=1` over everything. The budget is a claim about a cold run, so the run that
// measures it must not answer out of Go's cache at all.

// An ordinary run lets that cache answer, and no check is forced past it. Go keys the test cache on
// the module and hashes every file a case opens inside that root, skipping only what lies outside.
// The module file sits at the repository root, so every file a case reads lies inside the module.

// Break `ai/kk-flavor/standards/records.md` or `.github/workflows/gates.yml` and the package reading
// it goes red on the next plain `go test`. Both ways were measured before and after the module
// moved. `ai/kk-flavor/standards/testing.md` asks for exactly this: a suite reads only what its
// runner's cache keys on.

// `go test ./...` is one check, not two. It was split while the root package was forced with
// `-count=1`, and `./...` then paid for that package a second time out of the cache. With no package
// forced there is one run and one duration for the slowest-first report to name.

// The six, or a table a suite handed over.
func (g *gate) plan(env Env, full bool) ([]check, int) {
	if env.Checks != "" {
		return g.checksFromFile(env.Checks)
	}
	// gofmt as well as go, because a check that cannot find its binary is no check at all. A machine
	// with no gofmt has measured none of the tree, and both shapes this check has carried report that
	// as something else. The listing shape, `test -z "$(gofmt -l .)"`, reports a clean tree, and
	// formatCmd, this file's gofmt command, reports a format finding.
	for _, tool := range []string{"go", "gofmt"} {
		if _, err := exec.LookPath(tool); err != nil {
			return nil, g.fail("no %s on this machine, so the Go checks cannot run — nothing ran", tool)
		}
	}
	bound := fmt.Sprintf("-timeout %ds", suiteTimeoutSeconds)
	suite := "go test " + bound + " ./..."
	if full {
		suite = "go test -count=1 " + bound + " ./..."
	}
	return []check{
		{id: "gofmt", cmd: formatCmd},
		{id: "vet", cmd: "go vet ./..."},
		{id: "gotest", cmd: suite},
		{id: "wiring", cmd: wiringCmd},
		{id: "guide", cmd: "ECO_TOOLS_BUILD=1 ai/guide.sh --check"},
		// The instruction tree's baseline, which is a ratchet and only goes down. It had one reader
		// before it had a job: a sentence in a skill telling an agent to look at it, which is a
		// document and never a gate.
		{id: "baseline", cmd: "ai/kk-flavor/skills/kk-ecosystem/scripts/voice-baseline.sh"},
	}, 0
}

// This machine keeps whole checkouts inside this one, and a walk of the tree hands gofmt every .go
// file in all of them.

// `--others` as much as `--cached`, because a .go file that is written but unstaged is work the gate
// has to read. git also skips a nested repository, so those are dropped by git and stay dropped when
// someone deletes an ignore rule.

// The check reads gofmt's exit status as well as its listing. Handed a file it cannot parse, gofmt
// prints to stderr and lists no file, and a check reading the listing alone calls that formatted.

// xargs ends the pipeline and reports a non-zero gofmt as 123, so the substitution carries that
// status without pipefail. `sh` on a Linux runner has no pipefail.

// A tree with no .go file in it produces an empty listing, and the xargs that follows still cannot
// fail. macOS xargs runs the command zero times, and GNU xargs runs gofmt against /dev/null with no
// file, which gofmt reports as formatted. Both were measured, because the macOS xargs carries no
// `-r` flag to say this in the command.

// Lists every .go file git holds, and reports the unformatted ones.
const formatCmd = "unformatted=$(git ls-files --cached --others --exclude-standard -z -- '*.go' | xargs -0 gofmt -l) && " +
	"test -z \"$unformatted\" || { printf '%s\\n' \"$unformatted\" >&2; exit 1; }"

// Each run used to carry ECO_TOOLS_BUILD=1 and rebuild and re-stamp the same binary for itself,
// which was the floor under a warm gate.

// The flag stays where the build is. Without it the binary measured can be a downloaded release,
// when the whole point is to measure this tree. The runs after the build reach those bytes through
// resolve.sh, which rebuilds any binary with a stamp that no longer matches the source beside it.

// Each run's output goes to a file of its own, because two reports interleaved line by line name
// neither agent. Both statuses are read, claude's first where both are non-zero, matching what the
// `&&` that used to chain them reported.

// One build, then both agents against those bytes at once.
const wiringCmd = `
ECO_TOOLS_BUILD=1 ai/tools/resolve.sh eco-check >/dev/null || exit 2
work=$(mktemp -d "${TMPDIR:-/tmp}/gate-wiring.XXXXXX") || exit 2
check=ai/kk-flavor/skills/kk-ecosystem/scripts/check.sh
"$check" --agent=claude --gate >"$work/claude" 2>&1 &
claude=$!
"$check" --agent=codex --gate >"$work/codex" 2>&1 &
codex=$!
wait $claude; claude_status=$?
wait $codex; codex_status=$?
cat "$work/claude" "$work/codex"
rm -rf "$work"
[ $claude_status -eq 0 ] || exit $claude_status
exit $codex_status
`

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

// Runs every check at once and prints them in declared order. The printer blocks on each in turn, so
// the report reads the same whatever order they finish in.
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
			// Exit 2 means the check did not run: a fixture that failed to build, a tool this machine
			// lacks. The gate keeps this apart from a failure, because calling it a failure blames the
			// code for something the machine did.
			g.line("NO MEASURE", c.id, took+"  it exited 2 — it did not run, so nothing is known")
			g.tail(c.out, 10)
			unmeasured++
		default:
			g.line("FAILED", c.id, took)
			g.failure(c)
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
	// The budget is checked last, and only over a clean run. A red gate already has a reason, and
	// adding "and it was slow" on top of it buries the reason under the symptom.
	if wall > g.budget {
		g.overBudget(checks, wall)
		return 1
	}
	return 0
}

// What a run over budget says. The wall clock alone does not say what to fix, so the checks come out
// slowest-first. Naming the thing to speed up is the whole point of the bound.
func (g *gate) overBudget(checks []check, wall time.Duration) {
	slowest := append([]check(nil), checks...)
	sort.SliceStable(slowest, func(i, j int) bool { return slowest[i].took > slowest[j].took })
	fmt.Fprintf(g.errOut, "\nthe gate took %ds, and the budget is %ds — this is a failure, not a slow pass.\n",
		int(wall.Round(time.Second).Seconds()), int(g.budget.Seconds()))
	fmt.Fprintln(g.errOut, "`ai/kk-flavor/standards/testing.md` rule 6 states the bound and why no other rule buys time against it.")
	fmt.Fprintln(g.errOut, "slowest first:")
	for _, c := range slowest {
		fmt.Fprintf(g.errOut, "    %-12s %ds\n", c.id, int(c.took.Round(time.Second).Seconds()))
	}
}

// One check's command. Through a shell, because the commands are written as shell and several of them
// cd. Both streams go into one buffer: a check that refuses on stderr with an empty stdout would
// otherwise print FAILED and stay silent about why.
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
	// 127 is what a shell reports for a command it could not find, and that is what this is. The
	// command never ran. Its exit status is therefore no verdict about anything.
	return string(out), 127
}

func (g *gate) line(state, id, detail string) {
	fmt.Fprintf(g.out, "  %-11s %-12s %s\n", state, id, detail)
}

func (g *gate) tail(output string, n int) {
	lines := outputLines(output)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, line := range lines {
		g.quote(line)
	}
}

// The most lines one failed check prints. A run that panics in every package carries thousands, and a
// report read on every commit that prints all of them goes unread.
const failureLineBudget = 40

// What a FAILED check shows. The last n lines will not do: `go test ./...` prints an `ok` line per
// package after the package that broke. A positional tail of a failure in an early package is forty
// lines of successes, and carries no word about what a reader has to fix.
func (g *gate) failure(c check) {
	all := outputLines(c.out)
	if len(all) == 0 {
		return
	}
	shown := failureLines(all)
	switch {
	case len(shown) == 0:
		// This output has no `go test` failure in it: gofmt's listing, a wiring finding. Those print
		// what they found and stop, so the end of the output is the report.
		shown = all
		if len(shown) > failureLineBudget {
			shown = shown[len(shown)-failureLineBudget:]
		}
	case len(shown) > failureLineBudget:
		// The first of them, with the tail dropped. Where a run breaks in several places the earliest
		// failure is what a reader needs, and the ones after it are often that failure again.
		shown = shown[:failureLineBudget]
	}
	for _, line := range shown {
		g.quote(line)
	}
	// What was dropped, and what prints all of it. A reader who is not told about the gap cannot tell
	// a whole report from one that cut the part they needed.
	if dropped := len(all) - len(shown); dropped > 0 {
		g.quote(fmt.Sprintf("... %d of %d line(s) not shown, and `%s` is what prints all of them",
			dropped, len(all), shell.Oneline(c.cmd)))
	}
}

// The lines of a check's output that carry its failure: each `--- FAIL`, each line a failing package
// or a panic opens with, and the output belonging to them. Empty for a check with no such lines in
// its output. That is every check apart from `go test`.
func failureLines(all []string) []string {
	var kept []string
	carrying := false
	for _, line := range all {
		switch {
		case opensFailure(line):
			carrying = true
		case closesFailure(line):
			carrying = false
		}
		if carrying {
			kept = append(kept, line)
		}
	}
	return kept
}

// Checks whether a line starts something a reader has to act on. `#` is the compiler's own package
// header, which is the whole of what a package reported as `[build failed]` says about why.
func opensFailure(line string) bool {
	// A subtest indents its own `--- FAIL`.
	return hasAnyPrefix(line, "FAIL", "panic:", "fatal error:", "# ") ||
		hasAnyPrefix(strings.TrimLeft(line, " \t"), "--- FAIL")
}

// Checks whether a line ends the failure it follows. Everything here is `go test` reporting a case or
// a package that came back fine, and none of that belongs to a failure.
func closesFailure(line string) bool {
	return hasAnyPrefix(line, "ok ", "ok\t", "? ", "?\t", "PASS") ||
		hasAnyPrefix(strings.TrimLeft(line, " \t"), "--- PASS", "--- SKIP", "=== ")
}

func hasAnyPrefix(line string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// A check's output as lines. Empty output gives no lines at all.
func outputLines(output string) []string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// Prints one line of a check's own output, indented under the line that named the check.
func (g *gate) quote(line string) {
	fmt.Fprintf(g.out, "              %s\n", line)
}
