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
func TestNoCaseGivesARollAWallClockBudget(t *testing.T) {
	names, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, name := range names {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		budgets, rolls := rollDeadlines(t, name, string(source))
		found += rolls
		for _, budget := range budgets {
			t.Errorf("%s bounds a roll at %s, which is a budget. Under concurrent gates this package "+
				"spent ten such seconds before a fake that only echoes could answer, and nine cases "+
				"failed on the bound rather than on their subjects. Pass notTheSubject, or — where "+
				"the deadline is what the case asks about — a sub-second one against a fake that "+
				"never returns.", name, budget)
		}
	}
	if found == 0 {
		t.Fatal("no case here bounds a roll at all, so this would pass over the suite in any state")
	}
}

// The scan above driven over text, so the spelling it exists to catch is a case rather than something
// the tree merely happens not to hold today.
func TestWhatCountsAsARollDeadlineBudget(t *testing.T) {
	for _, row := range []struct {
		name    string
		source  string
		budgets []string
		rolls   int
	}{
		{"the shared constant", "func f() { ClaudeCaller(notTheSubject, s()) }", nil, 1},
		{"a sub-second bound the case is about", "func f() { CodexCaller(100*time.Millisecond, s()) }", nil, 1},
		{"a budget in seconds", "func f() { ClaudeCaller(10*time.Second, s()) }", []string{"10*time.Second"}, 1},
		{"a budget in minutes", "func f() { CodexCaller(time.Minute, s()) }", []string{"time.Minute"}, 1},
		{"a second constant beside the shared one", "func f() { ClaudeCaller(generous, s()) }", []string{"generous"}, 1},
		{"a duration in a call that bounds no roll", "func f() { Waiting(10*time.Second, 3) }", nil, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			budgets, rolls := rollDeadlines(t, "fixture_test.go", "package readerjudge\n"+row.source+"\n")
			if rolls != row.rolls {
				t.Errorf("found %d roll(s), want %d", rolls, row.rolls)
			}
			if strings.Join(budgets, ",") != strings.Join(row.budgets, ",") {
				t.Errorf("budgets = %q, want %q", budgets, row.budgets)
			}
		})
	}
}

// rollDeadlines reports the deadlines `source` hands a caller constructor that are spelled as budgets,
// and how many it hands one at all — the second so a scan that matched nothing can say so rather than
// read as a clean sweep. The deadline is taken as the bytes the author wrote, not as a reconstruction
// of them, so what the failure quotes is what the reader will search the file for.
func rollDeadlines(t *testing.T, name, source string) ([]string, int) {
	t.Helper()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, name, source, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	var budgets []string
	found := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall || len(call.Args) == 0 {
			return true
		}
		callee, isName := call.Fun.(*ast.Ident)
		if !isName || (callee.Name != "ClaudeCaller" && callee.Name != "CodexCaller") {
			return true
		}
		found++
		deadline := call.Args[0]
		spelled := source[fileSet.Position(deadline.Pos()).Offset:fileSet.Position(deadline.End()).Offset]
		if deadlineIsABudget(spelled) {
			budgets = append(budgets, spelled)
		}
		return true
	})
	return budgets, found
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
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestClaudeCallerAnswersWhatTheModelPrinted(t *testing.T) {
	fakeClaude(t, "echo none")
	reply, err := ClaudeCaller(notTheSubject, testSettings())("prompt", "view")
	if err != nil || strings.TrimSpace(reply) != "none" {
		t.Fatalf("got %q %v, want none", reply, err)
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

// The case the group kill exists for. `claude` starts children, and a child that outlives the one we
// signalled keeps holding the output pipe — so killing the process alone leaves this blocked long
// after the deadline passed, which is the hang the deadline was supposed to remove. Bounded well
// under the WaitDelay, since returning after that would prove only that the fallback fired.
func TestClaudeCallerDoesNotWaitOnAChildThatOutlivesTheRoll(t *testing.T) {
	fakeClaude(t, "sleep 30 &\nsleep 30")
	started := time.Now()
	if _, err := ClaudeCaller(500*time.Millisecond, testSettings())("prompt", "view"); err == nil {
		t.Fatal("a roll that never answered came back with no error")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("the roll took %s to give up on a 500ms deadline — the grandchild still held the pipe", elapsed)
	}
}

// The two ends tied together: an expiry has to reach the caller as exit 2 saying the judge did not
// run, never as a clean pass over unjudged text.
func TestAnExpiredRollExitsDidNotRunAndSaysSo(t *testing.T) {
	path := write(t, instructions)
	var out, errOut strings.Builder
	fakeClaude(t, "sleep 30")
	code := Run("reader-judge.sh", []string{"instruction", path}, nil, &out, &errOut, ClaudeCaller(300*time.Millisecond, testSettings()), nil)
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
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(grandchild, syscall.Signal(0)); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	syscall.Kill(grandchild, syscall.SIGKILL)
	t.Errorf("the grandchild %d outlived the group kill, so a timed-out roll leaves work running",
		grandchild)
}

// The branch that exists only because of the race: once the child is reaped its pid may already
// belong to somebody else, so the group kill must not be issued at all.
func TestKillingAReapedRollSignalsNothing(t *testing.T) {
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
