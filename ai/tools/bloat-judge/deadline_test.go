package bloatjudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	reply, err := ClaudeCaller(10*time.Second)("prompt", "view")
	if err != nil || strings.TrimSpace(reply) != "none" {
		t.Fatalf("got %q %v, want none", reply, err)
	}
}

// The bound is named in the error, because the caller has to be able to tell a judge that was cut off
// from one whose model crashed: only the first is worth another run.
func TestClaudeCallerNamesTheDeadlineItCutTheRollOffAt(t *testing.T) {
	fakeClaude(t, "sleep 30")
	_, err := ClaudeCaller(300*time.Millisecond)("prompt", "view")
	if err == nil || !strings.Contains(err.Error(), "did not answer within 300ms") {
		t.Fatalf("got %v, want the deadline named", err)
	}
}

func TestClaudeCallerReportsAModelThatFailedRatherThanTimedOut(t *testing.T) {
	fakeClaude(t, "exit 1")
	_, err := ClaudeCaller(10*time.Second)("prompt", "view")
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
	if _, err := ClaudeCaller(500*time.Millisecond)("prompt", "view"); err == nil {
		t.Fatal("a roll that never answered came back with no error")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("the roll took %s to give up on a 500ms deadline — the grandchild still held the pipe", elapsed)
	}
}

// The two ends tied together: an expiry has to reach the caller as exit 2 saying the judge did not
// run, never as a clean pass over unjudged text.
func TestAnExpiredRollExitsDidNotRunAndSaysSo(t *testing.T) {
	path := write(t, source)
	var out, errOut strings.Builder
	fakeClaude(t, "sleep 30")
	code := Run("bloat-judge.sh", []string{"comment", path}, nil, &out, &errOut, ClaudeCaller(300*time.Millisecond), nil)
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
	if deadline, ok := ResolveRollDeadline("bloat-judge.sh", quiet, quiet, failingWriter{t}); !ok || deadline != defaultRollDeadline {
		t.Fatalf("got %s ok=%v with no override, want the default and silence", deadline, ok)
	}

	tuned := t.TempDir()
	writeOverride(t, tuned, "roll-timeout 45\n")
	var said strings.Builder
	if deadline, ok := ResolveRollDeadline("bloat-judge.sh", tuned, tuned, &said); !ok || deadline != 45*time.Second {
		t.Fatalf("got %s ok=%v, want 45s", deadline, ok)
	}
	if !strings.HasPrefix(said.String(), "bloat-judge.sh: ") || !strings.Contains(said.String(), "45s") {
		t.Fatalf("the announcement is not the tool's own voice: %q", said.String())
	}

	broken := t.TempDir()
	writeOverride(t, broken, "timeout 45\n")
	said.Reset()
	if _, ok := ResolveRollDeadline("bloat-judge.sh", broken, broken, &said); ok {
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
// `cfg/kk-flavor/bloat-judge.conf` would set the bound for every run made from inside it.
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
	for _, want := range []string{"45s", "bloat-judge.conf", defaultRollDeadline.String()} {
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
	if err := os.MkdirAll(filepath.Join(config, "kk-flavor", "bloat-judge.conf"), 0o700); err != nil {
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
	path := filepath.Join(config, "kk-flavor", "bloat-judge.conf")
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
	path := filepath.Join(configHome, "kk-flavor", "bloat-judge.conf")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
