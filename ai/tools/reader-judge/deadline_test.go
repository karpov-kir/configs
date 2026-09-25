package readerjudge

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// notTheSubject is what a case bounds its roll at when the deadline is not what the case asks about,
// which is nearly every case here and in provider_test.go. An hour, and the hour is the point: the
// gate and both workflows give each package `-timeout 30m` (gate/run.go, goSuiteTimeout), so this sits
// past the bound that already stops a hang and can never be the one that fires.
//
// A roll deadline exists so a run ends, not so it ends soon; deadline.go carries the production figure
// and the measurements behind it. A case driving a fake that answers and exits is already certain to
// end, so the only thing left for this to catch is a caller that hangs forever — which the suite
// timeout catches, and which is a real defect. That is the one place its confusing goroutine dump is
// worth paying for. A machine that was merely busy is not a defect and must not go red at all.
//
// Sized out of reach, not sized generously. The 10s this replaces was itself a raise from 1s made for
// this same reason, and gates over sibling worktrees still spent all ten of those seconds before a
// fake that only echoes could answer — nine cases in this package at once, under a reproduction of
// that load. Any figure chosen against load is one a busier machine erodes.
const notTheSubject = time.Hour

// The constant above cannot hold its own line. It already carried this reasoning while two cases in
// this package typed `10*time.Second` instead of using it, and one of those two is a case concurrent
// gates turned red on a green tree.
//
// So: a roll deadline in a case here is spelled one of two ways. Sub-second is the shape of a deadline
// that IS the subject — such a case drives a fake that never returns, so the bound fires whatever else
// the machine is doing. Anything coarser is the shape of a budget, an allowance for work expected to
// finish sooner, and a budget is what load eats.
//
// The second spelling the scan reads is a roll a case makes slow by sleeping. A sleep is a budget the
// caller constructors never see. A case reading one against the announcer's interval asserts that one
// real duration outruns another. That is the defect the announcer case carried past a scan that
// watched only ClaudeCaller and CodexCaller.
func TestNoCaseGivesARollAWallClockBudget(t *testing.T) {
	names, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var total rollScan
	for _, name := range names {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		found := scanRolls(t, name, string(source))
		total.bounded += found.bounded
		total.wrapped += found.wrapped
		for _, budget := range found.budgets {
			t.Errorf("%s bounds a roll at %s, which is a budget. Under concurrent gates this package "+
				"spent ten such seconds before a fake that only echoes could answer, and nine cases "+
				"failed on the bound rather than on their subjects. Pass notTheSubject, or — where "+
				"the deadline is what the case asks about — a sub-second one against a fake that "+
				"never returns.", name, budget)
		}
		for _, slept := range found.sleeps {
			t.Errorf("%s makes a roll slow by sleeping %s, so what the case asserts is that one real "+
				"duration outruns another, which a loaded machine makes false. Hold the roll on "+
				"something the case releases — a channel it closes, or a clock it ticks — so the "+
				"ordering is the ordering and not a race.", name, slept)
		}
	}
	if total.bounded == 0 {
		t.Error("no case here bounds a roll at all, so that half would pass over the suite in any state")
	}
	if total.wrapped == 0 {
		t.Error("no case here hands a roll to a wrapper, so that half would pass over the suite in any state")
	}
}

// The scan above driven over text, so each spelling it exists to catch is exercised here instead of
// merely absent from today's tree.
func TestWhatCountsAsARollRacingTheWallClock(t *testing.T) {
	for _, row := range []struct {
		name             string
		source           string
		budgets, sleeps  []string
		bounded, wrapped int
	}{
		{name: "the shared constant", source: "func f() { ClaudeCaller(notTheSubject, s()) }", bounded: 1},
		{name: "a sub-second bound the case is about", source: "func f() { CodexCaller(100*time.Millisecond, s()) }", bounded: 1},
		{name: "a budget in seconds", source: "func f() { ClaudeCaller(10*time.Second, s()) }", budgets: []string{"10*time.Second"}, bounded: 1},
		{name: "a budget in minutes", source: "func f() { CodexCaller(time.Minute, s()) }", budgets: []string{"time.Minute"}, bounded: 1},
		{name: "a second constant beside the shared one", source: "func f() { ClaudeCaller(generous, s()) }", budgets: []string{"generous"}, bounded: 1},
		{name: "a duration in a call that bounds no roll", source: "func f() { Waiting(10*time.Second, 3) }"},
		{name: "a budget reaching the announcer instead", source: "func f() { announcingASlowRoll(quick, 900*time.Second, tick, w) }", budgets: []string{"900*time.Second"}, bounded: 1, wrapped: 1},
		{name: "a roll the case releases", source: "func f() { announcingOnEachTick(held, notTheSubject, w, c) }", bounded: 1, wrapped: 1},
		// The shape this scan was extended for, as the file held it before the repair.
		{
			name:    "the announcer case as it stood when it went red",
			source:  "func f() { slow := func(string, string) (string, error) { time.Sleep(60 * time.Millisecond); return \"none\", nil }; announcingASlowRoll(slow, 900*time.Second, 10*time.Millisecond, &said) }",
			budgets: []string{"900*time.Second"},
			sleeps:  []string{"60 * time.Millisecond"},
			bounded: 1, wrapped: 1,
		},
		{
			name:    "a sleeping roll handed to the vote",
			source:  "func f() { slow := func(string, string) (string, error) { time.Sleep(time.Second); return \"none\", nil }; Voting(slow, 3) }",
			sleeps:  []string{"time.Second"},
			wrapped: 1,
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			found := scanRolls(t, "fixture_test.go", "package readerjudge\n"+row.source+"\n")
			if found.bounded != row.bounded || found.wrapped != row.wrapped {
				t.Errorf("found %d bounded and %d wrapped roll(s), want %d and %d",
					found.bounded, found.wrapped, row.bounded, row.wrapped)
			}
			if strings.Join(found.budgets, ",") != strings.Join(row.budgets, ",") {
				t.Errorf("budgets = %q, want %q", found.budgets, row.budgets)
			}
			if strings.Join(found.sleeps, ",") != strings.Join(row.sleeps, ",") {
				t.Errorf("sleeps = %q, want %q", found.sleeps, row.sleeps)
			}
		})
	}
}

// rollScan is what one file gave up: the two spellings that make a case race the wall clock, and the
// count of each seam it was read for. The counts are there because a scan that matched no roll must
// say as much. It must never read as a clean sweep.
type rollScan struct {
	budgets, sleeps  []string
	bounded, wrapped int
}

// rollSeam is the argument position at which a function takes each of the two things spelled as wall
// clock: the bound, and the roll the case makes slow. A case hands its roll to one of those functions.
// -1 is a position this function does not take.
type rollSeam struct{ deadline, roll int }

func rollSeamOf(callee string) (rollSeam, bool) {
	switch callee {
	case "ClaudeCaller", "CodexCaller":
		return rollSeam{deadline: 0, roll: -1}, true
	case "announcingASlowRoll", "announcingOnEachTick":
		return rollSeam{deadline: 1, roll: 0}, true
	case "Voting", "namingTheFileThatDecides":
		return rollSeam{deadline: -1, roll: 0}, true
	}
	return rollSeam{}, false
}

// scanRolls reads `source` for the rolls it bounds or wraps. Both spellings are taken as the bytes the
// author wrote, and never as a reconstruction of them. What a failure quotes is then what the reader
// will search the file for.
func scanRolls(t *testing.T, name, source string) rollScan {
	t.Helper()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, name, source, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	spelling := func(node ast.Node) string {
		return source[fileSet.Position(node.Pos()).Offset:fileSet.Position(node.End()).Offset]
	}
	// One declaration at a time. Two cases in a file both calling their roll `slow` are then read as
	// the two literals they are, and never as whichever the file bound last.
	var found rollScan
	for _, declared := range parsed.Decls {
		found.gather(declared, spelling)
	}
	return found
}

// gather reads one declaration for the rolls it bounds and the rolls it hands to a wrapper.
func (s *rollScan) gather(declared ast.Decl, spelling func(ast.Node) string) {
	bound := rollLiterals(declared)
	ast.Inspect(declared, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		callee, isName := call.Fun.(*ast.Ident)
		if !isName {
			return true
		}
		seam, known := rollSeamOf(callee.Name)
		if !known {
			return true
		}
		if seam.deadline >= 0 && seam.deadline < len(call.Args) {
			s.bounded++
			if spelled := spelling(call.Args[seam.deadline]); deadlineIsABudget(spelled) {
				s.budgets = append(s.budgets, spelled)
			}
		}
		if seam.roll >= 0 && seam.roll < len(call.Args) {
			s.wrapped++
			for _, slept := range sleepsIn(bound, call.Args[seam.roll]) {
				s.sleeps = append(s.sleeps, spelling(slept))
			}
		}
		return true
	})
}

// rollLiterals maps every name a declaration binds to a function literal onto that literal. A roll is
// almost always handed to a wrapper by name, and seldom inline. The body the author wrote goes unread
// without this map.
func rollLiterals(scope ast.Node) map[string]*ast.FuncLit {
	literals := map[string]*ast.FuncLit{}
	ast.Inspect(scope, func(node ast.Node) bool {
		assigned, isAssignment := node.(*ast.AssignStmt)
		if !isAssignment {
			return true
		}
		for i, left := range assigned.Lhs {
			name, isName := left.(*ast.Ident)
			if !isName || i >= len(assigned.Rhs) {
				continue
			}
			if literal, isLiteral := assigned.Rhs[i].(*ast.FuncLit); isLiteral {
				literals[name.Name] = literal
			}
		}
		return true
	})
	return literals
}

// sleepsIn reports the durations a roll sleeps away, whether it was written inline or handed over by
// the name it was bound to.
func sleepsIn(literals map[string]*ast.FuncLit, roll ast.Expr) []ast.Expr {
	var body *ast.FuncLit
	switch given := roll.(type) {
	case *ast.FuncLit:
		body = given
	case *ast.Ident:
		body = literals[given.Name]
	}
	if body == nil {
		return nil
	}
	var slept []ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall || len(call.Args) == 0 {
			return true
		}
		sleeping, isSelected := call.Fun.(*ast.SelectorExpr)
		if !isSelected || sleeping.Sel.Name != "Sleep" {
			return true
		}
		if pkg, isName := sleeping.X.(*ast.Ident); isName && pkg.Name == "time" {
			slept = append(slept, call.Args[0])
		}
		return true
	})
	return slept
}

// The shared constant, or a unit finer than a second. There is no third legitimate way to bound a roll
// in a case here, so everything else is a budget.
func deadlineIsABudget(spelled string) bool {
	if spelled == "notTheSubject" {
		return false
	}
	for _, fine := range []string{"time.Millisecond", "time.Microsecond", "time.Nanosecond"} {
		if strings.HasSuffix(spelled, fine) {
			return false
		}
	}
	return true
}

// fakeClaude puts a `claude` on PATH that does what the case needs, so the deadline is driven against
// a real process, a real signal and a real pipe rather than a stand-in for them.
func fakeClaude(t *testing.T, script string) {
	t.Helper()
	// A call rewrites the served-model cache it contradicts, and a case must never reach the real one.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestClaudeCallerAnswersWhatTheModelPrinted(t *testing.T) {
	fakeClaude(t, `echo '{"result":"none","is_error":false,"modelUsage":{"claude-fixture":{"outputTokens":1}}}'`)
	reply, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view")
	if err != nil || strings.TrimSpace(reply) != "none" {
		t.Fatalf("got %q %v, want none", reply, err)
	}
}

// The call asks for JSON, so plain text is a fault in the CLI and never a verdict. A warning printed
// ahead of the object would otherwise be judged as the model's answer.
func TestAReplyThatIsNotJSONFails(t *testing.T) {
	fakeClaude(t, "echo none")
	if _, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view"); err == nil ||
		!strings.Contains(err.Error(), "no JSON") {
		t.Fatalf("got %v, want the reply refused as no JSON", err)
	}
}

// Two models tied on output tokens pick the model sorting first, so a report reads the same twice.
func TestATieOnOutputTokensPicksTheSameModelEveryTime(t *testing.T) {
	_, served, err := readClaudeReply(`{"result":"x","modelUsage":{"b-model":{"outputTokens":3},"a-model":{"outputTokens":3}}}`, "a")
	if err != nil || served.Answered != "a-model" {
		t.Fatalf("answered %q, %v", served.Answered, err)
	}
}

// The bound is named in the error, because the caller has to be able to tell a judge that was cut off
// from one whose model crashed: only the first is worth another run.
func TestClaudeCallerNamesTheDeadlineItCutTheRollOffAt(t *testing.T) {
	fakeClaude(t, "sleep 30")
	_, err := ClaudeCaller(300*time.Millisecond, testSettings())("prompt", "view")
	if err == nil || !strings.Contains(err.Error(), "did not answer within 300ms") {
		t.Fatalf("got %v, want the deadline named", err)
	}
}

func TestClaudeCallerReportsAModelThatFailedRatherThanTimedOut(t *testing.T) {
	fakeClaude(t, "exit 1")
	_, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view")
	if err == nil || strings.Contains(err.Error(), "within") {
		t.Fatalf("got %v, want a plain failure and no deadline in it", err)
	}
}

// The case the group kill exists for, driven through the caller and never through killRollGroup, the
// function that signals the group. That way Setpgid and Cancel are read against a real roll, and not
// only against their own correctness.

// `claude` starts children, and a child that outlives the process we signalled goes on holding the
// output pipe, which is the hang the deadline was supposed to remove.

// The child is what the case reads, and never a stopwatch. An elapsed-time budget for the same fact is
// what concurrent gates erode. As a 3-second one this went red on a green tree at 3.4s, with the group
// killed on time at 501ms and no part of it left alive. That is the reading notTheSubject, the deadline
// const, gives every wall clock figure in this file.
func TestAClaudeRollCutOffAtItsDeadlineLeavesNoChildRunning(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild")
	fakeClaude(t, "sleep 30 &\necho $! >"+pidFile+"\nsleep 30")
	if _, err := ClaudeCaller(500*time.Millisecond, testSettings())("prompt", "view"); err == nil {
		t.Fatal("a roll that never answered came back with no error")
	}

	grandchild, recorded := pidRecorded(t, pidFile)
	// A machine slow enough to keep the fixture shell from its own second line inside the deadline
	// built no child for the roll to outlive. A green over that would be a green over a state this run
	// never had.
	if !recorded {
		t.Skip("the fixture shell recorded no child inside the deadline, so this run never built one " +
			"for the roll to outlive")
	}
	if stillRunning(grandchild) {
		syscall.Kill(grandchild, syscall.SIGKILL)
		t.Errorf("the grandchild %d outlived the roll, so a roll cut off at its deadline leaves work "+
			"running and its caller waiting on the pipe that child holds", grandchild)
	}
}

// pidRecorded reads back the pid a fixture shell wrote, or false where the shell was cut off before it
// got that far. Anything present but unreadable fails the case, and never reads as absent. A fixture
// that half-wrote its own state is a broken fixture, and not a slow machine.
func pidRecorded(t *testing.T, path string) (int, bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("reading the pid the fixture recorded: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("the fixture recorded %q where a child's pid goes: %v", raw, err)
	}
	return pid, true
}

// stillRunning waits out the moment between SIGKILL and the kernel taking the process off the table.
// The answer is then about the signal having landed, and never about how busy the machine is.
func stillRunning(pid int) bool {
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if err := syscall.Kill(pid, syscall.Signal(0)); err != nil {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
	return true
}

// The two ends tied together: an expiry has to reach the caller as exit 2 saying the judge did not
// run, never as a clean pass over unjudged text.
func TestAnExpiredRollExitsDidNotRunAndSaysSo(t *testing.T) {
	path := write(t, instructions)
	var out, errOut strings.Builder
	fakeClaude(t, "sleep 30")
	code := Run("reader-judge.sh", []string{"instruction", path}, noRepository, nil, &out, &errOut, ClaudeCaller(300*time.Millisecond, testSettings()), nil)
	if code != exitDidNotRun {
		t.Fatalf("exit %d, want %d", code, exitDidNotRun)
	}
	if !strings.Contains(errOut.String(), "did not answer within 300ms") ||
		!strings.Contains(errOut.String(), "did NOT run") {
		t.Fatalf("the refusal does not say the judge was cut off: %q", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("a cut-off run still printed: %q", out.String())
	}
}

// What the command itself does with the answer: an override announced, a broken one refused before a
// single roll is spent, and an untuned machine saying nothing.
func TestResolveRollDeadlineAnnouncesRefusesOrStaysQuiet(t *testing.T) {
	quiet := t.TempDir()
	deadline, path, ok := ResolveRollDeadline("reader-judge.sh", quiet, quiet, failingWriter{t})
	if !ok || deadline != defaultRollDeadline {
		t.Fatalf("got %s ok=%v with no override, want the default and silence", deadline, ok)
	}
	// The path comes back from the quiet case too. It is what a timed-out roll sends the reader to,
	// and an untuned machine is precisely the one where the file does not exist yet.
	if path != overridePath(quiet, quiet) {
		t.Fatalf("got %q with no override, want the location one would go in", path)
	}

	tuned := t.TempDir()
	writeOverride(t, tuned, "roll-timeout 45\n")
	var said strings.Builder
	if deadline, path, ok := ResolveRollDeadline("reader-judge.sh", tuned, tuned, &said); !ok || deadline != 45*time.Second || path != overridePath(tuned, tuned) {
		t.Fatalf("got %s %q ok=%v, want 45s at the override", deadline, path, ok)
	}
	if !strings.HasPrefix(said.String(), "reader-judge.sh: ") || !strings.Contains(said.String(), "45s") {
		t.Fatalf("the announcement is not the tool's own voice: %q", said.String())
	}

	broken := t.TempDir()
	writeOverride(t, broken, "timeout 45\n")
	said.Reset()
	if _, _, ok := ResolveRollDeadline("reader-judge.sh", broken, broken, &said); ok {
		t.Fatal("a broken override let the judge run")
	}
	if !strings.Contains(said.String(), "the judge did NOT run") {
		t.Fatalf("the refusal does not use the tool's own words for not running: %q", said.String())
	}
}

// A writer that fails the case if anything is written to it, so "says nothing" is asserted rather than
// assumed from a buffer nobody looked at.
type failingWriter struct{ t *testing.T }

func (w failingWriter) Write(p []byte) (int, error) {
	w.t.Fatalf("an untuned machine wrote to stderr: %q", p)
	return len(p), nil
}

func TestRollDeadlineIsTheDefaultWithNoOverrideFile(t *testing.T) {
	deadline, override, err := rollDeadline(t.TempDir(), t.TempDir())
	if err != nil || deadline != defaultRollDeadline || override != "" {
		t.Fatalf("got %s %q %v, want the default and no announcement", deadline, override, err)
	}
}

// A config home that is not absolute is no config home: read as given, a checkout shipping
// `cfg/kk-flavor/reader-judge.conf` would set the bound for every run made from inside it.
func TestRollDeadlineIgnoresARelativeConfigHome(t *testing.T) {
	home := t.TempDir()
	writeOverride(t, filepath.Join(home, ".config"), "roll-timeout 300\n")
	deadline, override, err := rollDeadline("cfg", home)
	if err != nil || deadline != 300*time.Second || override == "" {
		t.Fatalf("got %s %q %v, want the home's own override", deadline, override, err)
	}
	if deadline, _, err := rollDeadline("cfg", "home"); err != nil || deadline != defaultRollDeadline {
		t.Fatalf("got %s %v with nowhere for an override to sit, want the default", deadline, err)
	}
}

// An override that took effect says so on every run, or a tuned machine is indistinguishable from an
// untuned one in the output.
func TestAnOverrideThatTookEffectAnnouncesItself(t *testing.T) {
	config := t.TempDir()
	writeOverride(t, config, "# tuned for a slow link\nroll-timeout 45\n")
	deadline, override, err := rollDeadline(config, t.TempDir())
	if err != nil || deadline != 45*time.Second {
		t.Fatalf("got %s %v, want 45s", deadline, err)
	}
	for _, want := range []string{"45s", "reader-judge.conf", defaultRollDeadline.String()} {
		if !strings.Contains(override, want) {
			t.Fatalf("the announcement does not carry %q: %q", want, override)
		}
	}
}

// Present but unusable refuses, every way it can be unusable. A default quietly restored is
// indistinguishable from the override working, so none of these may fall back to it.
func TestAnUnusableOverrideRefusesRatherThanFallingBack(t *testing.T) {
	for _, c := range []struct{ name, content, says string }{
		{"a line it does not understand", "timeout 45\n", "does not understand"},
		{"the key twice", "roll-timeout 45\nroll-timeout 60\n", "more than once"},
		{"a value that is not a number", "roll-timeout soon\n", "not a whole number"},
		{"zero seconds", "roll-timeout 0\n", "not a whole number"},
		{"a negative", "roll-timeout -5\n", "not a whole number"},
		{"nothing but comments", "# tuned, one day\n", "sets no roll-timeout"},
		{"an empty file", "", "sets no roll-timeout"},
	} {
		t.Run(c.name, func(t *testing.T) {
			config := t.TempDir()
			writeOverride(t, config, c.content)
			deadline, _, err := rollDeadline(config, t.TempDir())
			if err == nil {
				t.Fatalf("accepted %q as %s", c.content, deadline)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Fatalf("the refusal does not say %q: %v", c.says, err)
			}
		})
	}
}

// A directory where the file belongs, rather than a mode bit: root ignores mode bits, so a chmod
// fixture builds no refusal on a machine running as one (testing.md → 4).
func TestAnOverridePathThatIsNotAFileRefuses(t *testing.T) {
	config := t.TempDir()
	if err := os.MkdirAll(filepath.Join(config, "kk-flavor", "reader-judge.conf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rollDeadline(config, t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "not a readable regular file") {
		t.Fatalf("got %v, want a refusal naming the file", err)
	}
}

// A dangling link reads as absent to an existence test alone, so it is checked for by name.
func TestADanglingOverrideLinkRefusesInsteadOfReadingAsAbsent(t *testing.T) {
	config := t.TempDir()
	path := filepath.Join(config, "kk-flavor", "reader-judge.conf")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(config, "gone"), path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rollDeadline(config, t.TempDir()); err == nil {
		t.Fatal("a dangling override link was read as no override at all")
	}
}

func writeOverride(t *testing.T, configHome, content string) {
	t.Helper()
	path := filepath.Join(configHome, "kk-flavor", "reader-judge.conf")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A roll that timed out still has its whole group killed — the branch where the child is alive.
func TestKillingARollReachesTheGroupAndNotOnlyTheChild(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & echo $!; sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the fixture: %v", err)
	}
	var grandchild int
	if _, err := fmt.Fscanln(out, &grandchild); err != nil {
		t.Fatalf("reading the grandchild's pid: %v", err)
	}

	// The control: the grandchild has to be alive before the kill, or the assertion after it would
	// pass over a process that was never running.
	if err := syscall.Kill(grandchild, syscall.Signal(0)); err != nil {
		t.Fatalf("the grandchild %d was not running before the kill: %v", grandchild, err)
	}
	if err := killRollGroup(cmd.Process); err != nil {
		t.Fatalf("killing a live roll's group: %v", err)
	}
	cmd.Wait()

	// The child is reaped by Wait; the grandchild is only reached through the group.
	if stillRunning(grandchild) {
		syscall.Kill(grandchild, syscall.SIGKILL)
		t.Errorf("the grandchild %d outlived the group kill, so a timed-out roll leaves work running",
			grandchild)
	}
}

// The other half of the same behaviour, and the harder one: a provider that exits leaving a child
// behind has its own pid reaped before anybody cancels anything. That child is reparented to launchd,
// still holding the roll's output pipe. Reaped is a fact about the pid, and never about the group, so
// the group is what the kill has to answer for.
func TestAReapedRollsChildrenAreLeftToTheWaitDelay(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & echo $!; exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the fixture: %v", err)
	}
	var grandchild int
	if _, err := fmt.Fscanln(out, &grandchild); err != nil {
		t.Fatalf("reading the grandchild's pid: %v", err)
	}
	// Wait is what reaps the leader, which is the state the case is about: the group outlives it.
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting on a fixture that exits 0: %v", err)
	}
	if err := syscall.Kill(grandchild, syscall.Signal(0)); err != nil {
		t.Fatalf("the grandchild %d was not running before the kill: %v", grandchild, err)
	}

	if err := killRollGroup(cmd.Process); !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("killing a reaped roll answered %v, wanted os.ErrProcessDone", err)
	}
	if !stillRunning(grandchild) {
		t.Errorf("the grandchild %d was killed through a group this process no longer owns. A free "+
			"leader pid says the pid is free and says nothing about who the group belongs to", grandchild)
	}
	syscall.Kill(grandchild, syscall.SIGKILL)
}

// A roll that ended with an empty group is reported as finished, and never as a cancel that failed.
// os/exec reads that as "already over" instead of injecting an error of its own.
func TestKillingAReapedRollWithNothingLeftBehindReportsItFinished(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the fixture: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting on a fixture that exits 0: %v", err)
	}

	err := killRollGroup(cmd.Process)
	// ErrProcessDone and not ESRCH: an unguarded `kill(-pid)` on a freed pid answers ESRCH when the
	// pid happens to be unused, and succeeds when it has been recycled. Only the guard can answer
	// this, so this is what distinguishes the two.
	if !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("killing a reaped roll answered %v, wanted os.ErrProcessDone. Without the guard the "+
			"signal goes to whatever now holds that pid, and every roll sets Setpgid, so a recycled "+
			"pid is a live group leader belonging to somebody else", err)
	}
}

func writeShippedDefault(t *testing.T, home, content string) {
	t.Helper()
	path := filepath.Join(home, ".kk-flavor", "configs", configName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTheShippedDefaultSetsTheBoundAndSaysNothing(t *testing.T) {
	home := t.TempDir()
	writeShippedDefault(t, home, "# the flavor's own\nroll-timeout 120\n")
	deadline, override, err := rollDeadline(t.TempDir(), home)
	if err != nil || deadline != 120*time.Second || override != "" {
		t.Fatalf("got %s %q %v, want 120s and no announcement", deadline, override, err)
	}
}

// The announcement offers the number that removing the override restores. That is the shipped
// default, which may differ from the constant in code.
func TestAnOverrideWinsOverTheShippedDefaultAndNamesIt(t *testing.T) {
	home, config := t.TempDir(), t.TempDir()
	writeShippedDefault(t, home, "roll-timeout 120\n")
	writeOverride(t, config, "roll-timeout 45\n")
	deadline, override, err := rollDeadline(config, home)
	if err != nil || deadline != 45*time.Second {
		t.Fatalf("got %s %v, want 45s", deadline, err)
	}
	if !strings.Contains(override, "in place of the default 2m0s") {
		t.Fatalf("the announcement offers a default the override does not sit in front of: %q", override)
	}
}

// A shipped default that is present and unusable refuses the run. The constant in code is a real
// number, and a run that fell back to it reports success under a bound the human never chose.
func TestAnUnusableShippedDefaultRefusesRatherThanFallingBack(t *testing.T) {
	for _, c := range []struct{ name, content, says string }{
		{"a line it does not understand", "timeout 45\n", "does not understand"},
		{"no setting at all", "# nothing here\n", "sets no roll-timeout"},
		{"the key twice", "roll-timeout 45\nroll-timeout 60\n", "more than once"},
		{"a value that is not seconds", "roll-timeout soon\n", "not a whole number"},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			writeShippedDefault(t, home, c.content)
			_, _, err := rollDeadline(t.TempDir(), home)
			if err == nil {
				t.Fatal("an unusable shipped default fell back to the constant instead of refusing")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Fatalf("the refusal does not say %q: %v", c.says, err)
			}
		})
	}
}

// This case opens the file the flavor ships, through the same resolver an installed run uses, and it
// holds the shipped number to the constant in code. A typo there shows up here first.
func TestTheShippedJudgeConfigParsesAndMatchesTheConstant(t *testing.T) {
	flavor, err := filepath.Abs("../../kk-flavor")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.Symlink(flavor, filepath.Join(home, ".kk-flavor")); err != nil {
		t.Fatal(err)
	}
	deadline, announcement, err := rollDeadline(t.TempDir(), home)
	if err != nil {
		t.Fatalf("the shipped %s does not parse: %v", configName, err)
	}
	if deadline != defaultRollDeadline {
		t.Fatalf("the shipped %s bounds a roll at %s and the constant in code at %s", configName, deadline, defaultRollDeadline)
	}
	if announcement != "" {
		t.Fatalf("the shipped default announced itself, so a tuned machine would look like this one: %q", announcement)
	}
}

func TestARollTimeoutThatWouldOverflowIsRefused(t *testing.T) {
	for _, source := range []string{"override", "shipped default"} {
		t.Run(source, func(t *testing.T) {
			home, config := t.TempDir(), t.TempDir()
			if source == "override" {
				writeOverride(t, config, "roll-timeout 10000000000\n")
			} else {
				writeShippedDefault(t, home, "roll-timeout 10000000000\n")
			}
			deadline, _, err := rollDeadline(config, home)
			if err == nil {
				t.Fatalf("10000000000 seconds was accepted, giving a deadline of %s", deadline)
			}
			if !strings.Contains(err.Error(), "between 1 and 86400") {
				t.Fatalf("the refusal does not name the bound: %v", err)
			}
		})
	}
	// The bound itself is accepted. The refusals above are about overflow, and a legitimate long
	// timeout still passes.
	config := t.TempDir()
	writeOverride(t, config, "roll-timeout 86400\n")
	if deadline, _, err := rollDeadline(config, t.TempDir()); err != nil || deadline != 86400*time.Second {
		t.Fatalf("got %s %v, want the bound accepted", deadline, err)
	}
}

// The CLI's JSON names the model that wrote the answer, beside a small call of its own on another
// model. A call an account answered on another model is reported and still answers.
func TestAClaudeReplyNamesTheModelThatAnswered(t *testing.T) {
	fakeClaude(t, `echo '{"result":"none","is_error":false,"total_cost_usd":0.01,`+
		`"usage":{"input_tokens":2,"cache_creation_input_tokens":10,"cache_read_input_tokens":20,"output_tokens":4},`+
		`"modelUsage":{"claude-haiku-4-5":{"outputTokens":1},"claude-opus-5-5[1m]":{"outputTokens":4}}}'`)
	var got Served
	settings := testSettings()
	settings.Model = "sonnet"
	reply, err := ClaudeCallerObserved(notTheSubject, settings, func(s Served) { got = s })("prompt", "view")
	if err != nil || reply != "none" {
		t.Fatalf("got %q %v, want none", reply, err)
	}
	if got.Answered != "claude-opus-5-5[1m]" || !got.Substituted() || got.CacheRead != 20 {
		t.Fatalf("served %+v, want opus answering a sonnet request", got)
	}
}

// An error the model reports is an error, and never an answer.
func TestAClaudeReplyMarkedAsAnErrorFails(t *testing.T) {
	fakeClaude(t, `echo '{"result":"overloaded","is_error":true}'`)
	if _, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view"); err == nil ||
		!strings.Contains(err.Error(), "overloaded") {
		t.Fatalf("got %v, want the reported error", err)
	}
}

// A usage limit names the account the CLI ran on, since someone with several accounts may have
// switched the app and left the CLI on another.
func TestAUsageLimitNamesTheAccount(t *testing.T) {
	fakeClaude(t, `if [ "$1" = auth ]; then echo '{"loggedIn":true,"email":"a@example.invalid","orgName":"Org","subscriptionType":"team"}'; else echo "You've hit your session limit"; fi`)
	_, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view")
	if err == nil || !strings.Contains(err.Error(), "a@example.invalid (Org, team)") ||
		!strings.Contains(err.Error(), "claude auth login") {
		t.Fatalf("got %v, want the account and the switch", err)
	}
}
