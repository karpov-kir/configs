package density

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var seedRepo string

func TestMain(m *testing.M) {
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	// NOSYSTEM covers /etc/gitconfig and the HOME override below covers ~/.gitconfig, but git
	// reads $XDG_CONFIG_HOME/git/config as a global source too. A core.excludesFile there empties
	// `ls-files --others --exclude-standard` and reddens every untracked case on a machine that is
	// working perfectly. GIT_CONFIG_GLOBAL supersedes both files at once.
	os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	base, err := os.MkdirTemp("", "density-seed")
	if err != nil {
		panic("density tests: no temp dir, so nothing was tested: " + err.Error())
	}
	defer os.RemoveAll(base)
	os.Setenv("HOME", filepath.Join(base, "home"))
	os.MkdirAll(os.Getenv("HOME"), 0o755)

	seedRepo = filepath.Join(base, "seed")
	if err := buildSeed(seedRepo); err != nil {
		// Panic rather than skip: every fixture is a copy of this, so carrying on would report passes
		// over repositories that do not exist.
		panic("density tests: could not build the seed repository, so nothing was tested: " + err.Error())
	}
	// Removed explicitly rather than left to the defer above: os.Exit runs no deferred call, so the
	// defer covers only the panic path, and without this line every run leaves a seed repository
	// behind in the temp directory.
	code := m.Run()
	os.RemoveAll(base)
	os.Exit(code)
}

func buildSeed(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := git(dir, args...); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		return err
	}
	if err := git(dir, "add", "seed.txt"); err != nil {
		return err
	}
	return git(dir, "commit", "-qm", "base")
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

type repo struct {
	t      *testing.T
	dir    string
	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := copyTree(seedRepo, dir); err != nil {
		t.Fatalf("could not build a fixture repo: %v — stopping, since every case reads one", err)
	}
	return &repo{t: t, dir: dir}
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
}

func (r *repo) write(name, body string) {
	r.t.Helper()
	full := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatalf("could not create the parent for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		r.t.Fatalf("could not write the fixture %s: %v", name, err)
	}
}

func (r *repo) commit(message string) {
	r.t.Helper()
	if err := git(r.dir, "add", "-A"); err != nil {
		r.t.Fatalf("could not stage the fixture: %v", err)
	}
	if err := git(r.dir, "commit", "-qm", message); err != nil {
		r.t.Fatalf("could not commit the fixture: %v", err)
	}
}

// The thresholds a case gets unless it says otherwise. A function rather than a package var, so a
// case that moves one knob gets its own copy and the other two stay where the tool's defaults put them.
func baseConfig() Config {
	return Config{MaxRatio: defaultMaxRatio, MinLines: defaultMinLines, MaxFileBytes: defaultMaxFileBytes}
}

func (r *repo) run(args ...string) {
	r.runWith(baseConfig(), args...)
}

func (r *repo) runWith(cfg Config, args ...string) {
	r.stdout.Reset()
	r.stderr.Reset()
	r.code = Run("comment-density.sh", args, r.dir, cfg, &r.stdout, &r.stderr)
}

func (r *repo) expectCode(want int) {
	r.t.Helper()
	if r.code != want {
		r.t.Errorf("exit %d, wanted %d\nstdout: %s\nstderr: %s", r.code, want, r.stdout.String(), r.stderr.String())
	}
}

func (r *repo) expectStdoutHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stdout.String(), want) {
		r.t.Errorf("wanted %q on stdout, got: %s", want, r.stdout.String())
	}
}

func (r *repo) expectStdoutLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stdout.String(), unwanted) {
		r.t.Errorf("%q appears on stdout: %s", unwanted, r.stdout.String())
	}
}

// A refused run must leave nothing on stdout: anything there is what a caller capturing the report
// reads as a finding.
func (r *repo) expectNoStdout() {
	r.t.Helper()
	if r.stdout.Len() != 0 {
		r.t.Errorf("expected nothing on stdout, got: %s", r.stdout.String())
	}
}

func (r *repo) expectStderrHas(want string) {
	r.t.Helper()
	if !strings.Contains(r.stderr.String(), want) {
		r.t.Errorf("wanted %q on stderr, got: %s", want, r.stderr.String())
	}
}

func (r *repo) expectStderrLacks(unwanted string) {
	r.t.Helper()
	if strings.Contains(r.stderr.String(), unwanted) {
		r.t.Errorf("%q appears on stderr: %s", unwanted, r.stderr.String())
	}
}

func heavy(comments, code int) string {
	var b strings.Builder
	for i := 0; i < comments; i++ {
		fmt.Fprintf(&b, "// comment %d\n", i)
	}
	for i := 0; i < code; i++ {
		fmt.Fprintf(&b, "x := %d\n", i)
	}
	return b.String()
}
