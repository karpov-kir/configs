// The fixture every case in this file drives: a `repotest.Fake` for the questions this scan asks a
// repository, and a real directory for the untracked files it then opens. No case forks git.

// The two halves are not interchangeable. The fake answers a diff verbatim. Git is what says which
// lines a change added, and deriving one here would make every finding a property of this fixture.

// An untracked file is listed by the fake and then read off disk by `bodyToScan`. The symlink,
// binary and byte-cap cases need a real file, or they measure the open failing in place of the
// guard. File I/O is not what these suites pay for, and process spawns are.
package duplicates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo/repotest"
)

type fixture struct {
	t      *testing.T
	dir    string
	git    *repotest.Fake
	patch  strings.Builder
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	return &fixture{t: t, dir: dir, git: repotest.New(dir)}
}

// added says what git would print for these lines arriving in one file, and appends it to the patch
// this fixture's runs will be answered with. It is spelt out here. A fixture that built the diff
// from a before and an after would be asserting against its own diff implementation, and never
// against the diff a reviewer reads.
func (f *fixture) added(name string, lines ...string) {
	f.t.Helper()
	fmt.Fprintf(&f.patch, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ -0,0 +1,%d @@\n",
		name, name, name, name, len(lines))
	for _, line := range lines {
		f.patch.WriteString("+" + line + "\n")
	}
	f.git.Diff(f.patch.String())
}

// untracked puts a file in the listing AND on disk, because the scan opens every name the listing
// gives it.
func (f *fixture) untracked(name, body string) {
	f.t.Helper()
	f.onDisk(name, body)
	f.git.AddUntracked(name)
}

// onDisk writes a file the listing does not name — what an argument refusal stats, and the target a
// symlink case points at.
func (f *fixture) onDisk(name, body string) {
	f.t.Helper()
	full := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", name, err)
	}
}

func (f *fixture) run(args ...string) {
	f.runWith(Config{MinLength: defaultMinLength, MaxFileBytes: defaultMaxFileBytes}, args...)
}

func (f *fixture) runWith(cfg Config, args ...string) {
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run("dup-literals.sh", args, f.dir, f.git, cfg, &f.stdout, &f.stderr)
}

func (f *fixture) expectCode(want int) {
	f.t.Helper()
	if f.code != want {
		f.t.Errorf("exit %d, wanted %d\nstdout: %s\nstderr: %s", f.code, want, f.stdout.String(), f.stderr.String())
	}
}

func (f *fixture) expectStdoutHas(want string) {
	f.t.Helper()
	if !strings.Contains(f.stdout.String(), want) {
		f.t.Errorf("wanted %q on stdout, got: %s", want, f.stdout.String())
	}
}

func (f *fixture) expectStdoutLacks(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.stdout.String(), unwanted) {
		f.t.Errorf("%q appears on stdout: %s", unwanted, f.stdout.String())
	}
}

func (f *fixture) expectNoStdout() {
	f.t.Helper()
	if f.stdout.Len() != 0 {
		f.t.Errorf("expected nothing on stdout, got: %s", f.stdout.String())
	}
}

func (f *fixture) expectStderrHas(want string) {
	f.t.Helper()
	if !strings.Contains(f.stderr.String(), want) {
		f.t.Errorf("wanted %q on stderr, got: %s", want, f.stderr.String())
	}
}

func (f *fixture) expectStderrLacks(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.stderr.String(), unwanted) {
		f.t.Errorf("%q appears on stderr: %s", unwanted, f.stderr.String())
	}
}

func repeated(char rune, n int) string { return strings.Repeat(string(char), n) }
