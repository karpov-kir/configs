package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The developer's own git config must not reach these fixtures. The gate drives `git ls-files
	// --exclude-standard`, so a global core.excludesFile hides a fixture's own declared inputs and
	// every unit below takes the NO INPUTS refusal on correct code. NOSYSTEM alone does not stop it:
	// it blocks /etc/gitconfig, and ~/.gitconfig is the one that reaches in here.
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	os.Exit(m.Run())
}

type fixture struct {
	t      *testing.T
	root   string
	cache  string
	units  string
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "r")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	f := &fixture{t: t, root: root, cache: filepath.Join(base, "cache"), units: filepath.Join(base, "units")}
	return f
}

func (f *fixture) write(name, body string) {
	f.t.Helper()
	full := filepath.Join(f.root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", name, err)
	}
}

// One unit per line: id, kind, inputs, command — tab-separated, the shape the seam reads.
func (f *fixture) table(lines ...string) {
	f.t.Helper()
	if err := os.WriteFile(f.units, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		f.t.Fatalf("writing the units file: %v", err)
	}
}

// Pinned so a key does not move with the test binary, which would make every "fresh on the second
// run" case depend on nothing having recompiled in between.
func (f *fixture) run(args ...string) {
	f.t.Helper()
	f.runAs("test-digest", args...)
}

func (f *fixture) runAs(digest string, args ...string) {
	f.t.Helper()
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run(args, Env{
		Root:       f.root,
		Cache:      f.cache,
		UnitsFile:  f.units,
		SelfDigest: digest,
	}, &f.stdout, &f.stderr)
}

// Commit whatever is in the fixture tree, so a later deletion reads as tracked-but-gone.
func (f *fixture) commit() {
	f.t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "seed"}} {
		cmd := exec.Command("git", append([]string{"-C", f.root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func (f *fixture) out() string { return f.stdout.String() + f.stderr.String() }

func (f *fixture) expectCode(want int) {
	f.t.Helper()
	if f.code != want {
		f.t.Errorf("exit %d, wanted %d — output:\n%s", f.code, want, f.out())
	}
}

func (f *fixture) expectOut(want string) {
	f.t.Helper()
	if !strings.Contains(f.out(), want) {
		f.t.Errorf("wanted %q in:\n%s", want, f.out())
	}
}

func (f *fixture) expectNotOut(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.out(), unwanted) {
		f.t.Errorf("%q appears in:\n%s", unwanted, f.out())
	}
}

// How many times a unit's command has run, counted by the command itself appending to a file. The
// observable that separates "reported fresh" from "ran again and passed", which the report's own
// wording cannot: both end in a clean gate.
func (f *fixture) runCount(name string) int {
	body, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(body)))
}

func marker(name string, code int) string {
	return "printf 'x ' >> " + name + "; exit " + strconv.Itoa(code)
}
