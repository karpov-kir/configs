// The fixture the cases beside this file drive. A real directory holds the files a run opens, and a
// `repotest.Fake` answers the one question a run puts to a repository: where its top level is, which
// `--kind` reads its template from.
package voicecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"configs/ai/tools/repo/repotest"
)

// What this machine would have handed a case, had TestMain not overridden HOME and XDG_CONFIG_HOME.
var machineHome, machineConfigHome string

func TestMain(m *testing.M) {
	// These two are read before the overrides, so the isolation case can name what a leak reached
	// instead of guessing at it.
	machineHome, machineConfigHome = os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME")
	base, err := os.MkdirTemp("", "voice-check")
	if err != nil {
		panic("voice-check tests: no temp dir, so nothing was tested: " + err.Error())
	}
	// The tool reads its byte cap from under HOME. Pinning HOME and XDG_CONFIG_HOME keeps every case
	// off the config of whoever runs the suite.
	os.Setenv("HOME", filepath.Join(base, "home"))
	os.MkdirAll(os.Getenv("HOME"), 0o755)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	os.MkdirAll(os.Getenv("XDG_CONFIG_HOME"), 0o755)
	// os.Exit runs no deferred call, so the directory is removed here.
	code := m.Run()
	os.RemoveAll(base)
	os.Exit(code)
}

// fixture is one directory under test, plus what the run printed about it.
type fixture struct {
	t    *testing.T
	dir  string
	fake *repotest.Fake

	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRepo(t *testing.T) *fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("could not build a fixture directory: %v — stopping, since every case reads one", err)
	}
	return &fixture{t: t, dir: dir, fake: repotest.New(dir)}
}

// write puts a file under the fixture directory.
func (f *fixture) write(name, body string) {
	f.t.Helper()
	full := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatalf("could not create the parent for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		f.t.Fatalf("could not write the fixture %s: %v", name, err)
	}
}

func baseConfig() Config {
	return Config{MaxFileBytes: defaultMaxFileBytes}
}

func (f *fixture) run(args ...string) {
	f.runWith(baseConfig(), args...)
}

func (f *fixture) runWith(cfg Config, args ...string) {
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run("voice-check.sh", args, f.dir, f.fake, cfg, &f.stdout, &f.stderr)
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

// A refused run must leave nothing on stdout: anything there is what a caller capturing the report
// reads as a finding.
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

// housey is a text the register scan reports, one contrast finding per paragraph. A case about which
// file was read can then observe the scan through its findings.
func housey(paragraphs int) string {
	var b strings.Builder
	for i := 0; i < paragraphs; i++ {
		fmt.Fprintf(&b, "The reader climbs to entry %d rather than the entry asked for.\n\n", i)
	}
	return b.String()
}
