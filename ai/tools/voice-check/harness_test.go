// The fixture the cases beside this file drive: a `repotest.Fake` for the questions this tool puts to
// a repository, and a real directory for the files it then OPENS. Nothing here forks git.
//
// The two halves are not interchangeable. A diff is answered verbatim by the fake, because which
// lines a change added is git's answer and deriving one here would make every finding a property of
// this fixture. Whole files are read off disk by the bar and by the untracked walk, so the byte cap,
// the symlink and the binary cases need a real file or they measure the open failing rather than the
// guard. File I/O is not what a suite pays for; process spawns are, at about 100ms each on the
// machine this is written on.
//
// newRealRepo is the exception and names the four cases that take it.
package voicecheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"configs/ai/tools/repo"
	"configs/ai/tools/repo/repotest"
)

var seedRepo string

// What this machine would have handed a case, had the pins below not been set.
var machineHome, machineConfigHome string

func TestMain(m *testing.M) {
	// Kept before anything below overrides them, so the isolation case can name what a leak would have
	// reached rather than guess at it.
	machineHome, machineConfigHome = os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME")
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
	// XDG_CONFIG_HOME as well as HOME, because the tool reads its own machine override from
	// $XDG_CONFIG_HOME/kk-flavor/ and falls back to $HOME/.config only when that is unset. Pinning HOME
	// alone left every case that does not set it reading the config of whoever runs the suite: three
	// cases went red on this laptop over a keyword a newer build of this tool had written there. A unit
	// test must not be able to see that file at all, let alone fail over it.
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	os.MkdirAll(os.Getenv("XDG_CONFIG_HOME"), 0o755)

	seedRepo = filepath.Join(base, "seed")
	if err := buildSeed(seedRepo); err != nil {
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

// fixture is one repository under test, plus what the run printed about it.
type fixture struct {
	t   *testing.T
	dir string
	// The repository the run is answered from: the fake below, or repo.Exec for a real checkout.
	git repo.Git
	// The same fake, for the cases that state an answer. Nil where git is the real one.
	fake *repotest.Fake
	// What the working tree holds, mirrored on disk. The fake is told off this which files a commit
	// carries and which are merely present, because that division is the whole of what the two
	// listings a run takes differ by.
	tree map[string]string
	// Commits behind HEAD, so HEAD~1 and HEAD~2 name what came before.
	depth int
	patch strings.Builder

	stdout strings.Builder
	stderr strings.Builder
	code   int
}

func newRepo(t *testing.T) *fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("could not build a fixture repo: %v — stopping, since every case reads one", err)
	}
	fake := repotest.New(dir)
	f := &fixture{t: t, dir: dir, git: fake, fake: fake, tree: map[string]string{}}
	// One commit, which the fixture repository has always carried. A repository with no commit is a
	// case of its own — newUnbornRepo — rather than the state every other case starts from.
	f.commit("base")
	return f
}

// newUnbornRepo is a repository git answers for and that holds no commit, which is what `git init`
// leaves and what the bar has its own refusal for.
func newUnbornRepo(t *testing.T) *fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "unborn")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("could not build a fixture repo: %v", err)
	}
	fake := repotest.New(dir)
	return &fixture{t: t, dir: dir, git: fake, fake: fake, tree: map[string]string{}}
}

// newRealRepo forks git, and four cases take it because real git is their subject: that git refuses
// `diff HEAD` as ambiguous once a file named HEAD is in the tree, that a `* -diff` attribute does not
// collapse the diff body, and that git's own words about a bad revision reach the refusal. No fake can
// answer for any of those — one stating them would be agreeing with itself, which
// `ai/kk-flavor/standards/testing.md` rule 5 is about.
func newRealRepo(t *testing.T) *fixture {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := copyTree(seedRepo, dir); err != nil {
		t.Fatalf("could not build a real fixture repo: %v — stopping, since the case reads one", err)
	}
	return &fixture{t: t, dir: dir, git: repo.Exec{}, tree: map[string]string{}}
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

// write puts a file in the working tree and on disk. It says nothing about the diff: a file written
// and never committed is untracked, and one written over a committed file is modified, which is what
// sync below reads off the commits.
func (f *fixture) write(name, body string) {
	f.t.Helper()
	full := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatalf("could not create the parent for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		f.t.Fatalf("could not write the fixture %s: %v", name, err)
	}
	f.tree[name] = body
	f.sync()
}

// remove takes a file out of the working tree, which a later commit then records as a deletion.
func (f *fixture) remove(name string) {
	f.t.Helper()
	if err := os.Remove(filepath.Join(f.dir, name)); err != nil {
		f.t.Fatalf("could not delete the fixture %s: %v", name, err)
	}
	delete(f.tree, name)
	f.sync()
}

// symlink plants a link the listings name and no read may follow. Its content is not the fixture's to
// hold — the guard under test refuses the path before opening it — so the tree carries the name alone.
func (f *fixture) symlink(name, target string) {
	f.t.Helper()
	if err := os.Symlink(target, filepath.Join(f.dir, name)); err != nil {
		f.t.Fatalf("could not plant the symlink %s: %v", name, err)
	}
	f.tree[name] = ""
	f.sync()
}

// added says what git's diff would print for these lines arriving in this file, and appends it to the
// patch every later run is answered with. Spelt out rather than derived from a before and an after: a
// fixture deriving the diff would assert against its own diff implementation rather than against the
// one a reviewer reads.
func (f *fixture) added(name string, lines ...string) {
	f.t.Helper()
	f.addedUnder(fmt.Sprintf("b/%s", name), lines...)
}

// addedUnder is added with the destination field spelled by the caller, for the two cases whose
// subject is the field itself — a path git C-quotes, and one it prints bare.
func (f *fixture) addedUnder(field string, lines ...string) {
	f.t.Helper()
	if f.fake == nil {
		f.t.Fatal("a real repository answers with git's own diff, so this fixture states none")
	}
	fmt.Fprintf(&f.patch, "diff --git a/x %s\n--- a/x\n+++ %s\n@@ -0,0 +1,%d @@\n", field, field, len(lines))
	for _, line := range lines {
		f.patch.WriteString("+" + line + "\n")
	}
	f.fake.Diff(f.patch.String())
}

// changed writes a file and says the change added every line of it, which is what git prints for a
// file the base did not hold and for one rewritten whole.
func (f *fixture) changed(name, body string) {
	f.t.Helper()
	f.write(name, body)
	f.added(name, strings.Split(strings.TrimSuffix(body, "\n"), "\n")...)
}

// commit records the working tree as a commit and pushes the commits already there one step back, so
// HEAD~1 names what HEAD named before. A history in one line is all the revisions these cases spell.
func (f *fixture) commit(message string) {
	f.t.Helper()
	if f.fake == nil {
		if err := git(f.dir, "add", "-A"); err != nil {
			f.t.Fatalf("could not stage the fixture: %v", err)
		}
		if err := git(f.dir, "commit", "-qm", message); err != nil {
			f.t.Fatalf("could not commit the fixture: %v", err)
		}
		return
	}
	for at := f.depth; at >= 0; at-- {
		if held, known := f.fake.Revs[revisionAt(at)]; known {
			f.fake.Commit(revisionAt(at+1), held)
		}
	}
	f.depth++
	f.fake.Commit("HEAD", f.tree)
	for at := 0; at <= f.depth; at++ {
		name := revisionAt(at)
		// Each commit answers to its object id as well as to its name, because git names a merge base by
		// id and the listings that follow one are then asked for that id.
		f.fake.Revs[f.fake.Refs[name]] = f.fake.Revs[name]
		// The merge base of any two commits on one line is the earlier of them. Stated here because a
		// symmetric range asks git for it and the fake answers only what a case has put in it.
		for other := 0; other <= f.depth; other++ {
			f.fake.Bases[name+"\x00"+revisionAt(other)] = f.fake.Refs[revisionAt(max(at, other))]
		}
	}
	f.sync()
}

func revisionAt(behind int) string {
	if behind == 0 {
		return "HEAD"
	}
	return fmt.Sprintf("HEAD~%d", behind)
}

// sync tells the fake which of the tree's files HEAD carries and which it does not. git calls the
// second kind untracked and leaves them out of every diff, and the two listings a run takes are
// answered off that one division.
func (f *fixture) sync() {
	if f.fake == nil {
		return
	}
	held := f.fake.Revs["HEAD"]
	tracked := map[string]string{}
	var untracked []string
	for name, body := range f.tree {
		if _, committed := held[name]; committed {
			tracked[name] = body
			continue
		}
		untracked = append(untracked, name)
	}
	sort.Strings(untracked)
	f.fake.Revs[repotest.WorkTree] = tracked
	f.fake.UntrackedPaths = untracked
}

// notARepository is a directory this fixture's git answers for the way `rev-parse` answers outside a
// checkout. Every other directory has to be declared from here on, so the fixture's own is.
func (f *fixture) notARepository() string {
	f.t.Helper()
	f.fake.PerDirectory().TreeAt(f.dir, f.dir)
	return f.t.TempDir()
}

func baseConfig() Config {
	return Config{MaxFileBytes: defaultMaxFileBytes}
}

func (f *fixture) run(args ...string) {
	f.runWith(baseConfig(), args...)
}

func (f *fixture) runWith(cfg Config, args ...string) {
	f.runIn(f.dir, cfg, args...)
}

func (f *fixture) runIn(cwd string, cfg Config, args ...string) {
	f.stdout.Reset()
	f.stderr.Reset()
	f.code = Run("voice-check.sh", args, cwd, f.git, cfg, &f.stdout, &f.stderr)
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

func (f *fixture) expectStderrLacks(unwanted string) {
	f.t.Helper()
	if strings.Contains(f.stderr.String(), unwanted) {
		f.t.Errorf("%q appears on stderr: %s", unwanted, f.stderr.String())
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

// housey is a file the register scan reports. A test about which FILE was reached can then observe
// the scan through its findings. The spine it carries is `rather than`, caught since the first check.
func housey(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "// The reader climbs to entry %d rather than the entry asked for.\n", i)
	}
	b.WriteString("x := 1\n")
	return b.String()
}
